# AIOps 日配额保护（#124 降级预案第一步）：调用次数与 token 双上限

> 日期：2026-08-21 ｜ 关联：docs/aiops/AIOPS_OVERVIEW.md、docs/reference/CONFIGURATION_REFERENCE.md

## 为什么做

- 用户提供新 API key，要求"最高不可超过一定的量、别被别人刷爆"。已有单次 token 上限与会话级分钟限流，但缺全局日配额：key 泄露或被高频调用时没有兜底。
- 作为 #124 演示降级预案的第一步落地。

## 改成什么

1. `config.AIOpsConfig` 新增 `DailyMaxCalls`（`AIOPS_DAILY_MAX_CALLS`，默认 300）与 `DailyMaxTokens`（`AIOPS_DAILY_MAX_TOKENS`，默认 2,000,000）；Validate 校验非负。
2. 新增 `store.Store.SumAIOpsUsageSince`：按 `aiops_audit_log.created_at` 统计 24h 内调用次数与 token 总量。
3. 新增 `aiops.Service.CheckDailyQuota`：超限返回友好错误；接入三处——对话入口（429 `DAILY_QUOTA_EXCEEDED`）、分析入队（短路跳过，仅 Warn 日志）、分析执行前（失败带文案，不影响已有任务）。
4. 测试：`TestCheckDailyQuota`（次数/ token / 关闭三态）、`TestEnqueueQuotaExceeded`（超限不入队）。

## 关键行为

- 配额统计基于审计表，不新增表；0 表示不限制，默认开启（300 次 / 200 万 token 每 24h）。
- key 存放不变：`.runtime/aiops.env`（gitignore 覆盖），旧 key 已备份 `.runtime/aiops.env.bak-20260821`。

## 验证

- `make test` 全绿（含新增配额测试）；`make docs-sync` / `make docs-check` 通过。

## 回滚

- git revert 本提交；删除 CONFIGURATION_REFERENCE / AIOPS_OVERVIEW 配额说明。
