# 对外 AI 产出自查与披露：curl 黄金标准 + GitLab 式 trailer

> 提升日期：2026-08-19 ｜ 来源：调研 curl / GitLab / WordPress Gutenberg 等开源 AI 协作政策 ｜ 适用对象：本地 Agent / 远程 AI
> 触发条件（Use when）：对外提交（commit / PR / issue / 评论 / 文档）或给有 AI 披露政策的开源项目贡献时

## 现象

AI 生成的对外内容（PR / issue / 评论）带着明显的"AI 味"：堆术语、无证据链、安全结论未复核；或向要求披露 AI 的项目贡献时未声明 AI 辅助，被维护者质疑。

## 根因

对外产物没有"读者视角"自查；披露政策因项目而异，未先查证对方要求。

## 可复用规则

- **curl 黄金标准**：交付前重读对外内容，问一句"别人看得出这是 AI 写的吗？"——看得出，就继续打磨到看不出为止。
- AI 发现的安全 / 关键问题：必须先人工复核 + 附验证证据再提交；禁止直接粘贴 AI 生成的报告，禁止提交未验证或虚假结论（curl 政策：提交虚假报告直接封号）。
- 贡献前先查对方政策：`CONTRIBUTE.md` / `AGENTS.md` / 贡献指南中的 AI use 段落；要求披露则加 trailer。
- **GitLab 式 trailer（机器可读，可被 `git interpret-trailers` 枚举）**，写在提交信息末尾 trailer 区：

  ```text
  AI-Assisted: yes
  AI-Tools: <工具名>
  ```

- 本仓库内部提交无披露政策，不需要 trailer；但"黄金标准"适用于一切对外产物。

## 验证方法

- 提交前重读一遍对外内容；`git log -1 --format=%B` 检查 trailer 区格式。
- 需要披露的项目：提交后 `git interpret-trailers <commit>` 能枚举出 `AI-Assisted` 与 `AI-Tools`。
