# 测试报告

## 静态检查

- `node --check hack/night-run/day-watch.mjs`：通过。
- dashboard/backend：`gofmt` 干净、`go vet ./...` 通过、`go test ./internal/providers/prometheus/...` 通过。
- `make docs-check`：通过。

## 功能验证（docker-desktop 集群，2026-08-17 19:29-19:34 CST）

- 测试剧本：`--loop --interval 15~20s --until <+2~3min> --baseline-qps 35 --peak-qps 60 --peak-minutes 1 --cycle-minutes 2 --run-dir .runtime/longrun-test/`（隔离目录，不动正式数据与流量档位）。
- 截止钳制：第一轮测试 19:32 截止 → 19:32:02 停止（恢复 35qps + summary）；第二轮 19:34 截止 → 19:34:07 停止。均无多余轮次。
- 每轮指标：round JSON 含 `metrics`（qps/queue/ttft/errorRate）；stdout 摘要行带 metrics 字段。
- 峰值中点采样：round-004 进入峰值后 30s（= PEAK_MINUTES/2）触发并落盘 `mid-peak-round-004-*.json`；summary「轮内指标」去重后计数正确（修复前内存+落盘双路径重复计数）。
- meta.json：startIso/endIso/args 正确；`--resummarize` 重生成 summary 轮次 6、无陈旧扩缩容事件。
- 正式 run 重生成：补写 meta.json（05:29:49.074Z~10:14:56.750Z）后 `--resummarize .runtime/longrun/2026-08-17` → 总轮数 20、扩缩容事件 2 条（141→142、198→200）、快照 10 个（此前错误口径为 29 轮 / 6 条 / 13 个）。

## 部署与真机验证（errorRate 修复）

- `docker build -t hello-k8s-ai-dashboard-backend:dev dashboard/backend` + `kubectl kustomize config/dev | kubectl apply -f -` + rollout restart prometheus/backend，均成功 rollout。
- `controller.errorRate`：series 1 条、值 0（此前空）。`simulator.errorRate`：series 1 条、值 0（此前空）。

## 未验证

- 峰值中点采样未经历真实 4 小时剧本（修复在长跑结束后落地，下个剧本自动生效）。
- 长跑全程 errorRate=0 需下次长跑复核（修复时点晚于本次长跑结束）。
