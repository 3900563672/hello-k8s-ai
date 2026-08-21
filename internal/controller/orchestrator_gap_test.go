package controller

import (
	"context"
	"testing"
	"time"

	platformv1 "github.com/3900563672/hello-k8s-ai/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func sampleModel(name string, score, gpu, concurrency int) ModelInfo {
	return ModelInfo{
		Name: name, GPUUnits: gpu, AbsoluteScore: score, MaxConcurrency: concurrency,
		EligibleNodeNames: map[string]bool{"node-a": true, "node-b": true},
	}
}

func TestFindBestPlacement(t *testing.T) {
	models := []ModelInfo{
		sampleModel("model-high", 90, 4, 8),
		sampleModel("model-low", 40, 2, 4),
	}
	nodes := []NodeInfo{
		{Name: "node-a", RemainingGPU: 16, RemainingConcurrency: 64},
		{Name: "node-b", RemainingGPU: 4, RemainingConcurrency: 8},
	}
	instances := []InstanceInfo{
		{Name: "tenant-a-model-high", ModelName: "model-high", CurrentReplicas: 2, PlacementReady: true},
		{Name: "tenant-a-model-low", ModelName: "model-low", CurrentReplicas: 1, PlacementReady: true},
	}
	name, replicas, score, found := FindBestCandidate(models, nodes, instances)
	if !found || name != "tenant-a-model-high" || replicas != 3 || score != 90 {
		t.Fatalf("FindBestCandidate = %q %d %d %v", name, replicas, score, found)
	}

	// 资源不足的节点应被过滤：node-b 只有 2 GPU/4 并发，放不下 4 GPU/8 并发的 model-high。
	tight := []NodeInfo{{Name: "node-b", RemainingGPU: 2, RemainingConcurrency: 4}}
	name, _, _, found = FindBestCandidate(models, tight, instances)
	if !found || name != "tenant-a-model-low" {
		t.Fatalf("资源不足时应选 model-low: %q %v", name, found)
	}

	// 无实例 → 找不到
	if _, _, _, found := FindBestCandidate(models, nodes, nil); found {
		t.Fatal("无实例应找不到")
	}
	// 策略不允许 → 找不到
	blocked := []ModelInfo{sampleModel("model-blocked", 90, 4, 8)}
	blocked[0].EligibleNodeNames = map[string]bool{"node-x": true}
	if _, _, _, found := FindBestCandidate(blocked, nodes, instances); found {
		t.Fatal("策略不允许的模型应找不到")
	}
}

func TestBestEffectiveScoreForModel(t *testing.T) {
	model := sampleModel("model-a", 85, 4, 8)
	nodes := []NodeInfo{
		{Name: "node-a", RemainingGPU: 4, RemainingConcurrency: 8},
		{Name: "node-tiny", RemainingGPU: 1, RemainingConcurrency: 1},
	}
	score, found := bestEffectiveScoreForModel(model, nodes)
	if !found || score != 85 {
		t.Fatalf("bestEffectiveScoreForModel = %d %v", score, found)
	}
	blocked := sampleModel("model-b", 85, 4, 8)
	blocked.EligibleNodeNames = map[string]bool{"node-x": true}
	if _, found := bestEffectiveScoreForModel(blocked, nodes); found {
		t.Fatal("无可用节点应 not found")
	}
}

func TestNodePlacementContains(t *testing.T) {
	plan := nodePlacementPlan{Placements: []nodePlacement{{NodeName: "node-a"}}}
	if !nodePlacementContains(plan, "node-a") {
		t.Fatal("应包含 node-a")
	}
	if nodePlacementContains(plan, "node-b") {
		t.Fatal("不应包含 node-b")
	}
}

func TestDecisionTriggerIDStable(t *testing.T) {
	performance := &platformv1.TenantPerformance{
		ObjectMeta: metav1.ObjectMeta{Name: "perf", UID: types.UID("uid-1"), ResourceVersion: "3"},
	}
	tenant := &platformv1.Tenant{ObjectMeta: metav1.ObjectMeta{Name: "tenant-a", Generation: 2}}
	config := &platformv1.Orchestrator{ObjectMeta: metav1.ObjectMeta{Name: "orch", Generation: 1}}
	models := []ModelInfo{sampleModel("model-a", 90, 4, 8)}
	nodes := []NodeInfo{{Name: "node-a", RemainingGPU: 16, RemainingConcurrency: 64}}
	instances := []InstanceInfo{{Name: "inst-1", ModelName: "model-a", CurrentReplicas: 2}}
	first := decisionTriggerID(performance, tenant, config, models, nodes, instances)
	second := decisionTriggerID(performance, tenant, config, models, nodes, instances)
	if first != second {
		t.Fatal("相同输入应产生相同 triggerID")
	}
	models[0].AbsoluteScore = 50
	changed := decisionTriggerID(performance, tenant, config, models, nodes, instances)
	if first == changed {
		t.Fatal("模型变化应改变 triggerID")
	}
}

func TestNextOrchestratorRequeue(t *testing.T) {
	if got := nextOrchestratorRequeue(0); got != orchestratorSyncPeriod {
		t.Fatalf("nextOrchestratorRequeue(0) = %v", got)
	}
	if got := nextOrchestratorRequeue(time.Hour); got != orchestratorSyncPeriod {
		t.Fatalf("nextOrchestratorRequeue(1h) = %v", got)
	}
	if got := nextOrchestratorRequeue(5 * time.Second); got != 5*time.Second {
		t.Fatalf("nextOrchestratorRequeue(5s) = %v", got)
	}
}

func TestOrchestratorRequestsForTenant(t *testing.T) {
	scheme := newControllerTestScheme(t)
	orchestrator := &platformv1.Orchestrator{ObjectMeta: metav1.ObjectMeta{Name: "orch-a"}, Spec: platformv1.OrchestratorSpec{TenantRef: platformv1.ObjectRef{Name: "tenant-a"}}}
	other := &platformv1.Orchestrator{ObjectMeta: metav1.ObjectMeta{Name: "orch-b"}, Spec: platformv1.OrchestratorSpec{TenantRef: platformv1.ObjectRef{Name: "tenant-b"}}}
	kubernetesClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(orchestrator, other).
		WithIndex(&platformv1.Orchestrator{}, orchConfigTenantIndex, func(object client.Object) []string {
			return []string{object.(*platformv1.Orchestrator).Spec.TenantRef.Name}
		}).Build()
	reconciler := &OrchestratorReconciler{Client: kubernetesClient}
	requests := reconciler.mapNamedTenant(context.Background(), &platformv1.Tenant{ObjectMeta: metav1.ObjectMeta{Name: "tenant-a"}})
	if len(requests) != 1 || requests[0].Name != "orch-a" {
		t.Fatalf("mapNamedTenant = %+v", requests)
	}
	if requests := reconciler.mapNamedTenant(context.Background(), &platformv1.Tenant{ObjectMeta: metav1.ObjectMeta{Name: ""}}); len(requests) != 0 {
		t.Fatalf("空租户名应返回空: %+v", requests)
	}
	performance := &platformv1.TenantPerformance{Spec: platformv1.TenantPerformanceSpec{TenantRef: platformv1.ObjectRef{Name: "tenant-a"}}}
	requests = reconciler.mapReferencedTenant(context.Background(), performance)
	if len(requests) != 1 || requests[0].Name != "orch-a" {
		t.Fatalf("mapReferencedTenant(performance) = %+v", requests)
	}
	policy := &platformv1.TenantModelPolicy{Spec: platformv1.TenantModelPolicySpec{TenantRef: platformv1.ObjectRef{Name: "tenant-b"}}}
	requests = reconciler.mapReferencedTenant(context.Background(), policy)
	if len(requests) != 1 || requests[0].Name != "orch-b" {
		t.Fatalf("mapReferencedTenant(policy) = %+v", requests)
	}
	if requests := reconciler.mapReferencedTenant(context.Background(), &platformv1.Tenant{}); len(requests) != 0 {
		t.Fatalf("未知类型应返回空: %+v", requests)
	}
	_ = log.FromContext
}
