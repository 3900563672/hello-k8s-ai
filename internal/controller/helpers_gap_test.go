package controller

import (
	"context"
	"strings"
	"testing"

	platformv1 "github.com/3900563672/hello-k8s-ai/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func TestSimulatorInstanceFieldIndex(t *testing.T) {
	index := simulatorInstanceFieldIndex("spec.tenantRef.name", func(instance *platformv1.SimulatorInstance) string {
		return instance.Spec.TenantRef.Name
	})
	instance := &platformv1.SimulatorInstance{Spec: platformv1.SimulatorInstanceSpec{TenantRef: platformv1.ObjectRef{Name: "tenant-a"}}}
	values := index.extractor(instance)
	if len(values) != 1 || values[0] != "tenant-a" {
		t.Fatalf("extractor = %v", values)
	}
	empty := &platformv1.SimulatorInstance{}
	if values := index.extractor(empty); len(values) != 0 {
		t.Fatalf("空 tenant 应过滤: %v", values)
	}
}

func TestLifecyclePredicate(t *testing.T) {
	predicate := lifecyclePredicate(func(event.UpdateEvent) bool { return false })
	if !predicate.Create(event.CreateEvent{}) {
		t.Fatal("Create 应触发")
	}
	if !predicate.Delete(event.DeleteEvent{}) {
		t.Fatal("Delete 应触发")
	}
	if predicate.Generic(event.GenericEvent{}) {
		t.Fatal("Generic 应忽略")
	}
	if predicate.Update(event.UpdateEvent{}) {
		t.Fatal("Update 应由传入函数决定")
	}
}

func TestMapSimulatorInstanceToTenant(t *testing.T) {
	instance := &platformv1.SimulatorInstance{Spec: platformv1.SimulatorInstanceSpec{TenantRef: platformv1.ObjectRef{Name: "tenant-a"}}}
	requests := mapSimulatorInstanceToTenant(context.Background(), instance)
	if len(requests) != 1 || requests[0].Name != "tenant-a" {
		t.Fatalf("requests = %+v", requests)
	}
	if requests := mapSimulatorInstanceToTenant(context.Background(), &platformv1.Tenant{}); len(requests) != 0 {
		t.Fatalf("非实例对象应返回空: %+v", requests)
	}
	empty := &platformv1.SimulatorInstance{}
	if requests := mapSimulatorInstanceToTenant(context.Background(), empty); len(requests) != 0 {
		t.Fatalf("空租户名应返回空: %+v", requests)
	}
	_ = reconcile.Request{}
}

func TestOptionalEqualAndValueOrDefault(t *testing.T) {
	a, b := 1, 1
	c := 2
	if !optionalEqual(&a, &b) {
		t.Fatal("相等指针应 equal")
	}
	if optionalEqual(&a, &c) {
		t.Fatal("不同值应不 equal")
	}
	if !optionalEqual[int](nil, nil) {
		t.Fatal("双 nil 应 equal")
	}
	if optionalEqual(&a, nil) || optionalEqual(nil, &a) {
		t.Fatal("单 nil 应不 equal")
	}
	if got := valueOrDefault(0, 42); got != 42 {
		t.Fatalf("valueOrDefault(0) = %d", got)
	}
	if got := valueOrDefault(7, 42); got != 7 {
		t.Fatalf("valueOrDefault(7) = %d", got)
	}
}

func TestLabelValue(t *testing.T) {
	if got := labelValue("tenant-a"); got != "tenant-a" {
		t.Fatalf("labelValue(合法) = %q", got)
	}
	if got := labelValue(""); got == "" {
		t.Fatal("labelValue(空) 不应为空")
	}
	hashed := labelValue("包含非法字符!!!")
	if hashed == "包含非法字符!!!" || !strings.HasPrefix(hashed, "sha256-") {
		t.Fatalf("labelValue(非法) = %q", hashed)
	}
}

func TestRemoveLegacyInstanceFinalizers(t *testing.T) {
	scheme := newControllerTestScheme(t)
	const finalizer = "platform.study.com/cleanup"
	now := metav1.Now()
	deleting := &platformv1.SimulatorInstance{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "instance-deleting",
			DeletionTimestamp: &now,
			Finalizers:        []string{finalizer, "other"},
		},
		Spec: platformv1.SimulatorInstanceSpec{TenantRef: platformv1.ObjectRef{Name: "tenant-a"}},
	}
	active := &platformv1.SimulatorInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "instance-active", Finalizers: []string{finalizer}},
		Spec:       platformv1.SimulatorInstanceSpec{TenantRef: platformv1.ObjectRef{Name: "tenant-a"}},
	}
	kubernetesClient := fake.NewClientBuilder().WithScheme(scheme).WithObjects(deleting, active).
		WithIndex(&platformv1.SimulatorInstance{}, "spec.tenantRef.name", func(object client.Object) []string {
			return []string{object.(*platformv1.SimulatorInstance).Spec.TenantRef.Name}
		}).Build()
	err := removeLegacyInstanceFinalizers(context.Background(), kubernetesClient, "spec.tenantRef.name", "tenant-a", finalizer)
	if err != nil {
		t.Fatalf("removeLegacyInstanceFinalizers: %v", err)
	}
	updated := &platformv1.SimulatorInstance{}
	if err := kubernetesClient.Get(context.Background(), client.ObjectKey{Name: "instance-deleting"}, updated); err != nil {
		t.Fatalf("get deleting instance: %v", err)
	}
	if len(updated.Finalizers) != 1 || updated.Finalizers[0] != "other" {
		t.Fatalf("deleting 实例 finalizers = %v", updated.Finalizers)
	}
	kept := &platformv1.SimulatorInstance{}
	if err := kubernetesClient.Get(context.Background(), client.ObjectKey{Name: "instance-active"}, kept); err != nil {
		t.Fatalf("get active instance: %v", err)
	}
	if len(kept.Finalizers) != 1 {
		t.Fatalf("active 实例不应清理 finalizer: %v", kept.Finalizers)
	}
	// 空租户名直接返回
	if err := removeLegacyInstanceFinalizers(context.Background(), kubernetesClient, "spec.tenantRef.name", "", finalizer); err != nil {
		t.Fatalf("空租户名应直接返回: %v", err)
	}
}
