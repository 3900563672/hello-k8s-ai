# 1. 问题标题

Orchestrator 选点结果没有约束实际 Pod 调度

## 2. 当前状态描述

Orchestrator 在 `internal/controller/orchestrator_scoring.go` 中以“实例、节点”为组合计算扩容候选。`placementCandidate` 明确包含 `NodeName`，算法会检查目标 WorkerNode 的剩余 GPU、剩余并发和节点策略，并把节点名用于稳定排序。这说明当前算法不是只选择模型或实例，而是已经做出了具体选点判断。

选点结果进入 `internal/controller/orchestrator_decision.go` 后，公开的 `Decision` 只保留实例名、目标副本数和有效分数，没有保存节点名。随后 `internal/controller/orchestrator_executor.go` 中的 `scalingPlan` 也没有节点字段，执行阶段只修改 `SimulatorInstance.spec.replicas` 和相关状态/注解。

`internal/controller/simulatorinstance_controller.go` 最终创建 Deployment 时，会重新根据策略计算全部 eligible nodes，并把这些节点整体写入 required node affinity。它没有获得 Orchestrator 选中的具体节点，也没有为逻辑 GPU、逻辑并发写入 Kubernetes 可调度资源请求。实际 Pod 最终落在哪台 eligible node，由 Kubernetes Scheduler 独立决定。

Pod 调度完成后，`internal/controller/workernodeusage_controller.go` 才扫描 Pod 实际位置，并把逻辑 GPU 和并发占用回写到 `WorkerNode.status`。因此当前流向是：Orchestrator 基于旧快照选点，执行时丢失选点，Kubernetes 另行调度，最后再被动观察实际落点。

现有本地部署日志证明 Simulator Deployment 可以成功运行，但日志只验证“能调度并上报”，没有验证 Pod 是否落在 Orchestrator 计算出的节点上。

## 3. 问题定位

这是调度决策与执行结果之间的契约断裂。Orchestrator 使用节点剩余容量判断“该次扩容可行”，但真正执行的动作只是增加一个不绑定该节点的副本。若存在多个 eligible nodes，Kubernetes 可能把 Pod 放到另一台逻辑容量不足的节点。

由于逻辑 GPU 和并发不是 Kubernetes 原生资源请求，Scheduler 无法替 Orchestrator执行这层容量约束。并发扩容、缓存延迟或多个租户同时决策时，还可能基于同一份剩余容量重复批准扩容。事后回写使用量只能暴露偏差，不能阻止错误落点或超配。

该问题在单模型、单副本、节点资源宽裕的演示环境里不容易出现；节点策略增多、容量接近上限或多个 Orchestrator 同时扩容后，风险会明显上升。

## 4. 影响范围

- Controller：Orchestrator 的候选计算结果不能被 SimulatorInstance Controller 完整执行。
- CRD：当前没有持久化“本次扩容应落到哪个节点”或“已预留多少资源”的契约位置。
- Kubernetes：Scheduler 只看到 eligible node 集合，看不到业务侧 GPU/并发限制。
- Simulator：实例副本可能运行在与评分假设不同的节点上。
- Backend/Frontend：展示的是最终 Pod 落点和回写占用，无法说明决策是否按计划执行。
- 测试：现有纯函数评分测试能证明“选对候选”，但不能证明“Pod 最终落在该候选节点”。

当前已确认的是信息丢失和约束缺位；归档中的成功日志没有记录到明确超配事故，因此不能断言事故已经发生。

## 5. 根本原因分析

根本原因是系统同时存在两个调度者，却没有定义二者的交接协议：业务 Orchestrator 负责逻辑容量与评分，Kubernetes Scheduler 负责 Pod 放置。当前实现只把“扩几个副本”交给后者，没有把“为什么能扩、应在哪里扩”作为可恢复的执行意图传递下去。

从代码演进形态看，评分、执行、Deployment 管理和事后核算分别完成了各自职责，但它们之间共享的领域对象只覆盖副本数，没有覆盖 placement。问题不是单个函数复杂，而是跨 Controller 的状态契约不完整。

## 6. 修改方向建议

- 先明确逻辑调度与 Kubernetes 调度的权威边界：由 Orchestrator 指定精确节点，或把逻辑 GPU/并发建模为 Kubernetes 能执行的调度约束；不能继续由两个调度者各自决定。
- 让选点意图成为可持久化、可重试、可审计的信息，并从评分阶段一直传递到 Deployment/Pod 物化阶段。
- 引入资源预留与实际占用的差异检测，明确并发决策时的冲突处理和过期计划处理。
- 为“多 eligible nodes、并发扩容、目标节点容量不足、Pod 被重新调度”等场景增加跨 Controller 收敛测试。
- 保留现有评分算法和 Controller 分工，优先修补决策交接协议，不需要重写整体架构。

## 7. 优先级

优先级：P0

必须在把系统用于真实容量调度或多租户并发扩缩容前处理。它直接影响平台声称的核心调度正确性。
