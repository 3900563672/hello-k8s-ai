# 变更总览：#31 告警规则实测触发验证（含 LeaderMissing 规则修复）+ #32 扩容节奏参数化

> 日期：2026-08-18 ｜ 级别：P1 ｜ 对应 Issue：[#31 稳定性验收](https://github.com/3900563672/hello-k8s-ai/issues/31)、[#32 扩容节奏评估](https://github.com/3900563672/hello-k8s-ai/issues/32)

## 为什么做

- #31：8/18 凌晨的降级演练已修复两条告警表达式（issue #30），但"真实触发路径"缺最后一环：内存告警 `for: 10m` 完整触发周期、Simulator Leader 接管、LeaderMissing 告警真实触发均未实测。本次在 9:00-12:00 潮汐窗口内用真实集群完成全部验证，并发现 LeaderMissing 规则存在空向量缺陷。
- #32：模拟器无网关、`maxReplicas=0` 无限副本下，扩容节奏是"单副本逐次扩容"，高 QPS 突增时调度积压；需要可配置的批次步长并实测校准默认值。

## 改成什么

1. **扩容节奏参数化（#32）**：
   - 新增 `Orchestrator.spec.maxScaleUpBatch`（`api/v1/orchestrator_types.go`）：单次扩容决策最多补的副本数，validation Minimum=0，default=10；`scaleUpBatchLimit()` 配置值优先、0 使用默认 10。
   - 控制器：`DecisionInput.MaxScaleUpBatch` 接入；bootstrap 与队列缺口换算均按步长截断（`orchestrator_decision.go` / `orchestrator_data.go`）。
   - Backend 白名单：`commands.go` 的 `writableSpecFields["Orchestrator"]` 补 `maxScaleUpBatch`（此前会拦截该字段）。
   - 前端：表单/表格/预置模板（弹性 20 / 保守 2）/ schema / configApi / Guide 页"配置详解"全部同步。
   - 新增单测 `orchestrator_decision_unit_test.go`：步长上限与队列缺口截断。
2. **告警实测（#31）**：
   - 内存告警完整触发：`polinux/stress` 900M/1Gi 压力 Pod → pending 10 分钟 → firing → 删除 Pod → 自动恢复（cAdvisor 序列 5m stale 后 inactive）。
   - Leader 接管演练：删除 Leader Pod，15 秒内新 Leader 接管，`simulation_elapsed_seconds` 不回退（16800→18100s），快速接管不误报 LeaderMissing。
   - **LeaderMissing 规则缺陷修复**：原表达式 `sum by (simulator_instance) (hello_k8s_ai_simulator_leader) == 0` 在全部 Pod 消失时返回**空向量**（SD 移除后序列立即 stale），永远无法触发。改为 `(absent(sum by (simulator_instance)(leader)) == 1 and sum(container_memory_working_set_bytes{namespace="hello-k8s-ai-system",pod=~"simulator-.*"}) > 0) or (sum by (simulator_instance)(leader) == 0)`——absent 分支覆盖"全部消失"，容器指标门控覆盖 `for: 1m` 窗口且 clean 集群不误报；`==0` 分支保留"Pod 存活但无 Leader"场景。
   - 修复后实测：停 Controller + 双 Deployment 缩到 0 → `sum==0` → pending（01:51:56Z）→ firing（01:52:56Z，`for:1m`）→ 恢复后 resolved（01:54:28Z）。
3. **文档同步**：13 个 MAP 映射文档更新（CRD 设计 / 字段所有权 / 配置参考 / 架构 / 生命周期 / 可观测性等），journal 沉淀演练坑。

## 关键行为

- `maxScaleUpBatch`：0=默认 10；每轮扩容决策的上限，配合 `scaleUpCooldownSeconds` 形成批次节奏；弹性预置 20、保守预置 2。
- LeaderMissing 语义：模拟器实例要么有 Leader（sum=1）要么触发告警；clean 集群（从未部署模拟器）不触发；实例暂停（replicas=0）时因容器指标仍残留 5 分钟会短暂触发，属预期边界。
- 演练方法（可复用）：临时停 `hello-k8s-ai-controller-manager` → 两个 `simulator-*` Deployment（含 node 放置实例，名称带哈希需动态发现）缩到 0 → 观测告警 → 恢复。**副作用**：手工改 `spec.replicas` 会破坏 placement plan 注解不变量（orchestrator 原子写 replicas+plan），恢复方法是对齐 `spec.replicas` 到已持久化 plan 的副本数。

## 验证

- Go：`make fmt` / `make vet` / `make test` 全绿（含新增单测）；`make test-e2e-compile`、`make verify-deploy`、`make selfcheck`、`make test-frontend` 全绿。
- `make test-backend`：除 `TestGrafanaProxyPreservesSubPathAndForwards` / `TestGrafanaProxyRootPath` 外全绿；该两测试本地因 WSL `loopback0` 路由（table 127 把 127.0.0.1 指向 169.254.73.152 转发层）导致同进程回环连接被拒，CI 同版本通过（见 journal 2026-08-18-wsl-loopback-route）。
- 实测时间线（UTC，Prometheus rule eval 30s）：
  - 内存告警：activeAt 01:06:49Z → firing 01:17:48Z（10m 周期，01:16:48 差 1.6s 未满）→ Pod 删除 01:18:10Z → resolved ≤01:22:39Z。
  - Leader 接管：删除 Leader 01:19:22Z → 新 Leader 01:19:37Z（15s）→ elapsed 16800→18100s 不回退 → 无误报。
  - LeaderMissing（修复后）：pods=0 01:51:10Z → pending 01:51:56Z → firing 01:52:56Z → resolved 01:54:28Z。
- 未验证：CI 三 job（代码检查 / 部署验证 / E2E）提交后跑；本地 E2E 未跑（按规范用 CI 独立 Kind 集群）。

## 回滚

- `maxScaleUpBatch`：revert `api/v1/orchestrator_types.go` 与控制器/前端改动后需重新 `make manifests generate YEAR=2026`；存量 CR 带该字段时 CRD 移除字段会校验失败，先清字段。
- LeaderMissing 规则：`git checkout HEAD~1 -- config/observability/prometheus.yaml` 后 `kubectl apply -f ... -n hello-k8s-ai-system` + `curl -X POST /-/reload`（configmap 挂载 fsnotify 不触发，需 lifecycle reload）。
- 演练残留：若实例处于 plan 不变量错误循环，把 `spec.replicas` 对齐到 `platform.study.com/node-placements` 注解的副本总数即可自愈。
