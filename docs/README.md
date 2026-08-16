# hello-k8s-ai 文档体系

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
- Agent：读根目录 `AGENTS.md` 与 [agents/README.md](agents/README.md)。
- 远程 AI：读 [remote-ai/README.md](remote-ai/README.md)，包内先读 `CONTEXT_PACK.md`。
- 想快速了解系统：读 [overview/PROJECT_OVERVIEW.md](overview/PROJECT_OVERVIEW.md) 和 [overview/ARCHITECTURE_OVERVIEW.md](overview/ARCHITECTURE_OVERVIEW.md)。
- 要改代码：先查 [reference/SOURCE_MAP.md](reference/SOURCE_MAP.md)、[kubernetes/FIELD_OWNERSHIP.md](kubernetes/FIELD_OWNERSHIP.md) 和对应专题。
- 要部署或排障：从 [getting-started/DEPLOYMENT.md](getting-started/DEPLOYMENT.md) 与 [operations/TROUBLESHOOTING.md](operations/TROUBLESHOOTING.md) 开始。
- 完整目录见 [INDEX.md](INDEX.md)。

## 文档维护规则

1. 一个概念只指定一个主文档，其他位置使用链接和摘要，不复制大段说明。
2. API、CRD 或字段所有权变更时，同一提交必须更新对应主文档和 AI_CONTEXT 的状态/约束。
3. 版本号只在参考表中记录，依赖锁文件仍是最终依据。
4. 集群实况必须附采集时间、Context 和命令；无访问能力时写“未验证”，禁止根据清单推断 Ready。
5. Mermaid 节点名保持短小；复杂字段关系使用表格。

## 文档所有权

| 变化 | 必须更新 |
| --- | --- |
| CRD 字段或校验 | `kubernetes/CRD_DESIGN.md`、`kubernetes/FIELD_OWNERSHIP.md`、白皮书 |
| Controller 读写或 Watch | `kubernetes/CONTROLLER_ARCHITECTURE.md`、`RESOURCE_LIFECYCLE.md`、字段所有权 |
| Backend API/DTO | `backend/API_DESIGN.md`、对应 Frontend 数据流、`reference/API_EXAMPLES.md` |
| 数据库迁移 | `backend/DATABASE_DESIGN.md`、生产就绪度、备份/恢复说明 |
| 页面或路由 | `frontend/PAGE_STRUCTURE.md`、`frontend/DATA_FLOW.md` |
| PromQL/Trace 接入 | 对应 `observability/` 文档与 Backend 聚合文档 |
| 部署拓扑 | `getting-started/DEPLOYMENT.md`、`operations/CLUSTER_INFORMATION.md`、安全/生产就绪度 |
| 当前里程碑 | `overview/CURRENT_STATUS_AND_ROADMAP.md`、`AI_CONTEXT.md` |
