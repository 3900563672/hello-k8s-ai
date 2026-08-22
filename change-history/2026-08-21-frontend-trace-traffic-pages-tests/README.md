# 变更总览：Dashboard 前端测试第 4 批（trace/traffic/observatory 大页面 + 路由/布局）

> 日期：2026-08-21 ｜ 级别：P1

## 为什么做

- 第 3 批（frontend-test-coverage-batch2）遗留的"剩余低覆盖区域"：trace 工作台、traffic 画布、ObservatoryPage/GuidePage 等大页面仍无测试。
- 本批覆盖最后一批关键交互页面与路由骨架，PR #144 测试体系补齐到页面级。

## 改成什么

第 4 批新增 10 个测试文件 / 33 用例（累计 78 文件 / 376 用例），零业务源码改动：

1. `src/app/router.test.tsx`：根路由挂载 MainLayout、子路由注册、index 重定向 observatory。
2. `src/components/features/guide/GuidePage.test.tsx`：引导页分区渲染。
3. `src/components/shared/Layout/MainLayout.test.tsx`、`src/components/shared/TimeTravelBar/MiniTimeline.test.tsx`：布局与迷你时间线。
4. `src/components/features/trace/`：ExperimentPanel / SegmentPanel / TraceWorkbench（抽屉 tab、切面分析、决策序列）。
5. `src/components/features/traffic/`：TrafficCanvas（echarts mock）/ TrafficPage（视图切换、draw 模式、历史基线）。
6. `src/components/features/observatory/ObservatoryPage.test.tsx`：标题/统计徽标/六分区、子面板挂载、刷新 refetch、侧边导航。

## 关键行为（测试中发现的语义/坑位）

- TimelineChart 直接 echarts/core init：需 mock 模块本身；TrafficCanvas/SegmentPanel/TraceWorkbench 用 echarts-for-react mock 即可。
- ObservatoryPage 需 stub IntersectionObserver；CollapsibleSection 默认折叠，子面板断言需 mock 为直接渲染。
- TrafficPage 无"历史只读"横幅：历史模式 header 显示"历史基线"/HIST；draw 入口按钮文案是"绘制新模板"。
- 类型修复：OverlayInstance 需 color/createdAt；ExperimentRecord 需 updatedAt；segmentQuery mock 需接收 query 参数。

## 验证

- `npm test`（vitest run）：78 文件 / 376 用例全绿。
- `npm run typecheck`（tsc -b）：通过。
- `npm run lint`（oxlint）：0 warning / 0 error。

## 回滚

- 未合并：`git reset --hard HEAD~1`。
- 已合并：删除 `dashboard/frontend/my-app/src/**/*.test.*` 中本批次新增文件即可；业务源码零改动，无运行风险。

## 未验证/待办

- 剩余大文件：ConfigPage(1145 行)、DrawCanvas(896)、DataOverviewPage(975)、TimelineChart/FullscreenTimeline、ApplyOverlayDialog、CanvasDropZone、PreviewCanvas。
- 覆盖率概况以 `npm run test:coverage` 实测为准（累计约 39%+，待更新）。
