# 可观测性与存储

> 日期：2026-08-17 ｜ 触发者：本地 Agent ｜ 相关：原 docs/agents/KNOWN_PITFALLS.md 归档（2026-08-18 文档体系重构拆分为 journal/lessons）

### 2026-08-17 Prometheus 单副本 + RWO PVC：滚动更新 TSDB 锁冲突 CrashLoop（Recreate 策略）
- 现象：给 Prometheus Deployment 换配置后 `rollout restart`，新 Pod CrashLoopBackOff，日志 `Fatal error: opening storage failed: lock DB directory: resource temporarily unavailable`；旧 Pod 仍 Running 持锁，新 Pod 抢不到同一 PVC 上的 TSDB 锁。
- 原因：默认 RollingUpdate 先起新 Pod 后停旧 Pod；单副本 + RWO PVC 时新旧 Pod 同时挂载同一数据卷。
- 解决：Deployment `strategy.type: Recreate`（先删后建），配置变更直接 `rollout restart` 即可；与 Jaeger badger 的 scale-to-zero 同类问题，Prometheus 用 Recreate 免去手工缩扩。
- 验证：改为 Recreate 后 `successfully rolled out`，TSDB 数据保留（重启前序列重启后仍可查）。

### 2026-08-17 preflight 实现：grep -v 无匹配在 pipefail + set -e 下返回 1 导致脚本静默退出
- 现象：`NOT_READY=$(... | grep -v ' Ready' | wc -l)` 在全部节点 Ready 时，`grep -v` 无匹配返回 1，配合 `set -Eeuo pipefail` 直接中止脚本——"无问题"反而让体检退出。
- 解决：命令替换末尾加 `|| true`（`... | wc -l || true`），并确认 `NOT_READY` 是 0 而不是空串。
- 验证：10 节点全 Ready 时 preflight 正常输出 `19 通过 / 0 失败 / 1 警告`。

### 2026-08-17 preflight 实现：bc 可能不存在，浮点比较用 awk
- 现象：体检脚本若用 `bc` 比较 Windows 空闲内存会因环境缺 bc 失败；`awk 'BEGIN{exit !(f < x)}'` 可直接在 `if` 里做浮点判断且无外部依赖。
- 解决：`awk -v f="$FREE_GB" 'BEGIN{exit !(f < 1.0)}'`；先校验输入是数字（`=~ ^[0-9.]+$`）再比较。
- 验证：Windows 空闲内存 3.5GB 时正确落在 WARN（<3GB 假）之外、PASS 分支。
