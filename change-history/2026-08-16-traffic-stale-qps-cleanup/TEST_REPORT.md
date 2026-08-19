# 测试报告

## 1. 执行的验证与真实结果

| 命令 | 结果 |
| --- | --- |
| `gofmt -w` 两个文件 | 通过 |
| `go vet ./internal/controller/` | 通过 |
| `go test ./internal/controller/ -run "TestZeroStaleTrafficQPSOnScaledToZeroInstances\|TestAllocateTraffic\|TestMetricIsFresh"` | ok |
| `go test ./internal/controller/ -count=1` | ok（0.101s） |

## 2. 未验证项

- 真实集群中的缩容场景（依赖 CI E2E / 本地部署验证）。
- `make lint` / CI 全量在推送后由 GitHub Actions 执行。
