# 页面结构

## 1. 实际路由与产品概念

用户要求的五个概念在当前实现中不是五条独立路由：

| 产品概念 | 当前实现 | 路由 | 说明 |
| --- | --- | --- | --- |
| Dashboard | `MainLayout` 应用外壳 | 所有页面共享 | Sidebar、ClusterStatus、执行控件、全局时间条；没有独立 landing。 |
| Config | `ConfigPage` | `/config` | Model、WorkerNode、Tenant 配置。 |
| Traffic | `TrafficPage` | `/traffic` | 真实流量基线 + 本地场景草稿/Overlay。 |
| Trace | `DataOverviewPage` 内的 Trace 区域 | `/trace` | Trace 搜索摘要与 Span 树。 |
| Data View | 同一个 `DataOverviewPage` | `/trace` | 资源、指标、事件、时间和 Trace 的综合视图。 |

`/` 重定向到 `/config`，未知路由显示 NotFound。未来如新增独立 Dashboard/Data View，应先确定 URL 迁移和导航兼容。

## 2. Dashboard 应用外壳

### 目的

提供跨页面一致的集群连接、能力、时间上下文和导航。它不是数据统计页面。

### 数据

- cluster/environment/server version、cache readiness。
- provider readiness/warnings。
- command 和 simulationRun 能力。
- server/actual/logical time、latest/historical 状态。
- replay snapshot timeline。

### API 来源

- `GET /api/v1/bootstrap`
- `GET /api/v1/capabilities`
- `GET /api/v1/clock`
- `GET /api/v1/replay`
- `GET /api/v1/stream`

### 状态管理

Backend 数据经 Query/同步 hook 更新；跨页面选择存入 `controlPlaneSlice` 与 `timeSlice`。`simulationRunSupported=false` 时运行控制必须禁用，不能伪造成功。

## 3. Config 页面

### 目的

维护用户拥有的基础配置，展示 desired 与 observed 信息，并在历史时间点只读浏览。

### 当前页面内容

- Model：displayName、absoluteScore、gpuUnits、maxConcurrency、coldStartMs、performance 参数。
- WorkerNode：displayName、gpu、maxConcurrency；表格可显示 status 使用量。
- Tenant：displayName、priority、qps、TTFT/Queue 上下阈值。
- 创建、重命名（实际修改 displayName，不改 metadata.name）、删除和批量操作。

Backend 还允许 TenantModelPolicy、TenantNodePolicy、ModelNodePolicy、Orchestrator，但当前页面没有编辑器。

### 数据与 API

| 操作 | API | 说明 |
| --- | --- | --- |
| 最新读取 | `GET /configuration` | 从 informer cache 组合。 |
| 历史读取 | `GET /configuration?at=...` | 从 snapshot；只读。 |
| 创建/修改 | `POST /configuration:apply` | 需要 Idempotency-Key，支持 dry-run/resourceVersion。 |
| 删除 | `DELETE /configuration/{kind}/{name}` | 可 dry-run，使用 If-Match/resourceVersion。 |

### 状态管理

TanStack Query 拥有配置数据和 mutations；表单由 react-hook-form/Zod；选择、弹窗为组件状态。不得恢复旧 `configSlice/localStorage` 主状态。

### 已知问题

- 批量删除是前端并发发出多个 DELETE，不是单个原子命令。
- `metadata.name` 不可通过 Rename 改变；Rename 只更新 displayName。

## 4. Traffic 页面

### 目的

查看 Tenant 请求 QPS 与实例分配基线，设计流量场景并预览 Overlay 对曲线的影响。

### 数据

- Tenant requested QPS、priority 和阈值。
- SimulatorInstance 分配 QPS、replicas、availableReplicas、score/performance/phase。
- TenantPerformance / TenantRuntime 摘要（取决于 DTO）。
- 页面内模板、画布和 Overlay 草稿。

### API 来源

| 操作 | API | 当前行为 |
| --- | --- | --- |
| 最新基线 | `GET /traffic` | 真实 Backend/Kubernetes 数据。 |
| Tenant 过滤 | `GET /traffic?tenant=...` | 服务端过滤。 |
| 历史基线 | `GET /traffic?at=...` | snapshot 只读。 |
| 真实 QPS 写入 | `PATCH /tenants/{name}/traffic` | Backend 已实现，但当前 Overlay 工作流未调用。 |

### 状态管理

真实基线由 TanStack Query 管理；模板/Overlay 在 `trafficSlice` 内存中。刷新页面会丢失草稿，这是当前明确行为，不是 Bug 隐藏。

### 重要语义

Traffic 页面上的“应用 Overlay”目前只是应用到本地预览，不能解释为 Controller 已收到流量。未来完成命令闭环时，建议明确三步：Preview Diff -> Confirm -> PATCH Tenant QPS -> Watch 收敛结果。

## 5. Trace 区域

### 目的

搜索 Controller/Simulator Trace，查看具体 Trace 的 Span 层级、持续时间、属性和错误，关联到 tenant/model/instance。

### 数据与 API

- `/overview` 返回 Trace summaries 与其他页面块。
- `GET /traces` 支持时间窗口、service、operation、tenant/model/instance、duration、limit。
- `GET /traces/{traceID}` 返回规范化 Span tree。

### 状态管理

Trace list/detail 是 TanStack Query 状态；选中 traceId 是页面 UI 状态。Latest 可定期刷新；Historical 查询固定时间窗，不自动跳到现在。

### 局限

Jaeger 是可选 provider，失败时页面应显示 warning 并继续展示 Kubernetes/Prometheus 数据。开发 Jaeger 没有持久存储保证，重启后旧 Trace 可能消失。

## 6. Data View 综合区域

### 目的

让用户在一个时间上下文下理解“资源现在是什么、性能怎样、发生了什么、调用链如何”。当前与 Trace 合并在 `/trace`。

### 主要模块

- Clock：Server、Logical、Observed、Freshness。
- Current counts：Tenant、Model、Node、Instance、Pod 等。
- 6 个核心 Prometheus metric cards（含 Simulator timeScale）。
- Traffic/Performance：QPS、TTFT、Queue、Score、样本新鲜度。
- 11 个 CRD 的配置和状态概览。
- Workloads：Pod、Deployment、Node、Service、Lease。
- Kubernetes Events。
- Provider freshness/warnings。
- Trace summaries 和 Span tree。

### API 来源

- `GET /overview` 或 `GET /replay/frame`
- `GET /traces/{traceID}`

Latest 默认约 15 秒 refetch；Historical 视图应保持不可变，除非用户选择另一个 snapshot。

## 7. Mock 到真实 Backend 的迁移矩阵

| 旧实现 | 当前替代 | 状态 |
| --- | --- | --- |
| Config localStorage CRUD | `/configuration` + apply/delete | 已替换并清理旧文案。 |
| 两套前端假 Tenant/Model ID | Backend 统一 Kubernetes metadata.name | 已替换主链路。 |
| 9 个假 Worker | `/bootstrap`、Kubernetes Node/WorkerNode read model | 已替换。 |
| 805 个确定性 mock snapshots | `/replay` + PostgreSQL resource_snapshots | 已替换为离散历史，语义不同。 |
| Trace API 恒定失败/占位页 | Jaeger provider + DataOverviewPage | 已替换。 |
| Mock traffic baseline | `/traffic` | 已替换。 |
| Traffic templates/overlays | 仍是本地内存草稿 | 有意保留为编辑态，不是远端数据。 |
| 浏览器逻辑时钟假状态 | `/clock` 墙钟 + Kubernetes Simulator rate | 已替换；倍速通过 Backend/CRD/Controller 真正作用于 Simulator。 |

迁移的核心不是“删除所有本地数据”，而是只允许本地数据承担 UI 草稿；任何声称是集群事实的内容必须来自 Backend。
