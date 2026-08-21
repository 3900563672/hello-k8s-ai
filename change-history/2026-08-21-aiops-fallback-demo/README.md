# AIOps 降级预案收尾（#124）：浮窗失败态文案 + 历史预生成脚本

> 日期：2026-08-21 ｜ 关联：docs/aiops/AIOPS_OVERVIEW.md、dashboard/frontend/my-app/src/components/features/aiops/AiChatWidget.tsx

## 为什么做

- 配额保护（日配额 PR）落地后，演示链路仍缺两块：① 浮窗失败态此前可能裸抛后端原文（英文/原始错误，演示观感差）；② 演示需要"打开页面即有 AI 分析历史"，不能等现场跑完实验才有。
- #124 剩余项的收尾：友好文案、预生成历史、开关关闭不报错。

## 改成什么

1. `AiChatWidget` 新增 `friendlyChatError` / `friendlySettingsError`：区分配额超限（429 `DAILY_QUOTA_EXCEEDED`）、限流（429 `CHAT_RATE_LIMITED`）、未启用（404）、网络/超时四类错误，输出用户可读中文；设置读写保留后端校验文案，仅网络/超时转友好提示。
2. 新增 `hack/aiops-preseed.sh`：批量创建→开始→完成切面实验，自动入队并等待分析完成，用于演示前预生成历史分析（数量可配，默认 3 条；`AIOPS_API_BASE` / `PRESEED_TENANT` 可覆盖）。
3. `AIOPS_OVERVIEW` 新增第 8 节「演示与降级预案」：预生成用法、失败态文案分类、开关关闭时前端空态说明。

## 关键行为

- 开关关闭时：`/aiops/*` 分析接口 404，前端区块显示"未启用/空态"，页面不报错；重新打开开关后恢复。
- preseed 脚本前置检查后端就绪 + AIOps 启用且已配 Key，避免"创建了实验却没有分析"的演示事故；分析由 worker 异步产出（含 LLM 调用，受日配额约束）。

## 验证

- `bash -n hack/aiops-preseed.sh` 语法通过；`make test-frontend` / `make lint-md` / `make docs-check` 通过（演示链路端到端验证在 #122）。

## 回滚

- git revert 本提交；删除 AIOPS_OVERVIEW 第 8 节与 hack/aiops-preseed.sh。
