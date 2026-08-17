# 实现细节

## 1. 批量扩容决策（orchestrator_decision.go）

```go
// 原逻辑：findBestPlacement(...) 固定 +1
// 新逻辑：
delta := scaleUpDelta(input)          // 队列缺口/单副本并发，上限 10；TTFT-only 仍 +1
if needBootstrap {
    delta = clampInt(floor-totalReplicas, 1, maxScaleUpBatch)  // 引导期一次补到地板
}
candidate, found := findBestPlacement(..., delta)
for !found && delta > 1 { delta /= 2; ... }  // 单节点放不下整批则减半回退
```

- `scaleUpDelta`：`ceil((AvgQueue - QueueThresholdUp) / maxModelConcurrency)`，钳制到 [1, 10]；只有队列指标能线性换算成副本缺口，TTFT-only 保持 +1 避免按延迟比例盲目扩。
- `ObservedReplicas = TargetReplicas - delta`，ScalingRecord 记录整批 Old→New。

## 2. 批量放置（orchestrator_scoring.go）

- `findBestPlacement(models, nodes, instances, extraReplicas)`：
  - 节点容量用除法判定整批可放：`RemainingGPU/extraReplicas >= GPUUnits && RemainingConcurrency/extraReplicas >= MaxConcurrency`（避免乘法溢出）。
  - 溢出守卫改为 `CurrentReplicas > maxInt - extraReplicas`。
  - `NodeGPULeft = RemainingGPU - GPUUnits*extraReplicas` 保持紧凑排序语义。

## 3. 批量落地（placement_plan.go / orchestrator_executor.go）

- `addNodePlacement` 委托 `addNodePlacements(plan, node, 1)`；批量版一次 `Replicas += count`，PrimaryNode 语义不变。
- `persistScalePlan` 的 scale-up 分支改调 `addNodePlacements(plan, node, NewReplicas-OldReplicas)`；放置计划副本总数校验、幂等 trigger、stale 检查全部复用。

## 4. 关键语义保持

- `maximum_replicas` / `scale_up_cooldown` / `no_feasible_placement` / `replica_floor` reason 不变。
- 决策仍是纯函数：输入不变输出不变。
- 批量为 1 时行为与旧版完全一致（既有测试全部通过）。
