# AI_CONTEXT - hello-k8s-ai 交接上下文

> 给任何新 AI 的第一条指令应是：**请先完整阅读 `docs/AI_CONTEXT.md`，再阅读与你任务相关的主文档；不要依据旧聊天或旧 DOCS 推断当前实现。**

## 1. 项目是什么

hello-k8s-ai 是一个 Kubernetes 原生 AI 推理调度与仿真平台。用户通过 React Dashboard 配置租户、模型和逻辑 WorkerNode；Dashboard Backend 把允许的配置写成 `platform.study.com/v1` CR，并从 Kubernetes informer cache 读取当前态。六个 Controller 根据策略创建 SimulatorInstance、Deployment 和 Pod，分配 QPS、聚合性能、统计容量并扩缩容。Simulator Pod 用离散事件模型生成 TTFT、队列、分数、Prometheus 指标和 OpenTelemetry Trace。Backend 再聚合当前态、PostgreSQL 历史、Prometheus 和 Jaeger，回传给前端。

项目不是生产推理网关；当前 Simulator 模拟推理工作负载。它也不是一个由 PostgreSQL 驱动的控制平面：Kubernetes API Server 才是配置和最新收敛状态的主要事实来源。

## 2. 当前状态基线（2026-08-14）

### 已实现

- 10 个 Cluster-scoped CRD：Model、WorkerNode、Tenant、TenantModelPolicy、TenantNodePolicy、ModelNodePolicy、SimulatorInstance、TenantPerformance、TenantRuntime、Orchestrator。
- 6 个 Reconciler：`TenantModelPolicyReconciler`、`SimulatorInstanceReconciler`、`TrafficReconciler`、`PerformanceCollectorReconciler`、`WorkerNodeUsageReconciler`、`OrchestratorReconciler`。
- Simulator：Lease 选主、5 秒 Tick、离散事件仿真、冷启动、状态写回、Prometheus 指标、OTel Trace。
- Dashboard Backend：shared informer/cache、读模型聚合、安全写命令、幂等、PostgreSQL 快照/事件/审计、Prometheus/Jaeger provider、REST、SSE、健康检查。
- Frontend：`/config`、`/traffic`、`/trace`；真实 Backend 查询；最新/历史时间模式；Data Overview 展示运行态、指标、Trace。
- 本地部署：复用现有 `docker-desktop` Kubernetes；单命令构建并向全部 Node 导入镜像，部署 Controller、Simulator、Prometheus、OTel Collector、Jaeger、Grafana、PostgreSQL、Backend 与 Frontend，并执行链路验收。

### 部分实现

- Traffic 页面读取真实 Tenant 基线，但场景模板和 Overlay 只保存在内存；尚未调用 Traffic PATCH 写回。
- 历史浏览读取 PostgreSQL 离散快照；不是完整事件溯源或确定性仿真重放。
- Config UI 只编辑 Model、WorkerNode、Tenant；Backend 已允许三类 Policy 和 Orchestrator，但 UI 未覆盖。
- `/trace` 实际是 Data Overview（指标、资源、事件、Trace 合并页）；没有单独 `/dashboard` 或 `/data-view` 路由。

### 未实现或未生产化

- Simulator/Controller 逻辑时间倍速、暂停、Seek、确定性重放。
- 生产身份认证、用户级授权、多租户隔离、TLS、NetworkPolicy 完整策略。
- PostgreSQL HA/备份、Prometheus/Jaeger 持久化与 HA、Alertmanager。
- 真实推理数据面；当前是模拟器。

## 3. 不允许破坏的架构约束

1. **Kubernetes 为当前事实源。** 不得把 PostgreSQL 或前端 Zustand 变成当前 CR/Pod 的主状态。
2. **Status 字段有明确所有者。** 不得为了“方便”让 Backend 或其他 Controller 覆盖非自己拥有的状态。
3. **Spec 与 Status 分离。** 用户/Backend 写允许的 Spec；Controller/Simulator 写收敛结果 Status。
4. **Controller 通过 API Server 协作。** 不直接调用其他 Controller，不共享进程内业务状态。
5. **Read Model 不执行控制决策。** Backend 可以解释和派生展示字段，但不能重新做扩缩容并写回。
6. **历史不能冒充当前，当前不能冒充历史。** `at` 查询无快照时应返回不可用，而不是用当前数据填充。
7. **跨来源不是强一致事务。** Kubernetes、Prometheus、Jaeger、PostgreSQL 必须保留来源时间、新鲜度、partial/warning。
8. **命令必须可审计、可幂等。** 修改 API 需要 `Idempotency-Key`；资源版本冲突必须显式返回。
9. **不编辑生成文件。** `config/crd/bases/*.yaml`、`config/rbac/role.yaml`、`**/zz_generated.*.go` 由生成器维护。
10. **不宣称未验证的运行状态。** 清单里有资源不代表集群里 Ready。

## 4. 字段所有权速查

| 对象/字段 | 唯一或主要写入者 | 备注 |
| --- | --- | --- |
| 用户可写 CR `spec` | kubectl / Dashboard Backend Command Gateway | Backend 仅允许 7 种配置 CR；不允许写派生 CR。 |
| `Model.spec.absoluteScore` | kubectl / Dashboard Backend Command Gateway | 必填正整数；模型能力配置的唯一权威来源。旧 `status.absoluteScore` 只读兼容。 |
| `SimulatorInstance.spec.replicas` | Orchestrator | TenantModelPolicy 创建时初始为 0。 |
| `SimulatorInstance.spec.traffic.qps` | Traffic Controller | Frontend 应改 Tenant.spec.qps，而不是直接改实例分配。 |
| `SimulatorInstance.status.phase/availableReplicas/Ready` | SimulatorInstance Controller | 从 Deployment 收敛状态获得。 |
| `SimulatorInstance.status.effectiveScore` | Orchestrator | 扩容选择输出。 |
| `SimulatorInstance.status.score/performance/observedAt/reporterID` | Simulator Leader | 非 Leader 不写。 |
| `TenantPerformance.status` | PerformanceCollector | 只聚合新鲜 Running 样本。 |
| `TenantRuntime.status` | SimulatorInstance Controller | `instanceCount` 实际是可用副本合计，不是 CR 数。 |
| `WorkerNode.status.usedGPU/usedConcurrency` | WorkerNodeUsage Controller | 根据已调度的非终态 Simulator Pod 推算。 |
| `Orchestrator.status` | Orchestrator | 含动作、原因、冷却时间。 |
| PostgreSQL 当前态 | 无 | DB 只存历史/审计/索引，不能拥有最新 Kubernetes 状态。 |

完整矩阵见 [kubernetes/FIELD_OWNERSHIP.md](kubernetes/FIELD_OWNERSHIP.md)。

## 5. 六个 Controller 的真实名称

用户或旧材料中的简称与实现类型并不完全一致：

| 常用称呼 | 源码类型 | 实际主资源 |
| --- | --- | --- |
| Tenant Controller | `TenantModelPolicyReconciler` | TenantModelPolicy；不是监听 Tenant 的独立 Controller。 |
| SimulatorInstance Controller | `SimulatorInstanceReconciler` | SimulatorInstance。 |
| Traffic Controller | `TrafficReconciler` | Tenant。 |
| Performance Controller | `PerformanceCollectorReconciler` | Tenant。 |
| WorkerNode Controller | `WorkerNodeUsageReconciler` | WorkerNode。 |
| Orchestrator Controller | `OrchestratorReconciler` | Orchestrator。 |

不要新增一个同名 `TenantController` 来“补齐文档”，除非业务确实需要并经过设计评审。

## 6. 重要目录

| 路径 | 内容 | 修改前必读 |
| --- | --- | --- |
| `api/v1/` | 10 个 CRD Go 类型与校验标记 | CRD 设计、字段所有权、AGENTS.md |
| `internal/controller/` | 六个控制器、算法、元数据、遥测 | Controller 架构、资源生命周期 |
| `simulator/` | Leader、Tick、仿真引擎、指标、健康 | Simulator 两份文档 |
| `cmd/main.go` | Manager 装配、参数、Controller 注册 | 架构总览、配置参考 |
| `config/` | CRD/RBAC/Manager/Demo/Observability/Kustomize | 部署、安全、集群信息 |
| `dashboard/backend/` | Go Backend 独立 module | Backend 四份文档 |
| `dashboard/frontend/my-app/` | React/Vite 前端 | Frontend 三份文档 |
| `dashboard/deploy/` | PostgreSQL、Backend、Frontend、RBAC | 部署、生产就绪度 |
| `docs/` | 本知识库 | INDEX 与维护规则 |

详见 [reference/SOURCE_MAP.md](reference/SOURCE_MAP.md)。

## 7. 修改规范

### CRD/API 修改

1. 先写字段语义、默认值、校验、所有者和迁移方案。
2. 修改 `api/v1/*_types.go`。
3. 执行 `make manifests generate`，不要手改生成 CRD/DeepCopy/RBAC。
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

## 8. 已知易误判点

- Traffic Overlay 是本地草稿；页面有真实数据不等于场景已写回控制平面。
- `TenantRuntime.status.instanceCount` 的实现含义是可用 Replica 总数。
- `Model.spec.absoluteScore` 是用户/Backend 提供的必填能力基准；旧 `status.absoluteScore` 仅用于滚动升级兼容，不应再写入。
- TenantNodePolicy、ModelNodePolicy 的 Status 当前没有 writer；空 Conditions 不等于失败。
- Backend watch ReplicaSet 并记录事件，但 Workloads DTO 当前未直接展示 ReplicaSet。
- 数据库有 `clock_state` 表，但运行时 Clock 仍是不可控制的真实 UTC、rate=1。
- 配置批次会先 dry-run 全部对象，再顺序写入；跨对象写入并非数据库式原子事务。
- SSE 是非持久通知流，慢客户端可能丢事件；30 秒轮询是安全网。

## 9. 验证基线

2026-08-14 源码校正后的本地验证：

- 恢复被误删的 `internal/controller/constants.go` 后，根 Go module 的测试、vet 和 golangci-lint v2.12.2 通过。
- Dashboard Backend 的测试与 vet 通过；E2E 源码可独立编译。
- `setup.sh`、`hack/local-cluster.sh`、`hack/cleanup-obsolete.sh` 通过 `bash -n`。
- `config/dev`、`config/demo`、`dashboard/deploy` 均可成功进行 Kustomize 渲染。
- Frontend 源码完成独立语法解析；当前交付环境无法访问 npm registry，因此完整 `npm ci && npm run check` 交由 CI 执行，不伪造结果。

当前交付环境没有 Docker、kubectl、Kind 和目标集群，不能代替用户机器执行镜像启动、Pod Ready、Prometheus target、Jaeger Trace、数据库或页面访问验收。`make cluster-up` 会在用户机器上逐门验证，失败时停止并写入 `.runtime/last-failure.log`。

GitHub Actions 现在分别验证 Controller、Backend、Frontend、生成文件、部署渲染、四类镜像和固定版本 Kind E2E。E2E 无论成功或失败都会兜底清理独立测试集群。

## 10. 推荐下一步顺序

1. 补最小 Frontend 组件测试。
2. 把 Traffic Overlay 明确转成可预览、可提交、可审计的 Tenant QPS 命令。
3. 把现有 Controller/Simulator Kind E2E 扩展到 Dashboard 与可观测组件的完整栈验收。
4. 补认证授权、Secret/TLS/NetworkPolicy、PostgreSQL 备份与可观测存储持久化。
5. 评审 `SimulationRun/SimulationClock` API 后再实现逻辑时间，禁止只在前端伪造倍速。

路线图和优先级依据见 [overview/CURRENT_STATUS_AND_ROADMAP.md](overview/CURRENT_STATUS_AND_ROADMAP.md)。
