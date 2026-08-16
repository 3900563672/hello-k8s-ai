# 收窄 WorkerNodeUsage 事件映射与用量统计范围

- 变更日期：2026-08-16
- 关联问题：Fixes #20（Project Review issue-08）
- 变更级别：P1 性能优化
- 变更范围：`internal/controller/workernodeusage_controller.go`、相关测试
- CRD 变化：无
- 数据库变化：无

## 1. 完成结果

WorkerNodeUsage 此前对每个 Pod 事件为全部 WorkerNode 入队，且每个节点 Reconcile 都会遍历集群全部 Pod 再过滤，事件放大为 O(W×(P+M))。本次收窄：

- **事件映射**：已调度 Pod（`spec.nodeName` 非空）只入队其所在节点；Model 事件与未调度 Pod 无法定位节点，保持广播全部节点兜底。
- **用量统计**：为 Pod 注册 `spec.nodeName` 字段索引，`calculateNodeUsage` 只 List 目标节点的 Pod，不再遍历全量 Pod。

## 2. 关键行为

- 一次已调度 Pod 事件从"全部 W 个节点 × 全量 Pod 扫描"降为"1 个节点 × 该节点 Pod 扫描"。
- 语义不变：Pod 只影响其所在节点；未调度 Pod 不影响任何节点用量。

## 3. 影响范围

| 模块 | 影响 |
| --- | --- |
| WorkerNodeUsage Controller | Pod 精确入队 + NodeName 索引过滤 |
| 测试 | 新增事件映射与按节点统计单测；集成测试补 WithIndex |

## 4. 资料入口

- [实现修改明细](IMPLEMENTATION_DETAILS.md)
- [测试报告](TEST_REPORT.md)
- [升级与回滚](MIGRATION_AND_ROLLBACK.md)

## 5. 实测结论与停止线

- `go test ./internal/controller/` 通过。
- 停止线：只改 WorkerNodeUsage 的事件放大；TenantModelPolicy / Orchestrator 的较轻全量映射不动，避免本轮扩大改动面。
