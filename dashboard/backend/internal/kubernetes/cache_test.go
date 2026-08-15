package kubernetes

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	clientcache "k8s.io/client-go/tools/cache"
)

func TestSimulationRateReadsDesiredAndConvergedState(t *testing.T) {
	informer := clientcache.NewSharedIndexInformer(
		&clientcache.ListWatch{},
		&unstructured.Unstructured{},
		0,
		clientcache.Indexers{},
	)
	object := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "platform.study.com/v1",
		"kind":       "SimulationClock",
		"metadata": map[string]any{
			"name":            "default",
			"resourceVersion": "27",
			"generation":      int64(7),
		},
		"spec": map[string]any{"rate": int64(10)},
		"status": map[string]any{
			"observedGeneration":    int64(7),
			"appliedRate":           int64(10),
			"synchronizedInstances": int64(3),
			"totalInstances":        int64(3),
			"conditions": []any{map[string]any{
				"type":   "Ready",
				"status": "True",
			}},
		},
	}}
	if err := informer.GetStore().Add(object); err != nil {
		t.Fatalf("add clock to cache: %v", err)
	}
	state := &Cache{platform: map[string]clientcache.SharedIndexInformer{
		"SimulationClock": informer,
	}}

	desired, applied, synchronized, total, version, ready, found := state.SimulationRate()
	if !found || !ready {
		t.Fatalf("clock found=%t ready=%t", found, ready)
	}
	if desired != 10 || applied != 10 || synchronized != 3 || total != 3 || version != "27" {
		t.Fatalf(
			"clock state = desired:%d applied:%d synchronized:%d total:%d version:%q",
			desired,
			applied,
			synchronized,
			total,
			version,
		)
	}
}

func TestSimulationRateRejectsStaleReadyCondition(t *testing.T) {
	informer := clientcache.NewSharedIndexInformer(
		&clientcache.ListWatch{},
		&unstructured.Unstructured{},
		0,
		clientcache.Indexers{},
	)
	object := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "platform.study.com/v1",
		"kind":       "SimulationClock",
		"metadata": map[string]any{
			"name":       "default",
			"generation": int64(8),
		},
		"spec": map[string]any{"rate": int64(10)},
		"status": map[string]any{
			"observedGeneration":    int64(7),
			"appliedRate":           int64(5),
			"synchronizedInstances": int64(2),
			"totalInstances":        int64(3),
			"conditions": []any{map[string]any{
				"type":   "Ready",
				"status": "True",
			}},
		},
	}}
	if err := informer.GetStore().Add(object); err != nil {
		t.Fatalf("add stale clock to cache: %v", err)
	}
	state := &Cache{platform: map[string]clientcache.SharedIndexInformer{
		"SimulationClock": informer,
	}}

	desired, applied, synchronized, total, _, ready, found := state.SimulationRate()
	if !found || ready {
		t.Fatalf("stale clock found=%t ready=%t", found, ready)
	}
	if desired != 10 || applied != 5 || synchronized != 2 || total != 3 {
		t.Fatalf(
			"stale state = desired:%d applied:%d synchronized:%d total:%d",
			desired,
			applied,
			synchronized,
			total,
		)
	}
}

func TestSimulationRateFallsBackWhileClockIsAbsent(t *testing.T) {
	state := &Cache{platform: map[string]clientcache.SharedIndexInformer{}}
	desired, applied, synchronized, total, version, ready, found := state.SimulationRate()
	if found || ready || desired != 1 || applied != 1 || synchronized != 0 || total != 0 || version != "" {
		t.Fatalf(
			"fallback = desired:%d applied:%d synchronized:%d total:%d version:%q ready:%t found:%t",
			desired,
			applied,
			synchronized,
			total,
			version,
			ready,
			found,
		)
	}
}
