# 测试报告：降级演练缺陷修复

## 本地验证（2026-08-18 00:30-01:00 CST）

| 检查项 | 命令/方式 | 结果 |
| --- | --- | --- |
| Go 静态检查 | `make fmt && make vet && go build ./...` | 通过 |
| 控制器测试 | `go test ./internal/controller/...` | ok 0.094s |
| lint | `make lint`（golangci-lint 自定义插件） | 0 issues |
| 规则语法 | `promtool check rules`（prom/prometheus:v3.13.2） | SUCCESS 9 rules |
| 模拟器 resources | `kubectl get deploy simulator-tenant-core-model-lite -o jsonpath` | requests=50m/64Mi limits=500m/256Mi |
| 重启告警实触发 | rollout restart backend/prometheus 后查 /api/v1/rules | firing（backend+prometheus，排除 simulator 后 2 条） |
| 内存告警无假阳性 | 实时 eval 告警表达式 | matches=0（有 limit 容器最大 ratio 61%） |
| 模拟器扩容不误报 | 实例持续扩容时查规则 | 无 simulator 告警 |

## CI（GitHub Actions）

| 提交 | 代码检查 | 源码与部署验证 | E2E |
| --- | --- | --- | --- |
| 5fa4da6（资源限制+内存告警+首版重启告警） | success 36s | success 4m24s | success 5m23s |
| 771962d（重启告警最终版） | success | success | success |

## 未验证 / 风险

- 内存告警完整触发（for: 10m）未做持续压力验证，用表达式实时查询验证正确性；建议后续长跑中观察。
- 重启告警排除 simulator 后，模拟器实例异常重启不再触发该告警（模拟器有 Leader 缺失告警兜底）。
- 部署中曾误将 prometheus.yaml apply 到 default 命名空间（清单无 namespace 字段），已清理并改为 `-n hello-k8s-ai-system`；建议后续在清单或脚本固化命名空间。
