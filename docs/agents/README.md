# Agent 操作手册（docs/agents/）

> 维护层：agent | last-reviewed：2026-08-18 | 事实源：源码与 docs/agents/
> 本目录给**能操作当前机器与仓库**的 Agent（如 Codex、Claude Code）：可以读写本仓库、执行命令、访问 GitHub。
> 只在自己工作区工作、收打包内容的远程 AI 见 [docs/remote-ai/](../remote-ai/README.md)；人类入口是根目录 [README.md](../../README.md) 与 [docs/INDEX.md](../INDEX.md)。

## 开工顺序（每次任务都走）

1. 阅读根目录 `AGENTS.md` 与本文件。
2. 按 [WORKFLOW.md](WORKFLOW.md) 第 8 节解析任务五要素；有对应任务的流程先读 WORKFLOW.md，按流程判断是否需要建 issue。
3. 动手前扫 [FAILURE_REGISTRY.md](FAILURE_REGISTRY.md) 末尾 3 条，并按任务类型匹配 [docs/lessons/README.md](../lessons/README.md) 的 **Use when 触发条件**（速查表见该文件），避免重复踩坑。
4. 涉及 CRD、Controller 或 API 时，先核对 [PRINCIPLES.md](PRINCIPLES.md) 与 `docs/kubernetes/FIELD_OWNERSHIP.md`。
5. 按任务选读 `docs/` 对应专题；事实以源码、生成清单和可执行测试为准，不依据说明文档猜实现。

## 阅读决策

| 任务 | 默认事实源 | 人类文档（按需背景） |
| --- | --- | --- |
| 新需求 / 大改 | WORKFLOW.md、PRINCIPLES.md、对应源码 | docs/INDEX.md |
| 修 bug | WORKFLOW.md、journal/lessons（坑位）、对应源码 | 对应专题 |
| 稳定性 / 长时运行 / 组件故障 | [RESILIENCE.md](RESILIENCE.md)、WORKFLOW.md（4.2）、对应源码 | docs/observability/ |
| CRD / API / 数据链路变更 | PRINCIPLES.md、`api/v1/*_types.go`、生成清单、FIELD_OWNERSHIP | 对应专题、白皮书 |
| 数据库 / Schema 变更 | PRINCIPLES.md、对应迁移文件、postgres_integration_test.go | DATABASE_DESIGN（人类文档，按需） |
| 文档维护 | WORKFLOW.md（第 9 节同步协议）、docs/README.md | 无 |
| CI / 工作流 / 变更归档 | WORKFLOW.md（第 10 节轮询节奏、第 9 节同步）、journal | 无 |
| Project / 批量任务 | [PROJECT_REVIEW.md](PROJECT_REVIEW.md)、WORKFLOW.md | project-review/（问题背景） |
| 提示词 / 任务转交 | WORKFLOW.md（第 8 节）、AI_COLLABORATION.md（人类侧） | 无 |
| 打包给远程 AI | `make context-pack`，见 [docs/remote-ai/README.md](../remote-ai/README.md)（含决策矩阵/踩坑速查/交付模板） | 无 |
| UI / 视觉验证 | [UI_VERIFICATION.md](UI_VERIFICATION.md)、journal/lessons | docs/observability/（按需背景） |

## 维护边界

- `docs/` 专题是**人类文档**：Agent 默认不读、不改；需要背景时按需阅读，事实一律以源码、生成清单和可执行测试为准。
- 本目录由 Agent 自己维护；只写"怎么做"与约束，不复制人类文档内容。
- 每次交付后按 [WORKFLOW.md](WORKFLOW.md) 第 9 节同步并记录时间戳；人类文档的过期项只列清单，不擅自改。

## 维护规则

- Agent 每次任务后：踩了新坑 → 追加 [docs/journal/](../journal/README.md)（3-5 行流水账）；完成交付 → 在 `change-history/` 追加日期条目，并按 [WORKFLOW.md](WORKFLOW.md) 第 9 节同步。
- 稳定性相关交付（优雅降级、容量、长时运行）同步更新 [RESILIENCE.md](RESILIENCE.md) 的矩阵与验收清单。
- 修改本目录与修改代码同等对待：提交、验证、可回滚。
