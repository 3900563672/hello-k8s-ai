package kubernetes

import (
	"context"
	"strings"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic/fake"
	clienttesting "k8s.io/client-go/testing"
)

func TestValidateIntentAllowsModelAbsoluteScore(t *testing.T) {
	intent := ApplyIntent{
		Kind: "Model",
		Name: "model-a",
		Spec: map[string]any{
			"displayName":    "模型 A",
			"gpuUnits":       float64(1),
			"maxConcurrency": float64(8),
			"absoluteScore":  float64(100),
			"coldStartMs":    float64(1000),
			"performance":    map[string]any{},
		},
	}

	descriptor, err := validateIntent(intent)
	if err != nil {
		t.Fatalf("validate Model intent: %v", err)
	}
	if descriptor.Kind != "Model" {
		t.Fatalf("descriptor kind = %q, want Model", descriptor.Kind)
	}
}

func TestValidateIntentRequiresModelAbsoluteScore(t *testing.T) {
	intent := ApplyIntent{
		Kind: "Model",
		Name: "model-a",
		Spec: map[string]any{
			"displayName":    "模型 A",
			"gpuUnits":       float64(1),
			"maxConcurrency": float64(8),
			"coldStartMs":    float64(1000),
		},
	}

	_, err := validateIntent(intent)
	if err == nil || !strings.Contains(err.Error(), "spec.absoluteScore") {
		t.Fatalf("validateIntent() error = %v, want missing absoluteScore error", err)
	}
}

func TestValidateIntentRejectsModelStatusFields(t *testing.T) {
	intent := ApplyIntent{
		Kind: "Model",
		Name: "model-a",
		Spec: map[string]any{
			"displayName":    "模型 A",
			"gpuUnits":       float64(1),
			"maxConcurrency": float64(8),
			"absoluteScore":  float64(100),
			"coldStartMs":    float64(1000),
			"status":         map[string]any{"absoluteScore": float64(100)},
		},
	}

	_, err := validateIntent(intent)
	if err == nil || !strings.Contains(err.Error(), "spec.status") {
		t.Fatalf("validateIntent() error = %v, want forbidden status field error", err)
	}
}

func TestSetSimulationRateCreatesAndUpdatesSingleton(t *testing.T) {
	dynamicClient := fake.NewSimpleDynamicClient(runtime.NewScheme())
	gateway := &Gateway{cache: &Cache{clients: &Clients{Dynamic: dynamicClient}}}

	created, action, err := gateway.SetSimulationRate(context.Background(), 5, "", false)
	if err != nil {
		t.Fatalf("create SimulationClock: %v", err)
	}
	if action != "create" || created.GetName() != "default" {
		t.Fatalf("create result = %q %#v", action, created)
	}
	rate, found, err := unstructured.NestedInt64(created.Object, "spec", "rate")
	if err != nil || !found || rate != 5 {
		t.Fatalf("created rate = %d, found=%t, err=%v", rate, found, err)
	}

	updated, action, err := gateway.SetSimulationRate(
		context.Background(),
		10,
		created.GetResourceVersion(),
		false,
	)
	if err != nil {
		t.Fatalf("update SimulationClock: %v", err)
	}
	if action != "update" {
		t.Fatalf("update action = %q", action)
	}
	rate, found, err = unstructured.NestedInt64(updated.Object, "spec", "rate")
	if err != nil || !found || rate != 10 {
		t.Fatalf("updated rate = %d, found=%t, err=%v", rate, found, err)
	}

	stored, err := dynamicClient.Resource(platformGVR("simulationclocks")).Get(
		context.Background(),
		"default",
		metav1.GetOptions{},
	)
	if err != nil {
		t.Fatalf("get stored SimulationClock: %v", err)
	}
	storedRate, _, _ := unstructured.NestedInt64(stored.Object, "spec", "rate")
	if storedRate != 10 {
		t.Fatalf("stored rate = %d, want 10", storedRate)
	}
}

func TestSetSimulationRateRejectsOutOfRangeValue(t *testing.T) {
	dynamicClient := fake.NewSimpleDynamicClient(runtime.NewScheme())
	gateway := &Gateway{cache: &Cache{clients: &Clients{Dynamic: dynamicClient}}}
	for _, rate := range []int{0, 21} {
		if _, _, err := gateway.SetSimulationRate(context.Background(), rate, "", false); err == nil {
			t.Fatalf("rate %d should be rejected", rate)
		}
	}
}

func TestSetSimulationRateRejectsStaleResourceVersion(t *testing.T) {
	clock := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "platform.study.com/v1",
		"kind":       "SimulationClock",
		"metadata": map[string]any{
			"name":            "default",
			"resourceVersion": "11",
		},
		"spec": map[string]any{"rate": int64(1)},
	}}
	dynamicClient := fake.NewSimpleDynamicClient(runtime.NewScheme(), clock)
	gateway := &Gateway{cache: &Cache{clients: &Clients{Dynamic: dynamicClient}}}

	_, _, err := gateway.SetSimulationRate(context.Background(), 5, "10", false)
	if !apierrors.IsConflict(err) {
		t.Fatalf("stale resourceVersion error = %v, want conflict", err)
	}
}

func TestSetSimulationRateHandlesConcurrentSingletonCreation(t *testing.T) {
	clock := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "platform.study.com/v1",
		"kind":       "SimulationClock",
		"metadata": map[string]any{
			"name":            "default",
			"resourceVersion": "11",
		},
		"spec": map[string]any{"rate": int64(1)},
	}}
	dynamicClient := fake.NewSimpleDynamicClient(runtime.NewScheme(), clock)
	firstGet := true
	dynamicClient.PrependReactor("get", "simulationclocks", func(clienttesting.Action) (bool, runtime.Object, error) {
		if !firstGet {
			return false, nil, nil
		}
		firstGet = false
		return true, nil, apierrors.NewNotFound(platformGVR("simulationclocks").GroupResource(), "default")
	})
	gateway := &Gateway{cache: &Cache{clients: &Clients{Dynamic: dynamicClient}}}

	updated, action, err := gateway.SetSimulationRate(context.Background(), 10, "", false)
	if err != nil {
		t.Fatalf("update concurrently created SimulationClock: %v", err)
	}
	if action != "update" {
		t.Fatalf("action = %q, want update", action)
	}
	rate, found, err := unstructured.NestedInt64(updated.Object, "spec", "rate")
	if err != nil || !found || rate != 10 {
		t.Fatalf("updated rate = %d, found=%t, err=%v", rate, found, err)
	}
}
