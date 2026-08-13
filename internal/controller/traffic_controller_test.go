package controller

import (
	"reflect"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
