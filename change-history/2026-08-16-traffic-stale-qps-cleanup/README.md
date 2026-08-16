# 流量分配零副本残留 QPS 清理

- 变更日期：2026-08-16
- 关联问题：Fixes #18（Project Review issue-06）
- 变更级别：P1 行为修复
- 变更范围：`internal/controller/traffic_controller.go`、测试
- CRD 变化：无
- 数据库变化：无

## 1. 完成结果

流量分配此前只更新参与分配的活跃实例；实例被 Orchestrator 缩容到零副本后会被排除出集合，但旧 `spec.traffic.qps` 不会清零，导致 Backend 聚合 `allocatedQPS > requestedQPS`，`allocationBalanced` 恒为 false，并进入历史 Snapshot。

修复：`distributeTrafficForTenant` 在每次分配前调用新增的 `zeroStaleTrafficQPS`，把不在活跃集合但仍残留旧 QPS 的实例显式清零；`updateInstanceQPS` 对不存在、删除中或值相同的实例自动跳过，无副作用。新增单测覆盖"缩到零副本实例清零、活跃实例保持不变"。

## 2. 关键行为

- 每次分配（含 `no_instances` 场景）都以租户下完整实例集合为边界：活跃实例参与分配，退出集合的实例 QPS 归零。
- 分配不变量端到端成立：`requestedQPS == sum(全部实例 QPS)`。

## 3. 影响范围

| 模块 | 影响 |
| --- | --- |
| Traffic Controller | 分配前清零 stale QPS；`no_instances` 也会清理 |
| Backend 聚合 | allocatedQPS 不再虚高，allocationBalanced 恢复正确 |
| 测试 | 新增 TestZeroStaleTrafficQPSOnScaledToZeroInstances |

## 4. 资料入口

- [实现修改明细](IMPLEMENTATION_DETAILS.md)
- [测试报告](TEST_REPORT.md)
- [升级与回滚](MIGRATION_AND_ROLLBACK.md)

## 5. 实测结论与停止线

- `go test ./internal/controller/` 通过；单测覆盖清零路径。
- 停止线：不做分配算法重构、不动 aggregator（清零后聚合自然正确），保留 Largest Remainder 语义。
