package kubernetes

import (
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestMapPlatformResourcePreservesStatusAndFreshness(t *testing.T) {
	now := time.Date(2026, time.August, 12, 12, 0, 20, 0, time.UTC)
	object := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "platform.study.com/v1",
		"kind":       "SimulatorInstance",
		"metadata": map[string]any{
			"name": "instance-a", "resourceVersion": "42", "generation": int64(3),
			"creationTimestamp": "2026-08-12T11:00:00Z",
		},
		"spec": map[string]any{"replicas": int64(2)},
		"status": map[string]any{
			"phase": "Running", "observedAt": "2026-08-12T12:00:00Z",
			"conditions": []any{map[string]any{
				"type": "Ready", "status": "True", "observedGeneration": int64(3),
			}},
		},
	}}

	resource := MapPlatformResource(object, now)
	if resource.Ref.Name != "instance-a" || resource.Metadata.ResourceVersion != "42" {
		t.Fatalf("resource identity was not mapped: %#v", resource)
	}
	if resource.Derived["freshness"] != "fresh" || len(resource.Conditions) != 1 {
		t.Fatalf("resource status was not mapped: %#v", resource)
	}
}

func TestMapPodUsesFullIdentityAnnotations(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "simulator-a", Namespace: "hello-k8s-ai-system",
			Annotations: map[string]string{
				identityInstanceAnnotation: "instance/with/full-name",
				identityTenantAnnotation:   "tenant-a",
				identityModelAnnotation:    "model-a",
			},
		},
		Status: corev1.PodStatus{
			Phase:      corev1.PodRunning,
			Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}},
		},
	}
	mapped := MapPod(pod)
	if !mapped.Ready || mapped.SimulatorInstance != "instance/with/full-name" {
		t.Fatalf("pod readiness or identity was not mapped: %#v", mapped)
	}
}

func TestValidateIntentRejectsControllerOwnedResources(t *testing.T) {
	_, err := validateIntent(ApplyIntent{Kind: "SimulatorInstance", Name: "instance-a", Spec: map[string]any{}})
	if err == nil {
		t.Fatal("SimulatorInstance must not be writable through the Dashboard gateway")
	}
	_, err = validateIntent(ApplyIntent{Kind: "Tenant", Name: "tenant-a", Spec: map[string]any{"qps": 10}})
	if err != nil {
		t.Fatalf("valid Tenant write was rejected: %v", err)
	}
}
