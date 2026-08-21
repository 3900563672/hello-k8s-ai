# 默认不截图：预览交给用户、验证用 DOM 断言

> 日期：2026-08-20 ｜ 关联：AGENTS.md「必须」第 10 条、frontend-redesign DEV-NOTES

## 为什么做

- 前端迭代中为“截图给用户看”反复搭建/调试浏览器驱动（内置浏览器不可用、Windows 侧 Node 被系统权限拦截、REPL 加载不了自动化库），最终在 WSL 重装 Chromium + 系统依赖 + 中文字体，耗时耗 Token 且截图多数环节失败。
- 用户明确表态：他自己打开网页就能看，不麻烦，截图既耗 Token 又没有意义。

## 改成什么

1. AGENTS.md「必须（Always）」新增第 10 条：默认不截图；UI 效果由用户自己在浏览器查看，Agent 只告知本地服务地址；截图仅在用户明确要求或必须视觉确认（文档/绘图/设计产物）时使用。
2. 前端验证改为：`npm run check`（lint + build + state-check）+ 无头浏览器 DOM 断言（文本/抽屉/控制台错误），不产出截图。
3. frontend-redesign DEV-NOTES.md 同步记录预览与验证方式。

## 验证

- 规则写入后，后续前端迭代不再主动截图；交互验证全部走 DOM 断言（无页面错误、抽屉/筛选文本正确）。

## 回滚

- 纯文档规则，删除 AGENTS.md 第 10 条与对应 change-history 即可，无运行时影响。
