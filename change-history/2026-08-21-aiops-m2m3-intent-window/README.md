# 变更总览：AIOps M2 意图执行 + M3 时间聚合警戒（#94/#95）

> 日期：2026-08-21 ｜ 级别：P1

## 变更内容

- **M2 意图执行（#94）**：一句话起实验。新增 `internal/aiops/command.go`（LLM 意图解析 + 模板目录 + 权限边界校验）、`POST /api/v1/aiops/commands`（解析落库 parsed）、`POST /api/v1/aiops/commands/{id}/confirm`（gate 校验 → 写流量/调倍速 → 创建并启动实验 → 记录 steps）、`GET /api/v1/aiops/templates`（只读模板目录）。执行只复用既有写通道（gateway/store/aggregator），不新增越权入口。
- **M3 时间聚合与警戒（#95）**：新增 `internal/aiops/aggregator.go`（L3 窗口总结 + L4 日总结，LLM 聚合 + 规则兜底，Upsert 幂等）、`internal/aiops/alerts.go`（连续低分/趋势下滑规则 → aiops_alerts，alert_id 幂等派生）。`GET /api/v1/aiops/windows`、`GET /api/v1/aiops/alerts`。
- **数据**：`aiops_commands`（M2）/`aiops_window_summaries`/`aiops_alerts`（M3）表启用；配置新增 `AIOPS_WINDOW_INTERVAL`、`AIOPS_WINDOW_GRANULARITY`、`AIOPS_ALERT_THRESHOLD`、`AIOPS_ALERT_CONSECUTIVE`（默认关闭，不影响现有链路）。

## 验证

- `go build ./...` / `go vet ./...` / `go test ./...` / `gofmt -l` 全绿
- 新增单测：意图解析/目录校验/越权拒绝、窗口聚合、连续低分警戒、store 集成
- `make docs-check` / `make docs-sync-check` 全绿
