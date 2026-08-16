package controller

import (
	"context"
	"testing"

	platformv1 "github.com/3900563672/hello-k8s-ai/api/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// 已调度 Pod 的事件只入队其所在节点，避免每次 Pod 事件都触发全部 WorkerNode。
func TestWorkerNodePodEventMapsToScheduledNodeOnly(t *testing.T) {
	scheme := newControllerTestScheme(t)
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add core scheme: %v", err)
	}
	nodeA := &platformv1.WorkerNode{ObjectMeta: metav1.ObjectMeta{Name: "node-a"}}
	nodeB := &platformv1.WorkerNode{ObjectMeta: metav1.ObjectMeta{Name: "node-b"}}
	scheduledPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "simulator-pod", Namespace: "default"},
		Spec:       corev1.PodSpec{NodeName: "node-a"},
	}
	unscheduledPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "pending-pod", Namespace: "default"},
	}
	model := &platformv1.Model{ObjectMeta: metav1.ObjectMeta{Name: "model-a"}}
	kubernetesClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(nodeA, nodeB, scheduledPod, unscheduledPod, model).
		Build()
	reconciler := &WorkerNodeUsageReconciler{Client: kubernetesClient}

	got := reconciler.allWorkerNodeRequests(context.Background(), scheduledPod)
	if len(got) != 1 || got[0].Name != "node-a" {
		t.Fatalf("scheduled pod requests = %v, want [node-a]", got)
	}

	got = reconciler.allWorkerNodeRequests(context.Background(), unscheduledPod)
	if len(got) != 2 {
		t.Fatalf("unscheduled pod requests = %v, want both nodes", got)
	}

	got = reconciler.allWorkerNodeRequests(context.Background(), model)
	if len(got) != 2 {
		t.Fatalf("model event requests = %v, want both nodes", got)
	}
}

// calculateNodeUsage 只统计调度到目标节点的 Pod（走 NodeName 索引过滤）。
func TestCalculateNodeUsageFiltersByNodeIndex(t *testing.T) {
	scheme := newControllerTestScheme(t)
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("add core scheme: %v", err)
	}
	model := &platformv1.Model{
		ObjectMeta: metav1.ObjectMeta{Name: "model-a"},
		Spec:       platformv1.ModelSpec{GPUUnits: 250, MaxConcurrency: 4},
	}
	podOnA := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "simulator-pod-a",
			Namespace: "default",
			Annotations: map[string]string{
				instanceNameAnnotation: "instance-a",
				modelNameAnnotation:    "model-a",
			},
		},
		Spec:   corev1.PodSpec{NodeName: "node-a"},
		Status: corev1.PodStatus{Phase: corev1.PodRunning},
	}
	podOnB := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "simulator-pod-b",
			Namespace: "default",
			Annotations: map[string]string{
				instanceNameAnnotation: "instance-b",
				modelNameAnnotation:    "model-a",
			},
		},
		Spec:   corev1.PodSpec{NodeName: "node-b"},
		Status: corev1.PodStatus{Phase: corev1.PodRunning},
	}
	kubernetesClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(model, podOnA, podOnB).
		WithIndex(&corev1.Pod{}, podNodeNameIndex, func(obj client.Object) []string {
			return []string{obj.(*corev1.Pod).Spec.NodeName}
		}).
		Build()
	reconciler := &WorkerNodeUsageReconciler{Client: kubernetesClient}

	gpuA, concurrencyA, err := reconciler.calculateNodeUsage(context.Background(), "node-a")
	if err != nil {
		t.Fatalf("calculate node-a usage: %v", err)
	}
	if gpuA != 250 || concurrencyA != 4 {
		t.Fatalf("node-a usage = (%d, %d), want (250, 4)", gpuA, concurrencyA)
	}

	gpuB, concurrencyB, err := reconciler.calculateNodeUsage(context.Background(), "node-b")
	if err != nil {
		t.Fatalf("calculate node-b usage: %v", err)
	}
	if gpuB != 250 || concurrencyB != 4 {
		t.Fatalf("node-b usage = (%d, %d), want (250, 4)", gpuB, concurrencyB)
	}

	gpuMissing, concurrencyMissing, err := reconciler.calculateNodeUsage(context.Background(), "node-missing")
	if err != nil {
		t.Fatalf("calculate missing node usage: %v", err)
	}
	if gpuMissing != 0 || concurrencyMissing != 0 {
		t.Fatalf("missing node usage = (%d, %d), want (0, 0)", gpuMissing, concurrencyMissing)
	}
}
