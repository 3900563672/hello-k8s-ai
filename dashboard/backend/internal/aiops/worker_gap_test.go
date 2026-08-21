package aiops

import (
	"context"
	"testing"
	"time"

	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/config"
)

func TestOpenAIConfigLifecycle(t *testing.T) {
	logger := testLogger()
	client := NewOpenAI(config.AIOpsConfig{
		OpenAIBaseURL: "https://api.example.com/v1", OpenAIAPIKey: "sk-1",
		Model: "m1", Timeout: time.Second, MaxTokensPerCall: 100,
	}, logger)
	baseURL, model, configured := client.Snapshot()
	if baseURL != "https://api.example.com/v1" || model != "m1" || !configured {
		t.Fatalf("Snapshot() = %q %q %v", baseURL, model, configured)
	}
	client.UpdateConfig(" https://new.example.com/v1/ ", "", "m2")
	baseURL, model, configured = client.Snapshot()
	if baseURL != "https://new.example.com/v1" || model != "m2" {
		t.Fatalf("UpdateConfig 后 = %q %q", baseURL, model)
	}
	client.UpdateConfig("", "", "")
	baseURL, _, _ = client.Snapshot()
	if baseURL != "https://new.example.com/v1" {
		t.Fatalf("空字段不应覆盖: %q", baseURL)
	}
	client.UpdateConfig("", "sk-2", "")
	_, _, configured = client.Snapshot()
	if !configured {
		t.Fatal("apiKey 更新后 Snapshot 应显示已配置")
	}
}

func TestTruncate(t *testing.T) {
	if got := truncate("short", 10); got != "short" {
		t.Fatalf("truncate(short) = %q", got)
	}
	if got := truncate("a very long string", 7); got != "a very ..." {
		t.Fatalf("truncate(long) = %q", got)
	}
}

func TestWorkerRunStopsOnCancel(t *testing.T) {
	cfg := config.AIOpsConfig{
		PollInterval: 5 * time.Millisecond, WindowInterval: 10 * time.Millisecond,
		StaleRequeueInterval: time.Minute,
	}
	service := NewService(cfg, newFakeStore(nil), newFakeLLM(nil, nil), testLogger())
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- service.Run(ctx) }()
	time.Sleep(30 * time.Millisecond)
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not stop after cancel")
	}
}

func TestChatSystemPrompt(t *testing.T) {
	service := chatTestService(t, config.AIOpsConfig{})
	if prompt := service.ChatSystemPrompt(); prompt == "" {
		t.Fatal("ChatSystemPrompt 不应为空")
	}
}

func TestIsErrorRateMetric(t *testing.T) {
	if !isErrorRateMetric("simulator.errorRate") || !isErrorRateMetric("controller.errorRate") {
		t.Fatal("错误率指标应命中")
	}
	if isErrorRateMetric("simulator.ttft") {
		t.Fatal("ttft 不应命中错误率指标")
	}
}
