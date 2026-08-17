# 实现修改明细

## `hack/local-cluster.sh`

- `open_ports` 新增 `start_port_forward dashboard-internal hello-k8s-ai-dashboard-frontend 18080 80`（WSL 内脚本专用）。
- `stop_port_forwards` 循环加入 `dashboard-internal` 键。
- 8080 转发保留（Windows 浏览器入口，dllhost 转发依赖）。

## `hack/night-run/day-watch.mjs`

- 默认 `BASE = http://localhost:18080`。
- HTTP 层从 undici `fetch` 改为 `node:http` + `agent:false`（每次新连接），网络错误重试 3 次（间隔 1s）。
- spawn keepalive/snapshot 时显式传 `--base-url`，保证全链路同端口。
- PATCH 失败记录每次状态码/错误，便于诊断。
