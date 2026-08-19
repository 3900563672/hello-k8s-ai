# 快照（2026-08-16 现状基线）

> 本目录是"UI 视觉验证链路沉淀"条目的基线快照：后续任何前端 / 面板改动的"改前"参照图。

| 文件 | 页面 | 说明 |
| --- | --- | --- |
| before-monitor.png | <http://localhost:8080/monitor> | 监控面板（Grafana 12 面板）现状 |
| before-config.png | <http://localhost:8080/config> | 配置页现状 |
| before-traffic.png | <http://localhost:8080/traffic> | 流量页现状 |

拍摄命令（可复现）：

```bash
node hack/ui-check/grafana-panels.mjs --url http://localhost:8080/<page> --out change-history/<条目>/screenshots/before-<page>.png
```

约定：UI / 面板视觉改动条目须在 `screenshots/` 下成对提交 `before-<page>.png` 与 `after-<page>.png`；详见 `docs/agents/UI_VERIFICATION.md`「快照约定」。
