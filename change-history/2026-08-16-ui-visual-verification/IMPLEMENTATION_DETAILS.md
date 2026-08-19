# 实现修改明细

## 1. 改动前状态

- 每次"看 UI / 监控面板"都要临时写 Chrome CDP 脚本，工具链路没有沉淀：in-app browser 的 node_repl 不可用、Chrome `--screenshot` 报多 target 错误、`view_image` 本环境不可用，能用的只有 WSL Node + Windows Chrome CDP 这一条，但每次重写。
- Grafana 13 面板容器无 `data-panelid`、屏外面板懒渲染，导致读面板文本时缺下半屏（Leader 面板一度被误判为"空面板"）。
- 监控面板现状（12 面板、Leader 面板设计问题、Reconcile 错误比例、流量"未配置"显示、时间轴闪屏、Grafana Live WS 400）散落在对话里，没有落盘。

## 2. 修改

### 新增文件

- `hack/ui-check/grafana-panels.mjs`：可复用视觉验证脚本。
  - 参数：`--url`（默认 `http://localhost:8080/monitor`）、`--out`（截图路径）、`--wait`（加载秒数，默认 25）。
  - 行为：spawn Windows Chrome（headless=new + CDP 端口）→ 打开 URL → 滚动 iframe 到底 → `Runtime.evaluate` 读全部 `[class*="panel-container"]` 的标题与正文 → `Page.captureScreenshot` 存 PNG → stdout 输出 JSON、stderr 输出进度与控制台错误。
  - 前置：WSL node >= 22（fetch / WebSocket 全局可用）；`CHROME_PATH` 环境变量可覆盖 Chrome 路径。
- `docs/agents/UI_VERIFICATION.md`：Agent 视觉验证操作手册。
  - 能力边界（view_image 不可用 → DOM 文本为主、截图为辅）。
  - 一键用法与输出格式。
  - 链路一句话版：monitor 单入口 → Backend 反代 `/grafana/*` → Grafana kiosk → Prometheus datasource。
  - 常用直查命令表（dashboard JSON、datasource proxy、Prometheus Pod 备用、psql 备用）。
  - 监控面板现状快照（12 面板 + 运行状态：tenant-core/model-lite 10 Pod、20x、Leader=hjf2g）。
  - 已知前端问题清单 5 条（3 条用户提出 + Leader 面板设计问题 + Grafana Live WS 400），标注"待用户确认，未修"。

### 更新文件

- `docs/agents/KNOWN_PITFALLS.md`：命令与终端节加"apply_patch 无法写 UNC 路径"；可观测性节加 4 条（Grafana 13 无 data-panelid + 懒渲染、view_image 不可用、Chrome --screenshot 多 target、Grafana Live WS 400）；头部对应变更更新。
- `docs/agents/README.md`：阅读决策表加"UI / 视觉验证"行；头部对应变更更新。
- `docs/agents/WORKFLOW.md`：第 4 节验证加 UI 视觉验证一行；头部对应变更更新。
- `docs/AI_CONTEXT.md`：内容去向加 UI_VERIFICATION 一行。

## 3. 未做

- 不改任何面板、前端、后端或 Go 代码；不动用户提出的 3 个前端问题与 Leader 面板（等用户确认）。
- 不修改 `docs/` 人类专题（仅登记入口，符合分层边界）。
- 不把脚本接进 CI（本机 Windows Chrome 依赖，不适合 CI）。
