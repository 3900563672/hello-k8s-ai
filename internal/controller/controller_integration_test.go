package controller

import (
	"context"
	"reflect"
	"testing"
	"time"

	platformv1 "github.com/3900563672/hello-k8s-ai/api/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
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

func TestPlacementReservationCoversPodsNotYetMaterialized(t *testing.T) {
	scheme := newControllerTestScheme(t)
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add core scheme: %v", err)
	}
	payload, err := encodeNodePlacementPlan(nodePlacementPlan{
		Version:     placementPlanVersion,
		PrimaryNode: "node-a",
		Placements:  []nodePlacement{{NodeName: "node-a", Replicas: 2}},
	})
	if err != nil {
		t.Fatalf("encode placement: %v", err)
	}
	model := &platformv1.Model{
		ObjectMeta: metav1.ObjectMeta{Name: "model-a"},
		Spec:       platformv1.ModelSpec{GPUUnits: 2, MaxConcurrency: 3},
	}
	instance := &platformv1.SimulatorInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "instance-a",
			Annotations: map[string]string{nodePlacementsAnnotation: payload},
		},
		Spec: platformv1.SimulatorInstanceSpec{
			TenantRef: platformv1.ObjectRef{Name: "tenant-a"},
			ModelRef:  platformv1.ObjectRef{Name: model.Name},
			Replicas:  2,
		},
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "scheduled-replica",
			Namespace: defaultNamespace,
			Labels:    map[string]string{instanceLabelKey: labelValue(instance.Name)},
			Annotations: map[string]string{
				instanceNameAnnotation: instance.Name,
				modelNameAnnotation:    model.Name,
			},
		},
		Spec:   corev1.PodSpec{NodeName: "node-a"},
		Status: corev1.PodStatus{Phase: corev1.PodRunning},
	}
	kubernetesClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(model, instance, pod).
		Build()
	reconciler := &OrchestratorReconciler{Client: kubernetesClient, Scheme: scheme}

	usage, err := reconciler.collectExpectedNodeUsage(context.Background())
	if err != nil {
		t.Fatalf("collect expected node usage: %v", err)
	}
	if usage["node-a"].GPU != 4 || usage["node-a"].Concurrency != 6 {
		t.Fatalf("reserved usage = %#v, want GPU=4 concurrency=6", usage["node-a"])
	}
}

func TestScalingPlanIsIdempotentAcrossRetries(t *testing.T) {
	scheme := newControllerTestScheme(t)
	placementPayload, err := encodeNodePlacementPlan(nodePlacementPlan{
		Version:     placementPlanVersion,
		PrimaryNode: "node-a",
		Placements:  []nodePlacement{{NodeName: "node-a", Replicas: 1}},
	})
	if err != nil {
		t.Fatalf("encode initial placement: %v", err)
	}
	instance := &platformv1.SimulatorInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "instance-a",
			Annotations: map[string]string{nodePlacementsAnnotation: placementPayload},
		},
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
		NodeName:         "node-b",
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
	placementPlan, persisted, err := decodeNodePlacementPlan(gotInstance.Annotations[nodePlacementsAnnotation])
	if err != nil || !persisted {
		t.Fatalf("decode persisted placement plan: persisted=%t err=%v", persisted, err)
	}
	if placementPlan.PrimaryNode != "node-a" ||
		nodePlacementReplicaCount(placementPlan) != 2 ||
		len(placementPlan.Placements) != 2 {
		t.Fatalf("placement plan = %#v, want one replica on node-a and node-b", placementPlan)
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

func TestScalingPlanMigratesLegacyPodPlacement(t *testing.T) {
	scheme := newControllerTestScheme(t)
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add core scheme: %v", err)
	}
	instance := &platformv1.SimulatorInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "legacy-instance"},
		Spec: platformv1.SimulatorInstanceSpec{
			TenantRef: platformv1.ObjectRef{Name: "tenant-a"},
			ModelRef:  platformv1.ObjectRef{Name: "model-a"},
			Replicas:  1,
		},
	}
	config := &platformv1.Orchestrator{
		ObjectMeta: metav1.ObjectMeta{Name: "config-a"},
		Spec:       platformv1.OrchestratorSpec{TenantRef: platformv1.ObjectRef{Name: "tenant-a"}},
	}
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "legacy-pod",
			Namespace: defaultNamespace,
			Labels: map[string]string{
				managedByLabelKey: managedByLabelVal,
				instanceLabelKey:  labelValue(instance.Name),
			},
			Annotations: map[string]string{instanceNameAnnotation: instance.Name},
		},
		Spec:   corev1.PodSpec{NodeName: "node-a"},
		Status: corev1.PodStatus{Phase: corev1.PodRunning},
	}
	kubernetesClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&platformv1.SimulatorInstance{}, &platformv1.Orchestrator{}).
		WithObjects(instance, config, pod).
		Build()
	reconciler := &OrchestratorReconciler{Client: kubernetesClient, Scheme: scheme}
	decision := Decision{
		Action:           ScaleUp,
		InstanceName:     instance.Name,
		NodeName:         "node-b",
		ObservedReplicas: 1,
		TargetReplicas:   2,
		EffectiveScore:   50,
	}
	input := DecisionInput{TenantName: "tenant-a", OrchestratorName: config.Name, TriggerID: "legacy-migration"}
	if err := reconciler.applyDecision(context.Background(), decision, input); err != nil {
		t.Fatalf("apply legacy migration decision: %v", err)
	}

	var updated platformv1.SimulatorInstance
	if err := kubernetesClient.Get(context.Background(), client.ObjectKey{Name: instance.Name}, &updated); err != nil {
		t.Fatal(err)
	}
	plan, persisted, err := decodeNodePlacementPlan(updated.Annotations[nodePlacementsAnnotation])
	if err != nil || !persisted {
		t.Fatalf("decode migrated placement plan: persisted=%t err=%v", persisted, err)
	}
	if plan.PrimaryNode != "node-a" || len(plan.Placements) != 2 || nodePlacementReplicaCount(plan) != 2 {
		t.Fatalf("migrated placement plan = %#v", plan)
	}
}

func TestPlacementRebalanceKeepsReplicaCount(t *testing.T) {
	scheme := newControllerTestScheme(t)
	payload, err := encodeNodePlacementPlan(nodePlacementPlan{
		Version:     placementPlanVersion,
		PrimaryNode: "node-a",
		Placements:  []nodePlacement{{NodeName: "node-a", Replicas: 1}},
	})
	if err != nil {
		t.Fatalf("encode placement: %v", err)
	}
	instance := &platformv1.SimulatorInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "instance-a",
			Annotations: map[string]string{nodePlacementsAnnotation: payload},
		},
		Spec: platformv1.SimulatorInstanceSpec{
			TenantRef: platformv1.ObjectRef{Name: "tenant-a"},
			ModelRef:  platformv1.ObjectRef{Name: "model-a"},
			Replicas:  1,
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
	reconciler := &OrchestratorReconciler{Client: kubernetesClient, Scheme: scheme}
	decision := Decision{
		Action:           Rebalance,
		InstanceName:     instance.Name,
		SourceNodeName:   "node-a",
		NodeName:         "node-b",
		ObservedReplicas: 1,
		TargetReplicas:   1,
	}
	input := DecisionInput{TenantName: "tenant-a", OrchestratorName: config.Name, TriggerID: "rebalance-a-b"}
	if err := reconciler.applyDecision(context.Background(), decision, input); err != nil {
		t.Fatalf("apply rebalance: %v", err)
	}

	var updated platformv1.SimulatorInstance
	if err := kubernetesClient.Get(context.Background(), client.ObjectKey{Name: instance.Name}, &updated); err != nil {
		t.Fatal(err)
	}
	plan, persisted, err := decodeNodePlacementPlan(updated.Annotations[nodePlacementsAnnotation])
	if err != nil || !persisted {
		t.Fatalf("decode rebalanced plan: persisted=%t err=%v", persisted, err)
	}
	if updated.Spec.Replicas != 1 ||
		plan.PrimaryNode != "node-b" ||
		len(plan.Placements) != 1 ||
		plan.Placements[0].NodeName != "node-b" {
		t.Fatalf("rebalanced instance = replicas %d, plan %#v", updated.Spec.Replicas, plan)
	}
	var updatedConfig platformv1.Orchestrator
	if err := kubernetesClient.Get(context.Background(), client.ObjectKey{Name: config.Name}, &updatedConfig); err != nil {
		t.Fatal(err)
	}
	if updatedConfig.Status.LastScaling != nil {
		t.Fatalf("rebalance unexpectedly changed scaling history: %#v", updatedConfig.Status.LastScaling)
	}
}

func TestSimulatorInstancePlacementCreatesNodePinnedDeployments(t *testing.T) {
	scheme := newControllerTestScheme(t)
	if err := appsv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add apps scheme: %v", err)
	}
	placementPayload, err := encodeNodePlacementPlan(nodePlacementPlan{
		Version:     placementPlanVersion,
		PrimaryNode: "node-a",
		Placements: []nodePlacement{
			{NodeName: "node-a", Replicas: 1},
			{NodeName: "node-b", Replicas: 2},
		},
	})
	if err != nil {
		t.Fatalf("encode placement: %v", err)
	}
	instance := &platformv1.SimulatorInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "instance-a",
			Annotations: map[string]string{nodePlacementsAnnotation: placementPayload},
		},
		Spec: platformv1.SimulatorInstanceSpec{
			TenantRef: platformv1.ObjectRef{Name: "tenant-a"},
			ModelRef:  platformv1.ObjectRef{Name: "model-a"},
			Replicas:  3,
		},
	}
	kubernetesClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(instance).Build()
	reconciler := &SimulatorInstanceReconciler{Client: kubernetesClient, Scheme: scheme}

	state, err := reconciler.reconcileDeploymentObjects(
		context.Background(),
		instance,
		[]string{"node-a", "node-b"},
	)
	if err != nil {
		t.Fatalf("reconcile placement Deployments: %v", err)
	}
	if len(state.Deployments) != 2 || state.DesiredReplicas != 3 {
		t.Fatalf("deployment state = %#v, want two Deployments and three replicas", state)
	}

	assertPlacementDeployment := func(name, nodeName string, replicas int32) {
		t.Helper()
		var deployment appsv1.Deployment
		if err := kubernetesClient.Get(
			context.Background(),
			client.ObjectKey{Namespace: defaultNamespace, Name: name},
			&deployment,
		); err != nil {
			t.Fatalf("get Deployment %q: %v", name, err)
		}
		if deployment.Spec.Replicas == nil || *deployment.Spec.Replicas != replicas {
			t.Fatalf("Deployment %q replicas = %#v, want %d", name, deployment.Spec.Replicas, replicas)
		}
		if got := requiredAffinityNodes(&deployment); !reflect.DeepEqual(got, []string{nodeName}) {
			t.Fatalf("Deployment %q required nodes = %v, want [%s]", name, got, nodeName)
		}
		if deployment.Spec.Template.Annotations[placementNodeAnnotation] != nodeName {
			t.Fatalf("Deployment %q placement annotation = %q", name, deployment.Spec.Template.Annotations[placementNodeAnnotation])
		}
	}
	assertPlacementDeployment(deploymentName(instance.Name), "node-a", 1)
	secondaryName := placementDeploymentName(instance.Name, "node-b")
	assertPlacementDeployment(secondaryName, "node-b", 2)

	// Policy 收窄后，已存在的失效节点 Deployment 只能缩小，不能在失效节点增加副本。
	placementPayload, err = encodeNodePlacementPlan(nodePlacementPlan{
		Version:     placementPlanVersion,
		PrimaryNode: "node-a",
		Placements: []nodePlacement{
			{NodeName: "node-a", Replicas: 2},
			{NodeName: "node-b", Replicas: 1},
		},
	})
	if err != nil {
		t.Fatalf("encode draining placement: %v", err)
	}
	var updated platformv1.SimulatorInstance
	if err := kubernetesClient.Get(context.Background(), client.ObjectKey{Name: instance.Name}, &updated); err != nil {
		t.Fatal(err)
	}
	updated.Annotations[nodePlacementsAnnotation] = placementPayload
	if err := kubernetesClient.Update(context.Background(), &updated); err != nil {
		t.Fatalf("update draining placement: %v", err)
	}
	if _, err := reconciler.reconcileDeploymentObjects(context.Background(), &updated, []string{"node-a"}); err != nil {
		t.Fatalf("reconcile draining placement: %v", err)
	}
	assertPlacementDeployment(deploymentName(instance.Name), "node-a", 2)
	assertPlacementDeployment(secondaryName, "node-b", 1)
	placementPayload, err = encodeNodePlacementPlan(nodePlacementPlan{
		Version:     placementPlanVersion,
		PrimaryNode: "node-a",
		Placements: []nodePlacement{
			{NodeName: "node-a", Replicas: 1},
			{NodeName: "node-b", Replicas: 2},
		},
	})
	if err != nil {
		t.Fatalf("encode invalid expansion placement: %v", err)
	}
	if err := kubernetesClient.Get(context.Background(), client.ObjectKey{Name: instance.Name}, &updated); err != nil {
		t.Fatal(err)
	}
	updated.Annotations[nodePlacementsAnnotation] = placementPayload
	if err := kubernetesClient.Update(context.Background(), &updated); err != nil {
		t.Fatalf("update invalid expansion placement: %v", err)
	}
	if _, err := reconciler.reconcileDeploymentObjects(context.Background(), &updated, []string{"node-a"}); err == nil {
		t.Fatal("reconcile increased replicas on a policy-denied node")
	}

	// 缩容计划移除 node-b 后，多余的节点 Deployment 必须被清理。
	placementPayload, err = encodeNodePlacementPlan(nodePlacementPlan{
		Version:     placementPlanVersion,
		PrimaryNode: "node-a",
		Placements:  []nodePlacement{{NodeName: "node-a", Replicas: 1}},
	})
	if err != nil {
		t.Fatalf("encode reduced placement: %v", err)
	}
	if err := kubernetesClient.Get(context.Background(), client.ObjectKey{Name: instance.Name}, &updated); err != nil {
		t.Fatal(err)
	}
	updated.Spec.Replicas = 1
	updated.Annotations[nodePlacementsAnnotation] = placementPayload
	if err := kubernetesClient.Update(context.Background(), &updated); err != nil {
		t.Fatalf("update reduced placement: %v", err)
	}
	if _, err := reconciler.reconcileDeploymentObjects(context.Background(), &updated, []string{"node-a", "node-b"}); err != nil {
		t.Fatalf("reconcile reduced placement: %v", err)
	}
	var obsolete appsv1.Deployment
	if err := kubernetesClient.Get(
		context.Background(),
		client.ObjectKey{Namespace: defaultNamespace, Name: secondaryName},
		&obsolete,
	); !apierrors.IsNotFound(err) {
		t.Fatalf("obsolete placement Deployment still exists or lookup failed: %v", err)
	}
}

func TestSimulatorInstanceLegacyDeploymentRemainsCompatible(t *testing.T) {
	scheme := newControllerTestScheme(t)
	if err := appsv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add apps scheme: %v", err)
	}
	instance := &platformv1.SimulatorInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "legacy-instance"},
		Spec: platformv1.SimulatorInstanceSpec{
			TenantRef: platformv1.ObjectRef{Name: "tenant-a"},
			ModelRef:  platformv1.ObjectRef{Name: "model-a"},
			Replicas:  2,
		},
	}
	kubernetesClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(instance).Build()
	reconciler := &SimulatorInstanceReconciler{Client: kubernetesClient, Scheme: scheme}

	state, err := reconciler.reconcileDeploymentObjects(
		context.Background(),
		instance,
		[]string{"node-b", "node-a"},
	)
	if err != nil {
		t.Fatalf("reconcile legacy Deployment: %v", err)
	}
	if len(state.Deployments) != 1 {
		t.Fatalf("legacy deployment count = %d, want 1", len(state.Deployments))
	}
	if got := requiredAffinityNodes(state.Deployments[0]); !reflect.DeepEqual(got, []string{"node-a", "node-b"}) {
		t.Fatalf("legacy required nodes = %v, want both eligible nodes", got)
	}
}

func TestTenantRuntimeAggregatesAllPlannedDeployments(t *testing.T) {
	scheme := newControllerTestScheme(t)
	if err := appsv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add apps scheme: %v", err)
	}
	payload, err := encodeNodePlacementPlan(nodePlacementPlan{
		Version:     placementPlanVersion,
		PrimaryNode: "node-a",
		Placements: []nodePlacement{
			{NodeName: "node-a", Replicas: 1},
			{NodeName: "node-b", Replicas: 2},
		},
	})
	if err != nil {
		t.Fatalf("encode placement: %v", err)
	}
	tenant := &platformv1.Tenant{ObjectMeta: metav1.ObjectMeta{Name: "tenant-a"}}
	instance := &platformv1.SimulatorInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "instance-a",
			Annotations: map[string]string{nodePlacementsAnnotation: payload},
		},
		Spec: platformv1.SimulatorInstanceSpec{
			TenantRef: platformv1.ObjectRef{Name: tenant.Name},
			ModelRef:  platformv1.ObjectRef{Name: "model-a"},
			Replicas:  3,
		},
	}
	deploymentForRuntime := func(name string, desired, available int32) *appsv1.Deployment {
		return &appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: defaultNamespace,
				Labels: map[string]string{
					managedByLabelKey: managedByLabelVal,
					instanceLabelKey:  labelValue(instance.Name),
				},
				Annotations: map[string]string{instanceNameAnnotation: instance.Name},
			},
			Spec:   appsv1.DeploymentSpec{Replicas: new(desired)},
			Status: appsv1.DeploymentStatus{AvailableReplicas: available},
		}
	}
	primary := deploymentForRuntime(deploymentName(instance.Name), 1, 1)
	secondary := deploymentForRuntime(placementDeploymentName(instance.Name, "node-b"), 2, 2)
	obsolete := deploymentForRuntime("obsolete-placement", 10, 10)
	kubernetesClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&platformv1.TenantRuntime{}, &appsv1.Deployment{}).
		WithObjects(tenant, instance, primary, secondary, obsolete).
		WithIndex(&platformv1.SimulatorInstance{}, simInstanceTenantIndex, func(obj client.Object) []string {
			return []string{obj.(*platformv1.SimulatorInstance).Spec.TenantRef.Name}
		}).
		Build()
	reconciler := &SimulatorInstanceReconciler{Client: kubernetesClient, Scheme: scheme}
	if err := reconciler.reconcileTenantRuntime(context.Background(), tenant.Name); err != nil {
		t.Fatalf("reconcile TenantRuntime: %v", err)
	}

	var runtimeObject platformv1.TenantRuntime
	if err := kubernetesClient.Get(context.Background(), client.ObjectKey{Name: tenant.Name}, &runtimeObject); err != nil {
		t.Fatal(err)
	}
	if runtimeObject.Status.InstanceCount != 3 || runtimeObject.Status.Phase != phaseRunning {
		t.Fatalf("TenantRuntime status = %#v, want three available replicas", runtimeObject.Status)
	}
}

func TestSimulatorInstanceRejectsPlacementReplicaMismatch(t *testing.T) {
	scheme := newControllerTestScheme(t)
	if err := appsv1.AddToScheme(scheme); err != nil {
		t.Fatalf("add apps scheme: %v", err)
	}
	payload, err := encodeNodePlacementPlan(nodePlacementPlan{
		Version:     placementPlanVersion,
		PrimaryNode: "node-a",
		Placements:  []nodePlacement{{NodeName: "node-a", Replicas: 1}},
	})
	if err != nil {
		t.Fatalf("encode placement: %v", err)
	}
	instance := &platformv1.SimulatorInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "mismatched-instance",
			Annotations: map[string]string{nodePlacementsAnnotation: payload},
		},
		Spec: platformv1.SimulatorInstanceSpec{
			TenantRef: platformv1.ObjectRef{Name: "tenant-a"},
			ModelRef:  platformv1.ObjectRef{Name: "model-a"},
			Replicas:  2,
		},
	}
	kubernetesClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(instance).Build()
	reconciler := &SimulatorInstanceReconciler{Client: kubernetesClient, Scheme: scheme}
	if _, err := reconciler.reconcileDeploymentObjects(
		context.Background(),
		instance,
		[]string{"node-a"},
	); err == nil {
		t.Fatal("reconcile succeeded with a mismatched placement plan")
	}
	var deployments appsv1.DeploymentList
	if err := kubernetesClient.List(context.Background(), &deployments); err != nil {
		t.Fatal(err)
	}
	if len(deployments.Items) != 0 {
		t.Fatalf("created %d Deployments from an invalid plan", len(deployments.Items))
	}
}

func requiredAffinityNodes(deployment *appsv1.Deployment) []string {
	if deployment == nil ||
		deployment.Spec.Template.Spec.Affinity == nil ||
		deployment.Spec.Template.Spec.Affinity.NodeAffinity == nil ||
		deployment.Spec.Template.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution == nil {
		return nil
	}
	terms := deployment.Spec.Template.Spec.Affinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms
	if len(terms) == 0 || len(terms[0].MatchExpressions) == 0 {
		return nil
	}
	return append([]string(nil), terms[0].MatchExpressions[0].Values...)
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
