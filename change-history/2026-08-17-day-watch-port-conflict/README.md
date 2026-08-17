# day-watch 首次白天值守：流量调整失效根因（8080 端口冲突）与修复

- 变更日期：2026-08-17（Asia/Shanghai 12:10；UTC 04:10）
- 关联问题：无（值守产物分析）
- 变更级别：P2 值守工具修复（不改业务代码）
- 变更范围：`hack/local-cluster.sh`、`hack/night-run/day-watch.mjs`、`hack/night-run/README.md`、`docs/agents/KNOWN_PITFALLS.md`
- CRD 变化：无 ｜ 数据库变化：无

## 1. 上午值守结果（09:02–11:49）

- 系统整体稳定：35qps 基线全程健康，TTFT 321–416ms（阈值 800ms）、queue=0、Reconcile 错误率无异常、模拟器 14→16 副本 Running。
- 快照 6 份正常采集（每 30 分钟）；其中 11:18、11:49 两份部分字段缺失（overview/traffic 偶发失败，快照脚本容错记录 undefined）。
- **流量剧本未真正执行**：10:03 起 day-watch 读取流量全部失败（`read ECONNRESET`），50qps 压测段从未生效（仅 12:07 验证时手动触发成功）。

## 2. 根因

- Windows 侧 `dllhost.exe`（PID 53728，昨晚 20:45 启动）监听 `127.0.0.1:8080`，是 WSL2 localhost 转发宿主；WSL 内 kubectl port-forward 也监听 8080，同端口冲突导致 WSL 内连接被抢占/重置（时好时坏）。
- keepalive 每轮是独立新进程，首个连接成功即报 ok（假阳性），掩盖了问题；day-watch 是常驻进程，首版 undici fetch 连接池在冲突下持续失败。
- `local-cluster.sh` 的“已在运行”检查只验证端口可连一次，半死隧道永不重启。

## 3. 修复

- WSL 内脚本一律走独立端口 **18080**：`local-cluster.sh` 新增 `dashboard-internal` 转发（18080:80），8080 保留给 Windows 浏览器（用户访问不受影响）。
- `day-watch.mjs` 默认 `--base-url http://localhost:18080` 并透传给 keepalive/snapshot；HTTP 层改为 `node:http` + 每次新连接 + 网络重试（对齐 keepalive 的可靠行为）。
- 坑位沉淀：KNOWN_PITFALLS「集群操作与部署」新增条目。

## 4. 验证

- WSL 内 18080 连续 4 次 200；Windows 侧 8080 保持 200。
- PATCH 链路实测：50qps 生效 → 调回 35qps 生效（Traffic 分配异步收敛约 15s）。
- 新 day-watch（PID 248009）已重启，后续轮次走 18080。

## 5. 未验证 / 风险

- 18080 方案未经历长时间运行验证（持续观察中）；若 Windows 侧端口占用变化需重新评估。
- 压测段仍未在真实值守中跑满（验证后已恢复 35qps 基线）。
