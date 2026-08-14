package kubernetes

import (
	"strings"
	"testing"
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
