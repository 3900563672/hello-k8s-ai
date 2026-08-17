# 可观测性与 Grafana 嵌入

> 日期：2026-08-16 ｜ 触发者：本地 Agent ｜ 相关：原 docs/agents/KNOWN_PITFALLS.md 归档（2026-08-18 文档体系重构拆分为 journal/lessons）

### 2026-08-16 Grafana 13 面板无 data-panelid，屏外懒渲染读不全
- 现象：`querySelectorAll('[data-panelid]')` 返回 0；未滚动 iframe 时 `innerText` 缺下半屏面板（如 Leader 面板）。
- 原因：Grafana 13 面板容器是 `[class*="panel-container"]` 而非 data-panelid；屏外面板懒渲染不产出文本。
- 解决：先 `iframe.contentWindow.scrollTo(0, iframe.contentDocument.body.scrollHeight)` 再读；选择器用 `[class*="panel-container"]`。已内置到 `hack/ui-check/grafana-panels.mjs`。
- 验证：滚动后 12 个面板文本全部可读，Leader 面板 10 行 0/1 完整。

### 2026-08-16 本环境 view_image 不可用，视觉验证以 DOM 读取为主
- 现象：`view_image` 连最小 PNG 都返回 Unsupported Image。
- 解决：截图 + DOM 读取双通道；Agent 判定以 DOM 渲染文本为准，截图复制给用户核实。
- 验证：Leader 面板问题通过 DOM 文本确认（10 行 0/1，hjf2g=1）。

### 2026-08-16 Chrome --screenshot 多 target 报错，统一走 CDP 脚本
- 现象：`chrome --headless=new --screenshot` 报 "Multiple targets are not supported in headless mode"；in-app browser 的 node_repl 报 "failed to write kernel assets: 系统找不到指定的路径 (os error 3)"。
- 解决：统一用 `hack/ui-check/grafana-panels.mjs`（WSL Node + Windows Chrome CDP）：`Page.captureScreenshot` 截图 + `Runtime.evaluate` 读 DOM。
- 验证：脚本输出 12 面板文本 + 1578×902 截图。

### 2026-08-16 Grafana Live WebSocket 经反代握手 400
- 现象：控制台反复 `WebSocket connection to 'ws://localhost:8080/grafana/api/live/ws' failed: ... 400`。
- 原因：Backend 反代未对 `/grafana/api/live` 做 WS 升级，Grafana 自动退回轮询刷新。
- 解决：无需处理（面板 10s 刷新正常）；若要修，反代对该路径放行 Upgrade 头。
- 验证：仅控制台噪音，面板数据正常。

### 2026-08-16 Grafana sub-path 反代必须保留 /grafana 前缀
- 现象：iframe 白屏或显示控制台首页；`/grafana/d/...` 返回 301 到 `http://localhost:8080/grafana/...`，静态资源 404。
- 原因：Grafana 以 `GF_SERVER_SERVE_FROM_SUB_PATH=true` 部署时，页面与静态资源只认 `/grafana/...` 路径；反代剥前缀后 Grafana 把面板页 301 回外部入口，经 nginx SPA fallback 变成控制台首页。
- 解决：Backend 反代保留 `/grafana` 前缀原样转发；`GF_SERVER_ROOT_URL=http://localhost:8080/grafana/` 与前端 iframe 路径必须一致。
- 验证：`curl localhost:8080/grafana/d/hello-k8s-ai-overview` 200 且 HTML 含 `<base href="/grafana/" />`。

### 2026-08-16 Grafana 嵌入开关变量名
- 现象：设置了 `GF_SERVER_ALLOW_EMBEDDING=true`，面板响应仍带 `X-Frame-Options: deny`。
- 原因：`allow_embedding` 属于 `[security]` 段，环境变量是 `GF_SECURITY_ALLOW_EMBEDDING`；`GF_SERVER_*` 前缀对应 `[server]` 段，不生效。
- 解决：改用 `GF_SECURITY_ALLOW_EMBEDDING=true`。
- 验证：改后直接请求 Grafana 无 X-Frame-Options 头。

### 2026-08-16 Backend 安全中间件会覆盖 Grafana 放行
- 现象：Grafana 已放行嵌入，但经 Dashboard 8080 访问面板仍带 `X-Frame-Options: DENY`。
- 原因：Backend `securityHeadersMiddleware` 对所有响应强制加 DENY，包括 `/grafana/*` 反代响应。
- 解决：中间件对 `/grafana/` 前缀跳过 X-Frame-Options；API 路径保持 DENY。
- 验证：`security_headers_test.go` 覆盖两条路径；8080 全链路无 X-Frame-Options。
- 补充（同日）：引入幂等与写认证中间件后，Grafana 前端查询（`POST /api/ds/query`）被 `MISSING_IDEMPOTENCY_KEY` 400 拦截，面板全部 No data。修复：`idempotencyMiddleware` 与 `authMiddleware` 对 `/grafana/` 前缀直接放行（上游 UI 流量不是 Dashboard 命令），API 命令路径保持原有约束。
- 验证：`idempotency_test.go` / `auth_test.go` 覆盖；真实集群 `POST /grafana/api/ds/query` 返回 10 个 frame 时间序列，控制器/编排器/模拟器面板查询均返回 1000+ 数据点。

### 2026-08-16 Grafana 384MiB 内存上限运行中打满
- 现象：集群运行一段时间后 Grafana 探针间歇失败（`context deadline exceeded` / HTTP 503），日志大量 `http: Handler timeout` 与 8-10s 请求超时；RESTARTS 可能仍为 0。
- 排查：`kubectl exec <grafana-pod> -- cat /sys/fs/cgroup/memory.current /sys/fs/cgroup/memory.max`，实测 383MiB/384MiB（99.7%）。
- 解决：`config/observability/grafana.yaml` 限额提到 memory 1024Mi / requests 256Mi / cpu 1000m；其余组件按实测水位判断，不超限不动。
- 验证：滚动后 0 重启、无探针告警，水位稳定约 547MiB/1GiB。
- 教训：不要只看 RESTARTS 与就绪状态判断“意外停止”；先看 cgroup 水位与最近事件。

### 2026-08-16 Docker Desktop kubelet 缓存 :dev 标签 digest
- 现象：重建镜像并 `rollout restart` 后，Pod 仍在跑旧镜像（`imageID` 与本地 `docker image inspect` 不一致）。
- 原因：Docker Desktop 内嵌 kubelet 的 containerd 按标签缓存 digest，`imagePullPolicy: IfNotPresent` 不重新解析。
- 解决：dev 部署清单（backend/frontend）改 `imagePullPolicy: Always`；排查时对比 Pod `imageID` 与本地镜像 ID。
- 验证：改后重启 Pod 的 imageID 与本地一致。

### 2026-08-16 滚动更新会杀死端口转发
- 现象：`kubectl rollout restart` 后 `localhost:8080` 连接拒绝，转发日志报 "network namespace ... is closed"。
- 原因：`port-forward svc/` 在首次连接时 pin 到具体 Pod，Pod 被滚动更新删除后转发即断。
- 解决：部署/重启后重新执行 `make cluster-open`（脚本的存活检查只覆盖"检查时刻"，检查后 Pod 再被删仍会断）。
- 验证：重启后重开转发，8080 恢复。
