package config

import (
	"strings"
	"testing"
)

// TestValidateAIOpsRequiresAPIKey AIOps 开启时必须有 API Key。
func TestValidateAIOpsRequiresAPIKey(t *testing.T) {
	cfg := Config{AIOps: AIOpsConfig{Enabled: true}}
	err := cfg.validate()
	if err == nil || !strings.Contains(err.Error(), "AIOPS_OPENAI_API_KEY") {
		t.Fatalf("expected API key validation error, got %v", err)
	}
}

// TestValidateAIOpsBudgetBounds 开启时预算参数必须为正。
func TestValidateAIOpsBudgetBounds(t *testing.T) {
	cfg := Config{AIOps: AIOpsConfig{Enabled: true, OpenAIAPIKey: "key", MaxTokensPerCall: 100, MaxCallsPerAnalysis: 0}}
	err := cfg.validate()
	if err == nil || !strings.Contains(err.Error(), "AIOPS_MAX_CALLS_PER_ANALYSIS") {
		t.Fatalf("expected budget validation error, got %v", err)
	}
}

// TestValidateAIOpsDisabledNeedsNothing 未开启时无需任何 AIOps 参数。
func TestValidateAIOpsDisabledNeedsNothing(t *testing.T) {
	cfg := Config{
		HTTP:       HTTPConfig{Address: ":8080", MaxBodyBytes: 4096},
		Kubernetes: KubernetesConfig{QPS: 10, Burst: 20},
		Database:   DatabaseConfig{Required: false, MaxConnections: 10, MinConnections: 1},
		Persistence: PersistenceConfig{EventBuffer: 128, SegmentBurstReplicaDelta: 5,
			SegmentErrorRateThreshold: 0.05, SegmentTTFTThresholdMS: 2000},
	}
	if err := cfg.validate(); err != nil {
		t.Fatalf("default config should validate: %v", err)
	}
}
