# 变更总览：前端重构合入主仓库（覆盖 dashboard/frontend/my-app）

> 日期：2026-08-21 ｜ 级别：P1

## 为什么做

- 独立工作区 /root/frontend-redesign（17 轮重构迭代，基线 2026-08-20 09:00 主仓库前端副本）完成前端整体重构；经确认主仓库前端基线后无新改动（最后一次 a74c2d3 早于基线），覆盖不丢历史，本次把重构成果整体合入主仓库，后续 AIOps 前端开发回到主仓库单线进行。

## 改成什么

1. 前端整体覆盖：dashboard/frontend/my-app 47 个文件变更 + 新增（零删除）——新增 ObservatoryPage、TraceWorkbench、MonitorWall、CollapsibleSection、metrics 格式化工具，及 plugins/mock-fixtures.ts（vite mock 模式静态 import，缺它编译挂）。
2. 页面/功能保持：Config/Monitor/Trace/Guide/Traffic 全套页面与 experimentApi/trafficApi、Zustand trafficSlice 均在；修复 #101 配置模板加载被 defaultValues 重置覆盖的 bug。
3. 开发体验：新增 dev:mock 脚本（vite --mode mock）；依赖新增 @tailwindcss/vite、echarts、radix-ui 等。
4. 排除清单：node_modules/、dist/、.git/、.env、mock-server.py、DEV-NOTES.md、dev-fixtures/、__pycache__ 不入库；mock 插件只读 src/lib/mocks/fixtures/（主仓库已有）。

## 关键行为

- 验证：npm ci（157 包）→ npm run check（oxlint + build + verify:state）全绿；make docs-sync / docs-check 通过。
- 新沉淀：docs/lessons/process-wsl-port-stale-registration.md（WSL 反复使用端口残留失效状态：换全新端口根治）。

## 回滚

- 未推送或可 force：git revert 本次提交即可回到旧前端（工作区合入前曾 git stash 保护，覆盖前基线为 a74c2d3）。
- 注意：本分支仅含前端 + 相关文档；后端/Notion 等其他会话未提交内容不在本次提交内。

## 未验证/待办

- npm run check 为静态+构建验证；浏览器人工验收由用户执行（默认不截图）。
- docs/lessons/README.md 的 lessons 表新增行（该文件当前含其他会话未提交内容，待其提交后补）。
