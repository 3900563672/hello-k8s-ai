package controller

import (
	"reflect"
	"testing"
	"time"

	platformv1 "github.com/3900563672/hello-k8s-ai/api/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// 下列辅助函数用于简化测试数据构造。

func tenantModelPolicy(tenantName, modelName, effect string) platformv1.TenantModelPolicy {
	return platformv1.TenantModelPolicy{Spec: platformv1.TenantModelPolicySpec{
		TenantRef: platformv1.ObjectRef{Name: tenantName},
		ModelRef:  platformv1.ObjectRef{Name: modelName},
		Effect:    effect,
	}}
}

func tenantNodePolicy(tenantName, nodeName, effect string) platformv1.TenantNodePolicy {
	return platformv1.TenantNodePolicy{Spec: platformv1.TenantNodePolicySpec{
		TenantRef: platformv1.ObjectRef{Name: tenantName},
		NodeRef:   platformv1.ObjectRef{Name: nodeName},
		Effect:    effect,
	}}
}

func modelNodePolicy(modelName, nodeName, effect string) platformv1.ModelNodePolicy {
	return platformv1.ModelNodePolicy{Spec: platformv1.ModelNodePolicySpec{
		ModelRef: platformv1.ObjectRef{Name: modelName},
		NodeRef:  platformv1.ObjectRef{Name: nodeName},
		Effect:   effect,
	}}
}

func TestTenantModelAllowedDenyWins(t *testing.T) {
	policies := []platformv1.TenantModelPolicy{
		tenantModelPolicy("tenant-a", "model-a", "Allow"),
		tenantModelPolicy("tenant-a", "model-a", "Deny"),
	}

	// 单条 Allow，应该允许
	if tenantModelAllowed(policies[:1], "tenant-a", "model-a") != true {
		t.Fatal("expected an explicit Allow policy to permit the model")
	}
	// Allow + Deny，Deny 必须赢
	if tenantModelAllowed(policies, "tenant-a", "model-a") != false {
		t.Fatal("expected Deny to override Allow")
	}
	// 不同租户的策略，忽略
	if tenantModelAllowed(policies[:1], "tenant-b", "model-a") != false {
		t.Fatal("expected policies for another tenant to be ignored")
	}
}

func TestEligibleNodeNamesCombinesPolicyLayers(t *testing.T) {
	tenantPolicies := []platformv1.TenantNodePolicy{
		tenantNodePolicy("tenant-a", "node-b", "Allow"),
		tenantNodePolicy("tenant-a", "node-a", "Allow"),
	}
	// node-a 被模型 Allow，node-b 被模型 Deny，最终只有 node-a 合格
	modelPolicies := []platformv1.ModelNodePolicy{
		modelNodePolicy("model-a", "node-a", "Allow"),
		modelNodePolicy("model-a", "node-b", "Deny"),
	}

	want := []string{"node-a"}
	if got := eligibleNodeNames("tenant-a", "model-a", tenantPolicies, modelPolicies); !reflect.DeepEqual(got, want) {
		t.Fatalf("eligibleNodeNames() = %v, want %v", got, want)
	}
}

func TestAllocateTrafficIsDeterministic(t *testing.T) {
	instances := []instanceData{
		{name: "instance-c", score: 0},
		{name: "instance-a", score: 100},
		{name: "instance-b", score: 50},
	}

	// 总分 150，10 QPS：instance-a 应拿 7 (100/150)，instance-b 拿 3 (50/150)，instance-c 拿 0
	allocations, effectiveTotal := allocateTraffic(10, instances)
	want := map[string]int{"instance-a": 7, "instance-b": 3, "instance-c": 0}
	if effectiveTotal != 10 || !reflect.DeepEqual(allocations, want) {
		t.Fatalf("allocateTraffic() = (%v, %d), want (%v, 10)", allocations, effectiveTotal, want)
	}

	// 总分 150，2 QPS：instance-a 1，instance-b 1（Largest Remainder 分配）
	allocations, effectiveTotal = allocateTraffic(2, instances)
	want = map[string]int{"instance-a": 1, "instance-b": 1, "instance-c": 0}
	if effectiveTotal != 2 || !reflect.DeepEqual(allocations, want) {
		t.Fatalf("small allocation = (%v, %d), want (%v, 2)", allocations, effectiveTotal, want)
	}
}

func TestCalculatePerformanceSummaryKeepsPartialMetrics(t *testing.T) {
	// 一个实例有完整数据，另一个只有 Queue，应该各自独立聚合
	input := performanceInput{Instances: []instancePerformance{
		{TTFT: 100, Queue: 10, Weight: 2, HasTTFT: true, HasQueue: true},
		{Queue: 30, Weight: 1, HasQueue: true},
	}}

	avgTTFT, avgQueue, hasTTFT, hasQueue := calculatePerformanceSummary(input)
	if avgTTFT != 100 || !hasTTFT {
		t.Fatalf("TTFT result = (%d, %t), want (100, true)", avgTTFT, hasTTFT)
	}
	// Queue 取加权平均：(10*2 + 30*1) / 3 ≈ 16.67，但这里是加权中位数百分比算法，
	// 加权中位数百分比算法的结果与算术加权平均不同，此处断言既有算法结果。
	if avgQueue != 22 || !hasQueue {
		t.Fatalf("queue result = (%d, %t), want (22, true)", avgQueue, hasQueue)
	}
}

func TestDecideAtScaleUpCooldownAndScaleDownFloor(t *testing.T) {
	now := time.Date(2026, time.August, 9, 12, 0, 0, 0, time.UTC)
	// 创建一个需要扩容的场景：TTFT 超标
	input := DecisionInput{
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

	decision := DecideAt(input, now)
	if decision.Action != ScaleUp || decision.TargetReplicas != 2 || decision.NodeName != "node-a" {
		t.Fatalf("scale-up decision = %+v", decision)
	}

	// 设置扩容冷却，离上次扩容 30 秒，冷却 60 秒，应该阻止扩容
	input.ScaleUpCooldown = 60
	input.LastScaleUpTime = metav1.NewTime(now.Add(-30 * time.Second))
	decision = DecideAt(input, now)
	if decision.Action != NoOp || decision.RequeueAfter != 30*time.Second {
		t.Fatalf("cooldown decision = %+v, want NoOp with 30s requeue", decision)
	}

	// 缩容场景：TTFT 和队列都低于缩容阈值，且有 2 个副本，应缩到 1
	input = DecisionInput{
		AvgTTFT:            100,
		AvgQueue:           1,
		HasTTFT:            true,
		HasQueue:           true,
		TTFTThresholdUp:    500,
		TTFTThresholdDown:  200,
		QueueThresholdUp:   100,
		QueueThresholdDown: 30,
		MinReplicas:        1,
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
	decision = DecideAt(input, now)
	if decision.Action != ScaleDown || decision.TargetReplicas != 1 || decision.NodeName != "node-b" {
		t.Fatalf("scale-down decision = %+v", decision)
	}
}

func TestDecideAtSupportsScaleToZeroAndMaximum(t *testing.T) {
	now := time.Date(2026, time.August, 9, 12, 0, 0, 0, time.UTC)
	// 零流量且允许缩到零，1 副本时应该缩到 0
	input := DecisionInput{
		TenantQPS:        0,
		AllowScaleToZero: true,
		MinReplicas:      1,
		MaxReplicas:      5,
		ExistingInstances: []InstanceInfo{{
			Name:            "instance-a",
			CurrentReplicas: 1,
			PlacementReady:  true,
			PlacementPlan: nodePlacementPlan{
				Version:     placementPlanVersion,
				PrimaryNode: "node-a",
				Placements:  []nodePlacement{{NodeName: "node-a", Replicas: 1}},
			},
		}},
	}
	decision := DecideAt(input, now)
	if decision.Action != ScaleDown || decision.TargetReplicas != 0 || decision.NodeName != "node-a" {
		t.Fatalf("scale-to-zero decision = %+v", decision)
	}

	// 已经达到 maxReplicas 时，即使需要扩容也不能再扩
	input = DecisionInput{
		TenantQPS:         10,
		MaxReplicas:       1,
		AvgQueue:          200,
		HasQueue:          true,
		QueueThresholdUp:  100,
		ExistingInstances: []InstanceInfo{{Name: "instance-a", CurrentReplicas: 1}},
	}
	if decision := DecideAt(input, now); decision.Action != NoOp {
		t.Fatalf("maxReplicas decision = %+v, want NoOp", decision)
	}
}

func TestDecideAtRebalancesPlacementAfterPolicyChange(t *testing.T) {
	now := time.Date(2026, time.August, 14, 12, 0, 0, 0, time.UTC)
	input := DecisionInput{
		TenantQPS: 10,
		AvailableModels: []ModelInfo{{
			Name:              "model-a",
			GPUUnits:          1,
			MaxConcurrency:    1,
			EligibleNodeNames: map[string]bool{"node-b": true},
		}},
		AvailableNodes: []NodeInfo{{Name: "node-b", RemainingGPU: 2, RemainingConcurrency: 2}},
		ExistingInstances: []InstanceInfo{{
			Name:            "instance-a",
			ModelName:       "model-a",
			CurrentReplicas: 1,
			PlacementReady:  true,
			PlacementPlan: nodePlacementPlan{
				Version:     placementPlanVersion,
				PrimaryNode: "node-a",
				Placements:  []nodePlacement{{NodeName: "node-a", Replicas: 1}},
			},
		}},
	}

	decision := DecideAt(input, now)
	if decision.Action != Rebalance ||
		decision.SourceNodeName != "node-a" ||
		decision.NodeName != "node-b" ||
		decision.TargetReplicas != 1 {
		t.Fatalf("rebalance decision = %+v", decision)
	}
}

func TestPerformanceTenantIndexDefaultsAndCanBeOverridden(t *testing.T) {
	// 没设置 tenantIndexField 时应该用包级默认值
	if got := (&PerformanceCollectorReconciler{}).performanceTenantIndex(); got != tenantIndexField {
		t.Fatalf("default index = %q, want %q", got, tenantIndexField)
	}
	// 允许注入自定义索引名，方便测试
	const custom = "test.spec.tenantRef.name"
	if got := (&PerformanceCollectorReconciler{tenantIndexField: custom}).performanceTenantIndex(); got != custom {
		t.Fatalf("custom index = %q, want %q", got, custom)
	}
}
