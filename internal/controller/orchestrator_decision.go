package controller

import (
	"cmp"
	"slices"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type DecisionAction int

const (
	NoOp DecisionAction = iota
	ScaleUp
	ScaleDown
	Rebalance
)

const reasonModelAbsoluteScoreMissing = "model_absolute_score_missing"

type Decision struct {
	Action           DecisionAction
	Reason           string
	InstanceName     string
	SourceNodeName   string
	NodeName         string
	ObservedReplicas int
	TargetReplicas   int
	EffectiveScore   int
	RequeueAfter     time.Duration
}

// DecideAt 是纯函数决策入口，输入不变则输出不变。
// TriggerID 仅用于跳过已执行过的相同计划，不是冷却控制的替代。
func DecideAt(input DecisionInput, now time.Time) Decision {
	// 如果本轮的 triggerID 跟某个实例上次应用的一样，说明输入没变且计划已落地，
	// 直接返回 NoOp，避免重复操作。
	if input.TriggerID != "" {
		for _, instance := range input.ExistingInstances {
			if instance.LastScaleTrigger == input.TriggerID {
				return Decision{Action: NoOp, Reason: "trigger_already_applied"}
			}
		}
	}

	// 当前总副本数
	totalReplicas := 0
	for _, instance := range input.ExistingInstances {
		totalReplicas += nonNegative(instance.CurrentReplicas)
	}
	if decision, needed := placementRebalanceDecision(input); needed {
		return decision
	}

	// 地板：至少要保持的副本数（考虑最小副本、零流量等）
	floor := desiredReplicaFloor(input)

	// 是否需要扩容：低于地板，或者性能超过上阈值
	needBootstrap := totalReplicas < floor
	needUp := needBootstrap ||
		(input.HasTTFT && input.AvgTTFT > input.TTFTThresholdUp) ||
		(input.HasQueue && input.AvgQueue > input.QueueThresholdUp)

	// 是否需要缩容：零流量或双指标同时低于下阈值
	// 注意：有流量但没有完整低负载样本时不能缩，避免冷启动瞬态误判。
	needDown := input.TenantQPS == 0 ||
		(input.HasTTFT && input.HasQueue &&
			input.AvgTTFT < input.TTFTThresholdDown &&
			input.AvgQueue < input.QueueThresholdDown)

	// 物理水位保护：任一可调度节点接近物理上限时停止扩容（不增加副本），
	// 负载由现有副本排队消化；水位恢复后自动解除。缩容与重平衡不改变副本总量，不受影响。
	if needUp && anyNodeUnderPhysicalPressure(input.AvailableNodes) {
		return Decision{Action: NoOp, Reason: decisionReasonResourceLimited, RequeueAfter: orchestratorSyncPeriod}
	}

	if needUp {
		return scaleUpDecision(input, now, floor, totalReplicas, needBootstrap)
	}

	// 不需要扩容，再看不缩容的理由
	if !needDown {
		return Decision{Action: NoOp, Reason: "pressure_not_low"}
	}
	if remaining := cooldownRemaining(input.LastScaleDownTime, input.ScaleDownCooldown, now); remaining > 0 {
		return Decision{Action: NoOp, Reason: "scale_down_cooldown", RequeueAfter: remaining}
	}

	// 不能低于地板
	if totalReplicas <= floor {
		return Decision{Action: NoOp, Reason: "replica_floor"}
	}

	// 缩容：优先找副本多、分数低的实例，挨个削
	candidates := append([]InstanceInfo(nil), input.ExistingInstances...)
	slices.SortFunc(candidates, func(left, right InstanceInfo) int {
		// 副本多的排前面
		if left.CurrentReplicas != right.CurrentReplicas {
			return cmp.Compare(right.CurrentReplicas, left.CurrentReplicas)
		}
		// 副本一样多，分数低的优先
		if left.EffectiveScore != right.EffectiveScore {
			return cmp.Compare(left.EffectiveScore, right.EffectiveScore)
		}
		// 最后按名字稳定排序
		return cmp.Compare(left.Name, right.Name)
	})
	for _, instance := range candidates {
		if instance.CurrentReplicas == 0 || !instance.PlacementReady || totalReplicas-1 < floor {
			continue
		}
		nodeName, found := scaleDownPlacementNode(instance.PlacementPlan)
		if !found {
			continue
		}
		return Decision{
			Action:           ScaleDown,
			Reason:           scaleDownReason(input),
			InstanceName:     instance.Name,
			NodeName:         nodeName,
			ObservedReplicas: instance.CurrentReplicas,
			TargetReplicas:   instance.CurrentReplicas - 1,
			RequeueAfter:     time.Duration(input.ScaleDownCooldown) * time.Second,
		}
	}
	return Decision{Action: NoOp, Reason: "no_scale_down_candidate"}
}

// anyNodeUnderPhysicalPressure 返回是否有任意节点物理水位超阈值。
func anyNodeUnderPhysicalPressure(nodes []NodeInfo) bool {
	for _, node := range nodes {
		if node.PhysicalPressure {
			return true
		}
	}
	return false
}

func placementUnavailableDecision(models []ModelInfo, instances []InstanceInfo) Decision {
	hasValidScore, missingModels := placementModelScoreState(models, instances)
	if !hasValidScore && len(missingModels) > 0 {
		return Decision{Action: NoOp, Reason: reasonModelAbsoluteScoreMissing}
	}
	return Decision{Action: NoOp, Reason: "no_feasible_placement"}
}

// placementModelScoreState 只检查已有实例引用的可用模型。
// 有任一模型具备有效分数时，后续失败仍按容量或策略问题处理；只有全部候选都缺分数时，
// 才把原因明确标记为模型配置缺失。
func placementModelScoreState(models []ModelInfo, instances []InstanceInfo) (bool, []string) {
	instanceModels := make(map[string]struct{}, len(instances))
	for _, instance := range instances {
		instanceModels[instance.ModelName] = struct{}{}
	}

	missing := make([]string, 0)
	for _, model := range models {
		if _, referenced := instanceModels[model.Name]; !referenced {
			continue
		}
		if model.AbsoluteScore > 0 {
			return true, nil
		}
		missing = append(missing, model.Name)
	}
	slices.Sort(missing)
	return false, slices.Compact(missing)
}

// placementRebalanceDecision 在节点策略收窄后，一次迁移一个副本。
// 该动作不改变总副本数，优先于性能驱动的扩缩容。
func placementRebalanceDecision(input DecisionInput) (Decision, bool) {
	models := make(map[string]ModelInfo, len(input.AvailableModels))
	for _, model := range input.AvailableModels {
		models[model.Name] = model
	}
	nodes := append([]NodeInfo(nil), input.AvailableNodes...)
	slices.SortFunc(nodes, func(left, right NodeInfo) int {
		if left.RemainingGPU != right.RemainingGPU {
			return cmp.Compare(right.RemainingGPU, left.RemainingGPU)
		}
		if left.RemainingConcurrency != right.RemainingConcurrency {
			return cmp.Compare(right.RemainingConcurrency, left.RemainingConcurrency)
		}
		return cmp.Compare(left.Name, right.Name)
	})
	nodeExists := make(map[string]bool, len(nodes))
	for _, node := range nodes {
		nodeExists[node.Name] = true
	}

	instances := append([]InstanceInfo(nil), input.ExistingInstances...)
	slices.SortFunc(instances, func(left, right InstanceInfo) int {
		return cmp.Compare(left.Name, right.Name)
	})
	for _, instance := range instances {
		if !instance.PlacementReady || instance.CurrentReplicas == 0 {
			continue
		}
		model, exists := models[instance.ModelName]
		if !exists {
			continue
		}
		for _, placement := range sortedNodePlacements(instance.PlacementPlan.Placements) {
			allowed := nodeExists[placement.NodeName]
			if model.EligibleNodeNames != nil {
				allowed = allowed && model.EligibleNodeNames[placement.NodeName]
			}
			if allowed {
				continue
			}
			for _, node := range nodes {
				if model.EligibleNodeNames != nil && !model.EligibleNodeNames[node.Name] {
					continue
				}
				if node.RemainingGPU < model.GPUUnits || node.RemainingConcurrency < model.MaxConcurrency {
					continue
				}
				return Decision{
					Action:           Rebalance,
					Reason:           "placement_policy_changed",
					InstanceName:     instance.Name,
					SourceNodeName:   placement.NodeName,
					NodeName:         node.Name,
					ObservedReplicas: instance.CurrentReplicas,
					TargetReplicas:   instance.CurrentReplicas,
					EffectiveScore:   instance.EffectiveScore,
				}, true
			}
			return Decision{Action: NoOp, Reason: "placement_rebalance_blocked"}, true
		}
	}
	return Decision{}, false
}

// scaleUpDecision 生成一次扩容决策；无法扩容时返回 NoOp 及原因。
func scaleUpDecision(input DecisionInput, now time.Time, floor, totalReplicas int, bootstrap bool) Decision {
	// 到上限了就别扩了
	if input.MaxReplicas > 0 && totalReplicas >= input.MaxReplicas {
		return Decision{Action: NoOp, Reason: "maximum_replicas"}
	}
	// 扩容冷却期内，等一等
	if remaining := cooldownRemaining(input.LastScaleUpTime, input.ScaleUpCooldown, now); remaining > 0 {
		return Decision{Action: NoOp, Reason: "scale_up_cooldown", RequeueAfter: remaining}
	}
	// 决定这次补多少副本：引导期一次补到地板，高负载按队列缺口批量补，其余保持 +1。
	delta := scaleUpDelta(input)
	if bootstrap {
		delta = clampInt(floor-totalReplicas, 1, scaleUpBatchLimit(input))
	}
	// 单节点放不下整批时逐级减半，直到最小 +1；+1 都放不下才按容量不足处理。
	candidate, found := findBestPlacement(input.AvailableModels, input.AvailableNodes, input.ExistingInstances, delta)
	for !found && delta > 1 {
		delta /= 2
		candidate, found = findBestPlacement(input.AvailableModels, input.AvailableNodes, input.ExistingInstances, delta)
	}
	if !found {
		return placementUnavailableDecision(input.AvailableModels, input.ExistingInstances)
	}
	return Decision{
		Action:           ScaleUp,
		Reason:           scaleUpReason(input, bootstrap),
		InstanceName:     candidate.InstanceName,
		NodeName:         candidate.NodeName,
		ObservedReplicas: candidate.TargetReplicas - delta,
		TargetReplicas:   candidate.TargetReplicas,
		EffectiveScore:   candidate.EffectiveScore,
		RequeueAfter:     time.Duration(input.ScaleUpCooldown) * time.Second,
	}
}

// defaultMaxScaleUpBatch 单次扩容决策默认最多补的副本数（配置为 0 时生效）。
// 模拟器没有网关，副本可以无限增长；批量扩容配合冷却形成批次节奏，
// 既避免大并发下“10→100 要等 90 次冷却”，也防止单次决策把副本数冲过头。
const defaultMaxScaleUpBatch = 10

// scaleUpBatchLimit 返回单次扩容步长上限：配置值优先，0 表示使用默认 10。
func scaleUpBatchLimit(input DecisionInput) int {
	if input.MaxScaleUpBatch > 0 {
		return input.MaxScaleUpBatch
	}
	return defaultMaxScaleUpBatch
}

// scaleUpDelta 估算一次扩容决策应补的副本数，范围 [1, scaleUpBatchLimit(input)]。
// 队列指标能直接换算成副本缺口：缺口 / 单副本并发容量，向上取整。
// TTFT 超标不参与批量放大，保持原有 +1 节奏，避免按延迟比例盲目扩。
func scaleUpDelta(input DecisionInput) int {
	perReplica := maxModelConcurrency(input.AvailableModels)
	if perReplica <= 0 {
		perReplica = 1
	}
	if input.HasQueue && input.AvgQueue > input.QueueThresholdUp && input.QueueThresholdUp > 0 {
		gap := input.AvgQueue - input.QueueThresholdUp
		return clampInt((gap+perReplica-1)/perReplica, 1, scaleUpBatchLimit(input))
	}
	return 1
}

// maxModelConcurrency 取候选模型里最大的单副本并发容量，作为“每副本能吸收多少队列”的估算。
func maxModelConcurrency(models []ModelInfo) int {
	best := 0
	for _, model := range models {
		if model.MaxConcurrency > best {
			best = model.MaxConcurrency
		}
	}
	return best
}

// clampInt 把 value 限制在 [low, high] 内。
func clampInt(value, low, high int) int {
	return max(low, min(value, high))
}

// 根据扩容触发原因生成具体 reason 字符串
func scaleUpReason(input DecisionInput, bootstrap bool) string {
	if bootstrap {
		return "replica_floor"
	}
	ttftHigh := input.HasTTFT && input.AvgTTFT > input.TTFTThresholdUp
	queueHigh := input.HasQueue && input.AvgQueue > input.QueueThresholdUp
	switch {
	case ttftHigh && queueHigh:
		return "ttft_and_queue_high"
	case ttftHigh:
		return "ttft_high"
	default:
		return "queue_high"
	}
}

func scaleDownReason(input DecisionInput) string {
	if input.TenantQPS == 0 {
		return "zero_traffic"
	}
	return "pressure_low"
}

// desiredReplicaFloor 计算至少应保持的副本数。
func desiredReplicaFloor(input DecisionInput) int {
	floor := nonNegative(input.MinReplicas)
	// 零流量且允许缩到零时，地板是 0
	if input.TenantQPS == 0 && input.AllowScaleToZero {
		floor = 0
	} else if input.TenantQPS > 0 {
		// 有流量时至少留 1 个
		floor = max(floor, 1)
	}
	// 但别超过上限
	if input.MaxReplicas > 0 {
		floor = min(floor, input.MaxReplicas)
	}
	return floor
}

// cooldownRemaining 返回剩余的冷却时间，冷却已过或没设置则返回 0。
func cooldownRemaining(last metav1.Time, cooldownSeconds int, now time.Time) time.Duration {
	if last.IsZero() || cooldownSeconds <= 0 {
		return 0
	}
	remaining := last.Add(time.Duration(cooldownSeconds) * time.Second).Sub(now)
	if remaining <= 0 {
		return 0
	}
	return remaining
}
