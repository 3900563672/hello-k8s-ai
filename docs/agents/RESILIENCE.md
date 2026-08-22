# 稳定性与优雅降级矩阵（RESILIENCE）

> 维护层：agent | last-reviewed：2026-08-18 | 事实源：源码与 docs/agents/
> 目的：组件挂掉后系统"应该怎样表现"的对照表；长时运行前按此矩阵做验收（验收暂未执行）。

## 1. 总原则

- Kubernetes API Server 是当前状态的唯一事实源；它不可用时整个控制面、Simulator 与 Backend 都不可用，这不是降级场景，而是停机场景。
- 遥测（Prometheus / Grafana / Jaeger / OTel Collector）只消费，不驱动：遥测失败不能阻止控制面或 Simulator 启动（`AGENTS.md` 边界，simulator 代码 `SetupTracing` 失败回落空实现）。
- PostgreSQL 只保存历史快照、资源事件、审计和幂等记录，不能反向驱动 Controller；PG 故障只影响历史与审计类能力，不影响扩缩容与流量。
- Frontend 只调用 Dashboard Backend，不直连 K8s / 遥测 / PG；Backend 故障只影响前端展示，控制面继续按最后状态运行。

## 2. 优雅降级矩阵

| 组件 | 故障表现（已由代码路径佐证） | 对控制面影响 | 恢复行为 |
| --- | --- | --- | --- |
| Kubernetes API Server | 所有读写在超时/重试后失败 | 全部停机 | 恢复后各 Controller 自动续跑；扩缩容计划有幂等注解，不会重复扩缩 |
| PostgreSQL | Backend 历史/快照/审计接口失败；启动时 `Migrate` 失败会阻止 Backend 启动（未验证 PG 运行中故障对已启动 Backend 的影响） | 无（不驱动 Controller） | 恢复后历史数据按幂等记录补写 |
| Prometheus | 面板无数据；Grafana 依赖它 | 无（Controller 指标写入是 push 式内存注册表，Prometheus 只是抓取方） | 恢复后自动恢复抓取 |
| Grafana | 面板不可用 | 无 | 恢复即可 |
| Jaeger / OTel Collector | Trace 丢失；SDK 导出失败不阻塞业务（simulator 显式回落空 tracer） | 无 | 恢复后新 Trace 继续上报 |
| Performance Collector（Controller） | TenantPerformance 停更 → 指标过期 → Orchestrator `HasTTFT/HasQueue=false` → 暂停扩缩容（Ready=False / MetricsNotReady） | 扩缩容暂停，流量与模拟器继续 | 指标恢复后自动继续 |
| Orchestrator（Controller） | 扩缩容暂停；流量、模拟器、性能采集继续 | 无 | 恢复后按最新指标决策 |
| SimulatorInstance（Controller） | 副本物化/回收暂停；已有副本继续运行 | 无 | 恢复后按 spec 收敛 |
| Traffic（Controller） | QPS 保持最后一次写入值，不回落 | 无 | 恢复后继续按模板调整 |
| SimulationClock（Controller） | timeScale 保持最后值 | 无 | 恢复后继续 |
| Simulator Leader | 租约 15s/10s/2s；Leader 只运行引擎与状态上报，Follower 空闲 | 状态与性能指标停更 ≤15s；性能过期后 Orchestrator 暂停扩缩 | 新 Leader 接管，冷启动进度从 Status 恢复（`SimulationElapsedMs`），不归零 |
| Simulator 全部副本 | 无引擎处理流量 → 性能指标无来源 → MetricsNotReady | 扩缩容暂停 | 副本拉起后恢复 |

### 2.1 告警矩阵（2026-08-17 新增，含 cAdvisor 抓取）

| 告警 | 触发条件 | 含义 | 处置 |
| --- | --- | --- | --- |
| HelloK8sAITargetDown | `up{job!="prometheus"} == 0` 持续 2 分钟 | 某抓取目标不可达 | 看对应组件 Pod 状态与日志 |
| HelloK8sAIControllerErrorRatioHigh | Reconcile 错误比例 >5% 持续 5 分钟 | 控制器持续报错 | 查 controller 日志与 Trace |
| HelloK8sAITraceExportFailure | OTel Collector 导出 span 失败 | Trace 链路中断 | 查 collector 与 Jaeger |
| HelloK8sAISimulatorLeaderMissing | 实例池无 Leader 持续 1 分钟 | 状态/性能指标停更，Orchestrator 将暂停扩缩 | 查模拟器租约与 Pod |
| HelloK8sAIDashboardEventsDropped | 历史事件丢弃/写失败持续 5 分钟 | 时间线将出现缺口 | 查 backend 缓冲与 PG |
| HelloK8sAIContainerMemoryHigh | 容器内存 >85% limit 持续 10 分钟（仅统计有 limit 的容器，无 limit 不参与） | 内存逼近上限，可能 OOMKilled | 按 3.5 预算规则清理负载或调 limit |
| HelloK8sAIContainerRestarted | 组件容器 `container_start_time_seconds` 与 10 分钟前差值 >60s（按 namespace+container 聚合，排除 simulator） | 稳定组件容器重启 | 查 `kubectl describe pod` 的 OOMKilled / Last State；模拟器按实例扩缩容不触发 |

抓取侧：Prometheus 现经 API Server proxy 抓 cAdvisor（`/api/v1/nodes/${1}/proxy/metrics/cadvisor`，RBAC `nodes/proxy`），容器内存与重启指标由此而来；kube-state-metrics 未部署，重启检测用 start_time 突变近似。

## 3. 容量校准公式（设计剧本前必须先算，2026-08-17 实测确立）

- 模拟器是单 Leader 引擎：总 QPS 均摊到 `availableReplicas`，引擎并发槽 = 模型 `maxConcurrency`。所以"副本数"= 虚拟容量，节点/模型配置决定天花板，不是真实机器。
- **单副本容量（qps/副本）≈ `maxConcurrency ÷ 平均服务时长`**。
- **平均服务时长（ms）= `prefillBaseMs + prefillPerTokenUs×0.5 + decodePerTokenMs×200`**（prompt 500 token / output 200 token 固定）。
- model-lite 实例：50 + 500×0.5 + 20×200 = 4300ms → 单副本 ≈ 3.7 qps。
- **TTFT 特性（关键）**：TTFT 只在"排队"时上升（每副本负载 ρ→1 后），低负载时恒等于服务基线（model-lite ≈ 320ms）。`TTFT=320ms 不代表没负载`，判断是否触发扩容以 `queue` 为主、TTFT 为辅。
- 所需副本 ≈ `QPS × 平均服务时长 / maxConcurrency`；单节点可承载副本 = `min(⌊gpu/gpuUnits⌋, ⌊maxConcurrency/模型maxConcurrency⌋)`。
- `maxReplicas=0`（无限制）+ 节点容量调大 = 副本理论上无限；真实上限只剩 Docker Desktop 宿主资源。
- 批量扩容：单次决策按队列缺口最多补 10 副本，冷却（默认 60s）作为批次间隔；扩容总速度 = 每批 1..10 × 冷却节奏。

### 剧本设计规则（长跑/压测前必读）

1. 先算单副本容量，再定 QPS：`峰值 QPS ÷ 当前副本数 > 单副本容量` 才会产生队列并触发扩容；否则整个剧本是无效负载。
2. 实测校准（2026-08-17，rate=20，model-lite）：
   - 400 QPS @ 20 副本（20/副本 = 5.4×容量）→ 队列 2 分钟冲到 7 万、TTFT 小时级。
   - 300 QPS @ 141 副本（2.1/副本 = 0.57×容量）→ 队列 0、TTFT 320ms（稳定基线）。
   - 650 QPS @ 141 副本（4.6/副本 = 1.25×容量）→ 触发队列与批量扩容 141→200（10 批、60s/批），queue 峰值 ~2491、TTFT 峰值 ~678s，14:24 到顶后 ~6 分钟排空（2026-08-17 实测）。
3. 剧本示例（4 小时）：基线 300（稳定）+ 峰值 650（触发扩容至 ~176 副本，实测撞 200 容量顶），周期 60min（45 基线 + 15 峰值）。
4. 缩容滞回（观察，未改策略）：model-lite TTFT 基线 320ms > 缩容下阈值 300ms，队列排空后 `needDown` 被 TTFT 挡住 → 峰值副本数保持不回落。长跑结束后副本保持峰值规模属预期，不是故障。
5. 延迟舒适区：吞吐 break-even ≠ 延迟目标。650 QPS @ 200 副本（ρ≈0.88）queue 保持 0-20 但 TTFT 升到 ~1s（偶发 3-4.5s），而 300 QPS @ 141 副本（ρ≈0.57）TTFT=320ms。要维持基线延迟，峰值副本按 `QPS × 服务时长 ÷ (maxConcurrency × 0.6)` 预留余量（650 QPS ≈ 350 副本），否则需接受延迟升高。

## 3.5 宿主内存预算与治理（31.4GB 机器，2026-08-17 实测）

- **总预算**：物理 31.4GB。WSL2 VM 上限 16GB（`.wslconfig`，2026-08-22 起；此前 12GB 时 `autoMemoryReclaim` 误放 `[wsl2]` 段失效、顶格概率挂死，见 issue #181），Windows 侧进程约 17-20GB。
- **VM 内固定开销（实测）**：Docker Desktop 内置 K8s 每节点容器（kubelet/containerd/kindnet）约 0.8-1.2GB，`KubernetesNodesCount=10` 时 **10 节点 ≈ 8-10GB**，加上可观测组件（Prometheus/Jaeger/Collector/Grafana/Backend/PG ≈ 2-3GB），VM 有 16GB 上限（医生脚本按 `.wslconfig` 动态告警），仍需给模拟器负载留空间。
- **结论**：日常开发必须缩减节点数（10→4~5，省 4-6GB）后才谈得上跑负载；缩减前先备份 CRD/CR/PVC（改节点数可能重置内置 K8s，见 Issue #29 待办）。
- **负载预算公式**：`可跑模拟负载 = 16GB - 节点开销(节点数×~1GB) - 可观测 2.5GB - 系统余量 1GB`。例：5 节点 → 16 - 5 - 2.5 - 1 ≈ 7.5GB 余量，约等于 30-50 个模拟器 Pod（每个 ~50-80MB）；10 节点 → 余量 < 0，必爆。
- **长跑后强制清理（硬步骤）**：长时运行/大负载测试结束后必须：① `make cluster-down`；② 删除长跑 `TenantModelPolicy`（自动删除 SimulatorInstance 与模拟器 Deployment；`replicas=0` 不是停止态，Orchestrator 会按流量扩容，见 docs/lessons/deploy-cluster-down-revive.md）；③ 确认 `kubectl get pods -n hello-k8s-ai-system` 只剩系统组件（≤8 个）；④ Windows 侧确认空闲内存 ≥ 5GB。
- **内存告警阈值**：`hack/doctor.sh` 按 `.wslconfig` 上限动态告警（当前 16GB → `vmmemWSL > 15.5GB` WARN）；Windows 空闲内存 < 3GB WARN / < 1GB FAIL。
## 4. 长时运行验收清单（2026-08-17 14:00-18:00 已完成）

> 状态：已完成（13:29-18:14 实跑，产物 `.runtime/longrun/2026-08-17/`，summary 已按 20 轮口径重生成）。执行规范先读 `hack/night-run/README.md` 与 `docs/agents/WORKFLOW.md` 4.2 节。

- [x] 4 小时连续运行无 CrashLoop、无 keepalive/snapshot 失败（20 轮 0 失败）。
- [x] 基线 300 QPS 下 queue≈0、TTFT ~320ms、副本稳定（141）。
- [x] 峰值 650 QPS 触发队列与批量扩容：141→200 共 10 批（60s/批，+10×4/+5×2/+2×4 按队列缺口自适应），撞节点容量顶（200）。
- [x] 峰值结束后队列排空（到顶后 ~6 分钟）、TTFT 回落 320ms；扩容节奏在 PG `resource_events` 5s 序列可见（rounds/ 快照粒度太粗，见 docs/journal/2026-08-17-longrun-tools.md）。
- [x] 18:14 脚本恢复 35qps（旧版 --until 缺陷多跑一轮，已修复为整点停止）；副本保持 200 属预期（缩容滞回，见 3.4）。
- [ ] 降级演练（可选，单组件逐项）：停 Prometheus / Grafana / Jaeger / PG 后系统继续扩缩；恢复后无数据损坏。
- [ ] Simulator Leader 手动删除后 ≤30s 新 Leader 接管，性能指标继续，`SimulationElapsedMs` 不回退。
- [ ] errorRate=0 复核：PromQL 空集问题已修复（controller/simulator errorRate 现在返回 0），下次长跑确认全程为 0。

## 5. 已知风险（2026-08-17 压测实测）

- 400 QPS 在 20 副本（容量上限）下队列 2 分钟冲到 7 万、TTFT 到小时级：不是调度 bug，是"目标容量远小于负载需求"的数学结果；扩容被节点并发上限拦住时 Orchestrator 正常返回 `no_feasible_placement`（Ready=True）。
- 缩容仍是逐副本 + 冷却（120s），大规模缩容较慢；4 小时测试后如需快速回收，可临时把副本缩到 floor 或重启实例。
- 队列清空后 TTFT 平均值会携带峰值尾巴（已完成请求的等待时间），看面板时以 queue 回落为准。
- 200 副本天花板（2×1600÷16）下 650 QPS 的 TTFT 约 1s（ρ≈0.88）：吞吐可扛但延迟有余压；要维持基线延迟需扩节点容量或按「延迟舒适区」公式预留副本（见 3.5）。
- controller/simulator errorRate 在修复前因 PromQL 空集恒为 null（零错误反而无数据），2026-08-17 已加 `or on() vector(0)` 保护（见 change-history/2026-08-17-longrun-tooling-fixes/）。
