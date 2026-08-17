# hello-k8s-ai 文档体系

> 维护层：human | last-reviewed：2026-08-18 | 事实源：docs/MAP.yaml、源码、change-history/
> 本目录是**人类文档**。本地 Agent 默认不读本目录（见根目录 `AGENTS.md` 与 `docs/agents/`）；远程 AI 的默认上下文包（`make context-pack`）会包含本目录，但只作背景，事实以包内源码与 [remote-ai/](remote-ai/README.md) 手册为准。

本目录是项目工程知识库。内容以当前源码、生成的 CRD、Kustomize 清单和可执行测试为依据；已删除与当前实现冲突的旧文档副本。

## 阅读原则

- **实现事实优先级**：API 类型与 Controller/Backend/Frontend 源码 > 生成清单 > 测试 > 本文说明 > 历史文档。
- 文档出现“已实现”时，表示源码中存在对应路径；“已验证”必须同时说明验证日期和范围。
- “清单声明”只说明 Kustomize 会生成该资源，不等于当前集群中资源 Ready。
- “规划/建议”绝不写成现有能力。
- Markdown 是内容源；PDF 从 `whitepaper/COMPLETE_OVERVIEW.md` 生成，不单独维护正文。

## 文档分层

| 层 | 读者 | 入口 |
| --- | --- | --- |
| 人类 | 开发与使用者 | 根目录 `README.md`、本索引、[INDEX.md](INDEX.md) |
| Agent | 能操作本机仓库的 AI | 根目录 `AGENTS.md` + [agents/](agents/README.md) |
| 远程 AI | 只在自己工作区、收打包内容的 AI | [remote-ai/](remote-ai/README.md)，包内先读 `CONTEXT_PACK.md`（`make context-pack` 生成） |

## 从哪里开始

- 人类第一次接手：读根目录 [README.md](../README.md) 或 [PROJECT_OVERVIEW_NEW.md](../PROJECT_OVERVIEW_NEW.md)。
- 想用 AI 协作开发：读 [getting-started/AI_COLLABORATION.md](getting-started/AI_COLLABORATION.md)（任务提示词模板与交付审核）。
- Agent：读根目录 `AGENTS.md` 与 [agents/README.md](agents/README.md)。
- 远程 AI：读 [remote-ai/README.md](remote-ai/README.md)，包内先读 `CONTEXT_PACK.md`。
- 想快速了解系统：读 [overview/PROJECT_OVERVIEW.md](overview/PROJECT_OVERVIEW.md) 和 [overview/ARCHITECTURE_OVERVIEW.md](overview/ARCHITECTURE_OVERVIEW.md)。
- 要改代码：先查 [reference/SOURCE_MAP.md](reference/SOURCE_MAP.md)、[kubernetes/FIELD_OWNERSHIP.md](kubernetes/FIELD_OWNERSHIP.md) 和对应专题。
- 要部署或排障：从 [getting-started/DEPLOYMENT.md](getting-started/DEPLOYMENT.md) 与 [operations/TROUBLESHOOTING.md](operations/TROUBLESHOOTING.md) 开始。
- 完整目录见 [INDEX.md](INDEX.md)。

## 文档维护规则

1. 一个概念只指定一个主文档，其他位置使用链接和摘要，不复制大段说明。
2. API、CRD 或字段所有权变更时，同一提交必须更新 MAP 映射的主文档（见下方所有权表）。
3. 版本号只在参考表中记录，依赖锁文件仍是最终依据。
4. 集群实况必须附采集时间、Context 和命令；无访问能力时写“未验证”，禁止根据清单推断 Ready。
5. Mermaid 节点名保持短小；复杂字段关系使用表格。

## 文档所有权（由 docs/MAP.yaml 生成，运行 make docs-sync 更新）

<!-- docs-sync:ownership-start -->

| 文档 | 映射源码路径 |
| --- | --- |
| `docs/backend/API_DESIGN.md` | `dashboard/backend/internal/api/` |
| `docs/backend/BACKEND_ARCHITECTURE.md` | `dashboard/backend/cmd/`、`dashboard/backend/internal/`、`dashboard/backend/internal/api/`、`dashboard/backend/internal/kubernetes/` |
| `docs/backend/DATABASE_DESIGN.md` | `dashboard/backend/deploy/`、`dashboard/backend/internal/store/` |
| `docs/backend/DATA_AGGREGATION.md` | `dashboard/backend/internal/readmodel/` |
| `docs/data-flow/EVENT_FLOW.md` | `dashboard/backend/internal/` |
| `docs/data-flow/TIME_AND_REPLAY.md` | `dashboard/backend/internal/store/` |
| `docs/frontend/DATA_FLOW.md` | `dashboard/backend/internal/api/`、`dashboard/frontend/my-app/src/`、`dashboard/frontend/my-app/src/components/features/trace/`、`dashboard/frontend/my-app/src/components/features/traffic/` |
| `docs/frontend/FRONTEND_ARCHITECTURE.md` | `dashboard/frontend/my-app/src/`、`dashboard/frontend/my-app/src/components/features/config/`、`dashboard/frontend/my-app/src/components/features/monitor/` |
| `docs/frontend/PAGE_STRUCTURE.md` | `config/observability/`、`dashboard/frontend/my-app/src/components/features/config/`、`dashboard/frontend/my-app/src/components/features/guide/`、`dashboard/frontend/my-app/src/components/features/monitor/`、`dashboard/frontend/my-app/src/components/features/trace/`、`dashboard/frontend/my-app/src/components/features/traffic/` |
| `docs/getting-started/DEPLOYMENT.md` | `config/default/`、`config/demo/`、`config/dev/`、`config/manager/`、`config/samples/`、`dashboard/backend/deploy/`、`hack/`、`setup.sh` |
| `docs/getting-started/LOCAL_RUN.md` | `config/demo/`、`config/dev/`、`config/samples/` |
| `docs/kubernetes/CONTROLLER_ARCHITECTURE.md` | `cmd/`、`internal/controller/`、`internal/k8sutil/` |
| `docs/kubernetes/CRD_DESIGN.md` | `api/v1/` |
| `docs/kubernetes/FIELD_OWNERSHIP.md` | `api/v1/`、`internal/controller/` |
| `docs/kubernetes/RESOURCE_LIFECYCLE.md` | `internal/controller/` |
| `docs/observability/JAEGER.md` | `config/observability/`、`dashboard/backend/internal/providers/`、`internal/observability/` |
| `docs/observability/OPENTELEMETRY.md` | `config/observability/`、`internal/observability/` |
| `docs/observability/PROMETHEUS.md` | `config/observability/`、`config/prometheus/`、`dashboard/backend/internal/providers/`、`simulator/` |
| `docs/operations/SECURITY_AND_RBAC.md` | `config/rbac/` |
| `docs/operations/TROUBLESHOOTING.md` | `hack/` |
| `docs/overview/ARCHITECTURE_OVERVIEW.md` | `cmd/`、`internal/controller/` |
| `docs/reference/API_EXAMPLES.md` | `dashboard/backend/internal/api/` |
| `docs/reference/CONFIGURATION_REFERENCE.md` | `api/v1/`、`config/rbac/`、`dashboard/frontend/my-app/src/components/features/guide/` |
| `docs/remote-ai/README.md` | `hack/context-pack-template.md` |
| `docs/simulator/SIMULATION_FLOW.md` | `simulator/` |
| `docs/simulator/SIMULATOR_ARCHITECTURE.md` | `simulator/` |

<!-- docs-sync:ownership-end -->
