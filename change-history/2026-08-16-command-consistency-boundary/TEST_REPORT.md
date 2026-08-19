# 测试报告

## 1. 执行的验证与真实结果

| 命令 | 结果 |
| --- | --- |
| `gofmt -w` 涉及文件 | 通过 |
| `go vet ./...`（dashboard/backend） | 通过 |
| `go test ./internal/api/ -run "Idempotency\|ApplyConfigurationBatch" -v` | 6 个用例全部通过 |
| `go test ./... -skip "Grafana"`（dashboard/backend） | 全部 ok |

新增用例：

- `TestIdempotencyCompletionFailureReleasesReservation`：完成写失败后占位被释放，相同 key 可再次执行且不重放缓存。
- `TestApplyConfigurationBatchReturnsPartialResultsOnMidBatchFailure`：第 2 项失败时返回第 1 项成功结果 + 失败明细（`convergence=failed` + `error`）。
- `TestApplyConfigurationBatchSucceedsForAllResources`：dry-run 批量全成功，逐项 `convergence=dry-run`。

## 2. 未验证项

- 真实集群批量写入的端到端 partial 表现（依赖 CI E2E / 本地部署验证）。
- 数据库连接中断场景下的释放行为（本机无故障注入环境；逻辑与单测等价）。
- `make lint` / CI 全量在推送后由 GitHub Actions 执行。
