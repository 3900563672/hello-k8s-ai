# 迁移与回滚：运行前体检、工具链自检与 Prometheus 告警

## 数据迁移

- 无数据迁移。Prometheus 数据继续使用既有 PVC（20Gi），本次只是配置与 Deployment 策略变化；Recreate 重启会保留全部 TSDB 数据（已验证重启后 24 条内存序列持续可查）。

## 行为变化

- 一键启动（`run_up`）与长跑（`start-longrun.sh`）现在会先执行 `hack/preflight.sh`：任何 FAIL 项直接拒绝启动。首次遇到时可能因环境问题（如端口被其他进程占用、内存不足）被拦下，这是预期行为，修掉 FAIL 项再启动。
- 长跑强制 sleep-guard：未开启 `guard=on` 不再"警告后继续"，直接不启动（避免宿主机休眠冻结 WSL 的旧事故）。
- Prometheus 配置更新流程：`kubectl apply -k config/dev` 后 `kubectl -n hello-k8s-ai-system rollout restart deploy/hello-k8s-ai-prometheus`（Recreate 自动先删后建，不再需要手工 scale 0/1）。

## 回滚

- 撤销本次提交：`git revert <commit>` 即可还原所有脚本、Makefile 与 Prometheus 清单。
- 回滚 Prometheus 单独项：删除 cAdvisor job 与两条告警、RBAC 去掉 nodes/proxy、Deployment 策略改回 RollingUpdate（需注意：改回 RollingUpdate 后配置变更必须手工 scale 0 再 scale 1，否则复现 TSDB 锁冲突）。
- 体检拦截与启动流程无关时（如想跳过），可用 `PREFLIGHT_SKIP_WINDOWS=1` 只跳过 Windows 内存检查；不支持整体跳过（设计上强制）。

## 风险

- cAdvisor 经 API Server proxy 抓取 10 节点：每 10s 一轮，每个节点一个 HTTPS 请求，对 API Server 与 Prometheus 的负载增量可接受（实测无压力）；若未来节点数大幅增加，可降 `scrape_interval`。
- 告警阈值未调参，属于经验值；后续长跑若误报/漏报，按 RESILIENCE.md 告警矩阵调整。
