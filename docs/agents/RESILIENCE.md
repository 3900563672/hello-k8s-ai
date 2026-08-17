# 稳定性与优雅降级矩阵（RESILIENCE）

> 维护层：agents ｜ 最后同步：2026-08-17 ｜ 对应变更：change-history/2026-08-17-scaleup-acceleration/
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

## 3. 容量与"无限流量"事实（写前端指南的底稿）

- 模拟器是单 Leader 引擎：总 QPS 均摊到 `availableReplicas`，引擎并发槽 = 模型 `maxConcurrency`。所以"副本数"= 虚拟容量，节点/模型配置决定天花板，不是真实机器。
- 单副本吞吐 ≈ `maxConcurrency / 平均服务时长`；model-lite（prefill 50ms + 500token×500us + 200token×20ms ≈ 4.3s）单副本 ≈ 3.7 qps。
- 所需副本 ≈ `QPS × 平均服务时长 / maxConcurrency`：400 QPS × 4.3s / 16 ≈ 108 副本。
- 单节点可承载副本 = `min(⌊gpu/gpuUnits⌋, ⌊maxConcurrency/模型maxConcurrency⌋)`；当前 2 节点 × 160 并发 ÷ 16 = 20 副本，是 2026-08-17 实测扩容的停止点（`no_feasible_placement`，非错误）。
- `maxReplicas=0`（无限制）+ 节点容量调大 = 副本理论上无限；真实上限只剩 Docker Desktop 宿主资源。
- 批量扩容：单次决策按队列缺口最多补 10 副本，冷却（默认 60s）作为批次间隔；扩容总速度 = 每批 1..10 × 冷却节奏。

## 4. 长时运行验收清单（暂未执行，等有时间跑）

> 状态：未执行。跑前先读 `hack/night-run/README.md` 与 `docs/agents/WORKFLOW.md` 4.2 节。

- [ ] 4 小时连续运行无 CrashLoop、无 Reconcile 错误率上升（Grafana Reconcile 错误比例面板为 0）。
- [ ] 阶梯流量（如 50 → 100 → 200 → 400 QPS）下 queue 与 TTFT 收敛到阈值内，副本随批次扩容到容量上限。
- [ ] 队列缺口大时单次扩容 ≥ 2 副本（批量生效）；冷却间隔内每轮一批。
- [ ] 降级演练（可选，单组件逐项）：停 Prometheus / Grafana / Jaeger / PG 后系统继续扩缩；恢复后无数据损坏。
- [ ] Simulator Leader 手动删除后 ≤30s 新 Leader 接管，性能指标继续，`SimulationElapsedMs` 不回退。
- [ ] 流量结束后副本按缩容冷却逐级回收，最终回到 minReplicas/floor。
- [ ] 快照目录 `.runtime/night-run/YYYY-MM-DD/snapshots/` 每轮齐全，无端口冲突假阳性（走 18080）。

## 5. 已知风险（2026-08-17 压测实测）

- 400 QPS 在 20 副本（容量上限）下队列 2 分钟冲到 7 万、TTFT 到小时级：不是调度 bug，是"目标容量远小于负载需求"的数学结果；扩容被节点并发上限拦住时 Orchestrator 正常返回 `no_feasible_placement`（Ready=True）。
- 缩容仍是逐副本 + 冷却（120s），大规模缩容较慢；4 小时测试后如需快速回收，可临时把副本缩到 floor 或重启实例。
- 队列清空后 TTFT 平均值会携带峰值尾巴（已完成请求的等待时间），看面板时以 queue 回落为准。
