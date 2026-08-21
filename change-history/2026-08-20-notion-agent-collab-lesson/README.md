# 浏览器自动化与人工分工经验沉淀（Notion 中枢搭建复盘）

> 日期：2026-08-20 ｜ 关联：docs/lessons/process-human-agent-division-of-labor.md、docs/journal/2026-08-20-notion-hub-browser-automation.md

## 为什么做

- Notion 个人中枢搭建中，浏览器自动化反复失败（坐标偏移 / 代理不稳 / 市集弹窗无反馈），转人工后 15 秒即成功，暴露"全自动最优"的错误假设。

## 改成什么

1. 新增 lesson `process-human-agent-division-of-labor.md`：API/CLI 优先、交互密集操作转人工、失败 2 次即转交、点击前取真实坐标、代理不稳的判别法。
2. 新增 journal 流水账 `2026-08-20-notion-hub-browser-automation.md` 并在原文标注 promoted。
3. `docs/lessons/README.md` 速查表追加触发条件行。

## 关键行为

- 纯 Agent 层文档变更，无源码 / 行为 / 部署变更；MAP 门禁无命中路径。

## 验证

- `make lint-md`（markdownlint）与 `make docs-check`（链接 + MAP）通过。

## 回滚

- 删除上述两个新文件并移除 README 速查表行即可。
