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

	if needUp {
		// 到上限了就别扩了
		if input.MaxReplicas > 0 && totalReplicas >= input.MaxReplicas {
			return Decision{Action: NoOp, Reason: "maximum_replicas"}
		}
		// 扩容冷却期内，等一等
		if remaining := cooldownRemaining(input.LastScaleUpTime, input.ScaleUpCooldown, now); remaining > 0 {
			return Decision{Action: NoOp, Reason: "scale_up_cooldown", RequeueAfter: remaining}
		}
		// 找个最合适的实例扩容
		candidate, found := findBestPlacement(input.AvailableModels, input.AvailableNodes, input.ExistingInstances)
		if !found {
			return Decision{Action: NoOp, Reason: "no_feasible_placement"}
		}
		return Decision{
			Action:           ScaleUp,
			Reason:           scaleUpReason(input, needBootstrap),
			InstanceName:     candidate.InstanceName,
			NodeName:         candidate.NodeName,
			ObservedReplicas: candidate.TargetReplicas - 1,
			TargetReplicas:   candidate.TargetReplicas,
			EffectiveScore:   candidate.EffectiveScore,
			RequeueAfter:     time.Duration(input.ScaleUpCooldown) * time.Second,
		}
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
