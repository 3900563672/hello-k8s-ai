# 同步协议（SYNC）

> 维护层：agents ｜ 最后同步：2026-08-16 ｜ 对应变更：change-history/2026-08-16-docs-layered-ownership/
> 目的：代码或行为变更后，让三层文档与 change-history 时间线保持一致，避免漂移。

## 1. 谁维护什么

| 层 | 内容 | 维护者 |
| --- | --- | --- |
| human | `docs/` 专题（叙事 / 教程 / 白皮书） | 人；Agent 仅按用户明确要求代笔 |
| agents | `docs/agents/`（工作流 / 踩坑 / 原则 / 本文件） | 本地 Agent，人审核 |
| remote-ai | `docs/remote-ai/` + 上下文包 | 人 + Agent 代笔；远程 AI 通过交付物提建议 |
| 时间线 | `change-history/` | Agent 每次交付追加条目 |

## 2. 触发条件

- 任何代码、CRD、API、行为、部署方式的变更交付后。
- 文档自身结构或规则大改后。

## 3. 同步步骤（Agent 每次交付后执行）

1. 追加 `change-history/YYYY-MM-DD-<主题>/` 条目（README / IMPLEMENTATION_DETAILS / TEST_REPORT / MIGRATION_AND_ROLLBACK），日期用 UTC 日期。
2. 更新 `docs/agents/` 受影响文档：踩了新坑 → `KNOWN_PITFALLS.md`；契约或原则变化 → `PRINCIPLES.md`；流程变化 → `WORKFLOW.md`。
3. 更新 `docs/remote-ai/`：远程 AI 的阅读、产出或交接方式受影响时。
4. 重新生成上下文包：`make context-pack`（`CONTEXT_PACK.md` 顶部的生成时间即新时间戳）。
5. 列出"人类文档待同步清单"（哪些 `docs/` 专题描述已过期），交给用户决定：自己改，或授权 Agent 代笔。
6. 提交信息与最终汇报中注明对应 `change-history/` 条目。

## 4. 时间戳规则

- 每个受维护文档头部维护一行元数据：

```text
> 维护层：<human|agents|remote-ai> ｜ 最后同步：YYYY-MM-DD ｜ 对应变更：change-history/<条目>/
```

- 变更只影响本层时，只更新本层时间戳；跨层影响按第 3 节执行。

## 5. 给远程 AI 的同步义务

- 交付物必须带日期，并声明"基于的包生成时间"（`CONTEXT_PACK.md` 顶部）。
- 发现包内文档与源码不一致时，作为交付物列出差异，不静默按文档写代码。
- 对 `docs/agents/`、`docs/remote-ai/`、`change-history/` 的建议可以直接给；对 `docs/` 人类文档的建议单独标注"人类文档"，由用户转交。

## 6. 可复用提示词（发给任何 AI）

> 你是 <本地 Agent | 远程 AI>。本次任务：<任务>。请先读 <你的层入口>（本地 Agent：`AGENTS.md` + `docs/agents/README.md`；远程 AI：包内 `CONTEXT_PACK.md` + `docs/remote-ai/README.md`），按对应 WORKFLOW 执行；交付后按 SYNC 协议同步，并给出时间戳与 change-history 条目。涉及人类文档的改动先列出清单，不要直接改。