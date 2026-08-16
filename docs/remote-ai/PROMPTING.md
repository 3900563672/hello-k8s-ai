# 远程 AI 提示词协议（PROMPTING）

> 维护层：remote-ai ｜ 最后同步：2026-08-16 ｜ 对应变更：change-history/2026-08-16-prompting-workflows/
> 本文件给只在自己工作区工作的远程 AI：收到任务后如何解析、如何组织产出、如何交接。流程主链仍在 [WORKFLOW.md](WORKFLOW.md)。
> 人类侧对应手册见 [docs/getting-started/AI_COLLABORATION.md](../getting-started/AI_COLLABORATION.md)；本地 Agent 见 [docs/agents/PROMPTING.md](../agents/PROMPTING.md)。

## 1. 收到任务后先做三件事

1. 读 `CONTEXT_PACK.md`：确认包生成时间、最近提交、open issues 与仓库地图；**你的结论必须基于该生成时间，不臆测更新**。
2. 读本协议与 [WORKFLOW.md](WORKFLOW.md)，明确能力边界：你能分析、能写代码与文档，但**不能**运行测试、访问 GitHub / 集群 / 数据库。
3. 提取任务五要素（目标 / 边界 / 约束 / 验收 / 交付）。缺项的按"最少解释"处理，并在交付物里标注"我假设了 X，如果不对请纠正"。

## 2. 提示词模板（人类或本地 Agent 转交时附带）

```text
附件：<上下文包>（生成时间见 CONTEXT_PACK.md 顶部）。
任务：<目标>。
请按 docs/remote-ai/PROMPTING.md 执行：
- 结论先行，依据引用包内文件路径；
- 推断与"代码看起来如此"必须和事实分开标注；
- 未验证的写"未验证（原因）"；
- 按 WORKFLOW.md 第 5 节固定交接格式产出。
```

## 3. 产出组织规则

- **结论先行**：第一段给出结论与推荐做法，再给依据与细节。
- **依据可回溯**：引用包内文件路径（`internal/controller/...`），不写"文档说"。
- **来源分色**：源码与 `change-history/` 是事实；`docs/` 人类文档只是背景；推断必须显式标注。
- **CRD/API 结论必须核对 `docs/kubernetes/FIELD_OWNERSHIP.md`**：说清"谁能写"，不要假设。
- **时间相关结论先读 `docs/data-flow/TIME_AND_REPLAY.md`**：倍速、历史、当前态语义容易误判（见 `docs/agents/KNOWN_PITFALLS.md` 领域节）。

## 4. 交接格式（与 WORKFLOW.md 第 5 节一致）

```text
标题：<任务名>（<日期>）
结论：<一两句话>
依据：<包内文件路径列表>
交付物：<报告 / diff / 文档片段>
未验证：<逐项列出与原因>
给 Agent 的落地建议：<本地 Agent 执行时的注意点>
```

## 5. 红线

- 不写"已测试 / 已部署 / 已验证"——你无法运行任何东西。
- 不静默按文档写代码：发现文档与源码不一致时，作为交付物的一部分列出差异。
- 不假设你不在包里的文件存在；查不到就写"包内未包含"。
- 对 `docs/agents/`、`docs/remote-ai/`、`change-history/` 的建议可以直接给；对 `docs/` 人类文档的建议单独标注"人类文档"。
