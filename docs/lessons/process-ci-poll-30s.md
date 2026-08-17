# 等待 CI 不要长 sleep：推完立刻 30 秒轮询

> 提升日期：2026-08-18 ｜ 来源：journal/2026-08-16-github-actions-ci.md ｜ 适用对象：本地 Agent

## 现象

长 sleep 等待导致用户空等；CI 3-6 分钟完成但 Agent 还在 sleep。

## 根因

把"轮询"写成了"固定等待"，错过完成时刻。

## 可复用规则

- 推送后立刻 `gh run list --limit 1` / `gh run view <id> --json jobs`，每 30 秒一次。
- 普通 job 预期 3-6 分钟；E2E/镜像构建最慢，最多等到 10 分钟再排查。
- 失败取 `gh run view <run-id> --log-failed`，不盲改重推。

## 验证方法

一次推送后记录各 job 完成时间，确认在预期窗口内被捕获。
