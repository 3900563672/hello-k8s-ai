# 测试与验证记录

- 验证日期：2026-08-16
- 环境：WSL Ubuntu + Docker Desktop 集群（docker-desktop context，真实集群全栈部署）

## 执行记录

| 类别 | 命令 | 结果 |
| --- | --- | --- |
| Backend 单测 | `go vet ./...`、`go test ./...`（golang:1.26 容器） | 通过，含 grafana 反代 4 例 + 安全头 2 例 |
| Frontend | `npm ci && npm run check`（lint + build + verify:state） | 通过 |
| 完整部署 | `make cluster-up` | 通过，验收 10 项全过 |
| 集群链路 | `curl localhost:8080/grafana/api/health` | 200，Grafana 13.1.3 JSON |
| 面板嵌入 | `curl localhost:8080/grafana/d/hello-k8s-ai-overview?kiosk` | 200，`<base href="/grafana/">`，无 `X-Frame-Options` |
| 静态资源 | `curl localhost:8080/grafana/public/build/runtime-*.js` | 200 |
| 端口收敛 | 3000 / 16686 本地无监听；9090 残留旧转发进程（手动清理） | 符合预期 |
| 反向安全 | `/api/v1/*` 响应仍带 `X-Frame-Options: DENY` | 通过 |

## 关键观察（踩坑实录）

1. **Grafana 反代不能剥 `/grafana` 前缀**：剥前缀后面板页被 Grafana 301 回 `http://localhost:8080/grafana/...`（经 nginx SPA fallback 变成控制台首页），静态资源 404。保留前缀后全部 200。
2. **嵌入开关变量名**：`GF_SERVER_ALLOW_EMBEDDING` 无效（Grafana 仍发 `X-Frame-Options: deny`）；正确变量是 `GF_SECURITY_ALLOW_EMBEDDING`。
3. **Backend 安全中间件**：即使 Grafana 放行，Backend 的 `securityHeadersMiddleware` 也会给所有响应加 `X-Frame-Options: DENY`；需对 `/grafana/*` 放行。
4. **Docker Desktop kubelet 的 :dev 标签缓存**：重建镜像后 rollout restart 仍可能跑旧 digest；改为 `imagePullPolicy: Always` 解决。
5. **滚动更新会杀死端口转发**：`kubectl rollout restart` 后必须重新 `make cluster-open`（转发 pin 在旧 pod 上，pod 删除即断）。

## 结论

Prometheus / Jaeger / Grafana 均已收敛到 Dashboard 单入口，真实集群全链路验证通过；浏览器内 iframe 视觉效果未做自动化截图验证（无头环境），面板 HTML、资源、安全头均已按嵌入要求验证。
