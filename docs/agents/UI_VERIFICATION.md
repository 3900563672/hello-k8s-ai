# UI 视觉验证与监控链路（UI_VERIFICATION）

> 维护层：agents ｜ 最后同步：2026-08-16 ｜ 对应变更：change-history/2026-08-16-ui-visual-verification/
> 给本地 Agent 的"看 UI / 监控面板"操作手册。目的：不再每次临时写脚本，一条命令拿到截图 + 面板文本。

## 什么时候用

- 用户问"页面 / 面板长什么样、哪里不对"时。
- 改前端或 Grafana 面板后需要验证渲染结果时。
- 需要核对 Grafana 面板查询与 Prometheus 数据是否一致时。

## 能力边界（诚实声明）

- 本环境 `view_image` 不可用，Agent 的"看"= 读页面 DOM 渲染文本（主通道）+ 截图（给用户核实）。
- 结构、文本、数值、布局、控制台报错都能读全；像素级审美判断受限。
- 截图要给用户 Windows 可打开的路径（见下）。

## 一键验证

```bash
cd /root/hello-k8s-ai
node hack/ui-check/grafana-panels.mjs --url http://localhost:8080/monitor --out .codex-tmp/monitor.png
```

- stdout：JSON（`url` / `iframeUrl` / `bodyLen` / `panels[{title, body}]`），可直接解析。
- stderr：进度、截图路径、控制台 error 汇总。
- 脚本已内置：等待 25s 加载、滚动 Grafana iframe 到底（处理懒渲染）、按 `[class*="panel-container"]` 定位面板。
- 截图给用户：正式快照进 `change-history/<条目>/screenshots/`（GitHub 可直接看）；临时预览可复制到 `C:\Users\hh\.codex\visualizations\<会话目录>\`。

## 快照约定（截图进仓库）

- 目录：`change-history/<条目>/screenshots/`，与改动同一次提交（不受 .gitignore 排除，随 git 版本化、可回滚、GitHub 可预览）。
- 命名：`before-<page>.png`（改前）与 `after-<page>.png`（改后）成对提交；`<page>` 用 `monitor` / `config` / `traffic` / `trace` / `dashboard`。
- 触发条件：涉及 UI / Grafana 面板 / 前端视觉的改动必须带快照；纯后端 / CRD / 文档改动不需要。
- 命令：
  ```bash
  node hack/ui-check/grafana-panels.mjs --url http://localhost:8080/<page> --out change-history/<条目>/screenshots/before-<page>.png
  # 改动完成后：
  node hack/ui-check/grafana-panels.mjs --url http://localhost:8080/<page> --out change-history/<条目>/screenshots/after-<page>.png
  ```
- 体积控制：默认 1600×1000（约 120-280KB/张）；页面加载不稳定时加 `--wait <秒>`；不要上传超大或重复截图。
- 基线参照：2026-08-16 现状基线在 `change-history/2026-08-16-ui-visual-verification/screenshots/`（monitor / config / traffic）。
- `.codex-tmp/` 与 `.runtime/` 仍保持 gitignore，只放临时截图；正式快照一律进 change-history。

## 链路（一句话版）

```text
浏览器 / Agent → http://localhost:8080/monitor（Dashboard 单入口）
  → Backend 反代 /grafana/*（保留前缀、放行查询与幂等/认证中间件）
  → Grafana kiosk 面板 hello-k8s-ai-overview（uid=hello-k8s-ai-overview）
  → Prometheus datasource（uid=prometheus）
```

数据源头：Simulator / Controller 暴露 `hello_k8s_ai_*` 指标 → Prometheus → Grafana 面板。
面板配置事实源：`config/observability/grafana.yaml`（2026-08-16 部署版与仓库一致：version 1 / schemaVersion 41 / 12 个面板）。

## 常用直查（不经浏览器）

| 目的 | 命令 |
| --- | --- |
| 部署版 dashboard JSON | `curl -s http://localhost:8080/grafana/api/dashboards/uid/hello-k8s-ai-overview` |
| Prometheus 查询（经 Grafana 代理） | `curl -s 'http://localhost:8080/grafana/api/datasources/proxy/uid/prometheus/api/v1/query?query=hello_k8s_ai_simulator_leader'` |
| 指标直查（备用，经 Prometheus Pod） | `kubectl exec -n hello-k8s-ai-system <prom-pod> -- wget -qO- 'http://localhost:9090/api/v1/query?query=...'` |
| 数据库直查（备用） | `kubectl exec -i -n hello-k8s-ai-system hello-k8s-ai-dashboard-postgresql-0 -- psql -U dashboard -d dashboard -f - < /tmp/x.sql` |

## 监控面板现状（2026-08-16 快照）

12 个面板：控制器 Reconcile 速率 / Reconcile 错误比例 / 当前时段扩缩容决策 / 扩缩容执行结果 / 模拟 TTFT / 模拟队列深度 / 实例池分配 QPS / 有效分与运行分 / 流量分配均值 / 性能样本分类 / Trace 管道吞吐 / 模拟器 Leader。

运行状态：tenant-core / model-lite 共 10 个 Simulator Pod，20x 倍速，当前 Leader=`simulator-tenant-core-model-lite-5df7469856-hjf2g`（`hello_k8s_ai_simulator_leader` 值=1，其余 9 个=0）。

### 已知前端问题清单（待用户确认，未修）

| # | 问题 | 状态 |
| --- | --- | --- |
| 1 | Grafana「Reconcile 错误比例」面板（用户报的第一个 Grafana 错） | 待核对：当前显示 0%，但 Reconcile 速率面板有 `simulator-instance / error` 计数 |
| 2 | 流量页始终显示"还没配置流量"（实际已配置 qps=25） | 待查 |
| 3 | 时间飞速时间轴每次自动刷新闪一遍（其他面板不闪） | 待查 |
| 4 | 「模拟器 Leader」面板：10 行 0/1 平铺、9 红 1 绿，一眼找不到"当前 Leader" | 已确认为展示设计问题；最小修法：expr 加 `== 1` 只留 Leader 行，再去掉红色阈值 |
| 5 | Grafana Live WebSocket `/grafana/api/live/ws` 握手 400（反代不支持 WS 升级） | 无害，面板走轮询；可后置修反代 |

## 相关坑位

- 屏外面板懒渲染：先滚动再读（脚本已内置）。
- Grafana 13 无 `data-panelid`：按 `[class*="panel-container"]` 定位（脚本已内置）。
- 其余见 `docs/agents/KNOWN_PITFALLS.md`「可观测性与 Grafana 嵌入」。