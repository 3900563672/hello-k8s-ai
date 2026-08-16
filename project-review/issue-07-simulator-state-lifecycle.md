# 1. 问题标题

Simulator 状态生命周期与 reporter Pod 生命周期错误耦合

> 处理状态（2026-08-16）：已修复（冷启动部分）。累计模拟时间持久化为 `SimulatorInstance.status.simulationElapsedMs`，Leader 切换后沿用，不再随 reporter 重启重置冷启动曲线；队列/随机序列随 Leader 重建仍为已知限制。原审查内容保留用于说明问题背景；实施记录见 `change-history/2026-08-16-simulator-coldstart-persistence/`。

## 2. 当前状态描述

每个 SimulatorInstance 对应一个 Deployment，可以有多个 Pod。`simulator/main.go` 使用 Kubernetes Lease 做 Leader election，确保同一实例只有一个 reporter 写 `SimulatorInstance.status`。这一机制避免了多个 Pod 同时覆盖状态，是当前设计中合理的并发保护。

Leader 获得租约时，代码创建新的 `Simulator`，并把 `startTime` 设置为当前 Pod 成为 Leader 的时间。`simulator/simulator.go` 使用这个时间和 Model 的 `coldStartMs` 计算整个实例池的冷启动因子，再乘以 `status.availableReplicas` 得到池总分。

这意味着冷启动时钟属于 reporter，而不是实际工作负载副本：

- Leader Pod 重启或租约切换时，即使所有业务副本已经运行很久，整个实例池也会重新进入冷启动。
- Deployment 从一个副本扩到多个副本时，新的 Pod 没有独立启动时间；如果 reporter 已经预热，新副本会立即被当作完全预热。
- 模拟引擎始终代表一个典型副本，server 数量取 Model 的 `maxConcurrency`；多副本只通过均摊 QPS 和 pool score 体现，无法表示不同副本各自的年龄与队列。

`SimEngine` 保存在 Leader 进程内存中，包含队列、虚拟队列、随机数状态和模拟时间。Leader 切换会创建新引擎，未完成队列直接丢失。反过来，同一 Leader 下新增或减少 Pod 时，队列状态不会按副本生命周期重新分片。

Simulator 最终把 Score、Performance、ObservedAt 和 ReporterID 写回 CR，PerformanceCollector 和 Orchestrator 再用这些数据做扩缩容判断，因此模拟状态会直接反馈到控制回路。

## 3. 问题定位

Leader election 解决的是“谁负责上报”，不应该定义“被模拟系统何时启动”。当前把 reporter 生命周期当作服务副本池生命周期，会让高可用行为本身改变业务测量结果。

在扩容场景中，系统最关注的正是新副本冷启动带来的容量变化，但当前只有一个全局冷启动因子，无法表示已预热副本与新副本并存。扩容效果可能被高估；Leader 切换时又可能被低估。

队列和随机状态不持久化可以是本地模拟器的合理取舍，但必须明确恢复语义。当前恢复被表现为正常新一轮指标，没有 epoch/reset 标记，Orchestrator 无法区分真实负载改善与模拟器状态丢失。

## 4. 影响范围

- Simulator：冷启动、队列、TTFT 和随机过程都受 reporter 变更影响。
- SimulatorInstance：Status 看起来连续，内部模拟状态实际可能已重置。
- Orchestrator：可能因虚假 TTFT/Queue 变化触发扩缩容，形成反馈抖动。
- Prometheus/Jaeger：能看到 leadership change，但指标序列没有统一的模拟 epoch 供查询端关联。
- Frontend：历史曲线可能把 Leader 变更造成的跳变解释为真实性能变化。
- 测试：冷启动纯函数和引擎均有单元测试，但没有多副本扩容、Leader 切换和状态连续性的集成测试。

现有部署日志显示 reporter 能正常写回状态；问题影响的是模拟结果语义，不是进程能否启动。

## 5. 根本原因分析

当前实现用一个 Leader 进程模拟整个 Deployment 池，简化了多 Pod 协调和重复写 Status 的问题。这个简化同时把“上报协调者”“模拟状态所有者”和“副本池生命周期”合并成同一个进程对象。

随着系统加入多副本、扩缩容和 Leader election，原有单实例内存模型没有同步扩展出副本级生命周期或显式恢复语义，于是基础设施事件会污染业务信号。

## 6. 修改方向建议

- 把 reporter 身份与模拟工作负载状态分离：Leader 可以变化，但冷启动基准和模拟 epoch 应由稳定的实例/副本生命周期决定。
- 明确模拟粒度。若继续由一个 Leader 模拟整个池，需要维护副本年龄分布和容量变化；若按 Pod 模拟，则需要定义聚合和唯一写入方式。
- 为 Leader 切换和引擎重建增加明确的 reset/epoch 信息，让下游能忽略或标记不连续样本。
- 规定扩容、缩容、QPS 归零、Model 性能参数变化时，队列和随机状态分别保留、迁移还是重置。
- 增加可重复随机种子或场景标识，便于测试与历史结果复现，同时保留生产演示中的随机性选项。
- 优先修正状态语义，不需要替换现有离散事件引擎或 Lease 机制。

## 7. 优先级

优先级：P1

建议在用模拟结果评价扩缩容策略、比较模型性能或做稳定性演示前处理。
