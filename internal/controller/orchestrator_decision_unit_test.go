package controller

import (
	"testing"
	"time"
)

// TestDecideAtResourceLimited 校验物理水位保护：任一可调度节点物理压力超阈值时
// 停止扩容并返回 resource_limited；压力解除后恢复扩容。
func TestDecideAtResourceLimited(t *testing.T) {
	now := time.Date(2026, time.August, 9, 12, 0, 0, 0, time.UTC)
	base := DecisionInput{
		AvgTTFT:         600,
		HasTTFT:         true,
		TTFTThresholdUp: 500,
		AvailableModels: []ModelInfo{{Name: "model-a", GPUUnits: 1, AbsoluteScore: 100, MaxConcurrency: 1}},
		AvailableNodes:  []NodeInfo{{Name: "node-a", RemainingGPU: 2, RemainingConcurrency: 2}},
		ExistingInstances: []InstanceInfo{{
			Name:            "instance-a",
			ModelName:       "model-a",
			CurrentReplicas: 1,
			PlacementReady:  true,
			PlacementPlan:   nodePlacementPlan{Version: placementPlanVersion, PrimaryNode: "node-a", Placements: []nodePlacement{{NodeName: "node-a", Replicas: 1}}},
		}},
	}

	// 无物理压力：正常扩容
	if decision := DecideAt(base, now); decision.Action != ScaleUp {
		t.Fatalf("unpressured decision = %+v, want ScaleUp", decision)
	}

	// 任一节点物理压力：停止扩容
	base.AvailableNodes[0].PhysicalPressure = true
	decision := DecideAt(base, now)
	if decision.Action != NoOp || decision.Reason != "resource_limited" {
		t.Fatalf("pressured decision = %+v, want NoOp/resource_limited", decision)
	}

	// 压力解除后自动恢复扩容
	base.AvailableNodes[0].PhysicalPressure = false
	if decision := DecideAt(base, now); decision.Action != ScaleUp {
		t.Fatalf("recovered decision = %+v, want ScaleUp", decision)
	}

	// 物理压力不影响缩容（不改变副本总量）
	downInput := DecisionInput{
		AvgTTFT:            100,
		AvgQueue:           1,
		HasTTFT:            true,
		HasQueue:           true,
		TTFTThresholdUp:    500,
		TTFTThresholdDown:  200,
		QueueThresholdUp:   100,
		QueueThresholdDown: 30,
		MinReplicas:        1,
		AvailableNodes:     []NodeInfo{{Name: "node-a", PhysicalPressure: true}},
		ExistingInstances: []InstanceInfo{{
			Name:            "instance-a",
			CurrentReplicas: 2,
			PlacementReady:  true,
			PlacementPlan: nodePlacementPlan{
				Version:     placementPlanVersion,
				PrimaryNode: "node-a",
				Placements: []nodePlacement{
					{NodeName: "node-a", Replicas: 1},
					{NodeName: "node-b", Replicas: 1},
				},
			},
		}},
	}
	if decision := DecideAt(downInput, now); decision.Action != ScaleDown {
		t.Fatalf("pressured scale-down decision = %+v, want ScaleDown", decision)
	}
}

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
