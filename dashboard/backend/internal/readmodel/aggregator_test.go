package readmodel

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/config"
	dashboardkubernetes "github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/kubernetes"
	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/model"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	coordinationv1 "k8s.io/api/coordination/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	kubernetesfake "k8s.io/client-go/kubernetes/fake"
)

func TestAggregatorBuildsTrafficAndWorkloadReadModelsFromInformerCache(t *testing.T) {
	now := time.Date(2026, time.August, 12, 12, 0, 0, 0, time.UTC)
	objects := []runtime.Object{
		platformObject("Tenant", "tenant-a", map[string]any{
			"displayName": "Tenant A", "priority": "High", "qps": int64(20),
		}, nil),
		platformObject("Model", "model-a", map[string]any{
			"displayName": "Model A", "gpuUnits": int64(1), "maxConcurrency": int64(8), "coldStartMs": int64(100),
		}, nil),
		platformObject("WorkerNode", "node-a", map[string]any{
			"displayName": "Node A", "gpu": int64(4), "maxConcurrency": int64(32),
		}, map[string]any{"usedGPU": int64(1), "usedConcurrency": int64(8)}),
		platformObject("SimulatorInstance", "tenant-a-model-a", map[string]any{
			"tenantRef": map[string]any{"name": "tenant-a"},
			"modelRef":  map[string]any{"name": "model-a"},
			"replicas":  int64(2),
			"traffic":   map[string]any{"qps": int64(20)},
		}, map[string]any{
			"availableReplicas": int64(2), "phase": "Running", "score": int64(40),
			"observedAt": now.Format(time.RFC3339Nano),
		}),
		platformObject("TenantPerformance", "tenant-a", map[string]any{
			"tenantRef": map[string]any{"name": "tenant-a"},
		}, map[string]any{
			"phase": "Running", "sampleCount": int64(1), "observedAt": now.Format(time.RFC3339Nano),
			"performance": map[string]any{
				"avgTTFT":  map[string]any{"value": int64(210), "unit": "ms"},
				"avgQueue": map[string]any{"value": int64(3), "unit": "requests"},
			},
		}),
		platformObject("TenantRuntime", "tenant-a", map[string]any{
			"tenantRef": map[string]any{"name": "tenant-a"},
		}, map[string]any{"instanceCount": int64(2), "phase": "Running"}),
	}

	listKinds := make(map[schema.GroupVersionResource]string, len(dashboardkubernetes.PlatformResources))
	for _, descriptor := range dashboardkubernetes.PlatformResources {
		listKinds[descriptor.GVR] = descriptor.Kind + "List"
	}
	dynamicClient := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), listKinds, objects...)
	replicas := int32(2)
	typedClient := kubernetesfake.NewSimpleClientset(
		&corev1.Node{
			ObjectMeta: metav1.ObjectMeta{Name: "node-a"},
			Status: corev1.NodeStatus{Conditions: []corev1.NodeCondition{{
				Type: corev1.NodeReady, Status: corev1.ConditionTrue,
			}}},
		},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name: "simulator-a", Namespace: "hello-k8s-ai-system",
				Annotations: map[string]string{
					"platform.study.com/instance-name": "tenant-a-model-a",
					"platform.study.com/tenant-name":   "tenant-a",
					"platform.study.com/model-name":    "model-a",
				},
			},
			Spec: corev1.PodSpec{NodeName: "node-a"},
			Status: corev1.PodStatus{Phase: corev1.PodRunning, Conditions: []corev1.PodCondition{{
				Type: corev1.PodReady, Status: corev1.ConditionTrue,
			}}},
		},
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name: "simulator-a", Namespace: "hello-k8s-ai-system",
				Annotations: map[string]string{"platform.study.com/instance-name": "tenant-a-model-a"},
			},
			Spec:   appsv1.DeploymentSpec{Replicas: &replicas},
			Status: appsv1.DeploymentStatus{ReadyReplicas: 2, AvailableReplicas: 2, UpdatedReplicas: 2},
		},
		&coordinationv1.Lease{
			ObjectMeta: metav1.ObjectMeta{Name: "simulator-a", Namespace: "hello-k8s-ai-system"},
			Spec: coordinationv1.LeaseSpec{
				HolderIdentity:       ptr("simulator-a-controller"),
				LeaseDurationSeconds: ptr(int32(15)),
				RenewTime:            ptr(metav1.NewMicroTime(now.Add(-3 * time.Second))),
			},
		},
	)
	events := make([]runtime.Object, 0, 501)
	for index := 0; index < 501; index++ {
		events = append(events, &corev1.Event{
			ObjectMeta: metav1.ObjectMeta{
				Name: fmt.Sprintf("event-%d", index), Namespace: "hello-k8s-ai-system",
			},
			Type:    corev1.EventTypeWarning,
			Reason:  "BackOff",
			Message: "restarting failed container",
			Count:   int32(index + 1),
		})
	}
	typedClient = kubernetesfake.NewSimpleClientset(append([]runtime.Object{
		&corev1.Node{
			ObjectMeta: metav1.ObjectMeta{Name: "node-a"},
			Status: corev1.NodeStatus{Conditions: []corev1.NodeCondition{{
				Type: corev1.NodeReady, Status: corev1.ConditionTrue,
			}}},
		},
		&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name: "simulator-a", Namespace: "hello-k8s-ai-system",
				Annotations: map[string]string{
					"platform.study.com/instance-name": "tenant-a-model-a",
					"platform.study.com/tenant-name":   "tenant-a",
					"platform.study.com/model-name":    "model-a",
				},
			},
			Spec: corev1.PodSpec{NodeName: "node-a"},
			Status: corev1.PodStatus{Phase: corev1.PodRunning, Conditions: []corev1.PodCondition{{
				Type: corev1.PodReady, Status: corev1.ConditionTrue,
			}}},
		},
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name: "simulator-a", Namespace: "hello-k8s-ai-system",
				Annotations: map[string]string{"platform.study.com/instance-name": "tenant-a-model-a"},
			},
			Spec:   appsv1.DeploymentSpec{Replicas: &replicas},
			Status: appsv1.DeploymentStatus{ReadyReplicas: 2, AvailableReplicas: 2, UpdatedReplicas: 2},
		},
		&coordinationv1.Lease{
			ObjectMeta: metav1.ObjectMeta{Name: "simulator-a", Namespace: "hello-k8s-ai-system"},
			Spec: coordinationv1.LeaseSpec{
				HolderIdentity:       ptr("simulator-a-controller"),
				LeaseDurationSeconds: ptr(int32(15)),
				RenewTime:            ptr(metav1.NewMicroTime(now.Add(-3 * time.Second))),
			},
		},
	}, events...)...)
	clients := &dashboardkubernetes.Clients{
		Kubernetes: typedClient,
		Dynamic:    dynamicClient,
		Context:    "test",
		Version:    "v1.36.0",
	}
	cacheState, err := dashboardkubernetes.NewCache(clients, config.KubernetesConfig{
		ResyncPeriod: time.Hour, CacheSyncTimeout: 5 * time.Second,
	}, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
	if err != nil {
		t.Fatalf("create informer cache: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = cacheState.Run(ctx) }()
	waitContext, waitCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer waitCancel()
	if err := cacheState.WaitUntilSynced(waitContext); err != nil {
		t.Fatalf("wait for informer cache: %v", err)
	}

	aggregator := NewAggregator(cacheState)
	configuration := aggregator.Configuration(now)
	if len(configuration.Tenants) != 1 || len(configuration.SimulatorInstances) != 1 {
		t.Fatalf("platform resources were not aggregated: %#v", configuration)
	}
	if configuration.Tenants[0].Derived["readyReplicaCount"] != 2 {
		t.Fatalf("TenantRuntime ready replicas were not derived: %#v", configuration.Tenants[0].Derived)
	}
	traffic := aggregator.Traffic(now)
	if len(traffic.Tenants) != 1 || traffic.Tenants[0].AllocatedQPS != 20 || traffic.Tenants[0].ReadyReplicaCount != 2 {
		t.Fatalf("traffic read model is wrong: %#v", traffic)
	}
	if traffic.Tenants[0].Performance.AvgTTFT == nil || traffic.Tenants[0].Performance.AvgTTFT.Value != 210 {
		t.Fatalf("TenantPerformance was not aggregated: %#v", traffic.Tenants[0].Performance)
	}
	if len(traffic.Tenants[0].Instances) != 1 || len(traffic.Tenants[0].Instances[0].Pods) != 1 {
		t.Fatalf("SimulatorInstance workload linkage is wrong: %#v", traffic.Tenants[0].Instances)
	}
	workloads := aggregator.Workloads(now)
	if len(workloads.Nodes) != 1 || !workloads.Nodes[0].Ready || len(workloads.Deployments) != 1 || len(workloads.Pods) != 1 {
		t.Fatalf("native Kubernetes workloads were not aggregated: %#v", workloads)
	}
	if len(workloads.Leases) != 1 || workloads.Leases[0].HolderIdentity != "simulator-a-controller" {
		t.Fatalf("Lease was not aggregated: %#v", workloads.Leases)
	}
	if len(workloads.Events) != 500 || workloads.Events[0].Type != corev1.EventTypeWarning {
		t.Fatalf("Events were not aggregated/truncated to 500: %d", len(workloads.Events))
	}
	snapshot := aggregator.CurrentSnapshot(now)
	if snapshot.CapturedAt != now || len(snapshot.Configuration.Tenants) != 1 || len(snapshot.Traffic.Tenants) != 1 || len(snapshot.Workloads.Nodes) != 1 {
		t.Fatalf("CurrentSnapshot composition is wrong: %#v", snapshot)
	}
}

func TestCountsComputesSummary(t *testing.T) {
	configuration := model.Configuration{
		Tenants:            []model.PlatformResource{{Ref: model.ResourceRef{Name: "tenant-a"}}, {Ref: model.ResourceRef{Name: "tenant-b"}}},
		Models:             []model.PlatformResource{{Ref: model.ResourceRef{Name: "model-a"}}},
		WorkerNodes:        []model.PlatformResource{{Ref: model.ResourceRef{Name: "node-a"}}},
		SimulatorInstances: []model.PlatformResource{{Ref: model.ResourceRef{Name: "tenant-a-model-a"}}},
	}
	traffic := model.Traffic{Tenants: []model.TenantTraffic{
		{ReadyReplicaCount: 3}, {ReadyReplicaCount: 1},
	}}
	workloads := model.Workloads{
		Nodes: []model.ClusterNode{
			{Ref: model.ResourceRef{Name: "node-a"}, Ready: true},
			{Ref: model.ResourceRef{Name: "node-b"}, Ready: false},
		},
		Pods:        []model.Pod{{Ref: model.ResourceRef{Name: "pod-a"}}},
		Deployments: []model.Deployment{{Ref: model.ResourceRef{Name: "dep-a"}}},
	}
	counts := (&Aggregator{}).Counts(configuration, traffic, workloads)
	want := map[string]int{
		"tenants": 2, "models": 1, "workerNodes": 1, "simulatorInstances": 1,
		"readyReplicas": 4, "nodes": 2, "readyNodes": 1, "pods": 1, "deployments": 1,
	}
	for key, expected := range want {
		if counts[key] != expected {
			t.Fatalf("Counts[%s] = %d, want %d (all=%v)", key, counts[key], expected, counts)
		}
	}
}

func ptr[T any](value T) *T {
	return &value
}

func platformObject(kind string, name string, spec map[string]any, status map[string]any) *unstructured.Unstructured {
	object := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "platform.study.com/v1",
		"kind":       kind,
		"metadata": map[string]any{
			"name": name, "resourceVersion": "1", "generation": int64(1),
		},
		"spec": spec,
	}}
	if status != nil {
		object.Object["status"] = status
	}
	return object
}
