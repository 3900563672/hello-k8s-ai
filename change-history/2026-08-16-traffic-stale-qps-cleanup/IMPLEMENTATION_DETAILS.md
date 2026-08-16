# 实现修改明细

## 1. 改动前状态

- `collectTrafficInstances` 跳过 `replicas == 0` 与删除中的实例，`distributeTrafficForTenant` 只更新活跃集合。
- 实例曾获得 QPS 后被缩到零，旧值残留；`no_instances` 场景直接返回，同样不清理。
- Backend `aggregator.go` 把租户下全部实例 `spec.traffic.qps` 相加（不排除零副本），造成不一致切面。

## 2. 修改

- `internal/controller/traffic_controller.go`：
  - `distributeTrafficForTenant`：`collectTrafficInstances` 后、任何 return 前调用 `zeroStaleTrafficQPS`。
  - 新增 `zeroStaleTrafficQPS(ctx, tenantName, active)`：按 `trafficTenantIndex` 列出租户全部实例，对不在活跃集合且 QPS != 0 的实例调 `updateInstanceQPS(0)`；错误聚合返回。
- `internal/controller/traffic_controller_test.go`：新增单测，fake client + `WithIndex(trafficTenantIndex)`，验证缩到零的实例清零、活跃实例不变。

## 3. 未做

- 未改 `allocateTraffic` 算法与 `aggregator.go`（清零后聚合不变量自动恢复）。
- 未处理 Frontend 流量工作台"预览 vs 真实命令"（属 #17 范围）。
