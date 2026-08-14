# 文档总索引

## 建议阅读路径

### 新人 60 分钟路径

1. [AI 交接上下文](AI_CONTEXT.md) - 当前状态、不可破坏约束和重要目录。
2. [项目总览](overview/PROJECT_OVERVIEW.md) - 业务问题、核心概念和边界。
3. [架构总览](overview/ARCHITECTURE_OVERVIEW.md) - 组件、事实源和数据链路。
4. [CRD 设计](kubernetes/CRD_DESIGN.md) - 业务语言。
5. [Controller 架构](kubernetes/CONTROLLER_ARCHITECTURE.md) - 控制循环。
6. [端到端数据流](data-flow/END_TO_END_DATA_FLOW.md) - 从用户操作回到页面。
7. [本地运行](getting-started/LOCAL_RUN.md) - 动手验证。

### 修改前端

- [Frontend 架构](frontend/FRONTEND_ARCHITECTURE.md)
- [页面结构](frontend/PAGE_STRUCTURE.md)
- [Frontend 数据流](frontend/DATA_FLOW.md)
- [Backend API 设计](backend/API_DESIGN.md)
- [时间与回放](data-flow/TIME_AND_REPLAY.md)

### 修改 Backend

- [Backend 架构](backend/BACKEND_ARCHITECTURE.md)
- [API 设计](backend/API_DESIGN.md)
- [数据库设计](backend/DATABASE_DESIGN.md)
- [数据聚合](backend/DATA_AGGREGATION.md)
- [安全与 RBAC](operations/SECURITY_AND_RBAC.md)

### 修改 CRD、Controller 或 Simulator

- [CRD 设计](kubernetes/CRD_DESIGN.md)
- [字段所有权](kubernetes/FIELD_OWNERSHIP.md)
- [Controller 架构](kubernetes/CONTROLLER_ARCHITECTURE.md)
- [资源生命周期](kubernetes/RESOURCE_LIFECYCLE.md)
- [Simulator 架构](simulator/SIMULATOR_ARCHITECTURE.md)
- [仿真流程](simulator/SIMULATION_FLOW.md)

### 部署与运维

- [开发环境](getting-started/DEVELOPMENT_ENVIRONMENT.md)
- [本地运行](getting-started/LOCAL_RUN.md)
- [部署](getting-started/DEPLOYMENT.md)
- [验证](getting-started/VERIFICATION.md)
- [集群信息](operations/CLUSTER_INFORMATION.md)
- [排障](operations/TROUBLESHOOTING.md)
- [安全与 RBAC](operations/SECURITY_AND_RBAC.md)
- [生产就绪度](operations/PRODUCTION_READINESS.md)

## 全部文档

| 分区 | 文档 | 主问题 |
| --- | --- | --- |
| 根 | [README](README.md) | 文档如何使用和维护？ |
| 根 | [AI_CONTEXT](AI_CONTEXT.md) | 下一位 AI 必须知道什么？ |
| overview | [PROJECT_OVERVIEW](overview/PROJECT_OVERVIEW.md) | 项目是什么、解决什么问题？ |
| overview | [ARCHITECTURE_OVERVIEW](overview/ARCHITECTURE_OVERVIEW.md) | 全系统如何分层连接？ |
| overview | [DESIGN_PHILOSOPHY](overview/DESIGN_PHILOSOPHY.md) | 为什么这样设计？ |
| overview | [CURRENT_STATUS_AND_ROADMAP](overview/CURRENT_STATUS_AND_ROADMAP.md) | 已完成、未完成、下一步是什么？ |
| overview | [IMPLEMENTATION_RETROSPECTIVE](overview/IMPLEMENTATION_RETROSPECTIVE.md) | 做过什么、怎么做、哪里还不够好？ |
| getting-started | [DEVELOPMENT_ENVIRONMENT](getting-started/DEVELOPMENT_ENVIRONMENT.md) | 工具与环境如何准备？ |
| getting-started | [LOCAL_RUN](getting-started/LOCAL_RUN.md) | 如何本地运行各层？ |
| getting-started | [DEPLOYMENT](getting-started/DEPLOYMENT.md) | 如何部署完整系统？ |
| getting-started | [VERIFICATION](getting-started/VERIFICATION.md) | 如何验证改动与交付？ |
| frontend | [FRONTEND_ARCHITECTURE](frontend/FRONTEND_ARCHITECTURE.md) | 前端结构和状态边界是什么？ |
| frontend | [PAGE_STRUCTURE](frontend/PAGE_STRUCTURE.md) | Dashboard、Config、Traffic、Trace/Data View 各做什么？ |
| frontend | [DATA_FLOW](frontend/DATA_FLOW.md) | 页面如何获取、刷新、提交数据？ |
| backend | [BACKEND_ARCHITECTURE](backend/BACKEND_ARCHITECTURE.md) | Handler 到 Storage 如何分层？ |
| backend | [API_DESIGN](backend/API_DESIGN.md) | 路由、契约、错误和权限是什么？ |
| backend | [DATABASE_DESIGN](backend/DATABASE_DESIGN.md) | PostgreSQL 保存什么、不保存什么？ |
| backend | [DATA_AGGREGATION](backend/DATA_AGGREGATION.md) | Kubernetes/Prometheus/Jaeger 如何组合？ |
| kubernetes | [CRD_DESIGN](kubernetes/CRD_DESIGN.md) | 10 个 CRD 的字段与生命周期是什么？ |
| kubernetes | [CONTROLLER_ARCHITECTURE](kubernetes/CONTROLLER_ARCHITECTURE.md) | 6 个 Reconciler 如何工作？ |
| kubernetes | [RESOURCE_LIFECYCLE](kubernetes/RESOURCE_LIFECYCLE.md) | 资源从配置到回收如何变化？ |
| kubernetes | [FIELD_OWNERSHIP](kubernetes/FIELD_OWNERSHIP.md) | 谁能写哪些字段？ |
| simulator | [SIMULATOR_ARCHITECTURE](simulator/SIMULATOR_ARCHITECTURE.md) | Leader、Tick、状态和遥测如何工作？ |
| simulator | [SIMULATION_FLOW](simulator/SIMULATION_FLOW.md) | 请求、队列、TTFT、冷启动如何计算？ |
| observability | [PROMETHEUS](observability/PROMETHEUS.md) | 指标如何产生、抓取和查询？ |
| observability | [OPENTELEMETRY](observability/OPENTELEMETRY.md) | Trace 如何采集和传输？ |
| observability | [JAEGER](observability/JAEGER.md) | Trace 如何存储、检索和展示？ |
| data-flow | [END_TO_END_DATA_FLOW](data-flow/END_TO_END_DATA_FLOW.md) | 用户到页面回显的完整闭环是什么？ |
| data-flow | [EVENT_FLOW](data-flow/EVENT_FLOW.md) | Watch、SSE、事件、审计如何传播？ |
| data-flow | [TIME_AND_REPLAY](data-flow/TIME_AND_REPLAY.md) | 当前态、历史快照、逻辑时间分别是什么？ |
| operations | [CLUSTER_INFORMATION](operations/CLUSTER_INFORMATION.md) | 当前可观察环境与声明拓扑是什么？ |
| operations | [TROUBLESHOOTING](operations/TROUBLESHOOTING.md) | 如何定位常见故障？ |
| operations | [SECURITY_AND_RBAC](operations/SECURITY_AND_RBAC.md) | 权限和安全边界是什么？ |
| operations | [PRODUCTION_READINESS](operations/PRODUCTION_READINESS.md) | 离生产还有哪些差距？ |
| reference | [CONFIGURATION_REFERENCE](reference/CONFIGURATION_REFERENCE.md) | 关键参数和默认值是什么？ |
| reference | [SOURCE_MAP](reference/SOURCE_MAP.md) | 事实对应哪个源码目录？ |
| reference | [GLOSSARY](reference/GLOSSARY.md) | 术语如何统一？ |
| reference | [API_EXAMPLES](reference/API_EXAMPLES.md) | API 如何调用？ |
| whitepaper | [COMPLETE_OVERVIEW](whitepaper/COMPLETE_OVERVIEW.md) | 十章完整技术白皮书内容源。 |
| whitepaper | [BUILD_PDF](whitepaper/BUILD_PDF.md) | 如何从 Markdown 再生成 PDF？ |

## 文档状态说明

- 本体系基于 2026-08-12 源码审计，并在 2026-08-13 至 2026-08-14 同步了 Docker Desktop 部署、Orchestrator 放置契约和 Model 能力基准链路。
- 2026-08-14 已复核根 Go module、Dashboard Backend、E2E 编译、Shell 语法和三套 Kustomize 渲染；完整验证入口见 [VERIFICATION.md](getting-started/VERIFICATION.md)。
- 当前交付环境没有 kubectl、Docker、Kind 和目标集群，真实运行结论仍以 `make cluster-up` 或 CI Kind E2E 的实际输出为准；见 [CLUSTER_INFORMATION.md](operations/CLUSTER_INFORMATION.md)。
