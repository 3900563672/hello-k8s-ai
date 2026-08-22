package controller

import (
	"context"
	"errors"
	"testing"

	platformv1 "github.com/3900563672/hello-k8s-ai/api/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// newOrchestratorMappingClient 构造带 Orchestrator/TenantPerformance 索引的 fake client。
func newOrchestratorMappingClient(t *testing.T, objects ...client.Object) client.Client {
	t.Helper()
	scheme := newControllerTestSchemeWithCore(t)
	builder := fake.NewClientBuilder().WithScheme(scheme).
		WithStatusSubresource(&platformv1.Orchestrator{}).
		WithIndex(&platformv1.Orchestrator{}, orchConfigTenantIndex, func(object client.Object) []string {
			return nonEmptyIndexValue(object.(*platformv1.Orchestrator).Spec.TenantRef.Name)
		}).
		WithIndex(&platformv1.TenantPerformance{}, orchPerformanceIndex, func(object client.Object) []string {
			return nonEmptyIndexValue(object.(*platformv1.TenantPerformance).Spec.TenantRef.Name)
		})
	return builder.WithObjects(objects...).Build()
}

func TestPerformanceForTenant(t *testing.T) {
	ctx := log.IntoContext(context.Background(), log.Log)

	t.Run("single", func(t *testing.T) {
		performance := &platformv1.TenantPerformance{
			ObjectMeta: metav1.ObjectMeta{Name: "perf-a"},
			Spec:       platformv1.TenantPerformanceSpec{TenantRef: platformv1.ObjectRef{Name: "tenant-a"}},
		}
		kc := newOrchestratorMappingClient(t, performance)
		reconciler := &OrchestratorReconciler{Client: kc}
		got, err := reconciler.performanceForTenant(ctx, "tenant-a")
		if err != nil {
			t.Fatalf("performanceForTenant: %v", err)
		}
		if got.Name != "perf-a" {
			t.Fatalf("got %q, want perf-a", got.Name)
		}
	})

	t.Run("missing", func(t *testing.T) {
		kc := newOrchestratorMappingClient(t)
		reconciler := &OrchestratorReconciler{Client: kc}
		_, err := reconciler.performanceForTenant(ctx, "tenant-a")
		if err == nil {
			t.Fatal("expected dependencyNotReadyError, got nil")
		}
		var notReady *dependencyNotReadyError
		if !errors.As(err, &notReady) {
			t.Fatalf("expected dependencyNotReadyError, got %T: %v", err, err)
		}
	})

	t.Run("multiple", func(t *testing.T) {
		one := &platformv1.TenantPerformance{
			ObjectMeta: metav1.ObjectMeta{Name: "perf-a"},
			Spec:       platformv1.TenantPerformanceSpec{TenantRef: platformv1.ObjectRef{Name: "tenant-a"}},
		}
		two := &platformv1.TenantPerformance{
			ObjectMeta: metav1.ObjectMeta{Name: "perf-b"},
			Spec:       platformv1.TenantPerformanceSpec{TenantRef: platformv1.ObjectRef{Name: "tenant-a"}},
		}
		kc := newOrchestratorMappingClient(t, one, two)
		reconciler := &OrchestratorReconciler{Client: kc}
		_, err := reconciler.performanceForTenant(ctx, "tenant-a")
		if err == nil {
			t.Fatal("expected error for multiple performances")
		}
	})
}

func TestSetOrchestratorConditions(t *testing.T) {
	ctx := log.IntoContext(context.Background(), log.Log)
	config := &platformv1.Orchestrator{
		ObjectMeta: metav1.ObjectMeta{Name: "orch-a", Generation: 3},
		Spec:       platformv1.OrchestratorSpec{TenantRef: platformv1.ObjectRef{Name: "tenant-a"}},
	}
	kc := newOrchestratorMappingClient(t, config)
	reconciler := &OrchestratorReconciler{Client: kc}

	if err := reconciler.setOrchestratorReadyCondition(ctx, "orch-a", metav1.ConditionTrue, "Reconciled", "ok"); err != nil {
		t.Fatalf("setOrchestratorReadyCondition: %v", err)
	}
	if err := reconciler.setOrchestratorResourceLimitedCondition(ctx, "orch-a", true, "Pressure", DecisionInput{
		AvailableNodes: []NodeInfo{
			{Name: "node-1", PhysicalPressure: true},
			{Name: "node-2", PhysicalPressure: false},
		},
	}); err != nil {
		t.Fatalf("setOrchestratorResourceLimitedCondition(true): %v", err)
	}
	var got platformv1.Orchestrator
	if err := kc.Get(ctx, client.ObjectKey{Name: "orch-a"}, &got); err != nil {
		t.Fatalf("get orchestrator: %v", err)
	}
	ready := meta.FindStatusCondition(got.Status.Conditions, conditionTypeReady)
	if ready == nil || ready.Status != metav1.ConditionTrue || ready.Reason != "ResourceLimited" {
		t.Fatalf("Ready condition = %+v", ready)
	}
	limited := meta.FindStatusCondition(got.Status.Conditions, conditionTypeResourceLimited)
	if limited == nil || limited.Status != metav1.ConditionTrue {
		t.Fatalf("ResourceLimited condition = %+v", limited)
	}
	if limited.Reason != "Pressure" {
		t.Fatalf("ResourceLimited reason = %q", limited.Reason)
	}

	if err := reconciler.setOrchestratorResourceLimitedCondition(ctx, "orch-a", false, "Recovered", DecisionInput{}); err != nil {
		t.Fatalf("setOrchestratorResourceLimitedCondition(false): %v", err)
	}
	if err := kc.Get(ctx, client.ObjectKey{Name: "orch-a"}, &got); err != nil {
		t.Fatalf("get orchestrator: %v", err)
	}
	if limited := meta.FindStatusCondition(got.Status.Conditions, conditionTypeResourceLimited); limited == nil || limited.Status != metav1.ConditionFalse {
		t.Fatalf("ResourceLimited condition after recovery = %+v", limited)
	}
}

func TestMapModelToOrchestrators(t *testing.T) {
	ctx := log.IntoContext(context.Background(), log.Log)
	config := &platformv1.Orchestrator{
		ObjectMeta: metav1.ObjectMeta{Name: "orch-a"},
		Spec:       platformv1.OrchestratorSpec{TenantRef: platformv1.ObjectRef{Name: "tenant-a"}},
	}
	policy := &platformv1.TenantModelPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "policy-a"},
		Spec: platformv1.TenantModelPolicySpec{
			TenantRef: platformv1.ObjectRef{Name: "tenant-a"},
			ModelRef:  platformv1.ObjectRef{Name: "model-1"},
			Effect:    "Allow",
		},
	}
	otherPolicy := &platformv1.TenantModelPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "policy-b"},
		Spec: platformv1.TenantModelPolicySpec{
			TenantRef: platformv1.ObjectRef{Name: "tenant-b"},
			ModelRef:  platformv1.ObjectRef{Name: "model-2"},
			Effect:    "Allow",
		},
	}
	kc := newOrchestratorMappingClient(t, config, policy, otherPolicy)
	reconciler := &OrchestratorReconciler{Client: kc}

	requests := reconciler.mapModelToOrchestrators(ctx, &platformv1.TenantModelPolicy{})
	if len(requests) != 0 {
		t.Fatalf("empty policy should map nothing, got %+v", requests)
	}

	requests = reconciler.mapModelToOrchestrators(ctx, &platformv1.ModelNodePolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "mnp-a"},
		Spec:       platformv1.ModelNodePolicySpec{ModelRef: platformv1.ObjectRef{Name: "model-1"}},
	})
	if len(requests) != 1 || requests[0].Name != "orch-a" {
		t.Fatalf("mapModelToOrchestrators = %+v", requests)
	}

	// policy-b 属于 tenant-b，没有对应 Orchestrator，因此不触发任何请求
	requests = reconciler.mapModelToOrchestrators(ctx, &platformv1.ModelNodePolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "mnp-b"},
		Spec:       platformv1.ModelNodePolicySpec{ModelRef: platformv1.ObjectRef{Name: "model-2"}},
	})
	if len(requests) != 0 {
		t.Fatalf("unexpected mapping for model-2: %+v", requests)
	}
}

func TestMapWorkerNodeToOrchestrators(t *testing.T) {
	ctx := log.IntoContext(context.Background(), log.Log)
	config := &platformv1.Orchestrator{
		ObjectMeta: metav1.ObjectMeta{Name: "orch-a"},
		Spec:       platformv1.OrchestratorSpec{TenantRef: platformv1.ObjectRef{Name: "tenant-a"}},
	}
	policy := &platformv1.TenantNodePolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "node-policy-a"},
		Spec: platformv1.TenantNodePolicySpec{
			TenantRef: platformv1.ObjectRef{Name: "tenant-a"},
			NodeRef:   platformv1.ObjectRef{Name: "node-1"},
			Effect:    "Allow",
		},
	}
	kc := newOrchestratorMappingClient(t, config, policy)
	reconciler := &OrchestratorReconciler{Client: kc}

	requests := reconciler.mapWorkerNodeToOrchestrators(ctx, &platformv1.WorkerNode{
		ObjectMeta: metav1.ObjectMeta{Name: "node-1"},
	})
	if len(requests) != 1 || requests[0].Name != "orch-a" {
		t.Fatalf("mapWorkerNodeToOrchestrators = %+v", requests)
	}

	requests = reconciler.mapWorkerNodeToOrchestrators(ctx, &platformv1.WorkerNode{
		ObjectMeta: metav1.ObjectMeta{Name: "node-other"},
	})
	if len(requests) != 0 {
		t.Fatalf("unrelated node should map nothing, got %+v", requests)
	}
}

func TestRemoveFinalizerBranches(t *testing.T) {
	ctx := log.IntoContext(context.Background(), log.Log)
	scheme := newControllerTestScheme(t)
	instance := &platformv1.SimulatorInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "inst-a", Finalizers: []string{"test-finalizer"}},
	}
	kc := fake.NewClientBuilder().WithScheme(scheme).WithObjects(instance).Build()

	// 无 finalizer 时直接成功，不写 API
	if err := removeFinalizer(ctx, kc, instance, "other-finalizer"); err != nil {
		t.Fatalf("removeFinalizer noop: %v", err)
	}
	// 实际移除
	if err := removeFinalizer(ctx, kc, instance, "test-finalizer"); err != nil {
		t.Fatalf("removeFinalizer: %v", err)
	}
	var got platformv1.SimulatorInstance
	if err := kc.Get(ctx, client.ObjectKey{Name: "inst-a"}, &got); err != nil {
		t.Fatalf("get: %v", err)
	}
	if len(got.Finalizers) != 0 {
		t.Fatalf("finalizers = %v", got.Finalizers)
	}
	// 对象不存在视为成功
	missing := &platformv1.SimulatorInstance{ObjectMeta: metav1.ObjectMeta{Name: "missing"}}
	if err := removeFinalizer(ctx, kc, missing, "test-finalizer"); err != nil {
		t.Fatalf("removeFinalizer missing: %v", err)
	}
}
