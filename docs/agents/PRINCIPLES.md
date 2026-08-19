# 设计与修改原则（PRINCIPLES）

> 维护层：agent | last-reviewed：2026-08-19 | 事实源：源码与 docs/agents/
> 本文件汇总"不允许破坏的架构约束"与"修改规范"，原为 `docs/AI_CONTEXT.md` 第 3–7 节，2026-08-16 迁移至此。
> 字段所有权完整版见 [docs/kubernetes/FIELD_OWNERSHIP.md](../kubernetes/FIELD_OWNERSHIP.md)。

## 1. 不允许破坏的架构约束

1. **Kubernetes 为当前事实源。** 不得把 PostgreSQL 或前端 Zustand 变成当前 CR/Pod 的主状态。
2. **Status 字段有明确所有者。** 不得为了"方便"让 Backend 或其他 Controller 覆盖非自己拥有的状态。
3. **Spec 与 Status 分离。** 用户/Backend 写允许的 Spec；Controller/Simulator 写收敛结果 Status。
4. **Controller 通过 API Server 协作。** 不直接调用其他 Controller，不共享进程内业务状态。
5. **Read Model 不执行控制决策。** Backend 可以解释和派生展示字段，但不能重新做扩缩容并写回。
6. **历史不能冒充当前，当前不能冒充历史。** `at` 查询无快照时应返回不可用，而不是用当前数据填充。
7. **跨来源不是强一致事务。** Kubernetes、Prometheus、Jaeger、PostgreSQL 必须保留来源时间、新鲜度、partial/warning。
8. **命令必须可审计、可幂等。** 修改 API 需要 `Idempotency-Key`；资源版本冲突必须显式返回。
9. **不编辑生成文件。** `config/crd/bases/*.yaml`、`config/rbac/role.yaml`、`**/zz_generated.*.go` 由生成器维护。
10. **不宣称未验证的运行状态。** 清单里有资源不代表集群里 Ready。
11. **工具结论必须带地面真值。** 探针/脚本自身也要做健康对照：先证明工具在健康态会正确通过，再谈故障态读数；工具缺陷（如输出路径在目标平台不可写、只 Listen 不 Accept）导致的读数一律作废，不作环境证据（32 号 WSL 探针 wincheck 教训）。

## 2. 字段所有权速查

完整字段所有权见 [docs/kubernetes/FIELD_OWNERSHIP.md](../kubernetes/FIELD_OWNERSHIP.md)，以源码与生成清单为准。本文件不再复制速查表（避免双源漂移）；涉及"谁能写哪个字段"的结论一律以该文档 + `api/v1/*_types.go` 为准。

## 3. 七个 Controller 的真实名称

| 常用称呼 | 源码类型 | 实际主资源 |
| --- | --- | --- |
| Tenant Controller | `TenantModelPolicyReconciler` | TenantModelPolicy；不是监听 Tenant 的独立 Controller。 |
| SimulationClock Controller | `SimulationClockReconciler` | SimulationClock；同步全局倍速到全部 SimulatorInstance。 |
| SimulatorInstance Controller | `SimulatorInstanceReconciler` | SimulatorInstance。 |
| Traffic Controller | `TrafficReconciler` | Tenant。 |
| Performance Controller | `PerformanceCollectorReconciler` | Tenant。 |
| WorkerNode Controller | `WorkerNodeUsageReconciler` | WorkerNode。 |
| Orchestrator Controller | `OrchestratorReconciler` | Orchestrator。 |

不要新增一个同名 `TenantController` 来"补齐文档"，除非业务确实需要并经过设计评审。

## 4. 重要目录

| 路径 | 内容 | 修改前必读 |
| --- | --- | --- |
| `api/v1/` | CRD Go 类型与 Kubebuilder 标记 | CRD_DESIGN、FIELD_OWNERSHIP |
| `cmd/main.go` | Controller Manager 入口 | CONTROLLER_ARCHITECTURE |
| `internal/controller/` | 7 个 Reconciler | CONTROLLER_ARCHITECTURE、RESOURCE_LIFECYCLE |
| `internal/observability/` | Controller Trace 接入 | OPENTELEMETRY |
| `simulator/` | Simulator、Lease 选主、Metrics、Trace | SIMULATOR_ARCHITECTURE、SIMULATION_FLOW |
| `config/` | CRD/RBAC/Manager/Demo/Observability/Kustomize | 部署、安全、集群信息 |
| `dashboard/backend/` | Go Backend 独立 module | Backend 四份文档 |
| `dashboard/frontend/my-app/` | React/Vite 前端 | Frontend 三份文档 |
| `dashboard/deploy/` | PostgreSQL、Backend、Frontend、RBAC | 部署、生产就绪度 |
| `docs/` | 本知识库 | INDEX 与维护规则 |

详见 [docs/reference/SOURCE_MAP.md](../reference/SOURCE_MAP.md)。

## 5. 修改规范

### CRD/API 修改

1. 先写字段语义、默认值、校验、所有者和迁移方案。
2. 修改 `api/v1/*_types.go`。
3. 执行 `make manifests generate YEAR=2026`，不要手改生成 CRD/DeepCopy/RBAC。
4. 更新映射、Controller、Backend Mapper、Frontend DTO 和文档。
5. 做向后兼容与现有对象升级验证。

### Controller 修改

- Reconcile 必须可重入，重复执行不产生额外副作用。
- 使用 Patch/冲突重试并只修改自己拥有的字段。
- Watch 要与输入依赖一致；周期 Requeue 只是兜底，不替代正确 Watch。
- 删除必须考虑 finalizer、OwnerReference 和残留工作负载。
- 算法需要确定性排序，避免 map 遍历造成抖动。

### Backend 修改

- Handler 只做协议与校验，领域聚合留在 Aggregator/Provider/Store。
- 当前态从 informer cache 读；旧时间点从 snapshot store 读。
- 命名 PromQL/Jaeger 过滤由服务端控制，不接受任意 PromQL。
- 写入只允许白名单 Spec；不得写 Status、Deployment、Pod、Lease。
- 新 mutation 保持幂等、审计、dry-run 与 resourceVersion 语义。

### Frontend 修改

- TanStack Query 拥有远端服务状态；Zustand 只保留跨页面 UI/时间/草稿状态。
- 历史模式只读；切回 Latest 后再允许命令。
- SSE 只是失效通知，重连或丢事件后必须 REST resync。
- 不重新引入 Mock/localStorage 作为生产数据源。
- Traffic Overlay 如果要执行，必须显式确认并调用 Backend，不应静默写集群。

### 数据库 / Schema 修改

- 新表或结构变更只追加 `internal/store/migrations/NNN_*.sql`，不修改已应用的迁移。
- 迁移必须幂等（`IF NOT EXISTS` / `ON CONFLICT`），由 Backend 启动自动应用，不需要人工建表。
- 数据库写路径失败不得阻断控制面或 Simulator（记录日志继续运行）。
- 涉及持久化行为时用 `TestPostgresLifecycle`（`TEST_DATABASE_URL`）验证迁移幂等与重启恢复。

### 展示读路径（Phase 3）

- 前端展示的当前态数据由 Backend 统一从 `resource_states`（数据库）读取；Kubernetes API Server 仍是 Controller 侧唯一事实源，数据库不反向驱动 Controller。
- 读路径降级边界：存储不可用、记录为空或查询失败时回退 informer 实时聚合，不返回伪造数据；历史回放（`at` 参数）继续读数据库快照。
- `GET /api/v1/resources` 直接暴露当前态记录；存储不可用时返回 503 problem（与 `/replay` 风格一致）。
- 当前态 `asOf` 取 `resource_states` 最新 `captured_at`（数据时间），不伪装成请求时间。

### 时间段切面（Run Segment）

- `GET /api/v1/segment?start=...&end=...` 是只读时间段聚合：起点/终点快照（`store.SnapshotAt`，各取"之前最近"）+ `[start,end]` 区间指标与 Trace + 段级覆盖告警；`start < end` 且窗口 ≤ 24h。
- 任一端无快照或存储不可用 → `availability=unavailable` + 明确告警，不伪造数据（同"历史不能冒充当前"）。
- 段查询不做"按事件重新执行"或"确定性回放"（AGENTS.md 边界不变）；它只是把既有快照流、Prometheus 区间与 Jaeger 区间组合成一次可分析的时间段。
- 前端段面板选择起点/终点快照（时间轴项）发起查询；时间轴"标记起点/终点"的交互属 UI 阶段，不改变 API 语义。

### 可观测性持久化与事件缺口

- 本地部署的 Prometheus/Jaeger 必须 PVC 持久化（`config/observability/`），retention 与历史回放承诺一致（当前 168h）；不允许回到 `emptyDir` 让"重启丢历史"复发。
- 事件丢弃与数据库写失败必须可观测：Prometheus 计数器（`hello_k8s_ai_dashboard_events_dropped_total` / `..._write_failures_total`）+ `TimelineGap` 时间线记录，不允许只有进程内计数或日志。
- `TimelineGap` 只声明"有事件丢失"与增量计数，不恢复丢失事件本身；精确顺序恢复依赖快照周期，不能把 gap 记录当事件源。
- gap 记录写入失败不阻塞主循环：水位不前进，数据库恢复后补记（遥测失败不阻断控制面）。
- 单副本 badger/TSDB + RWO PVC 的重启/升级必须先 scale 到 0 再扩回 1（目录锁），不使用 rollout restart。
