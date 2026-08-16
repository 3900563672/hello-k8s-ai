# 远程 AI 工作流

> 维护层：remote-ai ｜ 最后同步：2026-08-16 ｜ 对应变更：change-history/2026-08-16-prompting-workflows/
> 本文件给只在自己工作区工作的远程 AI。每次任务从"开工"走到"交接"，不跳步。
> 能操作本机仓库的 Agent 走 [docs/agents/WORKFLOW.md](../agents/WORKFLOW.md)。

## 1. 开工

1. 读打包根目录的 `CONTEXT_PACK.md`：确认包生成日期、最近提交、open issues 与仓库地图。
2. 读 [docs/remote-ai/README.md](README.md)、[PROMPTING.md](PROMPTING.md) 与本文，明确能力边界与任务解析规则。
3. 记录任务目标与交付物类型，不要急着写内容。

## 2. 任务分类与产出格式

| 任务类型 | 产出格式 |
| --- | --- |
| 分析 / 方案 | 中文报告：结论先行 + 依据（引用包内文件路径）+ 风险 |
| 写代码 | 完整文件或统一 diff，说明基于哪个提交/文件；注明"未运行验证" |
| 写文档 | 建议插入位置 + 完整内容；与现有文档冲突时指出冲突 |
| 审查代码 | 按严重度列出问题（文件:行）、理由、建议 |

## 3. 必须标注

- 所有推断与"代码看起来如此"的结论，必须与包内证据区分开。
- 未验证项写"未验证（原因）"：你不能运行测试、不能访问集群/GitHub。
- 涉及 CRD/API 的建议必须引用 `docs/kubernetes/FIELD_OWNERSHIP.md` 的归属，不要假设谁能写。
- 涉及时间、倍速、历史数据的结论，先读 `docs/data-flow/TIME_AND_REPLAY.md` 再下判断。

## 4. 版本与同步义务

- 交付物必须带日期，并声明"基于的包生成时间"（`CONTEXT_PACK.md` 顶部）。
- 发现包内文档与源码不一致时，作为交付物列出差异，不静默按文档写代码。
- 对 `docs/agents/`、`docs/remote-ai/`、`change-history/` 的建议可以直接给；对 `docs/` 人类文档的建议单独标注"人类文档"。
- 完整规则见 [docs/agents/SYNC.md](../agents/SYNC.md) 第 5 节。

## 5. 交接

交付物固定结构（协议与红线见 [PROMPTING.md](PROMPTING.md)）：

```text
标题：<任务名>（<日期>）
结论：<一两句话>
依据：<包内文件路径列表>
交付物：<报告 / diff / 文档片段>
未验证：<逐项列出与原因>
给 Agent 的落地建议：<本地 Agent 执行时的注意点>
```

- 用户把交付物交给本地 Agent 后，由 Agent 落地并写 `change-history/` 条目；你可在下一次任务中核对。
- 如果发现文档与源码不一致，把它作为交付物的一部分指出，不要静默按文档写代码。