# 变更总览：部署链路 AIOps 持久化 + port-forward 竞态修复（#136/#137）

> 日期：2026-08-21 ｜ 级别：P1

## 为什么做

- #136：`make cluster-up` 重新部署会把 AIOps 配置重置为默认（enabled=false / OpenAI），演示前常忘记手动 patch，已两次导致演示时 AIOps 面板不可用。上一轮已提供 `hack/aiops-enable.sh` 一键脚本，但"部署后自动恢复 + 部署输出提示"两个验收项仍未闭环。
- #137：pod 滚动重建（如 backend rollout）后旧 port-forward 进程退出，`start_port_forward` 仅凭"进程在 + 端口通"误判复用，留下 stale pid，8080 不可达且不再重建。

## 改成什么

1. `hack/local-cluster.sh` 新增 `apply_aiops_config()`：部署流程（open_ports 之后、验收输出之前）执行——若 `.runtime/aiops.env` 存在则自动调用 `bash hack/aiops-enable.sh` 恢复 AIOps；随后经 `/api/v1/aiops/settings` 读取并输出 AIOps 状态（enabled/keyConfigured）；无 env 文件时提示保持关闭及启用命令。满足 #136 两个验收项。
2. `start_port_forward()` 复用检查增加 HTTP 探活：进程在 + 端口监听 + `curl http://127.0.0.1:$local_port/` 通过（curl 不可用时退回原端口探测）才算"已在运行"；否则清理 stale pid 并重建。修复 #137 竞态。
3. 文档同步：DEPLOYMENT.md（一键启用→自动恢复说明）、TROUBLESHOOTING.md（#136/#137 排查项更新）。
4. 新增本 change-history 条目。

## 关键行为

- 幂等：`apply_aiops_config` 每次部署后执行；`aiops-enable.sh` 幂等，重复执行无副作用。
- 自动恢复仅在本机 `.runtime/aiops.env` 存在时发生（密钥不入库）；CI/新环境无 env 文件则保持关闭并明确提示。
- HTTP 探活只针对当前两个调用点（8080/18080 前端 HTTP 服务）；curl 缺失时退化原逻辑，不改变脚本前置依赖。

## 验证

- `bash -n` / `make lint-sh`（shellcheck）通过。
- 完整 `make cluster-up` 实测：AIOps 自动启用（enabled=true/keyConfigured=true/DeepSeek），部署输出含 AIOps 状态行；端口转发探活正常。
- `make docs-check`（含 MAP 门禁）、`make lint-md` 通过。

## 回滚

- 未合并：git reset --hard HEAD~1。
- 运行中：删除 `apply_aiops_config` 调用与函数、还原 `start_port_forward` 探活逻辑即可；AIOps 状态由 Deployment env 决定，`kubectl -n hello-k8s-ai-system set env deployment/hello-k8s-ai-dashboard-backend AIOPS_ENABLED=false` 可关闭。
