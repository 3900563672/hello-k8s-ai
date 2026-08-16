# 测试报告

测试日期：2026-08-14

## 1. 输入证据

首次 GitHub Actions 结果：

| 检查 | 结果 | 证据 |
| --- | --- | --- |
| Kind E2E | FAIL | Deployment affinity 可读取，但实例标签下始终没有 Pod，120 秒后超时。 |
| golangci-lint v2.12.2 | FAIL | `placement_plan.go:171` 命中 `modernize/slicesbackward`。 |

Controller Manager 在失败期间为 Running/Ready，镜像已加载且无重启，因此不是 Controller 启动故障。

## 2. 本次实际执行

| 命令或检查 | 结果 | 说明 |
| --- | --- | --- |
| `gofmt` | PASS | 五个修改的 Go 文件格式化完成。 |
| 限定范围 `git diff --check` | PASS | 无空白或 patch 格式错误。 |
| `go test ./internal/controller -count=1` | PASS | 包含 affinity 与 RuntimeDefault 回归断言。 |
| `go test -tags=e2e ./test/e2e -run '^$' -count=1` | PASS | 更新后的 E2E 测试成功编译。 |
| `go vet ./...` | PASS | 无 vet 错误。 |
| `go test ./... -count=1` | PASS | 根 Go module 全部包通过。 |
| `go test -race ./internal/controller -count=1` | PASS | Controller 包 race detector 通过。 |
| `go mod tidy -diff` | PASS | go.mod/go.sum 不需要修改。 |
| `make lint-config` | PASS | golangci-lint 配置有效。 |
| `make lint` | PASS | golangci-lint v2.12.2 输出 `0 issues`。 |

Go 工具在当前容器会提示 telemetry sidecar 无法读取 `/proc/self/exe`，所有命令本身均以状态码 0 完成；该环境提示与项目代码无关。

## 3. 覆盖矩阵

| 风险 | 验证 |
| --- | --- |
| `restricted` Namespace 拒绝 Simulator Pod | fake-client 测试断言 Pod 级 `RuntimeDefault`；E2E 同时读取实际 Deployment 字段。 |
| 修复只覆盖主 Deployment | 同一测试检查主节点和次节点两个 Deployment。 |
| modernize 再次阻断 CI | 使用与 CI 相同的 golangci-lint v2.12.2 完整扫描，结果为 0 issues。 |
| Kind 无法获得 Simulator 镜像 | E2E 显式构建、加载镜像并设置 pull policy 为 Never。 |
| Pod 查询在列表为空时输出 JSONPath 数组越界 | 使用 range 查询空列表并交由 Eventually 重试。 |
| Pod 只完成调度但应用不可用 | E2E 新增 Ready=True 断言。 |
| 放置算法被 lint 修复改变 | 既有 placement/scale-down 测试与全量 Controller 测试通过。 |

## 4. 尚待 CI 确认

当前交付容器没有 Docker、kubectl 和 kind，因此无法在这里重新执行真实 Kind E2E。源码、快速回归、E2E 编译和 lint 已通过；合并前仍应让 GitHub Actions 再执行：

```bash
make test-e2e
```

新的 E2E 会构建并运行真实 Simulator 镜像，并要求 Pod 最终 Ready。若仍失败，Namespace Event 将给出新的运行态原因，不能把本报告的静态结果冒充集群通过。
