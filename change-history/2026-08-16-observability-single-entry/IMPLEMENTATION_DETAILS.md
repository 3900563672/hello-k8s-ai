# 实现修改明细

- 变更日期：2026-08-16
- 关联问题：Fixes #14

## 1. 改动前状态

- 本地访问需要 4 个端口转发：Dashboard 8080、Grafana 3000、Prometheus 9090、Jaeger 16686（`hack/local-cluster.sh` 的 `open_ports`）。
- Grafana 面板需要用户另开浏览器页面，Dashboard 内没有统一入口。
- Backend 已有 Prometheus 指标与 Jaeger Trace 的代理接口（`/api/v1/metrics/query`、`/api/v1/traces`），前端「数据回显」页已有指标卡片与 Trace 列表。

## 2. 修改内容

### 2.1 Backend：Grafana 反向代理

- `dashboard/backend/internal/config/config.go`：新增 `Grafana ProviderConfig`（`GRAFANA_URL` 默认 `http://hello-k8s-ai-grafana:3000`、`GRAFANA_ENABLED` 默认 true、`GRAFANA_TIMEOUT` 30s）。
- 新文件 `dashboard/backend/internal/api/grafana_proxy.go`：`httputil.ReverseProxy` 把 `/grafana/*` 转发到 Grafana，**保留 `/grafana` 前缀**（Grafana 为 sub-path 部署，剥前缀会导致面板页被 301 回外部入口、静态资源 404）；配置无效 503、上游失败 502。
- `dashboard/backend/internal/api/server.go`：`Handler()` 中 `mux.Handle("/grafana/", ...)` 仅在 `Enabled` 时注册；链路与其他 API 一致走 CORS/安全/日志中间件。
- `dashboard/deploy/backend.yaml`：新增 `GRAFANA_URL` 环境变量。

### 2.2 Backend：安全头按路径放行

- `dashboard/backend/internal/api/middleware.go`：`securityHeadersMiddleware` 对 `/grafana/*` 不再设置 `X-Frame-Options: DENY`（同源 iframe 需要），其余路径保持 DENY。
- 新文件 `dashboard/backend/internal/api/security_headers_test.go`：验证 `/grafana/*` 无 X-Frame-Options、API 路径保留 DENY。

### 2.3 Grafana 部署（config/observability/grafana.yaml）

- 新增 `GF_SERVER_ROOT_URL=http://localhost:8080/grafana/`、`GF_SERVER_SERVE_FROM_SUB_PATH=true`、`GF_SECURITY_ALLOW_EMBEDDING=true`。
- 保留匿名 Viewer（`GF_AUTH_ANONYMOUS_ENABLED=true`、`GF_AUTH_DISABLE_LOGIN_FORM=true`）。
- 踩坑修正：嵌入开关是 `GF_SECURITY_ALLOW_EMBEDDING`（`[security]` 段），`GF_SERVER_ALLOW_EMBEDDING` 无效（Grafana 仍下发 `X-Frame-Options: deny`）。

### 2.4 Frontend

- 新文件 `dashboard/frontend/my-app/src/components/features/monitor/MonitorPage.tsx`：Grafana 面板 iframe（`/grafana/d/hello-k8s-ai-overview?kiosk`）+ 健康状态指示（每 30s 轮询 `/grafana/api/health`）+ 刷新/新窗口按钮；样式与现有页面一致（深色底、圆角卡片、细字号、lucide 图标）。
- `src/app/router.tsx`：新增 `monitor` 路由；`AppSidebar.tsx`：新增第 4 项「监控面板」导航。
- `src/components/features/trace/DataOverviewPage.tsx`：
  - `aggregateMetricPoints` 返回带时间戳的 `MetricPoint[]`；新增 `MetricTrendChart`（echarts 时序图，6 个指标各一）。
  - Trace 详情新增瀑布时长条（按 trace 相对时间偏移与时长定位，error span 琥珀色）。
- `vite.config.ts`：开发代理新增 `/grafana`；`nginx.conf`：新增 `location /grafana/` 反代到 Backend。

### 2.5 本地端口收敛

- `hack/local-cluster.sh`：`open_ports` 只转发 Dashboard 8080；`print_urls` 只输出 Dashboard 与 `/grafana` 子路径；`stop_port_forwards` 保留对旧 4 键的清理（兼容历史残留 PID 文件）；usage 文案同步。
- `Makefile`：`cluster-open` 注释更新。
- `docs/getting-started/LOCAL_RUN.md`：访问表更新为单入口。

### 2.6 部署镜像策略（dashboard/deploy）

- backend/frontend 的 `imagePullPolicy` 改为 `Always`：`:dev` 是可变标签，Docker Desktop kubelet 会缓存旧 digest 导致重启后仍跑旧镜像；Always 让每次 Pod 创建都重新解析标签。

## 3. 涉及文件

- 新增：`dashboard/backend/internal/api/grafana_proxy.go`、`grafana_proxy_test.go`、`security_headers_test.go`、`dashboard/frontend/my-app/src/components/features/monitor/MonitorPage.tsx`。
- 修改：config/observability/grafana.yaml、dashboard/backend（config/server/app/middleware）、dashboard/deploy（backend/frontend.yaml）、dashboard/frontend/my-app（router/AppSidebar/DataOverviewPage/vite/nginx.conf）、hack/local-cluster.sh、Makefile、docs/getting-started/LOCAL_RUN.md。
