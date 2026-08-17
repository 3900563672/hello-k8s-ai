package controller

import (
	"cmp"
	"math"
	"slices"
)

// placementCandidate 一个可扩容的候选组合：在哪个实例上扩，用哪个节点，扩完之后分数和资源占用如何。
type placementCandidate struct {
	InstanceName   string // 实例名
	NodeName       string // 被选中的节点名
	TargetReplicas int    // 扩到多少副本
	EffectiveScore int    // 打分结果
	ModelGPU       int    // 该模型需要多少 GPU
	NodeGPULeft    int    // 选完后节点还剩多少 GPU（用于 tie-break）
}

// FindBestCandidate 是公开的便捷封装，返回实例名、目标副本数、有效分数和是否找到。
func FindBestCandidate(
	models []ModelInfo,
	nodes []NodeInfo,
	instances []InstanceInfo,
) (instanceName string, targetReplicas int, effectiveScore int, found bool) {
	candidate, found := findBestPlacement(models, nodes, instances, 1)
	if !found {
		return "", 0, 0, false
	}
	return candidate.InstanceName, candidate.TargetReplicas, candidate.EffectiveScore, true
}

// findBestPlacement 在所有可行的 (实例, 节点) 组合中找出最佳扩容目标。
// extraReplicas 是本次决策要补的副本数（>=1），目标节点必须能一次容纳整批。
func findBestPlacement(models []ModelInfo, nodes []NodeInfo, instances []InstanceInfo, extraReplicas int) (placementCandidate, bool) {
	if extraReplicas < 1 {
		extraReplicas = 1
	}
	if len(models) == 0 || len(nodes) == 0 || len(instances) == 0 {
		return placementCandidate{}, false
	}

	// 按模型名分组实例，方便后面匹配
	instancesByModel := make(map[string][]InstanceInfo)
	for _, instance := range instances {
		instancesByModel[instance.ModelName] = append(instancesByModel[instance.ModelName], instance)
	}

	candidates := make([]placementCandidate, 0)
	for _, model := range models {
		// 模型没有分数或者 GPU/并发需求不合法，直接跳过
		if model.AbsoluteScore <= 0 || model.GPUUnits <= 0 || model.MaxConcurrency <= 0 {
			continue
		}
		modelInstances := instancesByModel[model.Name]
		for _, instance := range modelInstances {
			// 防止极端副本数导致溢出
			if instance.CurrentReplicas < 0 ||
				instance.CurrentReplicas > int(^uint(0)>>1)-extraReplicas ||
				(instance.CurrentReplicas > 0 && !instance.PlacementReady) {
				continue
			}
			for _, node := range nodes {
				// 节点策略不允许该模型运行。
				if model.EligibleNodeNames != nil && !model.EligibleNodeNames[node.Name] {
					continue
				}
				// 节点剩余资源必须满足整批副本的需求（用除法避免乘法溢出）。
				if node.RemainingGPU/extraReplicas < model.GPUUnits ||
					node.RemainingConcurrency/extraReplicas < model.MaxConcurrency {
					continue
				}
				score := scoreModel(model)
				if score <= 0 {
					continue
				}
				candidates = append(candidates, placementCandidate{
					InstanceName:   instance.Name,
					NodeName:       node.Name,
					TargetReplicas: instance.CurrentReplicas + extraReplicas,
					EffectiveScore: score,
					ModelGPU:       model.GPUUnits,
					NodeGPULeft:    node.RemainingGPU - model.GPUUnits*extraReplicas, // 扣完后的剩余量，越小表示越紧凑
				})
			}
		}
	}

	if len(candidates) == 0 {
		return placementCandidate{}, false
	}

	// 排序：先按分数降序；分数一样优先 GPU 需求小的模型；再一样优先扣完剩余多的节点；
	// 最后按实例名和节点名稳定排序
	slices.SortFunc(candidates, func(left, right placementCandidate) int {
		if left.EffectiveScore != right.EffectiveScore {
			return cmp.Compare(right.EffectiveScore, left.EffectiveScore)
		}
		if left.ModelGPU != right.ModelGPU {
			return cmp.Compare(left.ModelGPU, right.ModelGPU)
		}
		if left.NodeGPULeft != right.NodeGPULeft {
			return cmp.Compare(right.NodeGPULeft, left.NodeGPULeft)
		}
		if left.InstanceName != right.InstanceName {
			return cmp.Compare(left.InstanceName, right.InstanceName)
		}
		return cmp.Compare(left.NodeName, right.NodeName)
	})
	return candidates[0], true
}

// bestEffectiveScoreForModel 在给定节点列表中找一个能跑该模型的最优分数，用于初始化新实例的有效分数。
func bestEffectiveScoreForModel(model ModelInfo, nodes []NodeInfo) (int, bool) {
	best := 0
	found := false
	for _, node := range nodes {
		// 跳过策略不允许的节点。
		if model.EligibleNodeNames != nil && !model.EligibleNodeNames[node.Name] {
			continue
		}
		// 跳过资源不足的节点。
		if node.RemainingGPU < model.GPUUnits || node.RemainingConcurrency < model.MaxConcurrency {
			continue
		}
		score := scoreModel(model)
		if score > best {
			best = score
			found = true
		}
	}
	return best, found
}

// scoreModel 计算模型在不受资源限制下的理论分数，只考虑绝对分和冷启动衰减。
// 资源是否足够已由外部硬约束保证，所以这里不再乘资源折扣。
func scoreModel(model ModelInfo) int {
	if model.AbsoluteScore <= 0 {
		return 0
	}
	combined := float64(model.AbsoluteScore) * coldStartWeight(model.ColdStartMs)
	// 拒绝异常的 NaN 或非正数结果。
	if math.IsNaN(combined) || combined <= 0 {
		return 0
	}
	// 防止溢出 int 上限
	maxInt := int(^uint(0) >> 1)
	if combined >= float64(maxInt) {
		return maxInt
	}
	score := int(combined)
	if score == 0 {
		return 1 // 保证至少为 1，避免 zero score 被误判为无效
	}
	return score
}

// coldStartWeight 冷启动衰减因子：启动越慢权重越低，最低 0.7。
// baseMs 是基准冷启动时间（60s），decayFactor 控制衰减斜率。
func coldStartWeight(coldStartMs int) float64 {
	if coldStartMs <= 0 {
		return 1
	}
	const (
		baseMs      = 60000.0 // 60 秒基准
		decayFactor = 0.2     // 每 60 秒衰减 0.2
		minWeight   = 0.7     // 最多衰减到 0.7
	)
	weight := 1 - decayFactor*float64(coldStartMs)/baseMs
	if weight < minWeight {
		return minWeight
	}
	if weight > 1 {
		return 1
	}
	return weight
}
