package controller

import "testing"

// TestScaleUpBatchLimit 校验扩容步长上限：配置值优先，0 表示使用默认 10。
func TestScaleUpBatchLimit(t *testing.T) {
	if got := scaleUpBatchLimit(DecisionInput{}); got != defaultMaxScaleUpBatch {
		t.Fatalf("scaleUpBatchLimit(默认) = %d, want %d", got, defaultMaxScaleUpBatch)
	}
	if got := scaleUpBatchLimit(DecisionInput{MaxScaleUpBatch: 3}); got != 3 {
		t.Fatalf("scaleUpBatchLimit(3) = %d, want 3", got)
	}
	if got := scaleUpBatchLimit(DecisionInput{MaxScaleUpBatch: 50}); got != 50 {
		t.Fatalf("scaleUpBatchLimit(50) = %d, want 50", got)
	}
}

// TestScaleUpDeltaRespectsBatchLimit 校验队列缺口换算会按配置步长截断。
func TestScaleUpDeltaRespectsBatchLimit(t *testing.T) {
	base := DecisionInput{
		AvailableModels:  []ModelInfo{{MaxConcurrency: 16}},
		HasQueue:         true,
		AvgQueue:         500,
		QueueThresholdUp: 100,
		MaxScaleUpBatch:  10,
	}

	// 队列缺口 400，每副本并发 16：理论 25 副本，被步长截断到 10。
	if got := scaleUpDelta(base); got != 10 {
		t.Fatalf("scaleUpDelta(步长 10) = %d, want 10", got)
	}
	// 未配置步长时使用默认 10。
	base.MaxScaleUpBatch = 0
	if got := scaleUpDelta(base); got != 10 {
		t.Fatalf("scaleUpDelta(默认步长) = %d, want 10", got)
	}
	// 大步长允许一次补到缺口。
	base.MaxScaleUpBatch = 50
	if got := scaleUpDelta(base); got != 25 {
		t.Fatalf("scaleUpDelta(步长 50) = %d, want 25", got)
	}
	// 无队列指标时保持 +1 节奏。
	base.HasQueue = false
	if got := scaleUpDelta(base); got != 1 {
		t.Fatalf("scaleUpDelta(无队列) = %d, want 1", got)
	}
}
