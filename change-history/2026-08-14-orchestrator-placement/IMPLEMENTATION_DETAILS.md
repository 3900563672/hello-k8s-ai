# 实现修改明细

## 1. `internal/controller/placement_plan.go`（新增）

用途：集中定义 Orchestrator 与 SimulatorInstance Controller 共享的内部放置契约。

主要内容：

- `nodePlacementPlan`：版本、主节点、逐节点副本列表。
- `decodeNodePlacementPlan`：解析 annotation，拒绝未知版本、空节点、重复节点、非正副本和无效主节点。
- `encodeNodePlacementPlan`：按节点名稳定排序后编码，避免 map 顺序造成无意义更新。
- `newNodePlacementPlan`：从旧 Pod 的实际 `nodeName` 分布生成迁移计划。
- `addNodePlacement` / `removeNodePlacement`：每次只增减一个节点副本。
- `scaleDownPlacementNode`：优先回收非主节点，保证结果确定。

新增内部元数据：

| Key | 位置 | 用途 |
| --- | --- | --- |
| `platform.study.com/node-placements` | SimulatorInstance annotation | Orchestrator 持久化逐节点副本计划。 |
| `platform.study.com/placement-node` | Deployment/Pod template annotation | 保存完整目标节点名，便于诊断。 |
| `platform.study.com/placement` | Deployment/Pod label | 区分同一 Instance 的节点 Deployment；非法 label 值自动哈希。 |

## 2. `internal/controller/orchestrator_data.go`

### `InstanceInfo`

新增：

- `PlacementPlan`：决策快照中的逐节点计划；
- `PlacementReady`：计划或旧 Pod 落点是否与当前副本数一致。

目的：评分和缩容不能再只知道“实例有几个副本”，还要知道这些副本当前属于哪些节点。

### `collectExistingInstances`

变化：

- 已有 annotation：解析并校验 placement 合计与 `spec.replicas` 一致；
- 旧实例且 replicas=0：视为可直接建立空计划；
- 旧实例且 replicas>0：扫描非终态、已调度 Simulator Pod，从 `spec.nodeName` 恢复分布；
- 观察 Pod 数与 replicas 不一致：将 `PlacementReady=false`，阻止新扩缩容。

这样避免升级时把未知落点错误写成一个猜测节点。

### `decisionTriggerID`

把 placement ready 状态、主节点和逐节点副本数加入 trigger 输入。放置发生变化后会生成新 trigger，不会把旧决策误认为已应用。

### `collectExpectedNodeUsage`

新增逻辑容量计算：

1. 根据已调度非终态 Pod 计算当前可观察使用量；
2. 对每个已持久化 plan，计算 `计划副本 - 已观察 Pod` 的正差额；
3. 用 Model 的 GPU/并发需求把差额换算成节点预留；
4. `collectAvailableNodes` 使用 `max(WorkerNode.status, 实际 Pod + 计划差额)` 作为已用量。

目的：placement 已批准但 Pod 尚未创建或 WorkerNode Status 尚未回写时，下一轮决策不能再次使用同一份容量。

## 3. `internal/controller/orchestrator_scoring.go`

`findBestPlacement` 仍保留原评分公式和排序，只增加一条安全门：已有副本但 placement 尚未恢复完整的实例不参与扩容。replicas=0 的新实例仍可正常首次扩容。

未修改：Model 分数、冷启动权重、GPU/并发硬门和 tie-break 顺序。

## 4. `internal/controller/orchestrator_decision.go`

### `Decision`

新增：

- `NodeName`：扩容目标节点、缩容回收节点或 Rebalance 目标节点；
- `SourceNodeName`：Rebalance 来源节点。

### `DecideAt`

- 扩容把 `placementCandidate.NodeName` 传入 Decision，不再丢失；
- 缩容从 placement 中选择明确节点；
- placement 不完整时不执行会破坏计划一致性的扩缩容；
- 性能扩缩容前先检查 Policy 导致的失效 placement。

### `placementRebalanceDecision`

新增不改变总副本数的放置迁移：

- 找到第一个不再合法的节点副本；
- 按剩余 GPU、剩余并发、节点名稳定选择合法目标；
- 每轮迁移一个副本；
- 没有容量时返回 `placement_rebalance_blocked`，不绕过 Policy。

## 5. `internal/controller/orchestrator_executor.go`

### `scalingPlan`

新增 `nodeName` 和可选 `sourceNodeName`，使 pending plan 包含完整执行意图。

### `applyDecision`

在写入前拒绝缺少目标节点的动作；Rebalance 还必须有来源节点。Trace 增加 placement 节点属性。

### `persistScalePlan`

同一次带 resourceVersion 的 SimulatorInstance `Update` 中完成：

- 校验租户和旧副本数；
- 读取现有 placement，或从旧 Pod 恢复 placement；
- ScaleUp：目标节点副本 +1；
- ScaleDown：回收节点副本 -1；
- Rebalance：来源节点 -1、目标节点 +1；
- 校验新 placement 合计等于新 replicas；
- 写 last trigger、pending plan、node placements 和 `spec.replicas`。

这保证不会出现“副本数已变但选点计划没写”或相反的半完成对象状态。

### `observeNodePlacementPlan`

只读取带平台 managed-by label、instance annotation 匹配、已调度且非终态的 Pod。迁移快照仍由调用方与当前 replicas 比对。

### `completeScalePlan`

ScaleUp/ScaleDown 保持原有状态记录和 cooldown 时间语义。Rebalance 不伪装成扩容或缩容，不写 `Orchestrator.status.lastScaling`，只完成并清理 pending plan。

## 6. `internal/controller/orchestrator_controller.go` 与 `constants.go`

- 增加内部动作 `Rebalance` 和指标标签 `rebalance`；
- Reconcile Span 记录来源节点与目标节点；
- CRD 中 ScalingRecord 的枚举没有增加 Rebalance，因为该动作不改变总副本数，也不应影响扩缩容 cooldown。

## 7. `internal/controller/simulatorinstance_placement.go`（新增）

### `reconcileDeploymentObjects`

两条路径：

| 对象状态 | 行为 |
| --- | --- |
| 没有 node-placements annotation | 保持旧单 Deployment，affinity 为全部 eligible nodes。 |
| 有有效 plan | 每个节点一个 Deployment，affinity 只包含该节点。 |

计划合计与 replicas 不一致或 annotation 无法解析时拒绝物化。节点被新 Policy 拒绝后，只允许已经存在且带同一 placement identity 的 Deployment 缩小副本；不会在拒绝节点新建 Deployment 或增加副本。

### Deployment 命名与清理

- 主节点：`deploymentName(instance)`，兼容原运维入口；
- 其他节点：`placementDeploymentName(instance, node)`，使用节点 SHA256 前 6 字节；
- placement 节点移除后，`deleteObsoletePlacementDeployments` 删除多余 Deployment；
- Instance 删除时，`deleteDeploymentObjects` 删除该 Instance 的全部 Deployment，而不只删除主 Deployment。

列表时同时检查 managed-by label、instance label 和完整 instance annotation，避免哈希 label 极端碰撞误删其他对象。

## 8. `internal/controller/simulatorinstance_controller.go`

### Reconcile 主流程

- 单 Deployment 调用替换为 `reconcileDeploymentObjects`；
- 状态输入从单个 Deployment 改为聚合后的 `simulatorDeploymentState`；
- 日志记录 Deployment 数量和总期望副本；
- 删除路径清理全部 Deployment。

### `ensurePlacementDeployment`

- 接收独立 Deployment 名、该节点副本数、目标节点列表和 placement 节点；
- 主 Deployment 保留旧 immutable selector；其他节点 Deployment 增加 placement selector；
- Pod template 写 placement identity；
- `setRequiredNodeAffinity` 对已迁移工作负载收到的列表只有一个节点。

### Status 与 TenantRuntime

- `availableReplicas` 汇总全部节点 Deployment；
- 任一 Deployment ReplicaFailure 会使实例失败；
- TenantRuntime 按每个 Instance 当前 plan 计算目标 Deployment 名称，并汇总这些 Deployment 的 available replicas；等待删除的旧 placement Deployment 不计入。

### Watch

SimulatorInstance 更新 predicate 增加 node-placements annotation 比较。Rebalance 只修改 annotation、不修改 replicas 时，Instance Controller 仍会立即收到事件。

## 9. 测试文件

| 文件 | 新增或修改内容 |
| --- | --- |
| `internal/controller/placement_plan_test.go` | 计划排序、编码解码、增减副本、缩容节点选择、非法 JSON/版本/重复节点校验。 |
| `internal/controller/refactor_test.go` | 扩容节点传播、缩容节点选择、缩到零节点、Policy Rebalance 纯函数决策。 |
| `internal/controller/controller_integration_test.go` | 逻辑预留、原子/幂等计划、旧 Pod 迁移、Rebalance、逐节点 Deployment、精确 affinity、TenantRuntime 聚合、旧路径兼容、错误计划拒绝和清理。 |
| `test/e2e/e2e_test.go` | 在 Kind 中写入单节点 plan，验证 Deployment required affinity 与 Pod 实际 `spec.nodeName` 都等于目标节点。 |

## 10. 文档文件

同步修改：

- `docs/AI_CONTEXT.md`
- `docs/kubernetes/CONTROLLER_ARCHITECTURE.md`
- `docs/kubernetes/FIELD_OWNERSHIP.md`
- `docs/kubernetes/RESOURCE_LIFECYCLE.md`
- `docs/data-flow/END_TO_END_DATA_FLOW.md`
- `docs/getting-started/VERIFICATION.md`

这些文档删除了“项目只提供 eligible nodes、Scheduler 自行选具体节点”的旧描述，改为当前真实权威边界。

## 11. 明确未修改

- 未修改 `api/v1/`，因此没有手改 CRD 或 DeepCopy 生成文件；
- 未修改 `config/crd/bases/` 和 `config/rbac/role.yaml`；
- 未修改评分公式、Traffic 算法、Simulator 引擎、Backend、Frontend、数据库表；
- 未引入新框架、数据库、Scheduler、device plugin 或自定义 Kubernetes 资源。
