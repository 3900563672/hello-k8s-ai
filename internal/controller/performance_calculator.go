package controller

import (
	"cmp"
	"math"
	"slices"
)

// instancePerformance 实例性能采样，Weight 通常代表该采样对应的副本数。
// HasTTFT / HasQueue 用于区分冷启动阶段可能缺失某些指标的情况。
type instancePerformance struct {
	TTFT     int
	Queue    int
	Weight   int
	HasTTFT  bool
	HasQueue bool
}

type performanceInput struct {
	Instances []instancePerformance
}

// calculatePerformanceSummary 基于各实例采样计算租户级的加权平均 TTFT 和 Queue。
// 返回值包含是否有有效指标，用于判断数据是否就绪。
func calculatePerformanceSummary(input performanceInput) (avgTTFT, avgQueue int, hasTTFT, hasQueue bool) {
	ttftValues := make([]weightedValue, 0, len(input.Instances))
	queueValues := make([]weightedValue, 0, len(input.Instances))

	for _, sample := range input.Instances {
		weight := sample.Weight
		if weight <= 0 {
			weight = 1 // 防御性：权重至少为 1
		}
		if sample.HasTTFT {
			ttftValues = append(ttftValues, weightedValue{value: nonNegative(sample.TTFT), weight: weight})
		}
		if sample.HasQueue {
			queueValues = append(queueValues, weightedValue{value: nonNegative(sample.Queue), weight: weight})
		}
	}

	if len(ttftValues) > 0 {
		avgTTFT = weightedMedianPercentValues(ttftValues)
		hasTTFT = true
	}
	if len(queueValues) > 0 {
		avgQueue = weightedMedianPercentValues(queueValues)
		hasQueue = true
	}
	return avgTTFT, avgQueue, hasTTFT, hasQueue
}

type weightedValue struct {
	value  int
	weight int
}

// weightedMedianPercentValues 实现基于中位数的百分比加权平均。
// 对偏离中位数的值给予更大权重，使最终结果对异常值更敏感。
// 当所有值相等或中位数为 0 时退化为算术平均。
func weightedMedianPercentValues(values []weightedValue) int {
	if len(values) == 0 {
		return 0
	}

	// 副本值排序，不改变原切片
	sorted := append([]weightedValue(nil), values...)
	slices.SortFunc(sorted, func(left, right weightedValue) int {
		return cmp.Compare(left.value, right.value)
	})

	median := weightedMedian(sorted)

	// 所有值相同或中位数为 0 时无法做偏差加权，退回加权算术平均
	if sorted[0].value == sorted[len(sorted)-1].value || median == 0 {
		return weightedArithmeticMean(sorted)
	}

	weightedSum := 0.0
	totalWeight := 0.0
	for _, item := range sorted {
		baseWeight := float64(item.weight)
		deviation := math.Abs(float64(item.value) - median)
		// 权重 = 原始权重 * (1 + 偏离比例)，偏离越大加权越重
		weight := baseWeight * (1 + deviation/median)
		weightedSum += float64(item.value) * weight
		totalWeight += weight
	}
	if totalWeight == 0 {
		return clampRoundedInt(median)
	}
	return clampRoundedInt(weightedSum / totalWeight)
}

// weightedMedian 计算加权中位数，注意返回 float64 是为了支持偶数样本数的插值。
func weightedMedian(sorted []weightedValue) float64 {
	totalWeight := int64(0)
	for _, item := range sorted {
		totalWeight += int64(item.weight)
	}
	if totalWeight <= 0 {
		return 0
	}

	// 中位数的两个位置（奇数时两个位置相同）
	leftPosition := (totalWeight - 1) / 2
	rightPosition := totalWeight / 2
	left := valueAtWeightPosition(sorted, leftPosition)
	right := valueAtWeightPosition(sorted, rightPosition)
	return (float64(left) + float64(right)) / 2
}

// valueAtWeightPosition 从加权排序序列中找到指定累计权重位置处的值。
func valueAtWeightPosition(sorted []weightedValue, position int64) int {
	cumulative := int64(0)
	for _, item := range sorted {
		cumulative += int64(item.weight)
		if position < cumulative {
			return item.value
		}
	}
	return sorted[len(sorted)-1].value
}

func weightedArithmeticMean(values []weightedValue) int {
	sum := 0.0
	totalWeight := 0.0
	for _, item := range values {
		sum += float64(item.value) * float64(item.weight)
		totalWeight += float64(item.weight)
	}
	if totalWeight == 0 {
		return 0
	}
	return clampRoundedInt(sum / totalWeight)
}

// clampRoundedInt 四舍五入到 int 并限制在 [0, MaxInt] 范围内。
func clampRoundedInt(value float64) int {
	if math.IsNaN(value) || value <= 0 {
		return 0
	}
	maxInt := int(^uint(0) >> 1)
	if value >= float64(maxInt) {
		return maxInt
	}
	return int(math.Round(value))
}

// nonNegative 负数归零。
func nonNegative(value int) int {
	if value < 0 {
		return 0
	}
	return value
}
