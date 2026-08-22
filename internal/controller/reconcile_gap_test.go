package controller

import (
	"context"
	"testing"
	"time"

	platformv1 "github.com/3900563672/hello-k8s-ai/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

func TestSimulatorInstanceReconcileNotFound(t *testing.T) {
	ctx := log.IntoContext(context.Background(), log.Log)
	scheme := newControllerTestScheme(t)
	kubernetesClient := fake.NewClientBuilder().WithScheme(scheme).Build()
	reconciler := &SimulatorInstanceReconciler{Client: kubernetesClient, Scheme: scheme}
	result, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKey{Name: "missing"}})
	if err != nil {
		t.Fatalf("NotFound Reconcile: %v", err)
	}
	if !result.IsZero() {
		t.Fatalf("NotFound 应返回零值: %+v", result)
	}
}

func TestPerformanceCollectorReconcileNotFound(t *testing.T) {
	ctx := log.IntoContext(context.Background(), log.Log)
	scheme := newControllerTestSchemeWithCore(t)
	kubernetesClient := fake.NewClientBuilder().WithScheme(scheme).
		WithIndex(&platformv1.SimulatorInstance{}, tenantIndexField, func(object client.Object) []string {
			return []string{object.(*platformv1.SimulatorInstance).Spec.TenantRef.Name}
		}).Build()
	reconciler := &PerformanceCollectorReconciler{Client: kubernetesClient, Scheme: scheme}
	result, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKey{Name: "missing-tenant"}})
	if err != nil {
		t.Fatalf("NotFound Reconcile: %v", err)
	}
	if !result.IsZero() {
		t.Fatalf("NotFound 应返回零值: %+v", result)
	}
}

func TestPerformanceCollectorReconcileCreatesPerformance(t *testing.T) {
	ctx := log.IntoContext(context.Background(), log.Log)
	scheme := newControllerTestSchemeWithCore(t)
	tenant := &platformv1.Tenant{ObjectMeta: metav1.ObjectMeta{Name: "tenant-a"}}
	kubernetesClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(tenant).
		WithStatusSubresource(&platformv1.TenantPerformance{}).
		WithIndex(&platformv1.SimulatorInstance{}, tenantIndexField, func(object client.Object) []string {
			return []string{object.(*platformv1.SimulatorInstance).Spec.TenantRef.Name}
		}).Build()
	reconciler := &PerformanceCollectorReconciler{Client: kubernetesClient, Scheme: scheme, Now: func() time.Time { return time.Now() }}
	result, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: client.ObjectKey{Name: "tenant-a"}})
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if result.RequeueAfter == 0 {
		t.Fatalf("Reconcile 应返回 RequeueAfter: %+v", result)
	}
	var performance platformv1.TenantPerformance
	if err := kubernetesClient.Get(ctx, client.ObjectKey{Name: "tenant-a"}, &performance); err != nil {
		t.Fatalf("TenantPerformance 未被创建: %v", err)
	}
	if performance.Spec.TenantRef.Name != "tenant-a" {
		t.Fatalf("TenantPerformance.Spec = %+v", performance.Spec)
	}
}
