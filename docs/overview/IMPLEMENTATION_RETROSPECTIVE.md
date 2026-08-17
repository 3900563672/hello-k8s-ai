# 实现复盘与工程评价

> 维护层：human | last-reviewed：2026-08-18 | 事实源：docs/MAP.yaml、源码、change-history/

本文件回答交接时最重要的三个问题：已经做了什么、怎么做的、哪里仍不够好。结论来自代码、清单、测试和历史验证记录，不依赖未保留的聊天上下文。

## 1. 已完成的工作

### Kubernetes 控制面

- 保持 11 个 CRD 的 Go API 和生成清单一致，建立 Tenant、Model、WorkerNode、三类 Policy、SimulationClock、SimulatorInstance、TenantPerformance、TenantRuntime、Orchestrator 的领域模型。
- 将控制职责拆成七个可重入 Reconciler，而不是做一个超大 Controller。
- 建立字段所有权：倍速、流量、副本、部署状态、分数、性能、节点用量分别由不同组件写。
- 使用 Watch + 周期 Requeue 保证事件驱动与失联兜底。
- 为资源创建、Status Patch、删除、finalizer、OwnerReference、确定性命名与策略交集补充实现和测试。
- Orchestrator 使用 pending plan 注解处理跨对象更新的崩溃恢复。

### Simulator

- 以 Deployment 副本运行，通过 per-instance Lease 选出唯一 reporter。
- 每个真实 Tick 读取最新 CR，按 1x..20x 推进模拟时间、计算冷启动因子和实例池分数、推进离散事件引擎并写回性能；倍速变化不重启 Pod。
- 暴露健康、就绪、业务指标、领导权指标和 Trace；遥测配置缺失时不阻断仿真。

### Dashboard Backend

- 从“页面 Mock”升级为真实聚合层：动态 informer 监听全部 CRD，typed informer 监听原生资源。
- 以 Kubernetes cache 作为当前态源，构造稳定 DTO，而不是让前端理解 unstructured CR。
- 接入 PostgreSQL：迁移、快照、资源变更、审计、幂等和 Trace 索引。
- 接入 Prometheus/Jaeger：服务端命名查询、过滤、超时和 partial/warning。
- 建立读写边界：七种用户配置 CR 可写，三个派生 CR 和 SimulatorInstance 禁写。
- 提供 REST、SSE、严格 JSON、CORS、请求 ID、恢复中间件、健康/能力接口。

### Frontend

- 从 localStorage/Mock 主链路迁移到 Backend API。
- 保留并整理 Config、Traffic、时间轴和布局，将 `/trace` 接入真实 Data Overview。
- 用 TanStack Query 管理远端状态，用 Zustand 管理 UI/时间/草稿。
- 用 SSE 触发失效刷新，并保留 30 秒安全轮询。
- 支持 latest 与历史 `at` 语义；历史页面只读。
- 增加容器化 Nginx 反向代理和非 root 部署清单。

### 部署与可观测性

- 本地完整栈复用已有 `docker-desktop`，不创建或删除集群。
- Controller、Simulator、Backend、Frontend 镜像统一构建并导入全部 Kubernetes Node。
- Prometheus、OTel Collector、Jaeger、Grafana 都有声明式清单；Grafana 预置 12 个面板。
- PostgreSQL StatefulSet、Backend/Frontend Deployment、ServiceAccount/RBAC 已进入一键部署与链路验收。

## 2. 采用的方法

| 问题 | 采用方法 | 价值 |
| --- | --- | --- |
| 多组件协作 | CR Spec/Status + Watch，不直接互调 | 可恢复、可观察、可独立测试 |
| 多 writer 冲突 | 明确字段所有权并使用 Patch | 降低覆盖和振荡风险 |
| 流量整数分配 | Largest Remainder + 确定性排序 | 总量守恒、结果稳定 |
| 异常性能样本 | 新鲜度过滤 + 可用副本加权稳健聚合 | 过期/离群样本不主导决策 |
| 扩缩容跨对象步骤 | pending plan 持久化 | Manager 崩溃后可恢复 |
| Backend 当前态 | shared informer/cache | 避免每个请求打 API Server |
| 历史与审计 | PostgreSQL 异步记录 + 定时快照 | 不干扰 Controller 事实源 |
| 写命令可靠性 | dry-run、resourceVersion、幂等键、审计 | 防重复与丢失更新 |
| 外部数据源故障 | partial/warnings、可选 provider | 指标或 Trace 故障不拖垮配置页 |
| 前端刷新 | SSE 失效通知 + REST resync + 轮询 | 弱实时且能从丢事件恢复 |

## 3. 做得较好的地方

### 边界总体清楚

控制面、仿真、聚合、UI 已形成真正的数据闭环。尤其是没有让 Dashboard Backend 重新执行 Controller 算法，也没有让数据库变成影子控制平面，这是长期维护的关键。

### 控制循环考虑了恢复

显式 finalizer、OwnerReference、Status Condition、resourceVersion、pending plan 和 leader election 表明实现不是只覆盖 happy path 的 CRUD 示例。

### 历史语义比较诚实

Backend 对旧 `at` 查询使用数据库快照，无快照就返回不可用，不用当前值伪装历史；这个约束应继续保持。

### 开发清单具备安全基础

多数容器使用 non-root、drop capabilities、read-only root filesystem、探针和资源限制。尽管离生产还有距离，基线优于默认脚手架。

## 4. 做得不够好的地方

### 文档曾长期落后于实现

旧前端 DOCS 仍声称 localStorage、Mock Worker、805 快照和 Trace 占位；根 README 也没有 Backend/Database/Frontend 部署入口。这会让维护者比没有文档更容易做错。本次重构的首要目的就是消除多套事实源。

### 完整部署曾没有一键闭环

旧 `make kind-up-demo` 会另建同名 Kind 集群，且不构建/部署 Dashboard，容易把 Docker Desktop 与 Kind Context 混淆。2026-08-13 已改为复用现有 `docker-desktop`，构建四个项目镜像、向每个 Node 导入全部运行镜像、部署两套 Kustomize 并验证 API/Metrics/Trace/DB/Frontend。CI 仍缺少等价的独立全栈 E2E。

### Frontend 产品语义还有残口

- Config 实际写 Backend，但页面仍出现“本地存储”文案。
- Traffic 页面看起来像执行器，实际上 Overlay 只在内存预览。
- Backend 支持更多 CR，但 UI 只覆盖三个基础资源。
- 没有独立 Dashboard landing；Trace 和 Data View 合并在 `/trace`，命名需统一。

这些问题不是核心架构错误，却会直接误导用户。

### 数据与时间模型尚未闭合

PostgreSQL 有 `clock_state` scaffold，Frontend 有时间条，但 Controller/Simulator 都使用真实墙钟。当前“历史浏览”与“运行控制”必须继续严格分开。若未来直接给前端加倍率，会制造第二套时间真相。

### 生产运维能力不足

本地密码已改为部署时随机生成，但数据库连接仍是本机集群内明文；单副本数据库与单副本 Prometheus/Jaeger（已 PVC）、Grafana 易失、无完整 IAM/NetworkPolicy/备份/DR，使当前清单仍只能被称为开发环境。

### 若干技术债

- Model 能力基准分已迁入必填的 `spec.absoluteScore`，由用户/Backend 维护；旧 Status 仅保留升级兼容。
- `TenantRuntime.status.instanceCount` 命名与实现含义不一致。
- ReplicaSet 被 watch/持久化但不直接出现在 Workloads DTO。
- Backend 批量配置不是跨对象原子事务；API/文案必须避免暗示原子性。
- SSE 没有持久游标；慢客户端丢事件只能全量重同步。
- 前端测试以构建/状态脚本为主，缺少组件、API 错误态和用户流程自动化。

## 5. 测试与验证评价

### 已有覆盖

- Controller 单元/集成测试：策略、流量、性能、SimulatorInstance、Orchestrator、帮助函数和观察性。
- Simulator：引擎、HTTP 健康和重构约束。
- Backend：Handler、幂等、Clock、Mapper、Prometheus、Jaeger、Aggregator、PostgreSQL。
- Frontend：lint、TypeScript/Vite build、Zustand 状态契约脚本。
- 基础设施：Kustomize、历史 kubeconform/promtool/Grafana JSON 验证。

### 当前不足

- 无真实浏览器端到端测试。
- 无全栈 Kind E2E 覆盖 Dashboard、PostgreSQL、Prometheus 和 Jaeger。
- DB test 以轻量路径为主；需要真实 PostgreSQL 迁移/并发/保留策略测试。
- 缺少 chaos/restart：Manager 崩溃、Lease 切换、DB 短暂不可用、SSE 丢事件。
- 缺少规模基准：大量 CR、Pod、snapshot、metric series、trace span。

## 6. 推荐改进方式

优先做可验证性而不是立即扩功能：

1. 在目标机执行现有完整部署验收，并把等价 E2E 做成 CI 可信基线。
2. 修复所有会误导用户的 UI 文案/按钮状态。
3. 让 Traffic 命令闭环并补审计证据。
4. 加生产安全和持久化，再谈对外部署。
5. 用新 CRD 设计真正的仿真时间与实验可复现性。

详细优先级见 [ROADMAP.md](ROADMAP.md)。
