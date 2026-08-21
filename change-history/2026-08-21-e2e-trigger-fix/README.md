# 变更总览：E2E 双触发并行 flake 修复（push 限 main + go test 超时放宽）

> 日期：2026-08-21 ｜ 级别：P1

## 为什么做

- test-e2e.yml 同时监听 push 与 pull_request：同一分支推送触发两个并行 E2E run。
- 并行时 PR 事件 run（refs/pull/N/merge）稳定超时失败（panic: test timed out after 10m0s），push 事件 run 稳定通过——2026-08-21 四次样本 100% 复现（#120）。
- 根因：concurrency 组使用 github.ref，push 事件与 PR 事件 ref 不同（分支 ref vs refs/pull/N/merge），cancel-in-progress 不生效；两个 E2E 并行争抢 runner 资源，BeforeSuite 已耗 170s，time-scale spec 剩余预算不足 600s 超时。

## 改成什么

1. test-e2e.yml：push 触发限定 branches: [main]——PR 分支推送只触发一个 pull_request run，不再并行双跑；main 合入后仍会单跑一次验证。
2. Makefile test-e2e：go test 显式加 -timeout 20m（workflow timeout-minutes: 35 内安全），给慢 runner 留余量。

## 关键行为

- PR 分支推送：1 个 E2E run（pull_request 事件）。
- main 合入：1 个 E2E run（push 事件，branches: [main]）。
- 不再出现同 commit 两个 E2E 并行。

## 验证

- 本修复 PR 自身推送只触发单个 E2E run（此前同改动会双触发），单跑 6m+ 通过。
- make docs-check / lint 全绿。

## 回滚

- 恢复 test-e2e.yml 的 push paths-ignore 块（去掉 branches 行），或去掉 Makefile 的 -timeout 20m 参数。
