# 领域语义易误判点：策略、状态字段、时间与历史

> 提升日期：2026-08-18 ｜ 来源：原 KNOWN_PITFALLS「领域已知易误判点」（原 AI_CONTEXT 第 8 节） ｜ 适用对象：本地 Agent 与远程 AI

## 现象

按"常识"理解 CR 语义导致误判：以为建了 Model/Tenant 就有负载、以为 replicas=0 是停止、以为倍速会加速 Controller 冷却等。

## 根因

系统事实源与分层边界是 Kubernetes 声明式语义，与直觉不一致。

## 可复用规则（每条一行，勿复述现象）

- Traffic Overlay 是本地草稿；页面有真实数据 ≠ 场景已写回控制平面。
- `TenantRuntime.status.instanceCount` 的语义是可用 Replica 总数，不是 CR 数。
- `Model.spec.absoluteScore` 是必填能力基准；旧 `status.absoluteScore` 仅滚动升级兼容，不应再写入。
- TenantNodePolicy / ModelNodePolicy 的 Status 当前无 writer；空 Conditions ≠ 失败。
- 三类策略缺一不可：TenantModelPolicy(Allow) 物化实例、TenantNodePolicy(Allow) 决定可调度、ModelNodePolicy 过滤模型-节点范围；无可行 placement 时副本保持 0。
- 新租户没有 Simulator 时 Orchestrator 停在 MetricsNotReady 属正常引导态（性能指标来自运行中的 Simulator Pod）。
- Backend watch ReplicaSet 并记录事件，但 Workloads DTO 当前不直接展示 ReplicaSet。
- `clock_state` 不驱动运行时：`SimulationClock` 只控制 Simulator 引擎倍速；Backend 时间、Controller 冷却/freshness、Lease、采集周期仍用真实 UTC。
- 配置批次先 dry-run 全部对象再顺序写入；跨对象写入不是数据库式原子事务。
- SSE 是非持久通知流，慢客户端可能丢事件；30 秒轮询是安全网。

## 验证方法

涉及上述语义的结论先核对 `docs/kubernetes/FIELD_OWNERSHIP.md`、`docs/data-flow/TIME_AND_REPLAY.md` 与对应源码，再下判断。
