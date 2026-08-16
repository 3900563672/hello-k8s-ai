package controller

import (
	"context"
	"reflect"
	"testing"
	"time"

	platformv1 "github.com/3900563672/hello-k8s-ai/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// 验证 Largest Remainder 分配在不同 QPS 下的结果是否与预期一致
func TestAllocateTrafficPreservesConfiguredTotal(t *testing.T) {
	instances := []instanceData{
		{name: "instance-c", score: 0},
		{name: "instance-a", score: 100},
		{name: "instance-b", score: 50},
	}
	tests := []struct {
		name string
		qps  int
		want map[string]int
	}{
		{
			name: "weighted",
			qps:  10,
			want: map[string]int{"instance-a": 7, "instance-b": 3, "instance-c": 0},
		},
		{
			name: "less traffic than instances",
			qps:  2,
			want: map[string]int{"instance-a": 1, "instance-b": 1, "instance-c": 0},
		},
		{
			name: "zero traffic",
			qps:  0,
			want: map[string]int{"instance-a": 0, "instance-b": 0, "instance-c": 0},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, total := allocateTraffic(test.qps, instances)
			if total != test.qps || !reflect.DeepEqual(got, test.want) {
				t.Fatalf("allocateTraffic(%d) = (%v, %d), want (%v, %d)", test.qps, got, total, test.want, test.qps)
			}
		})
	}
}

// 所有实例分数为 0 时应该平均分配，且结果稳定
func TestAllocateTrafficUsesStableEqualFallback(t *testing.T) {
	instances := []instanceData{
		{name: "instance-c"},
		{name: "instance-a"},
		{name: "instance-b"},
	}
	want := map[string]int{"instance-a": 1, "instance-b": 1, "instance-c": 0}
	got, total := allocateTraffic(2, instances)
	if total != 2 || !reflect.DeepEqual(got, want) {
		t.Fatalf("equal fallback = (%v, %d), want (%v, 2)", got, total, want)
	}
}

// 测试指标新鲜度判断，边界值应该正确
func TestMetricIsFresh(t *testing.T) {
	now := time.Date(2026, time.August, 9, 12, 0, 0, 0, time.UTC)
	fresh := metav1.NewTime(now.Add(-instanceMetricMaxAge))
	stale := metav1.NewTime(now.Add(-instanceMetricMaxAge - time.Nanosecond))
	if !metricIsFresh(new(fresh), now) {
		t.Fatal("metric at the freshness boundary should be accepted")
	}
	if metricIsFresh(new(stale), now) {
		t.Fatal("expired metric should be rejected")
	}
	if metricIsFresh(nil, now) {
		t.Fatal("missing observation time should be rejected")
	}
}

// 缩容到零副本的实例不参与分配，但残留的旧 QPS 必须被清零，
// 保证租户下实例 QPS 总和不超过请求值（分配不变量端到端闭环）。
func TestZeroStaleTrafficQPSOnScaledToZeroInstances(t *testing.T) {
	scheme := newControllerTestScheme(t)
	active := &platformv1.SimulatorInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "tenant-a-model-a"},
		Spec: platformv1.SimulatorInstanceSpec{
			TenantRef: platformv1.ObjectRef{Name: "tenant-a"},
			ModelRef:  platformv1.ObjectRef{Name: "model-a"},
			Replicas:  2,
			Traffic:   platformv1.TrafficSpec{QPS: 100},
		},
	}
	stale := &platformv1.SimulatorInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "tenant-a-model-b"},
		Spec: platformv1.SimulatorInstanceSpec{
			TenantRef: platformv1.ObjectRef{Name: "tenant-a"},
			ModelRef:  platformv1.ObjectRef{Name: "model-b"},
			Replicas:  0,
			Traffic:   platformv1.TrafficSpec{QPS: 80},
		},
	}
	kubernetesClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(active, stale).
		WithIndex(&platformv1.SimulatorInstance{}, trafficTenantIndex, func(obj client.Object) []string {
			return []string{obj.(*platformv1.SimulatorInstance).Spec.TenantRef.Name}
		}).
		Build()
	reconciler := &TrafficReconciler{Client: kubernetesClient}

	if err := reconciler.zeroStaleTrafficQPS(context.Background(), "tenant-a", []instanceData{{name: "tenant-a-model-a"}}); err != nil {
		t.Fatalf("zero stale QPS: %v", err)
	}

	var got platformv1.SimulatorInstance
	if err := kubernetesClient.Get(context.Background(), client.ObjectKey{Name: "tenant-a-model-b"}, &got); err != nil {
		t.Fatalf("get scaled-to-zero instance: %v", err)
	}
	if got.Spec.Traffic.QPS != 0 {
		t.Fatalf("scaled-to-zero instance QPS = %d, want 0", got.Spec.Traffic.QPS)
	}
	if err := kubernetesClient.Get(context.Background(), client.ObjectKey{Name: "tenant-a-model-a"}, &got); err != nil {
		t.Fatalf("get active instance: %v", err)
	}
	if got.Spec.Traffic.QPS != 100 {
		t.Fatalf("active instance QPS = %d, want 100", got.Spec.Traffic.QPS)
	}
}
