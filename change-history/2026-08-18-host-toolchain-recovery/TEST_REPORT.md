# 测试报告：宿主工具链恢复

## 验证命令与真实结果（2026-08-18 00:00-00:15 CST）

| 检查项 | 命令 | 结果 |
| --- | --- | --- |
| WSL 通道 | `wsl -d Ubuntu -- bash -lc "echo WSL_OK"` | WSL_OK |
| Docker 引擎 | `docker version --format '{{.Server.Version}}'` | 29.7.2（启动后 ~5s） |
| 集群节点 | `kubectl get nodes` | 10 worker + control-plane 全 Ready（12d，v1.36.1） |
| 项目工作负载 | `kubectl get pods -n hello-k8s-ai-system` | controller/backend/frontend/postgresql/grafana/jaeger/otel/prometheus 全 1/1 Running |
| Dashboard | `Invoke-WebRequest http://localhost:8080/` | 200 |
| Grafana | `http://localhost:8080/grafana/api/health` | 200 |
| API 就绪 | `http://localhost:8080/api/v1/health/ready` | 200；database/kubernetesCache/providers 全 ready |
| 数据持久化 | ready 响应 | resourceEvents 346367、resourceSnapshots 2223、resourceStates 463（PostgreSQL 保留完整） |
| notify 修复 | Test-Path 各引用路径 | node_repl.exe / codex-computer-use.exe 均存在 |
| 模型有效性 | DeepSeek /models + 最小 chat 请求 | `deepseek-v4-flash` 与 `deepseek-v4-pro` 存在；`deepseek-chat` 别名可用 |
| gocyclo 修复 | git show d4dc41c + CI | 提取 `clearPlacementPlanWhenPaused`，逻辑零变化；CI 全绿 |

## 未验证 / 风险

- `config.toml` 修复需应用重启后完全生效（当前会话已正常，回合结束 notify 不再报错需下一次应用启动确认）。
- Codex 沙箱初始化窗口的"等 30-60 秒自愈"结论基于本次单次观测，若再遇 `setup refresh had errors` 建议等待后再重试，不要立即重启应用。
- 整机重启恢复顺序仅在本次（Docker Desktop 29.7.2 + 内置 K8s v1.36.1）验证，其他版本可能略有差异。
