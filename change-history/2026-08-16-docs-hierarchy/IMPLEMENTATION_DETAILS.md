# 实现修改明细

## 文件清单

### 新增

| 路径 | 说明 |
| --- | --- |
| `docs/agents/README.md` | 本地 Agent 手册索引与阅读决策 |
| `docs/agents/WORKFLOW.md` | Agent 完整工作流（需求 → issue → 开发 → 验证 → 提交 → 归档 → 汇报） |
| `docs/agents/KNOWN_PITFALLS.md` | 踩坑清单：命令/构建/gh/模板 + 领域易误判点 |
| `docs/agents/PRINCIPLES.md` | 架构约束、字段所有权速查、Controller 名称、修改规范（自 AI_CONTEXT 迁移） |
| `docs/remote-ai/README.md` | 远程 AI 手册：能力边界、开工顺序、阅读决策、反馈回路 |
| `docs/remote-ai/WORKFLOW.md` | 远程 AI 工作流：产出格式、必须标注、交接结构 |
| `hack/context-pack-template.md` | CONTEXT_PACK.md 生成模板（占位符 __GENERATED_AT__ 等） |
| `hack/gen-context-pack.sh` | 上下文包生成脚本：渲染模板 + 复制关键文件 + tar.gz |
| `hack/check-docs.py` | Markdown 相对链接/图片路径校验 |
| `change-history/2026-08-16-docs-hierarchy/` | 本次变更归档（README/IMPLEMENTATION_DETAILS/TEST_REPORT/MIGRATION_AND_ROLLBACK） |

### 修改

| 路径 | 说明 |
| --- | --- |
| `docs/AI_CONTEXT.md` | 重写为分层入口 + 基线速览，原内容迁移 |
| `AGENTS.md` | "开始前"改为指向 docs/agents/ 入口与工作流 |
| `docs/README.md` | 新增"文档分层"节，"从哪里开始"按读者区分 |
| `docs/INDEX.md` | 标题下加分层说明，新人路径第 1 条更新 |
| `Makefile` | 新增 `context-pack`、`docs-check` targets |
| `change-history/README.md` | 索引表追加本次条目 |

### 纳入版本控制

- `change-history/2026-08-14-model-absolute-score-production-path/`
- `change-history/2026-08-14-orchestrator-placement-ci-follow-up/`
- `change-history/2026-08-14-simulator-time-scale/`

## 设计要点

- 事实只有一份：`docs/` 专题描述"现在如何运行"；`docs/agents/` 只写"怎么做"与约束并链接专题；`docs/remote-ai/` 与上下文包只做自包含引导，不复制大段专题内容。
- 上下文包是快照：CONTEXT_PACK.md 标注生成时间、分支、最近提交、open issues，远程 AI 以它为准，不臆测更新。
- 所有输出在 `.runtime/`（已被 .gitignore 忽略），生成物不提交。
- 脚本与文档同等对待：语法校验、可回滚、随提交推送。