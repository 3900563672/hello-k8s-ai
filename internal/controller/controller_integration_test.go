package controller

import (
	"context"
	"testing"
	"time"

	platformv1 "github.com/3900563672/hello-k8s-ai/api/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestTenantModelPolicyReconcilerCreatesDormantInstance(t *testing.T) {
	scheme := newControllerTestScheme(t)
	tenant := &platformv1.Tenant{ObjectMeta: metav1.ObjectMeta{Name: "tenant-a"}}
	model := &platformv1.Model{ObjectMeta: metav1.ObjectMeta{Name: "model-a"}}
	kubernetesClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(tenant, model).Build()
	reconciler := &TenantModelPolicyReconciler{Client: kubernetesClient, Scheme: scheme}
	policy := &platformv1.TenantModelPolicy{Spec: platformv1.TenantModelPolicySpec{
		TenantRef: platformv1.ObjectRef{Name: tenant.Name},
		ModelRef:  platformv1.ObjectRef{Name: model.Name},
		Effect:    "Allow",
	}}

	// 首次调用应创建一个休眠状态的实例（副本数为0且无流量）
	if err := reconciler.ensureSimulatorInstance(context.Background(), policy); err != nil {
		t.Fatalf("ensure SimulatorInstance: %v", err)
	}
	var instance platformv1.SimulatorInstance
	if err := kubernetesClient.Get(context.Background(), client.ObjectKey{Name: "tenant-a-model-a"}, &instance); err != nil {
		t.Fatalf("get SimulatorInstance: %v", err)
	}
	if instance.Spec.Replicas != 0 || instance.Spec.Traffic.QPS != 0 {
		t.Fatalf("initial spec is not dormant: %#v", instance.Spec)
	}
	// OwnerReference 必须指向 Tenant，而不是旧的 TenantModelPolicy
	if len(instance.OwnerReferences) != 1 || instance.OwnerReferences[0].Kind != "Tenant" {
		t.Fatalf("owner references = %#v, want Tenant owner", instance.OwnerReferences)
	}
}

func TestWorkerNodeUsageIsGlobalAndResetsUnusedNodes(t *testing.T) {
	scheme := newControllerTestScheme(t)
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add core scheme: %v", err)
	}
	model := &platformv1.Model{
		ObjectMeta: metav1.ObjectMeta{Name: "model-a"},
		Spec:       platformv1.ModelSpec{GPUUnits: 250, MaxConcurrency: 4},
	}
	nodeA := &platformv1.WorkerNode{ObjectMeta: metav1.ObjectMeta{Name: "node-a"}}
	nodeB := &platformv1.WorkerNode{
		ObjectMeta: metav1.ObjectMeta{Name: "node-b"},
		Status:     platformv1.WorkerNodeStatus{UsedGPU: 999, UsedConcurrency: 999},
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "simulator-pod",
			Namespace: "default",
			Labels:    map[string]string{instanceLabelKey: "instance-a"},
			Annotations: map[string]string{
				modelNameAnnotation: "model-a",
			},
		},
		Spec:   corev1.PodSpec{NodeName: "node-a"},
		Status: corev1.PodStatus{Phase: corev1.PodRunning},
	}
	kubernetesClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&platformv1.WorkerNode{}).
		WithObjects(model, nodeA, nodeB, pod).
		Build()
	reconciler := &WorkerNodeUsageReconciler{Client: kubernetesClient, Scheme: scheme}

	for _, test := range []struct {
		name            string
		wantGPU         int
		wantConcurrency int
	}{
		{name: "node-a", wantGPU: 250, wantConcurrency: 4},
		{name: "node-b", wantGPU: 0, wantConcurrency: 0},
	} {
		if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: client.ObjectKey{Name: test.name}}); err != nil {
			t.Fatalf("reconcile WorkerNode %q usage: %v", test.name, err)
		}
		var node platformv1.WorkerNode
		if err := kubernetesClient.Get(context.Background(), client.ObjectKey{Name: test.name}, &node); err != nil {
			t.Fatalf("get WorkerNode %q: %v", test.name, err)
		}
		if node.Status.UsedGPU != test.wantGPU || node.Status.UsedConcurrency != test.wantConcurrency {
			t.Fatalf("WorkerNode %q status = %#v", test.name, node.Status)
		}
	}
}

func TestScalingPlanIsIdempotentAcrossRetries(t *testing.T) {
	scheme := newControllerTestScheme(t)
	instance := &platformv1.SimulatorInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "instance-a"},
		Spec: platformv1.SimulatorInstanceSpec{
			TenantRef: platformv1.ObjectRef{Name: "tenant-a"},
			ModelRef:  platformv1.ObjectRef{Name: "model-a"},
			Replicas:  1,
			Traffic:   platformv1.TrafficSpec{QPS: 1},
		},
	}
	config := &platformv1.Orchestrator{
		ObjectMeta: metav1.ObjectMeta{Name: "config-a"},
		Spec:       platformv1.OrchestratorSpec{TenantRef: platformv1.ObjectRef{Name: "tenant-a"}},
	}
	kubernetesClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&platformv1.SimulatorInstance{}, &platformv1.Orchestrator{}).
		WithObjects(instance, config).
		Build()
	now := time.Date(2026, 8, 9, 12, 0, 0, 0, time.UTC)
	reconciler := &OrchestratorReconciler{Client: kubernetesClient, Scheme: scheme, Now: func() time.Time { return now }}
	decision := Decision{
		Action:           ScaleUp,
		InstanceName:     instance.Name,
		ObservedReplicas: 1,
		TargetReplicas:   2,
		EffectiveScore:   80,
	}
	input := DecisionInput{TenantName: "tenant-a", OrchestratorName: config.Name, TriggerID: "trigger-a"}

	// 第一次应用
	if err := reconciler.applyDecision(context.Background(), decision, input); err != nil {
		t.Fatalf("first apply: %v", err)
	}
	// 重试一次，应该幂等（不会再次扩缩）
	if err := reconciler.applyDecision(context.Background(), decision, input); err != nil {
		t.Fatalf("retry apply: %v", err)
	}

	var gotInstance platformv1.SimulatorInstance
	if err := kubernetesClient.Get(context.Background(), client.ObjectKey{Name: instance.Name}, &gotInstance); err != nil {
		t.Fatal(err)
	}
	if gotInstance.Spec.Replicas != 2 {
		t.Fatalf("replicas = %d, want exactly 2 after retry", gotInstance.Spec.Replicas)
	}
	// 待处理计划已经被清除
	if gotInstance.Annotations[pendingScalePlanKey] != "" {
		t.Fatalf("pending plan was not cleared: %#v", gotInstance.Annotations)
	}
	var gotConfig platformv1.Orchestrator
	if err := kubernetesClient.Get(context.Background(), client.ObjectKey{Name: config.Name}, &gotConfig); err != nil {
		t.Fatal(err)
	}
	if gotConfig.Status.LastScaling == nil || gotConfig.Status.LastScaling.OldReplicas != 1 || gotConfig.Status.LastScaling.NewReplicas != 2 {
		t.Fatalf("scaling record = %#v", gotConfig.Status.LastScaling)
	}
	if gotConfig.Status.LastScaleUpTime == nil || !gotConfig.Status.LastScaleUpTime.Equal(&metav1.Time{Time: now}) {
		t.Fatalf("lastScaleUpTime = %#v, want %s", gotConfig.Status.LastScaleUpTime, now)
	}
}

func TestPerformanceCollectorRejectsStaleSamples(t *testing.T) {
	scheme := newControllerTestScheme(t)
	now := time.Date(2026, time.August, 9, 12, 0, 0, 0, time.UTC)
	performance := &platformv1.TenantPerformance{
		ObjectMeta: metav1.ObjectMeta{Name: "tenant-a"},
		Spec: platformv1.TenantPerformanceSpec{
			TenantRef: platformv1.ObjectRef{Name: "tenant-a"},
		},
	}
	freshObservedAt := metav1.NewTime(now.Add(-time.Second))
	staleObservedAt := metav1.NewTime(now.Add(-instanceMetricMaxAge - time.Second))
	fresh := &platformv1.SimulatorInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "fresh"},
		Spec: platformv1.SimulatorInstanceSpec{
			TenantRef: platformv1.ObjectRef{Name: "tenant-a"}, Replicas: 2,
		},
		Status: platformv1.SimulatorInstanceStatus{
			Phase: "Running", AvailableReplicas: 2, ObservedAt: new(freshObservedAt),
			Performance: &platformv1.InstancePerformance{
				TTFT:  &platformv1.InstancePerformanceMetric{Value: 120, Unit: "ms"},
				Queue: &platformv1.InstancePerformanceMetric{Value: 8, Unit: "requests"},
			},
		},
	}
	stale := fresh.DeepCopy()
	stale.Name = "stale"
	stale.Status.ObservedAt = new(staleObservedAt)
	stale.Status.Performance.TTFT.Value = 9000
	stale.Status.Performance.Queue.Value = 9000

	kubernetesClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&platformv1.TenantPerformance{}).
		WithObjects(performance, fresh, stale).
		WithIndex(&platformv1.SimulatorInstance{}, tenantIndexField, func(obj client.Object) []string {
			return []string{obj.(*platformv1.SimulatorInstance).Spec.TenantRef.Name}
		}).
		Build()
	reconciler := &PerformanceCollectorReconciler{
		Client: kubernetesClient,
		Scheme: scheme,
		Now:    func() time.Time { return now },
	}
	if err := reconciler.recalculateTenantPerformance(context.Background(), "tenant-a"); err != nil {
		t.Fatalf("recalculate tenant performance: %v", err)
	}

	var got platformv1.TenantPerformance
	if err := kubernetesClient.Get(context.Background(), client.ObjectKey{Name: "tenant-a"}, &got); err != nil {
		t.Fatal(err)
	}
	// 只有一个新鲜样本，过期的应该被忽略
	if got.Status.Phase != "Running" || got.Status.SampleCount != 1 {
		t.Fatalf("status = %#v, want one running sample", got.Status)
	}
	if got.Status.Performance.AvgTTFT == nil || got.Status.Performance.AvgTTFT.Value != 120 ||
		got.Status.Performance.AvgQueue == nil || got.Status.Performance.AvgQueue.Value != 8 {
		t.Fatalf("aggregated performance = %#v", got.Status.Performance)
	}
	if got.Status.ObservedAt == nil || !got.Status.ObservedAt.Equal(new(freshObservedAt)) {
		t.Fatalf("observedAt = %#v, want %s", got.Status.ObservedAt, freshObservedAt.Time)
	}
}

func newControllerTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := platformv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add platform scheme: %v", err)
	}
	return scheme
}
