# 架构总览

> 维护层：human | last-reviewed：2026-08-21 | 事实源：internal/controller/、cmd/、dashboard/backend/internal/aiops/

## 1. 系统分层

```mermaid
flowchart TB
  subgraph UX["交互层"]
    F["React Frontend"]
  end
  subgraph AP["聚合与命令层"]
    B["Dashboard Backend"]
    AI["AIOps 模块（可选，Backend 内）"]
    DB["PostgreSQL"]
  end
  subgraph AIX["外部 AI 服务（可选）"]
    LLM["OpenAI 兼容 LLM API"]
  end
  subgraph CP["Kubernetes 控制面"]
    K["API Server"]
    R["11 个 CRD"]
    C["7 个 Controller"]
  end
  subgraph DP["仿真数据面"]
    DEP["Deployment / Pod / Lease"]
    S["Simulator"]
  end
  subgraph OBS["可观测性"]
    P["Prometheus"]
    O["OTel Collector"]
    J["Jaeger / Grafana"]
  end
  F <--> B
  B <--> K
  B <--> DB
  AI <--> LLM
  AI <--> DB
  K <--> R
  R <--> C
  C --> DEP
  DEP --> S
  S --> K
  C --> P
  S --> P
  C --> O
  S --> O
  O --> J
  B <--> P
  B <--> J
```

## 2. 四种事实源

| 来源 | 拥有什么事实 | 保留特性 | 不能替代什么 |
| --- | --- | --- | --- |
| Kubernetes API | CR Spec/Status、Deployment、Pod、Lease、Event 的最新状态 | etcd 持久化、Watch、resourceVersion、最终一致 | 长期指标、Trace、完整历史快照 |
| PostgreSQL | resource event、定时 snapshot、audit、idempotency、trace index | 当前清单 30 天保留，可查询旧 `at` | Kubernetes 最新状态和 Controller 决策权 |
| Prometheus | Controller/Simulator 时间序列 | 当前开发清单 168h、PVC（20Gi RWO） | 对象完整 Spec/Status、命令审计 |
| Jaeger | Trace/Span | 当前开发清单 168h、badger + PVC（10Gi RWO） | 指标、资源当前态、精确业务审计 |

Backend 响应中的 `meta.partial`、`warnings`、`sourceVersions` 和时间字段用于承认这些来源并不构成一笔跨系统事务。

## 3. 控制面关系

```mermaid
flowchart TB
  TMP["TenantModelPolicy"] --> SI["SimulatorInstance"]
  SI --> DEP["Deployment / Pod"]
  SIM["Simulator Leader"] --> SI
  TEN["Tenant QPS"] --> TRA["Traffic Controller"] --> SI
  SI --> PERF["PerformanceCollector"] --> TP["TenantPerformance"]
  TP --> ORC["Orchestrator"] --> SI
  POD["Scheduled Pod"] --> WU["WorkerNodeUsage"] --> WN["WorkerNode Status"]
  WN --> ORC
  CLK["SimulationClock"] --> SC["SimulationClock Controller"] --> SI
```

Controller 之间没有方法调用。图中的箭头表示“读一个资源，写另一个资源”，API Server 是隐含中介。

## 4. Dashboard Backend 分层

```mermaid
flowchart TD
  H["HTTP Handler / Middleware"] --> S["Read Service / Command Service"]
  S --> A["ReadModel Aggregator"]
  A --> K["Kubernetes Cache + Mapper"]
  A --> P["Prometheus Provider"]
  A --> J["Jaeger Provider"]
  S --> G["Kubernetes Command Gateway"]
  S --> ST["PostgreSQL Store / Recorder"]
```

- Handler 负责 HTTP、严格 JSON、请求 ID、错误 envelope 和超时。
- Aggregator 负责跨对象关联和页面级 DTO，不拥有控制算法。
- Provider 把安全的 `metricId` 或 Trace filter 转换成外部查询。
- Gateway 只写用户拥有的 CR Spec，先 dry-run，再带 resourceVersion 写入。
- Store 负责历史、审计和幂等，不向 Controller 提供控制输入。
- AIOps 模块（可选）负责实验完成后的分层总结、窗口/日聚合、警戒与 SSE 对话；只读 segments 数据并写 `aiops_*` 表，不反向驱动控制面（见 [AIOPS_OVERVIEW](../aiops/AIOPS_OVERVIEW.md)）。

## 5. Frontend 分层

```mermaid
flowchart TD
  R["Router + MainLayout"] --> P["Observatory / Config / Traffic / Guide"]
  P --> Q["TanStack Query"]
  P --> Z["Zustand UI/Time/Draft"]
  Q --> API["Typed API Endpoints"]
  SYNC["SSE + 30s Poll"] --> Q
```

远端状态由 TanStack Query 管理；Zustand 只保留控制面连接信息、时间游标和 Traffic 草稿等客户端状态。刷新页面不能依赖 localStorage 恢复生产数据。MainLayout 全局挂载 AiChatWidget（AI 对话浮窗），仅经 Backend `/aiops/*` 与 LLM 交互，不直连外部服务。

## 6. 部署拓扑

Docker Desktop 本地环境保留两个 Kustomize 边界：

- `config/dev`：CRD、Controller、Simulator RBAC、OTel Collector、Jaeger、Prometheus、Grafana。
- `dashboard/deploy`：PostgreSQL、Dashboard Backend、Frontend 和 Backend RBAC。

它们使用同一命名空间 `hello-k8s-ai-system`。`bash setup.sh` 负责构建和向每个 Node 导入镜像、应用两个边界、写入动态演示节点策略并执行全链路验收；CI E2E 使用独立 Kind 集群 `hello-k8s-ai-test-e2e`（随 PR 自动运行），不复用日常开发集群。

## 7. 架构不变量

- Controller 的输出状态只能由约定所有者写。
- 实例是否允许存在由 TenantModelPolicy 决定；Simulator 时间倍速由 SimulationClock Controller 同步；实例分配流量由 Traffic 决定；副本由 Orchestrator 决定（批量扩容步长 maxScaleUpBatch 可配置）；Deployment 由 SimulatorInstance Controller 决定；性能由 Simulator 决定。
- Kubernetes Scheduler 负责最终 Node 选择；项目 Controller 只生成 required node affinity。
- AIOps 是可选辅助层：LLM 与规则结论只写 `aiops_*` 表，不写 CR、不驱动 Controller；LLM/存储/遥测失败均不阻断主链路。

## 8. AIOps 智能分析层（可选）

- 触发：实验 complete/fail 时入队；`AIOPS_ENABLED=false` 或面板运行时开关关闭时短路。
- 输出：L1 实体总结（Pod/Node/Tenant）→ L2 切面分数与理由 → L3 窗口认知 → L4 日总结；警戒由分数序列规则触发。
- 可靠性：硬指标规则先行 + LLM 出分、schema 校验、失败重试与规则兜底；对话与审计失败只记日志。
- 安全：LLM key 仅存 Backend 内存，设置接口只回显掩码；前端不直连 LLM。
- 主文档：[AIOPS_OVERVIEW](../aiops/AIOPS_OVERVIEW.md)；API 见 [API_DESIGN](../backend/API_DESIGN.md)。
- Prometheus 通过 Pod discovery 抓取 Simulator；当前没有为每个 SimulatorInstance 创建 Service。
- 最新查询从 cache 读；旧时间点从 snapshot 读；两者不能静默混用。

下一步深入：[端到端数据流](../data-flow/END_TO_END_DATA_FLOW.md) 与 [字段所有权](../kubernetes/FIELD_OWNERSHIP.md)。
