# 变更总览：Dashboard 前端测试体系从零搭建（vitest + Testing Library）

> 日期：2026-08-21 ｜ 级别：P1

## 为什么做

- Dashboard 前端（`dashboard/frontend/my-app`，src 下 116 个 ts/tsx）长期零单元测试：无 vitest/jest，没有任何 `*.test` 文件。
- 既有的 `verify:state` 脚本只校验 zustand 状态不变量（SSR 方式），覆盖不到组件渲染、交互、查询编排与错误态，是最大的质量缺口。
- 补上测试体系并覆盖核心逻辑：stores、api/queries、lib 工具、关键 features 组件（监控墙 / 拓扑气泡 / AIOps 面板与浮窗）。

## 改成什么

1. 测试框架：新增 `vitest` + `jsdom` + `@testing-library/react` + `@testing-library/jest-dom` + `@testing-library/user-event` + `@vitest/coverage-v8`（devDependencies）。
2. 配置与脚本：新增 `vitest.config.ts`（jsdom 环境、setup 文件、`@` alias、覆盖率排除 ui/fixtures）；`package.json` 新增 `test` / `test:watch` / `test:coverage`；`tsconfig.app.json` 开启 `resolveJsonModule`（复用 `src/lib/mocks` JSON fixture）；`tsconfig.node.json` 纳入 `vitest.config.ts`；`.gitignore` 增加 `coverage/`。
3. 测试基建：`src/test/setup.ts`（jest-dom 断言、自动 cleanup、ResizeObserver stub）。
4. 首批 17 个测试文件 / 113 个用例，覆盖矩阵见 PR body：
   - stores：`timeSlice`（时间回放/快照/视口）、`controlPlaneSlice`（执行模式/集群刷新/倍速提交）、`trafficSlice`（模板增删改/Overlay 级联）、`templateSlice`。
   - api：`client`（超时/错误映射）、`endpoints/controlPlaneApi`（PATCH/幂等头/回执）、`queries/queryKeys`（缓存键不变量）。
   - lib：`formatters`（指标聚合/时间格式化）、`validations`（zod schemas）、`clientId`。
   - components：`MonitorWall`（加载/成功/错误可重试）、`ClusterBubbleField`（Pod/节点/租户视图与抽屉）、`AiInsightPanel`（未启用/空列表/已完成详情）、`AiChatWidget`（AI 浮窗）、`trafficMath`、`timelineMath`。
5. CI 联动：`.github/workflows/test.yml` Frontend job 增加 `npm run test:coverage`（由 hello-k8s-ai-dev 在 #142 覆盖率门禁改动中一并加入，本变更提供配套脚本）。

## 关键行为（测试中发现的源码问题）

- #140：时间回放 `findSnapshotAtOrBefore` 目标早于起点时返回未来快照，注释与实现不一致——测试固化现状，语义待前后端确认。
- #141：`clampViewport` 对 NaN 输入传播 NaN，未按非法输入兜底。
- 测试期未改任何业务源码：以上两项以 issue 挂起，避免测试悄悄掩盖或改写行为。

## 验证

- `npm test`（vitest run）：17 文件 / 113 用例全绿。
- `npm run typecheck`（tsc -b）：通过。
- `npm run lint`（oxlint）：0 warning / 0 error。
- `npm run test:coverage`：All files 24.14% stmts / 24.96% lines；核心逻辑层 api 96%、stores 87.21%、lib/formatters 100%、lib 100%。
- `make docs-sync` / `DOCS_CHECK_BASE=origin/main make docs-check` / `make lint-md`：通过。

## 回滚

- 未合并：`git reset --hard HEAD~1`。
- 已合并：移除 `vitest` 相关 devDependencies、`vitest.config.ts`、`src/test/` 与 `*.test.*` 文件，删除 package.json 三个 test 脚本与 `.gitignore` 的 `coverage/` 即可；业务源码零改动，无运行风险。

## 未验证/待办

- 覆盖率仍是起步值（组件层大量 0%）：后续按优先级补 observatory 其余面板（TraceWaterfall/AlertList/WindowSummaryPanel）、traffic 画布与表单页、config 表单与对话框。
- CI 全绿依赖 #142（coverage job）与本次 PR 先后合入 main；中间态 main 的 Frontend job 会短暂引用尚不存在的 `test:coverage` 脚本（本 PR 合入后即恢复）。
