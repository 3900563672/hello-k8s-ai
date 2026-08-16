# 1. 问题标题

Controller 事件映射存在集群级放大效应

## 2. 当前状态描述

`internal/controller/workernodeusage_controller.go` 监听全部 Pod 和 Model。任一相关 Pod 的 NodeName、Phase、模型身份或删除状态变化，`allWorkerNodeRequests` 都会列出全部 WorkerNode，并为每个节点创建 Reconcile 请求。任一 Model generation 变化也会触发相同全量入队。

每个 WorkerNode Reconcile 又会调用 `calculateNodeUsage`，分别列出全部 Model 和全部 Pod，然后只统计目标节点。假设 W 个 WorkerNode、P 个 Pod、M 个 Model，一次 Pod 事件最坏会触发约 W 次扫描，每次处理 P+M 个对象，形成约 O(W×(P+M)) 的缓存遍历和状态写入尝试。

其他 Controller 也有相似但较轻的全量映射：TenantModelPolicy 在映射 Tenant、Model 和实例事件时扫描全部策略；Orchestrator 的 Model/ModelNodePolicy 事件扫描全部 TenantModelPolicy，WorkerNode 事件扫描全部 TenantNodePolicy，再为租户查 Orchestrator。

Orchestrator 自身每 10 秒周期 Reconcile，以弥补指标过期等不会产生 Kubernetes 事件的变化；其 Controller 设置 `MaxConcurrentReconciles: 1`。租户数量增加后，周期任务、事件映射和串行决策会共享一个处理通道。

当前 Backend informer 也缓存多类 cluster-scoped 和 Kubernetes 原生资源，但本问题主要发生在 Controller 的事件到 Reconcile 放大，而不是缓存技术本身。

## 3. 问题定位

事件驱动 Controller 的主要扩展优势是只重算受影响对象。当前实现为了保持正确性，使用“事件触发全部对象、每个对象再扫描全部依赖”的保守策略。小集群中逻辑直观且能运行，规模增长后会导致重复 CPU、缓存遍历和 Status Patch 冲突。

Pod 启动通常会产生多次 Phase 和调度状态变化；一次扩容可能连锁触发 WorkerNode Usage 更新、Orchestrator 决策和更多 Pod 变化，形成自激式事件风暴。单并发 Orchestrator 还可能让一个慢租户阻塞其他租户。

Controller Runtime Cache 的 List 通常不直接访问 API Server，但大量全量遍历和后续 Status 写入仍会占用进程 CPU、内存和 API Server 写配额。当前没有规模测试或队列延迟 SLO 来确定安全上限。

## 4. 影响范围

- WorkerNodeUsage：最明确的 W 倍全量扫描热点。
- Orchestrator：策略扫描、10 秒周期和单并发可能增加决策延迟。
- TenantModelPolicy：关系映射未充分利用字段索引，策略数量增长后重复遍历。
- Kubernetes API：无变化 Patch 已做部分抑制，但真实状态变化仍可能形成写突发。
- Prometheus：已有 Reconcile 指标可以观测次数和时延，但没有容量基线或告警阈值。
- 测试：当前以功能正确性为主，没有 100/1000 级对象规模、事件风暴和队列积压测试。

在用户给出的 10 节点本地集群和单样例资源下，归档日志显示部署成功；这是规模风险，不代表当前已有性能故障。

## 5. 根本原因分析

根本原因是关系查询策略不一致。项目已经为部分 SimulatorInstance 和 Orchestrator 关系建立字段索引，但其他 mapper 仍采用全表扫描；WorkerNode 使用量则按“节点”作为 Reconcile 粒度，却没有建立 Pod 到节点的定向索引或增量核算。

10 秒周期和单并发是为了优先保证控制循环稳定、避免并发覆盖，这一初始选择合理；问题在于没有随对象规模定义容量边界和逐步退化策略。

## 6. 修改方向建议

- 先用现有 Controller 指标建立基线，测量对象数量、事件频率、队列深度、Reconcile P95 和 API 写入量，避免无依据优化。
- 对能够从事件对象直接确定影响范围的关系使用定向索引；Pod 变更应至少只重算旧节点和新节点，而不是全部 WorkerNode。
- 将全量扫描留作低频一致性校准，而不是每个事件的主路径。
- 评估 Orchestrator 并发提升前，先确认同租户串行和跨租户并行的隔离边界，避免简单提高并发造成写冲突。
- 为 Controller 规定支持的租户、节点、模型和 Pod 数量，并建立规模回归测试。
- 保留现有 Controller 划分和最终一致性模型，不需要为此引入新的框架。

## 7. 优先级

优先级：P1

当前小规模演示可继续使用；在扩大节点、租户或 Simulator Pod 数量前应处理并量化容量上限。
