# 历史回放覆盖告警：明确 Provider 保留边界

- 变更日期：2026-08-16
- 关联问题：Fixes #21（Project Review issue-09，覆盖告警部分）
- 变更级别：P1 Backend 历史回放契约
- 变更范围：`dashboard/backend/internal/config/config.go`、`dashboard/backend/internal/api/handlers_read.go`、配置文档
- CRD 变化：无
- 数据库变化：无

## 1. 完成结果

审查指出 `/overview?at=...` 把 PostgreSQL 持久化切面与 Prometheus/Jaeger 实时查询组合成"历史回放"，但没有定义时间覆盖契约：目标时间超出 Provider 保留能力时，页面只显示空白，用户无法区分"当时没有数据"和"数据已丢失"。

本次为 Provider 增加保留窗口配置，并在历史 overview 返回 `meta.warnings` 覆盖告警：

- Prometheus：默认保留 24h（`PROMETHEUS_RETENTION`，与 `config/observability/prometheus.yaml` 一致），目标时间早于窗口时提示指标可能不完整。
- Jaeger：默认部署为内存存储（`JAEGER_RETENTION=0`），无固定保留窗口；目标时间超过 15 分钟查询窗口时提示历史 Trace 仅随进程存活保留、可能已丢失。配置 `JAEGER_RETENTION` 后按配置窗口告警。
- `providerStates` 的 Retention 字段同步输出真实窗口（Prometheus 配置值；Jaeger 显示 `in-memory（进程生命周期）`）。

前端已支持渲染 `meta.warnings`（Overview 顶部 Notice），无需前端改动。

## 2. 关键行为

- 只影响历史 `at` 查询（当前态查询不会触发覆盖告警）。
- 告警只解释缺口原因，不改变数据内容；缺失数据仍返回空结果。
- `PROMETHEUS_RETENTION` / `JAEGER_RETENTION` 为运行时可调配置，不修改代码即可适配长期部署。

## 3. 影响范围

| 模块 | 影响 |
| --- | --- |
| backend config | ProviderConfig 增加 `Retention`；Prometheus 默认 24h，Jaeger 默认 0 |
| backend api | `handleOverview` 增加覆盖告警；`providerStates` 输出真实保留窗口 |
| frontend | 无代码变化（`meta.warnings` 已渲染） |

## 4. 资料入口

- [实现修改明细](IMPLEMENTATION_DETAILS.md)
- [测试报告](TEST_REPORT.md)
- [升级与回滚](MIGRATION_AND_ROLLBACK.md)

## 5. 实测结论与停止线

- `go test ./... -skip Grafana`、`go vet ./...`（dashboard/backend）通过；新增覆盖告警单元测试。
- 停止线：本轮只补齐覆盖告警；事件丢弃重试/关闭排空、trace_index 读取路径、端到端关联 ID、Prometheus/Jaeger 持久化存储拆分留待后续单独评估（见 issue-09 其余方向）。
