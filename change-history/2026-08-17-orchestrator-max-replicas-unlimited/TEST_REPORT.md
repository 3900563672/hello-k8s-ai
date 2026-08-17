# 测试报告

测试日期：2026-08-17（Asia/Shanghai）

## 1. 本机执行结果（WSL Ubuntu）

| 检查 | 结果 | 说明 |
| --- | --- | --- |
| `gofmt -l internal/controller/ api/v1/` | PASS | 无未格式化文件 |
| `go vet ./internal/controller/... ./api/...` | PASS | 无诊断 |
| `go test ./...` | PASS | 全部包通过（含新增 maxReplicas=0 决策用例） |
| `go test ./internal/controller/ -run TestDecideAt -count=1` | PASS | 0.017s，新增用例覆盖“0 = 不限制，10 副本仍可扩容” |
| `make lint`（golangci-lint v2.12.2 自定义插件版） | PASS | 0 issues |
| Frontend `npm run check`（oxlint + tsc build + state-check） | PASS | lint、构建、状态检查全通过 |
| `kubectl kustomize config/dev` / `config/demo` / `dashboard/deploy` | PASS | 三个清单均正常渲染 |
| `make manifests` / `make generate` | PASS | CRD 生成差异符合预期（description/minimum/CEL） |

## 2. 未验证

- 未在运行集群执行真实流量压测验证扩容超过 10 副本（节点并发容量为真实上限，需用户在 Dashboard 或 kubectl 将 `orch-core.maxReplicas` 改为 0 后实测）。
- 未做 UI 视觉截图对比（按用户此前要求，视觉验证走人眼，不在本次 Token 预算内）。
