# UI 视觉验证链路沉淀：Chrome CDP 截图 + DOM 读取 + 监控面板现状

- 变更日期：2026-08-16
- 关联问题：无（用户要求沉淀视觉验证能力；本次不改面板与任何业务代码）
- 变更级别：P2 工具与文档
- 变更范围：`hack/ui-check/`、`docs/agents/`、`docs/AI_CONTEXT.md`、`change-history/`
- CRD 变化：无
- 数据库变化：无

## 1. 完成结果

- 新增可复用脚本 `hack/ui-check/grafana-panels.mjs`：WSL Node + Windows Chrome CDP，一条命令完成截图 + 读取页面 / Grafana iframe 全部面板文本；内置 25s 加载等待、滚动处理懒渲染、按 `[class*="panel-container"]` 定位面板、控制台 error 汇总。
- 新增 [docs/agents/UI_VERIFICATION.md](../../docs/agents/UI_VERIFICATION.md)：能力边界、一键用法、页面/数据链路、常用 API 直查、监控面板现状快照与已知前端问题清单。
- 坑位清单追加 6 条：UNC 路径 apply_patch 不可用、Grafana 13 无 `data-panelid` 与懒渲染、`view_image` 不可用、Chrome `--screenshot` 多 target 报错、Grafana Live WebSocket 400。
- Agent 入口登记：`docs/agents/README.md` 阅读决策表、`docs/agents/WORKFLOW.md` 验证节、`docs/AI_CONTEXT.md` 内容去向。

## 2. 关键行为

- 以后任何"看 UI / 监控面板"任务：`node hack/ui-check/grafana-panels.mjs` 一条命令即可，不再每次临时写 CDP 脚本。
- 能力边界诚实声明：本环境 `view_image` 不可用，Agent 判定以 DOM 渲染文本为准，截图复制给用户核实。
- 已确认「模拟器 Leader」面板为展示设计问题（10 行 0/1 平铺、9 红 1 绿，一眼找不到当前 Leader）；最小修法记录在 UI_VERIFICATION.md 问题清单，未实施，等用户确认。

## 3. 影响范围

| 模块 | 影响 |
| --- | --- |
| hack/ui-check | 新增 grafana-panels.mjs（Agent 本机工具，不进 CI） |
| docs/agents | 新增 UI_VERIFICATION.md；README / WORKFLOW / KNOWN_PITFALLS 更新 |
| docs | AI_CONTEXT 内容去向登记 |
| change-history | 新增本条 |