package controller

import (
	"context"
	"testing"

	platformv1 "github.com/3900563672/hello-k8s-ai/api/v1"
)

func TestProtectionMapWrappersAndBranches(t *testing.T) {
	ctx := context.Background()

	// mapInstanceToTenant：包装 mapSimulatorInstanceToTenant，正常租户名生成请求。
	collector := &PerformanceCollectorReconciler{}
	instance := &platformv1.SimulatorInstance{Spec: platformv1.SimulatorInstanceSpec{TenantRef: platformv1.ObjectRef{Name: "tenant-a"}}}
	requests := collector.mapInstanceToTenant(ctx, instance)
	if len(requests) != 1 || requests[0].Name != "tenant-a" {
		t.Fatalf("mapInstanceToTenant = %#v, want tenant-a", requests)
	}
	if requests := collector.mapInstanceToTenant(ctx, &platformv1.Tenant{}); len(requests) != 0 {
		t.Fatalf("mapInstanceToTenant(wrong type) = %#v, want none", requests)
	}

	// mapPerformanceToTenant：正常 / 类型不匹配 / 空租户引用。
	performance := &platformv1.TenantPerformance{Spec: platformv1.TenantPerformanceSpec{TenantRef: platformv1.ObjectRef{Name: "tenant-b"}}}
	requests = collector.mapPerformanceToTenant(ctx, performance)
	if len(requests) != 1 || requests[0].Name != "tenant-b" {
		t.Fatalf("mapPerformanceToTenant = %#v, want tenant-b", requests)
	}
	if requests := collector.mapPerformanceToTenant(ctx, &platformv1.SimulatorInstance{}); len(requests) != 0 {
		t.Fatalf("mapPerformanceToTenant(wrong type) = %#v, want none", requests)
	}
	if requests := collector.mapPerformanceToTenant(ctx, &platformv1.TenantPerformance{}); len(requests) != 0 {
		t.Fatalf("mapPerformanceToTenant(empty ref) = %#v, want none", requests)
	}
}

func TestProtectionNodePolicyMapBranches(t *testing.T) {
	ctx := context.Background()
	simulator := &SimulatorInstanceReconciler{}

	// 未知类型直接返回 nil。
	if requests := simulator.mapNodePolicyToInstances(ctx, &platformv1.Tenant{}); len(requests) != 0 {
		t.Fatalf("mapNodePolicyToInstances(unknown) = %#v, want none", requests)
	}
	// 空引用提前返回，不触发 List（零值 reconciler 也可安全调用）。
	if requests := simulator.mapTenantNodePolicyToInstances(ctx, &platformv1.TenantNodePolicy{}); len(requests) != 0 {
		t.Fatalf("mapTenantNodePolicyToInstances(empty ref) = %#v, want none", requests)
	}
	if requests := simulator.mapModelNodePolicyToInstances(ctx, &platformv1.ModelNodePolicy{}); len(requests) != 0 {
		t.Fatalf("mapModelNodePolicyToInstances(empty ref) = %#v, want none", requests)
	}
}

func TestProtectionDependencyNotReadyError(t *testing.T) {
	err := &dependencyNotReadyError{resource: "SimulatorInstance", name: "tenant-a-model-a"}
	if got := err.Error(); got != `SimulatorInstance "tenant-a-model-a" is not ready` {
		t.Fatalf("dependencyNotReadyError.Error = %q", got)
	}
}
