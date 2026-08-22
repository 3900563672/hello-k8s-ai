# 变更总览：DrawCanvas 清空后重做恢复错乱修复（Fixes #185）

> 日期：2026-08-22 ｜ 级别：P1

## 为什么做

- 前端测试第 7 批（PR #184）为 DrawCanvas 补测试时发现：点击「清空曲线」后再点「重做」，恢复出的笔画结构错乱（清空前 2 笔/4 采样点，重做后 1 笔/2 采样点）。
- 根因：`undo`/`redo` 以单笔（Point[]）为粒度，`clearCurve` 却把整个 `strokes`（Point[][]）塞进 redoStack——重做时把整个笔画数组当作"一笔"恢复，`flattenStrokes` 的笔数/采样点统计随之错乱，保存时 `prepareControlPoints` 会遍历到嵌套数组产生 NaN 控制点。

## 改成什么

- `DrawCanvas.tsx clearCurve`：清空时同时清空 redoStack——「清空」是不可重做的整批操作，与单笔粒度的 redo 历史不一致，混入必然错乱。
- 语义与"新笔画开始清 redoStack"（落笔处 `setRedoStack([])`）保持一致：清空后重做按钮禁用，redo 历史失效。
- 空笔画守卫：`strokes` 为空时 no-op（与 `undo` 的空守卫一致）。

## 关键行为

- 清空曲线 → 重做按钮禁用（不再能恢复出嵌套笔画）。
- 清空后继续作画、撤销、保存行为不变。
- undo/redo 的单笔粒度语义不变。

## 验证

- DrawCanvas 测试（从 PR #184 提取的 DrawCanvas.test.tsx 25 用例，含改为「清空后重做按钮禁用」的断言）全过。
- `npm run check`（lint + build + verify:state）：通过。
- `npm run test`（vitest 全量，main 现有 78 文件 377 用例）：全过，无回归。

## 回滚

- 未合并：git reset --hard HEAD~1。
- 行为恢复：`clearCurve` 恢复 `setRedoStack(current)` 原实现（注意：#184 合并后其「清空后重做按钮可用」断言需一并还原）。

## 未验证/待办

- 断言更新在 PR #184 的 DrawCanvas.test.tsx（frontend-tests 持有）：已协调按新语义改为「重做按钮禁用」，合并顺序建议 #184 先合、本修复 PR 再合（或 #184 内直接改）。
- 未在真实浏览器人工验证（Agent 不截图，页面效果由用户在浏览器确认）。