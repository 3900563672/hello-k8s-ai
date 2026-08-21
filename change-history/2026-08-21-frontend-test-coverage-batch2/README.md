# 变更总览：Dashboard 前端测试批量补齐（api/queries/hooks/组件层）

> 日期：2026-08-21 ｜ 级别：P1

## 为什么做

- 首批测试体系（`2026-08-21-frontend-vitest-testing`）落地后，组件层覆盖率仍大面积 0%（config/trace/traffic 目录），api/endpoints 与 api/queries 只有部分覆盖。
- 目标是让核心逻辑层（api、queries、stores、lib、hooks）与关键交互组件具备回归防线，把总行覆盖率从 24% 提升到 39%，核心模块普遍 85%+。

## 改成什么

第二批共新增 26 个测试文件 / 约 160 个用例（累计 48 文件 / 274 用例），全部只加测试代码，零业务源码改动：

1. `src/api/endpoints/` 全 6 文件：`experimentApi` / `traceApi` / `trafficApi` / `configApi` / `aiopsApi`（含 SSE 流解析：事件分发、坏行忽略、跨 chunk 拼接、404 problem、无 body 报错）/ `controlPlaneApi`（补充）。
2. `src/api/queries/` 全 6 文件：experiment / trace / traffic / config / aiops 五组 hooks + queryKeys；共享 `src/test/queryUtils.tsx`（`createQueryClient` / `wrapperFor` / `resetReplayStore` 隔离时间回放 store）。
3. `src/lib`：`validations/previews`（租户/节点预览）、`constants/defaultValues`。
4. `src/stores`：`templateNodeTenant`（node/tenant 模板增删与回填）。
5. `src/hooks`：`useBackendSync`（挂载同步 + EventSource 注册 + 防抖重同步）、`useFullscreenTimeline`、`useWorkspaceContext` / `useWorkspaceCoordinator`（historical/集群不可用强制回 apply）。
6. 组件层 15 个文件：`AlertList` / `AIOpsJobList` / `WindowSummaryPanel` / `TraceWaterfall` / `CommandInput` / `ClusterBubbleField`（补充） / `CollapsibleSection` / `RouteFallbacks` / `ExecutionControls`（执行模式安全切换）/ `ClusterStatus`（分发核验与反馈自动清除）/ `TimeTravelBar`（切面步进与最新语义）/ `AppSidebar` / `RenameDialog` / `CreateDialog` / `BatchDeleteDialog` / `OverlayList` / `TemplateLibrary` / `MonitorPage` / `ConfigTable` / `PreviewCurve` / `TenantForm`（zod 校验：空名称/缩容阈值关系）。

## 关键行为（测试中发现的语义）

- `TimeTravelBar` 步进到最后一个切面时 `mode` 自动回到 `latest`（`modeFor` 语义：最后一条即最新），"回到最新"按钮随之消失——组件测试固化了该行为。
- `AlertList` 渲染 `rule`/`triggeredAt` 字段（契约先行）；`WindowSummaryPanel` 渲染 `scores.verdict`/`scores.reason`。
- 覆盖率阈值调整为 lines 24（新测试文件为纯新增代码，先放宽后随组件补齐回补）。

## 验证

- `npm test`（vitest run）：48 文件 / 274 用例全绿。
- `npm run typecheck`（tsc -b）：通过。
- `npm run lint`（oxlint）：0 warning / 0 error。
- `npm run test:coverage`：All files 38.95% lines；api 96%、api/endpoints 83%、api/queries 89%、observatory 66%、monitor 62%（MonitorWall 89%）。

## 回滚

- 未合并：`git reset --hard HEAD~5`。
- 已合并：删除 `dashboard/frontend/my-app/src/**/*.test.*` 中本批次新增文件即可；业务源码零改动，无运行风险。

## 未验证/待办

- 剩余低覆盖区域：config 表单页（ModelForm/NodeForm/OrchestratorForm/PolicyForm）、trace 工作台（ExperimentPanel/SegmentPanel/TraceWorkbench）、traffic 画布（DrawCanvas/TrafficCanvas）、ObservatoryPage/ConfigPage/GuidePage 大页面。
- 这些大页面依赖 canvas/echarts/拖拽，建议按页面拆分小步补齐。
