package controller

import (
	"context"
	"testing"

	platformv1 "github.com/3900563672/hello-k8s-ai/api/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func newControllerTestSchemeWithCore(t *testing.T) *runtime.Scheme {
	t.Helper()
	scheme := newControllerTestScheme(t)
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add core scheme: %v", err)
	}
	return scheme
}

func TestGatherDecisionInputFullPath(t *testing.T) {
	scheme := newControllerTestSchemeWithCore(t)
	performance := &platformv1.TenantPerformance{
		ObjectMeta: metav1.ObjectMeta{Name: "perf-a"},
		Spec:       platformv1.TenantPerformanceSpec{TenantRef: platformv1.ObjectRef{Name: "tenant-a"}},
		Status: platformv1.TenantPerformanceStatus{
			Phase: phaseRunning,
			Performance: platformv1.PerformanceStatus{
				AvgTTFT:  &platformv1.PerformanceMetric{Value: 200, Unit: "ms"},
				AvgQueue: &platformv1.PerformanceMetric{Value: 60, Unit: "req"},
			},
		},
	}
	tenant := &platformv1.Tenant{ObjectMeta: metav1.ObjectMeta{Name: "tenant-a"}, Spec: platformv1.TenantSpec{
		TTFTThresholdMs: 100, QueueThreshold: 50,
		TTFTScaleDownThresholdMs: 40, QueueScaleDownThreshold: 10,
		QPS: 100,
	}}
	orchestrator := &platformv1.Orchestrator{ObjectMeta: metav1.ObjectMeta{Name: "orch-a"},
		Spec: platformv1.OrchestratorSpec{TenantRef: platformv1.ObjectRef{Name: "tenant-a"}, MinReplicas: 1, MaxReplicas: 4}}
	model := &platformv1.Model{ObjectMeta: metav1.ObjectMeta{Name: "model-a"}, Spec: platformv1.ModelSpec{
		GPUUnits: 4, MaxConcurrency: 8, AbsoluteScore: 90,
	}}
	modelPolicy := &platformv1.TenantModelPolicy{ObjectMeta: metav1.ObjectMeta{Name: "policy-a"},
		Spec: platformv1.TenantModelPolicySpec{TenantRef: platformv1.ObjectRef{Name: "tenant-a"}, ModelRef: platformv1.ObjectRef{Name: "model-a"}, Effect: policyEffectAllow}}
	node := &platformv1.WorkerNode{ObjectMeta: metav1.ObjectMeta{Name: "node-a"}, Spec: platformv1.WorkerNodeSpec{
		GPU: 16, MaxConcurrency: 64,
	}}
	nodePolicy := &platformv1.TenantNodePolicy{ObjectMeta: metav1.ObjectMeta{Name: "node-policy-a"},
		Spec: platformv1.TenantNodePolicySpec{TenantRef: platformv1.ObjectRef{Name: "tenant-a"}, NodeRef: platformv1.ObjectRef{Name: "node-a"}, Effect: policyEffectAllow}}
	instance := &platformv1.SimulatorInstance{ObjectMeta: metav1.ObjectMeta{Name: "inst-a", Annotations: map[string]string{
		nodePlacementsAnnotation: `{"version":1,"primaryNode":"node-a","placements":[{"nodeName":"node-a","replicas":2}]}`,
	}}, Spec: platformv1.SimulatorInstanceSpec{
		TenantRef: platformv1.ObjectRef{Name: "tenant-a"},
		ModelRef:  platformv1.ObjectRef{Name: "model-a"},
		Replicas:  2,
	}}

	kubernetesClient := fake.NewClientBuilder().WithScheme(scheme).
		WithObjects(performance, tenant, orchestrator, model, modelPolicy, node, nodePolicy, instance).
		WithIndex(&platformv1.Orchestrator{}, orchConfigTenantIndex, func(object client.Object) []string {
			return []string{object.(*platformv1.Orchestrator).Spec.TenantRef.Name}
		}).
		WithIndex(&platformv1.TenantModelPolicy{}, orchPolicyIndex, func(object client.Object) []string {
			return []string{object.(*platformv1.TenantModelPolicy).Spec.TenantRef.Name}
		}).
		WithIndex(&platformv1.TenantNodePolicy{}, orchNodePolIndex, func(object client.Object) []string {
			return []string{object.(*platformv1.TenantNodePolicy).Spec.TenantRef.Name}
		}).
		WithIndex(&platformv1.SimulatorInstance{}, orchSimIndex, func(object client.Object) []string {
			return []string{object.(*platformv1.SimulatorInstance).Spec.TenantRef.Name}
		}).
		Build()
	reconciler := &OrchestratorReconciler{Client: kubernetesClient, Scheme: scheme}

	input, err := reconciler.gatherDecisionInput(context.Background(), "tenant-a", performance)
	if err != nil {
		t.Fatalf("gatherDecisionInput: %v", err)
	}
	if !input.HasTTFT || input.AvgTTFT != 200 || !input.HasQueue || input.AvgQueue != 60 {
		t.Fatalf("指标未收集: %+v", input)
	}
	if input.TTFTThresholdUp != 100 || input.TTFTThresholdDown != 40 || input.TenantQPS != 100 {
		t.Fatalf("阈值未收集: %+v", input)
	}
	if len(input.AvailableModels) != 1 || input.AvailableModels[0].Name != "model-a" {
		t.Fatalf("models = %+v", input.AvailableModels)
	}
	if len(input.AvailableNodes) != 1 || input.AvailableNodes[0].Name != "node-a" ||
		input.AvailableNodes[0].RemainingGPU != 8 || input.AvailableNodes[0].RemainingConcurrency != 48 {
		t.Fatalf("nodes = %+v", input.AvailableNodes)
	}
	if len(input.ExistingInstances) != 1 || input.ExistingInstances[0].Name != "inst-a" {
		t.Fatalf("instances = %+v", input.ExistingInstances)
	}
	if input.TriggerID == "" {
		t.Fatal("triggerID 不应为空")
	}
}

func TestGatherDecisionInputRejectsMismatchedPerformance(t *testing.T) {
	scheme := newControllerTestScheme(t)
	performance := &platformv1.TenantPerformance{
		ObjectMeta: metav1.ObjectMeta{Name: "perf-x"},
		Spec:       platformv1.TenantPerformanceSpec{TenantRef: platformv1.ObjectRef{Name: "tenant-other"}},
	}
	tenant := &platformv1.Tenant{ObjectMeta: metav1.ObjectMeta{Name: "tenant-a"}, Spec: platformv1.TenantSpec{
		TTFTThresholdMs: 100, QueueThreshold: 50,
		TTFTScaleDownThresholdMs: 40, QueueScaleDownThreshold: 10,
	}}
	kubernetesClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(performance, tenant).Build()
	reconciler := &OrchestratorReconciler{Client: kubernetesClient}
	if _, err := reconciler.gatherDecisionInput(context.Background(), "tenant-a", performance); err == nil {
		t.Fatal("性能数据租户不匹配应报错")
	}
}

func TestGatherDecisionInputRejectsMissingThresholds(t *testing.T) {
	scheme := newControllerTestScheme(t)
	performance := &platformv1.TenantPerformance{
		ObjectMeta: metav1.ObjectMeta{Name: "perf-a"},
		Spec:       platformv1.TenantPerformanceSpec{TenantRef: platformv1.ObjectRef{Name: "tenant-a"}},
	}
	tenant := &platformv1.Tenant{ObjectMeta: metav1.ObjectMeta{Name: "tenant-a"}}
	kubernetesClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(performance, tenant).Build()
	reconciler := &OrchestratorReconciler{Client: kubernetesClient}
	if _, err := reconciler.gatherDecisionInput(context.Background(), "tenant-a", performance); err == nil {
		t.Fatal("缺失阈值应报错")
	}
}

func TestGatherDecisionInputRejectsBadHysteresis(t *testing.T) {
	scheme := newControllerTestScheme(t)
	performance := &platformv1.TenantPerformance{
		ObjectMeta: metav1.ObjectMeta{Name: "perf-a"},
		Spec:       platformv1.TenantPerformanceSpec{TenantRef: platformv1.ObjectRef{Name: "tenant-a"}},
	}
	tenant := &platformv1.Tenant{ObjectMeta: metav1.ObjectMeta{Name: "tenant-a"}, Spec: platformv1.TenantSpec{
		TTFTThresholdMs: 100, QueueThreshold: 50,
		TTFTScaleDownThresholdMs: 150, QueueScaleDownThreshold: 10,
	}}
	kubernetesClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(performance, tenant).Build()
	reconciler := &OrchestratorReconciler{Client: kubernetesClient}
	if _, err := reconciler.gatherDecisionInput(context.Background(), "tenant-a", performance); err == nil {
		t.Fatal("缩容阈值高于扩容阈值应报错")
	}
}

func TestOrchestratorReconcileNotFoundAndDeleting(t *testing.T) {
	ctx := log.IntoContext(context.Background(), log.Log)
	scheme := newControllerTestScheme(t)
	kubernetesClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	reconciler := &OrchestratorReconciler{Client: kubernetesClient, Scheme: scheme}

	result, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKey{Name: "missing"}})
	if err != nil {
		t.Fatalf("NotFound Reconcile: %v", err)
	}
	if !result.IsZero() {
		t.Fatalf("NotFound 应返回零值: %+v", result)
	}

	now := metav1.Now()
	deleting := &platformv1.Orchestrator{ObjectMeta: metav1.ObjectMeta{
		Name: "deleting", DeletionTimestamp: &now, Finalizers: []string{"test-cleanup"},
	}, Spec: platformv1.OrchestratorSpec{TenantRef: platformv1.ObjectRef{Name: "tenant-a"}}}
	kubernetesClient = fake.NewClientBuilder().WithScheme(scheme).WithObjects(deleting).Build()
	reconciler = &OrchestratorReconciler{Client: kubernetesClient, Scheme: scheme}
	result, err = reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKey{Name: "deleting"}})
	if err != nil {
		t.Fatalf("deleting Reconcile: %v", err)
	}
	if !result.IsZero() {
		t.Fatalf("deleting 应返回零值: %+v", result)
	}
}
