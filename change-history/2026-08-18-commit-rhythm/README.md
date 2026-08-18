# 变更总览：提交节奏规则——逻辑闭环 ≤2 commit 与 PR squash merge

> 日期：2026-08-18 ｜ 级别：P3 ｜ 对应 Issue：无（用户指定的流程治理）

## 为什么做

- 提交记录过密：8-16 单日 61 个 commit，8-17 24 个、8-18 15 个，大量是"验证-修错-沉淀"循环产生的小 commit，GitHub 记录难以阅读。
- 原因：AI 每步验证/修复就提交一次；仓库虽有"一个逻辑闭环一个 commit"的原则，但未明确落地为可执行的节奏约束。
- 用户明确不重写已有历史（保留可追溯性），要求从流程上治本。

## 改成什么

1. **提交节奏（docs/agents/WORKFLOW.md 第 5 节）**：
   - 一个逻辑闭环最多 2 个 commit（代码 1 + 沉淀 1）。
   - AI 本地可小步提交当检查点，交付前用 `git reset --soft` 归拢成最终形态再 push；只在本地未推送时做，零风险。
   - 需要频繁验证的任务走 PR + squash merge：分支随意提交，合并进 main 压成 1 个 commit，保持 main 每天 1~3 个 commit。
   - 例外：跨模块重构 / 长时任务分批交付可超 2 个，但交付说明须写明拆分理由。
2. **交付检查清单（8.6）**：同步为"一个逻辑闭环 ≤2 commit"，不再是"单 commit"。

## 关键行为

- 已有历史（8-13 至今 112 个 commit）不重写、不合并，保留可追溯性。
- `git reset --soft` 只允许在本地未推送的分支/提交上使用；已推送的 main 不允许整理后 force push。
- PR squash merge 后 main 每个 PR 只留 1 个 commit，PR 分支可随意提交。

## 验证

- `make docs-check`：全绿（WORKFLOW.md 链接与 front-matter 检查）。
- 本条目即按新规则示范：规则 + 归档合并在同一个提交内。

## 回滚

- revert 本提交，删除 WORKFLOW.md 第 5 节新增节奏段与 8.6 检查清单改动即可。
