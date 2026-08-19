# 可观测性收敛到 Dashboard 单入口（Prometheus / Jaeger / Grafana）

- 变更日期：2026-08-16
- 关联问题：Fixes #14
- 变更级别：P1 可观测性架构
- 变更范围：Dashboard Backend（Grafana 反代）、Frontend（监控页 + 回显增强）、Grafana 部署、本地端口收敛
- CRD 变化：无
- 数据库变化：无

## 1. 完成结果

部署后不再需要访问 4 个独立地址（8080 / 3000 / 9090 / 16686），全部收敛到 Dashboard 单入口 `http://localhost:8080`：

- Prometheus：Dashboard「数据回显」页（Backend 代理 `/api/v1/metrics/query`）。
- Jaeger：Dashboard「数据回显」页（Backend 代理 `/api/v1/traces`、`/api/v1/traces/{traceID}`）。
- Grafana：Dashboard「监控面板」页 iframe 嵌入（Backend 反代 `/grafana/*`，Grafana sub-path 部署）。

## 2. 关键行为

- 请求链路：浏览器 → Frontend nginx（`/grafana/` 反代）→ Backend（保留前缀转发）→ Grafana（`/grafana/...` sub-path）。
- Grafana 以匿名 Viewer + `serve_from_sub_path=true` 部署；`GF_SECURITY_ALLOW_EMBEDDING=true` 允许同源 iframe。
- Backend 安全中间件对 `/grafana/*` 不再下发 `X-Frame-Options: DENY`，其余 API 保持 DENY。
- `hack/local-cluster.sh` 只转发 Dashboard 8080；Grafana/Prometheus/Jaeger 不再单独暴露端口。

## 3. 数据回显增强

- 指标卡片下方新增 6 个 echarts 时序图（与卡片同一 Prometheus 指标源）。
- Trace 详情新增瀑布时长条：按 trace 内相对时间偏移与时长着色（error span 琥珀色）。

## 4. 影响范围

| 模块 | 影响 |
| --- | --- |
| dashboard/backend | `GRAFANA_URL/GRAFANA_ENABLED/GRAFANA_TIMEOUT` 配置；`/grafana/*` 反代；安全头按路径放行 |
| dashboard/deploy | backend/frontend `imagePullPolicy: Always`（:dev 可变标签每次重新解析 digest） |
| dashboard/frontend | 新增「监控面板」导航与路由；`/grafana` 开发代理；nginx `/grafana/` 反代；数据回显时序图与瀑布 |
| config/observability/grafana.yaml | sub-path 与嵌入环境变量（修复 `GF_SERVER_ALLOW_EMBEDDING` → `GF_SECURITY_ALLOW_EMBEDDING`） |
| hack/local-cluster.sh / Makefile / LOCAL_RUN.md | 端口收敛与访问表更新 |

## 5. 资料入口

- [测试与验证记录](TEST_REPORT.md)
- [实现修改明细](IMPLEMENTATION_DETAILS.md)
- [升级与回滚](MIGRATION_AND_ROLLBACK.md)
