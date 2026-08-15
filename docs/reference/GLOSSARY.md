# 术语表

| 术语 | 定义 | 不等于 |
| --- | --- | --- |
| Actual/Server Time | Backend 的真实 UTC 墙钟 | 可加速仿真时间 |
| Aggregator | 把多资源/Provider 组合成页面 DTO | Controller 决策器 |
| Allow/Deny | Policy effect；Deny 通常覆盖 Allow | Kubernetes RBAC allow/deny |
| Available Replicas | Deployment 可用副本，被 Instance Controller写入 CR | Spec 期望副本 |
| Backend | Dashboard 聚合/命令服务 | Controller Manager |
| Cache | client-go informer 本地最新态 | PostgreSQL snapshot |
| CapturedAt | DB snapshot 采集时间 | 用户请求 at |
| Condition | 标准 K8s 状态 type/status/reason/message | 单一 Phase |
| Controller | 持续把声明状态收敛到实际状态的 Reconciler | HTTP Handler |
| CR / CRD | Custom Resource / Definition | 普通数据库行 |
| Dashboard | React 应用外壳/产品概念 | 当前独立 `/dashboard` route（不存在） |
| Data View | 综合资源/指标/事件/Trace 区域 | 当前独立 `/data-view` route（不存在） |
| Desired | Spec 表达期望 | Status 已实现结果 |
| EffectiveScore | Orchestrator 计算的单副本静态分 | Simulator pool score |
| Freshness | 数据相对观察时间是否仍可用于决策 | HTTP 请求是否刚完成 |
| Historical | PostgreSQL 离散 snapshot 浏览 | 确定性事件 replay |
| Idempotency-Key | 防 mutation 重复副作用的请求键 | Authentication token |
| Instance | 通常指 SimulatorInstance 实例池 CR | 必然等于单个 Pod |
| Leader | 持有 per-instance reporter Lease 的 Simulator Pod | Controller Manager leader（另一个 Lease） |
| Logical Time | 当前 Clock DTO 中等于 Actual 的字段 | Simulator 引擎倍速或已实现的 pause/seek |
| ObservedAt | Simulator/资源最近观测/发布时间 | Backend servedAt |
| Orchestrator | 每 Tenant 的扩缩容 Controller/CR | Kubernetes Scheduler |
| Partial | 某些可选来源失败但响应主体可用 | 全部成功 |
| Policy | Tenant-Model/Node 业务 Allow/Deny CR | Kubernetes NetworkPolicy/RBAC Policy |
| Provider | Prometheus/Jaeger 查询适配器 | 事实源所有者 |
| QPS | 每秒请求数；Tenant 是总量，Instance 是分配量 | 并发数 |
| Read Model | 为页面组合的稳定 DTO | 可写领域模型 |
| Reconcile | 从当前输入计算并应用差异的可重入循环 | 每事件只执行一次的 handler |
| Replay | 当前产品中 snapshot timeline/frame | 完整逻辑时间重演 |
| Resource Event | Backend 观察到的对象变化记录 | 无损审计/事件溯源 |
| Score | Simulator 计算的冷启动/可用副本池能力 | `Model.spec.absoluteScore` 配置基线 / Instance effectiveScore |
| SimulationClock | 集群唯一的 Simulator 倍速 CR，名称固定 `default` | 全系统可暂停/Seek 的逻辑时钟 |
| Simulator Time Scale | 每个真实 Tick 推进的模拟时间倍数，当前 1..20 | Controller cooldown、Lease、采集或页面时间倍速 |
| SSE | Backend 到浏览器的失效通知流 | 持久事件总线 |
| Status | Controller/Simulator 的观测与派生输出 | 用户配置入口 |
| TenantRuntime.instanceCount | 当前实现的可用副本合计 | SimulatorInstance CR 数 |
| Trace | 多 Span 的调用/操作链 | 指标时间序列 |
| Traffic Overlay | Frontend 内存中的场景草稿 | 已写入 Tenant QPS 的命令 |
| WorkerNode | 项目业务容量 CR | core/v1 Node 本身 |

## 命名统一建议

- 文档称 `TenantModelPolicy Controller`，括号注明旧简称 Tenant Controller。
- 页面称 `Data Overview` 或“Data View + Trace 综合页”，直到路由重构。
- `instanceCount` 对用户展示称“Ready replicas/可用副本合计”。
- “回放”必须附“snapshot history”，除非真正 SimulationRun 已实现。
