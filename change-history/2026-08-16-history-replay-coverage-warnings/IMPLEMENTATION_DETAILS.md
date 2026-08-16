# 实现修改明细

## 1. 改动前状态

- `/overview?at=...` 从 PostgreSQL 读取快照后，用 `capturedAt` 实时查询 Prometheus `[T-15m, T]` 与 Jaeger `[T-15m, T]`。
- Prometheus 部署配置 `--storage.tsdb.retention.time=24h`；Jaeger 为 all-in-one 内存存储。超出保留范围的历史查询只返回空结果，无任何提示。
- `providerStates` 的 Retention 为占位字符串（`provider-configured` / `runtime-configured`），不反映实际窗口。

## 2. 修改

- `internal/config/config.go`：`ProviderConfig` 增加 `Retention time.Duration`；Prometheus 默认 `24h`（`PROMETHEUS_RETENTION`），Jaeger 默认 `0`（`JAEGER_RETENTION`，0 表示内存模式/未知）。
- `internal/api/handlers_read.go`：
  - 新增 `historyQueryWindow = 15 * time.Minute` 常量，统一历史查询窗口。
  - 新增 `historyCoverageWarnings(asOf, now)`：Prometheus 目标时间早于保留窗口 → 指标告警；Jaeger 配置了窗口 → 超窗告警；未配置（内存模式）且目标时间早于查询窗口 → 内存存储告警。
  - `handleOverview` 在 availability=available 分支追加覆盖告警，随 `meta.warnings` 返回。
  - `providerStates`：Prometheus Retention 输出配置值；Jaeger 输出 `in-memory（进程生命周期）` 或配置窗口（`jaegerRetentionLabel`）。

## 3. 未做

- 事件丢弃/写失败的重试与 dead-letter、关闭排空（recorder.go 仍为有限 channel 丢弃语义）。
- trace_index 持续采集与读取路径。
- 用户命令的端到端关联 ID 与 Backend HTTP Trace 边界。
- Prometheus/Jaeger 持久化存储（保留轻量本地配置的部署原则）。
