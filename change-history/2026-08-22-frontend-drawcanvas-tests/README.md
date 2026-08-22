# 变更总览：Dashboard 前端测试第 7 批（DrawCanvas 交互状态机全覆盖 + 发现清空重做嵌套 bug #185）

> 日期：2026-08-22 ｜ 级别：P1

## 为什么做

- DrawCanvas（曲线工作台，896 行）此前覆盖率 0%：canvas 交互（绘制/平移/缩放/撤销/保存）完全无测试，是"关键 features 组件"里最后的大缺口。
- 测试发现真实 bug：清空→重做恢复出嵌套笔画结构（#185），验证了补测试的价值。

## 改成什么

新增 1 个测试文件 / 25 用例（累计 86 文件 / 448 用例），零业务源码改动：

- `src/components/features/traffic/DrawCanvas.test.tsx`（25 用例）：
  - 渲染与初始状态：工具栏/空态提示/状态栏/100% 视图/保存与取消按钮。
  - 绘制交互：多笔绘制、原地短笔画丢弃、反向移动忽略、第二笔尾点续接、plot 外落笔忽略。
  - 平移：平移工具/中键/空格临时平移/向左上拖出边界相机钳制 0。
  - 缩放：放大缩小按钮、滚轮 plot 内缩放与 plot 外忽略、适配曲线、回到原点。
  - 撤销/重做/清空与 P/H/0 快捷键、输入框聚焦时快捷键守卫（isTypingTarget）。
  - 保存：空名/过短曲线提示、直线简化到端点、拐点保留、输入框回车保存、取消回调。
  - drawScene 执行路径：2D ctx Proxy mock + Path2D polyfill，断言 setTransform 调用与绘制重绘不抛错。

## 关键行为（测试中发现的语义/坑位）

- jsdom 无 2D ctx：`getContext` 返回 null 时组件安全 no-op；测试用 Proxy mock ctx + Path2D polyfill 走通 drawScene 渲染路径。
- 保存输出可观测：`onSave(name, prepared)` 的控制点即 simplifyPoints/prepareControlPoints 的结果（直线 5 点 → 端点 2 点；拐点曲线保留拐点且坐标 3/2 位小数）。
- 【发现 bug #185】清空曲线→重做后 strokes 嵌套（2 笔显示为 1 笔/2 采样点），根因 `clearCurve` 把整个 strokes 数组压栈而 `redo` 按单笔恢复；测试未固化错误行为，修复后补断言。
- 时间格式化坑：可视范围 88s ≥ 60s 显示 "1.5m"，断言需匹配 formatTime 分钟规则。

## 验证

- `npm test`（vitest run）：86 文件 / 448 用例全绿。
- 项目整体覆盖率：stmts 82.04% / branches 69.04% / funcs 79.02% / lines 85.65%（上批 70.97/64.02/74.94/73.81）。
- DrawCanvas 单文件：stmts 95.52% / lines 98.47% / funcs 93.65% / branches 85.41%（基线 0/0/0/0）。
- `npm run lint`（oxlint 0 错误）与 `npm run typecheck` 全绿。

## 回滚

- 未合并：`git reset --hard HEAD~1`；已合并：删除 DrawCanvas.test.tsx 即可，无业务代码依赖。
