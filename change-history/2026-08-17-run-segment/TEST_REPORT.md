# 测试报告

## 静态检查

- dashboard/backend：`gofmt -w` 干净；`go vet ./...` 通过；`go test ./internal/api/ -run "TestParseSegmentWindow|TestSegmentCoverageWarnings|TestHandleSegment" -v` 全部 PASS（9+4+3 个子用例）。
- 全量 `go test ./...`：仅本机已知 Grafana proxy 2 个失败（httptest 约 300ms 延迟，KNOWN_PITFALLS 已记录，以 CI 为准）。
- frontend：`npm run typecheck` 通过；`npm run check`（oxlint + tsc build + SSR state-check）通过。
- 控制面：`make fmt`、`make vet`、`make test` 通过；`bin/golangci-lint run` → `0 issues`（`make lint` 因本机 GOSUMDB 校验失败无法自动下载工具，直接运行已存在的 v2.12.2 二进制）。

## 单元测试（新增 16 个子用例）

- `TestParseSegmentWindow`：合法窗口、缺参、空参、非法时间、start≥end、超 24h 均按预期；带时区偏移入参归一化 UTC 且保持大小关系。
- `TestSegmentCoverageWarnings`：live 窗口 0 告警；Jaeger 内存模式超 15 分钟 1 条；起点早于 Prometheus 保留窗口 2 条；配置 Jaeger 保留窗口后按窗口告警。
- `TestHandleSegmentUnavailableWithoutSnapshots` / `TestHandleSegmentStoreDisabled`：unavailable + 无快照伪造。
- `TestHandleSegmentInvalidWindow`：3 种非法参数均 400。

## 真机验证（docker-desktop，2026-08-17 20:00-20:30 CST）

部署：`docker build -t hello-k8s-ai-dashboard-backend:dev dashboard/backend` + `kubectl kustomize config/dev | kubectl apply -f -` + rollout restart backend（tag 相同需 restart 才生效）；前端同法构建 + rollout restart。

接口：
- `GET /api/v1/segment?start=2026-08-17T06:00:00Z&end=2026-08-17T10:00:00Z` → `availability=available`，起点快照 05:59:56Z / 终点快照 09:59:55Z，5 指标均有 series，warnings 含 Jaeger 内存模式告警。
- 最近 10 分钟窗口（Prometheus 存活期内）→ 5 指标 series=1、points=121（5s 步长），traces=50，无告警。
- 缺参 / start>end → 400；2026-08-01 无快照窗口 → unavailable + "起点之前没有持久化快照"告警。
- 经前端 nginx 反代（port-forward 28080）同样可用。

页面（CDP 无头 Chrome，http://localhost:28080/trace）：
- 段面板渲染：`时间段切面（Run Segment）` 标题、起点/终点下拉（各 1000+ 快照选项）、"分析时间段"按钮；无 console error。
- 交互：脚本设置起点（19:42:34 事件）与终点（19:43:19 事件）→ 点击"分析时间段" → 6 秒后页面出现"起点状态 / 终点状态 / 区间指标 / 区间 Trace / 时长"，无 "Segment API 请求失败"。
- 快照：`change-history/2026-08-17-run-segment/screenshots/after-trace.png`（1600×1000，175KB）。

## 未验证

- Prometheus 06:00Z-10:00Z 原始指标因 11:41Z 重启（emptyDir）丢失，段接口如实返回空区间——"长跑全时段指标曲线"需 Prometheus 未重启窗口复核。
- 前端下拉在 1000+ 选项下的真实用户操作手感（下一步 UI 阶段优化为时间轴标记）。

