# Agent 操作手册（docs/agents/）

> 本目录给**能操作当前机器与仓库**的 Agent（如 Codex、Claude Code）：可以读写本仓库、执行命令、访问 GitHub。
> 只在自己工作区工作、收打包内容的远程 AI 见 [docs/remote-ai/](../remote-ai/README.md)；人类入口是根目录 [README.md](../../README.md) 与 [docs/INDEX.md](../INDEX.md)。

## 开工顺序（每次任务都走）

1. 阅读根目录 `AGENTS.md` 与本文件。
2. 有对应任务的流程先读 [WORKFLOW.md](WORKFLOW.md)，按流程判断是否需要建 issue。
3. 动手前扫一遍 [KNOWN_PITFALLS.md](KNOWN_PITFALLS.md)，避免重复踩坑。
4. 涉及 CRD、Controller 或 API 时，先核对 [PRINCIPLES.md](PRINCIPLES.md) 与 `docs/kubernetes/FIELD_OWNERSHIP.md`。
5. 按任务选读 `docs/` 对应专题；事实以源码、生成清单和可执行测试为准，不依据说明文档猜实现。

## 阅读决策

| 任务 | 必读 | 可选 |
| --- | --- | --- |
| 新需求 / 大改 | WORKFLOW.md、PRINCIPLES.md | docs/INDEX.md |
| 修 bug | WORKFLOW.md、KNOWN_PITFALLS.md、对应专题 | 踩坑记录 |
| CRD / API / 数据链路变更 | PRINCIPLES.md、kubernetes/FIELD_OWNERSHIP.md、对应专题 | whitepaper/COMPLETE_OVERVIEW.md |
| 文档维护 | WORKFLOW.md、docs/README.md | docs/INDEX.md |
| 打包给远程 AI | 根目录 `make context-pack`，见 docs/remote-ai/README.md | 本目录全部 |

## 维护规则

- Agent 每次任务后：踩了新坑 → 追加 [KNOWN_PITFALLS.md](KNOWN_PITFALLS.md)；完成交付 → 在 `change-history/` 追加日期条目。
- 本目录只写"怎么做"与约束；"是什么 / 为什么"链接到 `docs/` 专题，不复制大段说明。
- 修改本目录与修改代码同等对待：提交、验证、可回滚。