# 测试报告

## 1. 执行的验证与真实结果

| 命令 | 结果 |
| --- | --- |
| `make manifests generate YEAR=2026` | 通过，生成差异仅 4 个文件（2 类型 + 2 CRD） |
| 核对 `config/crd/bases/*.yaml` | 4 处 `x-kubernetes-validations`，rule 与 message 正确 |
| `gofmt -w`（2 个类型文件） | 通过 |
| `go vet ./...` | 通过 |
| `go test ./api/...` | 通过（无测试文件） |

## 2. 未验证项

- API Server 实际拒绝修改引用的行为：本机无可用集群，CEL admission 由 Kubernetes 执行，需在 E2E / 本地集群验证（CI E2E 会安装新 CRD）。
- 现有存量 CR 升级后首次修改引用被拒的体验（行为符合预期，未实测）。
