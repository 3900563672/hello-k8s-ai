# 字段所有权

本文件是修改 Controller、Simulator、Backend 和 CRD 前的强制检查表。`SimulatorInstance` 多 writer 设计只有在字段互不越界时安全。

## 1. 总原则

- 用户/Backend 写配置 CR Spec。
- Controller 写自己收敛出的派生资源和 Status。
- Simulator 只写实时仿真反馈。
- Dashboard Backend 读取和展示派生值，不写 Controller-owned 字段。
- Patch 必须只覆盖本组件字段；不要用整个对象 Update 把并发变化写回旧值。

## 2. CRD 所有权矩阵

| 资源 | 字段 | 写入者 | 读取者 | 禁止者/备注 |
| --- | --- | --- | --- | --- |
| Model | `spec.*` | 用户/Backend | Policy、Instance、Orchestrator、Simulator、WorkerUsage | Controller 不应规范化后回写 Spec |
| Model | `status.absoluteScore` | 外部后端/运维（当前无内部 writer） | Orchestrator | 需要未来明确 writer/RBAC/API |
| WorkerNode | `spec.*` | 用户/Backend | Instance、Orchestrator | - |
| WorkerNode | `status.used*`, Conditions | WorkerNodeUsage | Orchestrator/Backend | Backend 不写 |
| Tenant | `spec.*` | 用户/Backend | Traffic/Performance/Orchestrator | Traffic PATCH 只改 qps |
| Tenant | `status.*` | 当前无内部 writer | Backend | 空不是错误 |
| TenantModelPolicy | `spec.*` | 用户/Backend | Policy Controller/Orchestrator | - |
| TenantModelPolicy | Conditions | Policy Controller | Backend | - |
| TenantNodePolicy | `spec.*` | 用户/Backend | Instance/Orchestrator | - |
| TenantNodePolicy | Conditions | 当前无 writer | Backend | 不伪造 Ready |
| ModelNodePolicy | `spec.*` | 用户/Backend | Instance/Orchestrator | - |
| ModelNodePolicy | Conditions | 当前无 writer | Backend | 不伪造 Ready |
| SimulatorInstance | refs/identity metadata | TenantModelPolicy Controller | 所有控制环 | Backend/用户禁写 |
| SimulatorInstance | `spec.replicas` | Orchestrator | Instance Controller/Simulator | Policy Controller 仅创建初值 0 |
| SimulatorInstance | `spec.traffic.qps` | Traffic | Simulator/Backend | 用户改 Tenant.qps，不直写 |
| SimulatorInstance | `status.availableReplicas/phase/Ready` | Instance Controller | Traffic/Performance/Orchestrator/Backend | Simulator 不写 Phase |
| SimulatorInstance | `status.effectiveScore` | Orchestrator | Simulator/Backend | Instance Controller 不碰 |
| SimulatorInstance | `status.score/performance/observedAt/reporterID` | Simulator Leader | Traffic/Performance/Backend | follower/Backend 不写 |
| SimulatorInstance | pending plan annotation | Orchestrator | Orchestrator | Backend只读诊断 |
| TenantPerformance | `spec.tenantRef` | PerformanceCollector | Orchestrator/Backend | 用户/Backend禁写 |
| TenantPerformance | `status.*` | PerformanceCollector | Orchestrator/Backend | - |
| TenantRuntime | `spec/status.*` | Instance Controller | Backend | instanceCount=可用副本合计 |
| Orchestrator | `spec.*` | 用户/Backend | Orchestrator Controller | - |
| Orchestrator | `status.*` | Orchestrator Controller | Backend | - |

## 3. 原生资源所有权

| 资源 | 项目写入者 | 关键边界 |
| --- | --- | --- |
| Deployment `simulator-*` | SimulatorInstance Controller | 不允许 Backend/Orchestrator直接写；副本来源是 Instance Spec。 |
| Pod | Kubernetes Deployment Controller/Scheduler/kubelet | 项目不直接创建/改 Pod；只读其调度与状态。 |
| Lease `simulator-reporter-*` | client-go leader election in Simulator | Controller/Backend只读；每 Instance 一个 Lease。 |
| Event | Kubernetes 原生组件 | 项目 Controller 当前无完整领域 EventRecorder；Backend只读。 |
| Service | Kustomize 静态服务 | 当前不为每个 SimulatorInstance 创建 Service。 |

## 4. Backend 写入白名单

```text
允许：Model, WorkerNode, Tenant,
      TenantModelPolicy, TenantNodePolicy, ModelNodePolicy, Orchestrator

拒绝：SimulatorInstance, TenantPerformance, TenantRuntime,
      所有 status、Deployment、Pod、Lease
```

应用 allowlist 与 Kubernetes RBAC 必须同时存在。若新增可写字段：

1. 确认没有 Controller/Simulator writer。
2. 更新 Gateway allowlist 和 RBAC。
3. dry-run、resourceVersion、幂等、审计。
4. 更新 Frontend 历史只读逻辑。
5. 更新本表与 CRD/API 文档。

## 5. 并发写入注意

`SimulatorInstance.status` 有三个 writer：Instance Controller、Orchestrator、Simulator。安全做法：

- 每次读取最新对象。
- `Status().Patch` 使用 merge-from 最新 base。
- 构造 patch 时只改拥有字段。
- 遇 Conflict requeue，不盲目覆盖。
- 测试多个 writer 交错，确保一个 writer 的 patch 不清除其他字段。

错误示例：Simulator 为写 `performance` 构造一个空 Status，只填 performance 后全量 Update；这会清掉 phase/effectiveScore。

## 6. 数据库与前端所有权

| 数据 | 所有者 | 说明 |
| --- | --- | --- |
| 当前 CR/Pod | Kubernetes | DB snapshot 只是过去副本。 |
| 历史 snapshot/audit/idempotency | PostgreSQL | 不能反向覆盖 K8s 当前态。 |
| Prom metrics | Prometheus | Backend cache 只做短时查询优化。 |
| Trace/Span | Jaeger | `trace_index` 不是 Span 存储。 |
| 远端页面状态 | TanStack Query | Zustand 不复制 Configuration/Overview。 |
| 时间游标/Traffic 草稿 | Zustand | 不能声称是集群已执行状态。 |

## 7. Code Review 检查

- 本变更新增了 writer 吗？主文档和 RBAC 是否同步？
- Patch 是否可能清空其他组件字段？
- Spec/Status 是否混用？
- 展示派生值是否意外写回？
- 用户输入是否绕过 Backend allowlist？
- 历史对象是否可能被当作 current 更新？
- 测试是否覆盖两个 writer 的交错和 conflict？
