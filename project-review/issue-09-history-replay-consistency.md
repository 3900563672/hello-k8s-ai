# 1. 问题标题

历史回放与可观测性数据无法形成一致时间切面

## 2. 当前状态描述

Backend 以 Kubernetes informer 聚合当前状态，并按周期把 `CurrentSnapshot` 保存到 PostgreSQL。`dashboard/backend/internal/api/handlers_read.go` 在带 `at` 参数时读取请求时刻之前最近的 Snapshot，这部分能够提供持久化的 Kubernetes 配置、流量和工作负载切面。

同一个 `/overview?at=...` 响应中的 Metrics 和 Traces 并不来自 Snapshot。Backend 会以 Snapshot 的 capturedAt 为结束时间，实时查询 Prometheus 过去 15 分钟的数据，再实时查询 Jaeger。结果是否存在取决于这两个 Provider 当前是否仍保留对应时间段。

当前部署中，`config/observability/prometheus.yaml` 设置 24 小时 retention，数据卷为 `emptyDir`；Prometheus Pod 重建后历史全部消失。Jaeger 使用单 Deployment，未配置持久化 Trace 存储，只有 `/tmp` 的 `emptyDir`，因此 Trace 同样是运行时易失数据。PostgreSQL Snapshot 默认保留时间明显长于这两类数据。

事件历史也不是无损的。`dashboard/backend/internal/store/recorder.go` 为避免阻塞 informer 使用有限 channel；缓冲区满会直接丢弃，数据库写失败也只记录日志，没有重试、dead-letter 或 readiness 降级。进程退出时不会先排空 channel。Snapshot 能周期性恢复切面，但不能恢复丢失事件的精确顺序。

PostgreSQL 中的 `trace_index` 只在用户查询 Jaeger 后由 Backend 顺带写入；Store 接口没有从该索引读取 Trace 的能力，因此它不是稳定的历史 Trace 来源。`clock_state` 表也已创建，但当前 Backend Clock 使用进程内 actual time，数据库中的时钟状态没有进入运行链路。

OpenTelemetry 已覆盖 Controller、Simulator 及 Kubernetes client 调用，但 Backend HTTP 和聚合逻辑本身没有建立完整 Trace；跨 Kubernetes 异步边界也没有持久化关联标识。因此当前 Jaeger 能展示组件 Span，却不能稳定还原“用户命令—CR 变化—Controller—Simulator—页面结果”的单条因果链。

## 3. 问题定位

Frontend 把 Snapshot、Metrics 和 Traces 组合成一个历史页面，用户会自然理解为同一时刻的可重放视图。实际上三者保留策略不同、持久性不同、采样与查询时机也不同。超过 24 小时或可观测性 Pod 重启后，Kubernetes Snapshot 仍在，Metrics/Traces 却会为空，历史页面只能部分重建。

Recorder 的丢弃策略保护了实时控制链路，这是合理取舍；问题是丢失没有成为可查询的系统状态。仅写日志和内部计数无法让历史消费者知道时间线存在缺口。

这会影响故障复盘和审计可信度：无法证明某个 Snapshot 配套的性能数据、Trace 和事件是否完整，也无法区分“当时没有数据”与“数据已经过期或丢失”。

## 4. 影响范围

- PostgreSQL：Snapshot 可持续保存，但 event、trace_index 和 clock_state 的能力没有形成统一读模型。
- Prometheus/Jaeger：本地部署的易失存储和短 retention 与历史页面承诺不一致。
- OpenTelemetry：组件内可观测，但缺少 Backend 和异步命令的端到端关联。
- Backend：历史 overview 混合持久化切面和实时 Provider 查询，freshness 只描述 Provider 当前健康，不代表目标时间数据完整。
- Frontend：历史页面可能显示部分空白或不一致数据，用户难以判断原因。
- 运维：缺少事件丢弃、历史覆盖范围和数据断层的告警与容量规划。

归档日志证明当前时间窗口内 Prometheus 和 Jaeger 链路可用；问题发生在 Pod 重启、缓冲压力或超出保留窗口后的历史复现。

## 5. 根本原因分析

当前系统把 PostgreSQL 定位为 Kubernetes 历史与审计，把 Prometheus/Jaeger 定位为专业 Provider，这个职责划分本身合理。但“历史回放”在 API 层把三套独立保留策略组合成了一个统一产品概念，却没有定义统一的时间覆盖契约和缺口语义。

另一个根因是可观测性最初服务于本地验收：短 retention、单副本、`emptyDir` 可以降低部署成本；当历史页面和故障复盘能力加入后，部署级数据耐久性没有同步升级。

## 6. 修改方向建议

- 先定义历史回放承诺：需要保存哪些数据、保存多久、允许多大时间偏差、缺失时如何向 API 和 UI 表达。
- 让 `/overview` 返回每个数据源针对目标时间的覆盖范围、完整性和缺口原因，避免只报告 Provider 当前是否健康。
- 按目标环境拆分可观测性存储策略：保留轻量本地配置，同时为长期环境提供持久化和容量参数，不必替换 Prometheus、OpenTelemetry 或 Jaeger 技术选型。
- 让事件丢弃和数据库写失败成为 Prometheus 指标、健康状态或时间线 gap 记录，并设计受控重试与关闭排空。
- 明确 trace_index 的职责；如果用于历史查询，应建立持续采集和读取路径，如果只用于辅助索引，应避免让表结构暗示完整 Trace 存储。
- 为用户命令建立跨 HTTP、Kubernetes 对象和 Controller 的稳定关联 ID，并补齐 Backend 自身的 Trace 边界。
- 统一或删除未进入运行路径的 clock_state 契约，避免数据库结构与实际时钟来源分叉。

## 7. 优先级

优先级：P1

本地实时演示不受阻；在承诺历史回放、事故审计或延长数据保留前必须明确并实现一致性边界。
