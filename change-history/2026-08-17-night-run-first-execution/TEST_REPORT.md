# 测试报告：首次执行修复验证

## 1. 验证命令与真实结果（2026-08-17 07:55–08:05 CST）

| 项 | 命令 | 结果 |
| --- | --- | --- |
| 语法检查 | `node --check hack/night-run/snapshot.mjs && node --check hack/night-run/keepalive.mjs` | 通过（SYNTAX OK） |
| 快照实跑 | `node hack/night-run/snapshot.mjs --once --summary` | ok:true，写出 `2026-08-16T23-55-54-910Z.json`；摘要：qps=35、queue=3429、ttft=991873ms、controllerErrorRate=0、simulatorErrorRate=-、tickLatency=23.9ms、DB available |
| 残留示例检查 | `grep -rn '20→40\|40→80\|kubectl scale\|tenants/core\|nohup node hack/night-run/keepalive' hack/night-run/ docs/agents/` | 仅命中新增的"不做 kubectl scale""kubectl scale 不可用"说明，旧错误示例已清除 |
| 进程存活 | `pgrep -af 'keepalive.mjs'` | PID 122828 仍在运行（setsid 实例，跨命令存活） |
| 文档链接 | `make docs-check` | 待执行（提交前） |
| CI | lint / test / docs workflow | 待 PR 后轮询（hack/** 变更会触发 Go lint/test 全量检查） |

## 2. 运行环境事实

- 快照实跑时系统状态：18 Pod 全 Running、health ready、qps=35（收敛）、rate=20、队列 3429 回排中——50qps 事件的积压仍在消化，TTFT 约 16 分钟，与问题 #7 结论一致。
- 快照文件数：`.runtime/night-run/2026-08-17/snapshots/` 现有 3 份（16:20Z、16:29Z、23:55Z）。

## 3. 未验证项

- 整夜值守（禁用睡眠后的完整 00:00–04:30）未实测。
- Phase B 自动化 04:30 触发与 model 修复（automation.toml 显式 deepseek-chat）未实测。
- 50qps 积压完全回排所需时长未测量（恢复后队列仍在回排）。
