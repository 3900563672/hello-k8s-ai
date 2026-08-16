# 测试报告

## 1. 执行的验证与真实结果

| 命令 | 结果 |
| --- | --- |
| 根模块 `gofmt -l internal/ simulator/` | 无输出（全部已格式化） |
| 根模块 `go vet ./...` | 通过 |
| 根模块 `go test ./... -count=1` | 通过（controller / simulator / k8sutil 全部 ok） |
| Backend `gofmt -l .` | 无输出 |
| Backend `go vet ./...` | 通过 |
| Backend `go test ./... -skip Grafana -count=1` | 通过（含 Prometheus、Jaeger provider 测试） |
| 前端 `npm run check`（oxlint + tsc + vite build + verify:state） | 通过，0 警告 0 错误 |
| `make manifests generate YEAR=2026` | 未执行（本次不涉及 CRD / 生成文件） |

> 说明：Grafana 两个既有测试在本机环境失败，属于仓库已知环境问题，本次按既有约定 `-skip Grafana` 跳过，与本次重构无关。

## 2. 验证要点

- `internal/k8sutil` 的 `TestRetryOnConflict`（原 controller 单测迁移）通过：首次冲突重试第二次成功。
- Controller 既有测试（含 fake client 集成测试）全部通过，确认 9 处调用点替换后行为一致。
- Simulator 既有测试通过，`updateOwnedStatus` 改写后 Status 写回语义不变。
- Provider 测试通过：Prometheus / Jaeger 的查询、错误分支与超时行为未变（错误文案保持原样）。
- 前端构建通过：所有字段名、类型与校验 schema 未变；`verify:state` 状态校验通过。

## 3. 未验证项

- 前端视觉回归：仅通过类型与构建校验，未做浏览器级截图对比（本轮只改结构不换样式类，理论无视觉差异）。
- 真实集群行为：本机无可用集群，控制器 / Simulator 的写入行为由 CI E2E 验证。
