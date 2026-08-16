# 测试报告

## 1. 执行的验证与真实结果

| 命令 | 结果 |
| --- | --- |
| `go vet ./...`（dashboard/backend） | 通过 |
| `go test ./... -skip Grafana -count=1`（dashboard/backend） | 通过（Grafana 为本机既有环境失败，跳过） |
| `gofmt -l`（改动的 Go 文件） | 无输出 |

## 2. 新增测试覆盖

- `TestHistoryCoverageWarnings`：实时窗口无告警；超出 Jaeger 内存窗口（15 分钟）只告警 Trace；超出 Prometheus 24h 只告警指标；两者都超出时两条告警；配置 `JAEGER_RETENTION` 后按配置窗口告警。
- `TestJaegerRetentionLabel`：0 → `in-memory（进程生命周期）`；正值 → Duration 字符串。

## 3. 未验证项

- 真实集群上 Prometheus 24h 边界与 Jaeger 内存淘汰的实际行为：由 CI E2E / 本地部署验证（告警为保守提示，不改变数据内容）。
