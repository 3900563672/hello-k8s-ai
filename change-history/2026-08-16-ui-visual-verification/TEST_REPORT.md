# 测试报告

## 1. 执行的验证（真实结果）

| 验证项 | 命令 | 结果 |
| --- | --- | --- |
| 一键脚本读面板 | `node hack/ui-check/grafana-panels.mjs --out .codex-tmp/monitor-reuse.png` | stdout JSON 含 12 个面板 title+body；「模拟器 Leader」读到最后 10 行 0/1（hjf2g=1，其余 9 个 0）；截图 1578×902 PNG 保存成功 |
| Prometheus 数据 | `GET /grafana/api/datasources/proxy/uid/prometheus/api/v1/query?query=hello_k8s_ai_simulator_leader` | 10 条 series，与面板渲染一致 |
| 部署版面板清单 | `GET /grafana/api/dashboards/uid/hello-k8s-ai-overview` | 12 面板，与 `config/observability/grafana.yaml` 一致（version 1 / schemaVersion 41） |
| 控制台错误 | 脚本 stderr | 仅 Grafana Live WebSocket 400 噪音（已记录） |
| 文档链接 | `make docs-check` | 通过（python3 hack/check-docs.py，docs-check OK） |
| 上下文包 | `make context-pack` | 重新生成，生成时间 2026-08-16T14:50:54Z（UTC），`docs/agents/UI_VERIFICATION.md` 已入包（不提交） |

## 2. 未验证

- CI：docs-only 提交只触发"文档检查"，其余 workflow 不跑。
- 无集群变更操作；未动端口转发与运行中的模拟器。