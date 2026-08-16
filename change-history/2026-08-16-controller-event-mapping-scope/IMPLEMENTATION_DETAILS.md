# 实现修改明细

## 1. 改动前状态

- `allWorkerNodeRequests` 对每个 Pod / Model 事件 List 全部 WorkerNode 并入队全部节点。
- `calculateNodeUsage` List 集群全部 Pod 后在内存过滤目标节点。

## 2. 修改

- `internal/controller/workernodeusage_controller.go`：
  - 新增 `podNodeNameIndex = "spec.nodeName"`。
  - `allWorkerNodeRequests`：Pod 且 `Spec.NodeName != ""` → 只返回该节点请求；否则广播全部节点。
  - `calculateNodeUsage`：`r.List(ctx, &pods, client.MatchingFields{podNodeNameIndex: nodeName})`，并移除内层节点名过滤（索引已保证）。
  - `SetupWithManager`：通过 `registerFieldIndexes` 注册 Pod 的 NodeName 索引（空 NodeName 不索引）。
- 测试：
  - 新增 `workernodeusage_controller_test.go`：已调度 Pod 精确入队 / 未调度 Pod 与 Model 广播 / 按节点统计与缺失节点返回 0。
  - `controller_integration_test.go`：WorkerNodeUsage 集成测试的 fake client 补 `WithIndex`。

## 3. 未做

- TenantModelPolicy、Orchestrator 的全量策略扫描保持现状（改动面更大，收益较低，单独评估）。
