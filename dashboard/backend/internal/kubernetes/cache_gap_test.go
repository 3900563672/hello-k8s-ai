package kubernetes

import (
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/config"
	appsv1 "k8s.io/api/apps/v1"
	coordinationv1 "k8s.io/api/coordination/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynfake "k8s.io/client-go/dynamic/fake"
	k8sfake "k8s.io/client-go/kubernetes/fake"
	clientcache "k8s.io/client-go/tools/cache"
)

func typedInformerFor(kind string, object runtime.Object) clientcache.SharedIndexInformer {
	return clientcache.NewSharedIndexInformer(&clientcache.ListWatch{}, object, 0, clientcache.Indexers{})
}

func TestTypedListers(t *testing.T) {
	state := &Cache{native: map[string]clientcache.SharedIndexInformer{
		"Pod":        typedInformerFor("Pod", &corev1.Pod{}),
		"Node":       typedInformerFor("Node", &corev1.Node{}),
		"Service":    typedInformerFor("Service", &corev1.Service{}),
		"Event":      typedInformerFor("Event", &corev1.Event{}),
		"Deployment": typedInformerFor("Deployment", &appsv1.Deployment{}),
		"Lease":      typedInformerFor("Lease", &coordinationv1.Lease{}),
	}}
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "pod-1", Namespace: "default"}}
	if err := state.native["Pod"].GetStore().Add(pod); err != nil {
		t.Fatalf("add pod: %v", err)
	}
	if pods := state.ListPods(); len(pods) != 1 || pods[0].Name != "pod-1" {
		t.Fatalf("ListPods = %+v", pods)
	}
	node := &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-1"}}
	if err := state.native["Node"].GetStore().Add(node); err != nil {
		t.Fatalf("add node: %v", err)
	}
	if nodes := state.ListNodes(); len(nodes) != 1 || nodes[0].Name != "node-1" {
		t.Fatalf("ListNodes = %+v", nodes)
	}
	if services := state.ListServices(); len(services) != 0 {
		t.Fatalf("ListServices = %+v", services)
	}
	if deployments := state.ListDeployments(); len(deployments) != 0 {
		t.Fatalf("ListDeployments = %+v", deployments)
	}
	if leases := state.ListLeases(); len(leases) != 0 {
		t.Fatalf("ListLeases = %+v", leases)
	}
	// nil informer 兜底为空列表
	if pods := (&Cache{}).ListPods(); len(pods) != 0 {
		t.Fatalf("ListPods(nil) = %+v", pods)
	}
}

func TestListPlatformSortedAndEmpty(t *testing.T) {
	informer := clientcache.NewSharedIndexInformer(&clientcache.ListWatch{}, &unstructured.Unstructured{}, 0, clientcache.Indexers{})
	for _, name := range []string{"tenant-b", "tenant-a"} {
		object := &unstructured.Unstructured{Object: map[string]any{
			"apiVersion": "platform.study.com/v1", "kind": "Tenant",
			"metadata": map[string]any{"name": name},
		}}
		if err := informer.GetStore().Add(object); err != nil {
			t.Fatalf("add %s: %v", name, err)
		}
	}
	state := &Cache{platform: map[string]clientcache.SharedIndexInformer{"Tenant": informer}}
	items := state.ListPlatform("Tenant")
	if len(items) != 2 || items[0].GetName() != "tenant-a" || items[1].GetName() != "tenant-b" {
		t.Fatalf("ListPlatform = %+v", items)
	}
	if items := state.ListPlatform("Unknown"); len(items) != 0 {
		t.Fatalf("ListPlatform(unknown) = %+v", items)
	}
}

func TestCacheAccessors(t *testing.T) {
	clients := &Clients{Context: "ctx-demo", Version: "v1.30.0"}
	state := &Cache{clients: clients}
	if state.ContextName() != "ctx-demo" || state.ServerVersion() != "v1.30.0" {
		t.Fatalf("ContextName/ServerVersion = %q %q", state.ContextName(), state.ServerVersion())
	}
	if state.DynamicClient() != clients {
		t.Fatal("DynamicClient 应返回注入的 clients")
	}
	if state.Synced() {
		t.Fatal("零值 Synced 应为 false")
	}
	if !state.SyncedAt().IsZero() {
		t.Fatal("零值 SyncedAt 应为 zero")
	}
}

func TestRandomIDAndAPIVersion(t *testing.T) {
	first := randomID()
	second := randomID()
	if first == "" || first == second || len(first) != 32 {
		t.Fatalf("randomID = %q %q", first, second)
	}
	gvr := schema.GroupVersionResource{Group: "platform.study.com", Version: "v1", Resource: "tenants"}
	if got := apiVersionForGVR(gvr); got != "platform.study.com/v1" {
		t.Fatalf("apiVersionForGVR = %q", got)
	}
}

func TestNewCacheWithFakeClients(t *testing.T) {
	clients := &Clients{
		Kubernetes: k8sfake.NewSimpleClientset(),
		Dynamic:    dynfake.NewSimpleDynamicClient(runtime.NewScheme()),
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	state, err := NewCache(clients, config.KubernetesConfig{ResyncPeriod: time.Hour}, logger, nil)
	if err != nil {
		t.Fatalf("NewCache: %v", err)
	}
	if state == nil {
		t.Fatal("NewCache 返回 nil")
	}
	if state.Synced() {
		t.Fatal("新 cache 不应已同步")
	}
}

func TestNewCacheRejectsNilClients(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	if _, err := NewCache(nil, config.KubernetesConfig{}, logger, nil); err == nil {
		t.Fatal("nil clients 应报错")
	}
}

func TestNewClientsWithoutKubeconfigFails(t *testing.T) {
	// 测试环境无 in-cluster config 且无 kubeconfig → 应返回错误。
	if _, err := NewClients(config.KubernetesConfig{}); err == nil {
		t.Skip("环境存在默认 kubeconfig 时跳过")
	}
}
