# 变更总览：时间回放与视窗 NaN 健壮性修复（Fixes #140/#141）

> 日期：2026-08-21 ｜ 级别：P1

## 为什么做

- 前端测试体系搭建（分支 codex/frontend-tests）期间，测试发现两个与文档语义不一致 / 健壮性缺陷：
  - #140：`findSnapshotAtOrBefore` 注释声明「目标时刻之前的最后一个切面」，但目标早于最早快照时返回第一条（未来）快照；后端 `GET /api/v1/replay` 对早于最早 snapshot 的 `at` 返回 `availability=unavailable`（handlers_read.go `snapshotFor`：`store.SnapshotAt` 为空 → unavailable），前端应与后端一致（docs/frontend/DATA_FLOW.md：历史无 snapshot → 明确 unavailable，不回退 current）。
  - #141：`clampViewport` 对 NaN 输入 NaN 传播，`timeSlice.setViewport / focusDuration / revealSelected` 全链路可能产出 NaN 视窗边界，导致时间轴渲染错乱。

## 改成什么

1. `timelineMath.ts clampViewport`：入口对 start/end 非有限值归一化（回退 bounds），span 计算不再出现 NaN；NaN 输入回退到全跨度。
2. `timelineMath.ts findSnapshotAtOrBefore`：目标早于最早快照时返回 null（原返回 snapshots[0]），与注释、后端 unavailable 语义、DATA_FLOW 文档一致。
3. `timeSlice.ts jumpToTimestamp`：对 null 保持 no-op（原有行为，未改）。
4. `timeSlice.ts setSnapshots` 历史态兜底链：去掉 `?? ordered.at(-1)` 回退——历史态无可用快照时保持指向原时间点（查询层返回 unavailable），不再回退到最新快照冒充历史（PRINCIPLES：历史不能冒充当前）。
5. 测试断言更新：`timelineMath.test.ts` / `timeSlice.test.ts` 中 #140/#141 相关断言改为新语义，新增 clampViewport NaN/Infinity 用例（#141）。

## 关键行为

- 时间轴跳到早于最早快照的时间点 → 前端 no-op（不选中快照、不回退 current），与后端 `availability=unavailable` 语义一致。
- 任何上游 NaN/Infinity 视窗输入 → 输出 bounds 内的有限视窗，不产生 NaN。

## 验证

- `npm test`（vitest 4）：27 passed（timelineMath 12 + timeSlice 15）。
- `npm run typecheck`（tsc -b）：通过。
- `npm run lint`（oxlint）：0 warnings / 0 errors。
- 后端语义核验：`handlers_read.go snapshotFor` 对无快照 `at` 返回 `"unavailable"`（同类语义见 TestHandleSegmentUnavailableWithoutSnapshots）。

## 回滚

- 未合并：git reset --hard HEAD~1。
- 语义恢复：还原 `findSnapshotAtOrBefore` 的 `: snapshots[0]` 分支与 `clampViewport` 原实现；`setSnapshots` 兜底链恢复 `?? ordered.at(-1)`。

## 未验证/待办

- 未在真实集群做 UI 回放人工验证（Agent 默认不截图，页面效果由用户在浏览器确认）。
- 完整前端 `npm run check` 由 CI 覆盖（本条目验证了 test/typecheck/lint）。
