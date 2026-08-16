# 测试报告

## 1. 执行的验证与真实结果

| 命令 | 结果 |
| --- | --- |
| `gofmt -w`（3 个文件） | 通过 |
| `go vet ./internal/controller/` | 通过 |
| `go test ./internal/controller/ -run "TestWorkerNode|TestCalculateNodeUsage|TestWorkerNodePodEvent"` | ok |
| `go test ./internal/controller/ -count=1` | ok（0.109s） |

## 2. 未验证项

- 真实集群事件量与 Reconcile 次数（依赖 CI E2E / 本地部署观测，代码路径与原有行为等价）。
