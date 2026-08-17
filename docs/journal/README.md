# 踩坑流水账（journal）

> 维护层：agent ｜ 适用读者：本地 Agent（远程 AI 可读背景）｜ 本目录由 2026-08-18 从 `docs/agents/KNOWN_PITFALLS.md` 拆分而来

## 这是什么

踩坑即记录的**低门槛流水账**：现象 → 上下文 → 处理。允许粗糙、允许重复、无限追加；不追求完整，追求"当时发生了什么、后来怎么解决的"能被下一个接手者查到。

## 怎么写（新坑）

文件名：`YYYY-MM-DD-<slug>.md`（slug 用英文短横线）。模板：

```markdown
> 日期：YYYY-MM-DD ｜ 触发者：本地 Agent/远程 AI/人类 ｜ 相关：change-history/<条目>/ 或 PR 号

## 现象（1-2 行）
## 上下文（当时在做什么）
## 处理（结果 + 是否已解决）
```

规则：

- 每次踩坑当天追加，不攒；3-5 行即可，不必写"为什么"。
- 同一主题可追加到已有文件，也可新建；新建优先（时间线更清晰）。
- 修复类条目在"处理"里写提交号或 change-history 链接。
- **蒸馏是定期任务，不是写 journal 时做**：每攒约 20 条或每周一次，按 [docs/lessons/README.md](../lessons/README.md) 的提升流程把可复用规则提升到 lessons，并在原条目标注 `promoted: lessons/<文件>`。

## 归档历史

- 2026-08-18：原 `docs/agents/KNOWN_PITFALLS.md`（587 行 / 87 条）按主题拆入本目录，领域易误判点提升为 `docs/lessons/api-domain-misjudgments.md`。
