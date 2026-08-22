# 变更总览：Dashboard 前端测试第 5/6 批（剩余页面补齐：App/弹窗/时间轴 + DataOverviewPage/ConfigPage）

> 日期：2026-08-21 ｜ 级别：P1

## 为什么做

- 第 4 批（frontend-trace-traffic-pages-tests）之后，src 下仍有零覆盖页面与组件：App 路由布局、流量叠加弹窗、时间轴全屏/图表、DataOverviewPage（975 行）、ConfigPage（1145 行）。
- 本批把最后两张大页面补到合格线，前端测试体系进入"页面级全覆盖"状态。

## 改成什么

第 5/6 批新增 7 个测试文件 / 47 用例（累计 85 文件 / 423 用例），零业务源码改动：

1. `src/app/App.test.tsx`：根布局路由冒烟。
2. `src/components/features/traffic/ApplyOverlayDialog.test.tsx`、`PreviewCanvas.test.tsx`：流量叠加弹窗与预览画布。
3. `src/components/shared/TimeTravelBar/FullscreenTimeline.test.tsx`、`TimelineChart.test.tsx`：全屏时间轴与图表。
4. `src/components/features/trace/DataOverviewPage.test.tsx`（11 用例）：加载/错误/空态/富数据/指标卡（Sparkline 与 No samples）/Trace 详情三态/历史模式/刷新/CollapsibleSection 行为。
5. `src/components/features/config/ConfigPage.test.tsx`（18 用例）：Tab 切换/创建（含 identifier 冲突后缀与模板预填）/重命名/批量删除/策略创建（引用对象校验）/表单保存/单删/失败路径/历史只读。
6. `src/test/setup.ts`：补 rAF 同步执行、pointer capture / scrollIntoView polyfill（带 `typeof Element` 守卫，兼容 node 环境）。

## 关键行为（测试中发现的语义/坑位）

- DataOverviewPage 的 metric 卡标签与 MetricTrendChart 标签重复：断言需 `getAllByText`；`simulator.timeScale` 无 metric 时 Sparkline 显示 "No samples"。
- ConfigPage 的 mock 面大：25 个 configQueries hooks + 14 个子组件 stub；子组件 stub 需保留交互入口（onCreate/onConfirm/onValueChange）才能测到页面自身的状态机。
- ConfigPage 错误文案经 `mutationError` 归一化：非 `Error` 实例统一显示"操作失败"；策略名与已有策略冲突时 identifier 自动加后缀。
- CollapsibleSection 标题按钮与内部按钮同名时需限定 role + name 作用域。

## 验证

- `npm test`（vitest run）：85 文件 / 423 用例全绿。
- 项目整体覆盖率：stmts 70.97% / branches 64.02% / funcs 74.94% / lines 73.81%（基线 24/19/21/25）。
- 单文件覆盖率：DataOverviewPage stmts 91.35% / lines 96.47%；ConfigPage stmts 68.27% / lines 76.03%。
- `npm run typecheck` 与 `npm run lint`（oxlint）全绿。

## 回滚

- 未合并：`git reset --hard HEAD~1`；已合并：删除对应测试文件与 setup.ts 增量即可，无业务代码依赖。