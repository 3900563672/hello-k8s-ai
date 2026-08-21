package aiops

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/config"
)

func chatTestService(t *testing.T, cfg config.AIOpsConfig) *Service {
	t.Helper()
	if cfg.ChatMaxMessageLen == 0 {
		cfg.ChatMaxMessageLen = 4000
	}
	if cfg.ChatRatePerMinute == 0 {
		cfg.ChatRatePerMinute = 6
	}
	if cfg.Model == "" {
		cfg.Model = "gpt-4o-mini"
	}
	return NewService(cfg, newFakeStore(nil), newFakeLLM([]string{"你好，我是 AIOps 助手"}, nil), testLogger())
}

func TestChatValidateMessage(t *testing.T) {
	service := chatTestService(t, config.AIOpsConfig{})
	if err := service.ChatValidateMessage(""); err == nil {
		t.Fatal("empty message should be rejected")
	}
	if err := service.ChatValidateMessage("   "); err == nil {
		t.Fatal("whitespace message should be rejected")
	}
	long := strings.Repeat("长", 4001)
	if err := service.ChatValidateMessage(long); err == nil {
		t.Fatal("over-length message should be rejected")
	}
	if err := service.ChatValidateMessage("当前集群什么情况？"); err != nil {
		t.Fatalf("normal message rejected: %v", err)
	}
}

func TestChatAllowSessionRateLimit(t *testing.T) {
	service := chatTestService(t, config.AIOpsConfig{})
	now := time.Now().UTC()
	for i := 0; i < 6; i++ {
		if !service.ChatAllowSession("session-a", now) {
			t.Fatalf("request %d should be allowed", i+1)
		}
	}
	if service.ChatAllowSession("session-a", now) {
		t.Fatal("7th request in window should be rate limited")
	}
	if !service.ChatAllowSession("session-b", now) {
		t.Fatal("different session should not share rate limit")
	}
	// 窗口滑动：1 分钟后的请求重新放行。
	if !service.ChatAllowSession("session-a", now.Add(time.Minute+time.Second)) {
		t.Fatal("request after window should be allowed again")
	}
}

func TestChatBuildContext(t *testing.T) {
	service := chatTestService(t, config.AIOpsConfig{})
	contextText, err := service.ChatBuildContext(context.Background())
	if err != nil {
		t.Fatalf("build context: %v", err)
	}
	if !strings.Contains(contextText, "recentAnalyses") {
		t.Fatalf("context should contain recentAnalyses key: %s", contextText)
	}
}

func TestChatStreamEvents(t *testing.T) {
	service := chatTestService(t, config.AIOpsConfig{})
	var tools []string
	var deltas []string
	var usage TokenUsage
	err := service.ChatStream(context.Background(), "当前集群什么情况？",
		func(name, phase string) { tools = append(tools, name+":"+phase) },
		func(delta string) { deltas = append(deltas, delta) },
		func(value TokenUsage) { usage = value })
	if err != nil {
		t.Fatalf("chat stream: %v", err)
	}
	wantTools := []string{"读取切面总结:start", "读取切面总结:end", "生成回答:start", "生成回答:end"}
	if len(tools) != len(wantTools) {
		t.Fatalf("tool events = %v, want %v", tools, wantTools)
	}
	for i, want := range wantTools {
		if tools[i] != want {
			t.Fatalf("tool[%d] = %s, want %s", i, tools[i], want)
		}
	}
	if usage.PromptTokens != 0 || usage.CompletionTokens != 0 {
		t.Fatalf("fake LLM should not report usage, got %+v", usage)
	}
	joined := strings.Join(deltas, "")
	if joined != "你好，我是 AIOps 助手" {
		t.Fatalf("streamed answer = %q", joined)
	}
}

func TestChatConfigureAndSettings(t *testing.T) {
	service := chatTestService(t, config.AIOpsConfig{Model: "gpt-4o-mini", OpenAIBaseURL: "https://api.openai.com/v1"})
	state := service.Settings()
	if state.Configured || state.KeyConfigured {
		t.Fatalf("settings should start unconfigured: %+v", state)
	}
	service.ConfigureLLM("https://custom.example.com/v1/", "sk-test-key-123456", "gpt-4.1")
	state = service.Settings()
	if !state.Configured || !state.KeyConfigured {
		t.Fatalf("settings should be configured after update: %+v", state)
	}
	if state.Model != "gpt-4.1" || state.BaseURL != "https://custom.example.com/v1" {
		t.Fatalf("unexpected settings: %+v", state)
	}
	// 空字段保持不变。
	service.ConfigureLLM("", "", "")
	state = service.Settings()
	if state.Model != "gpt-4.1" || state.BaseURL != "https://custom.example.com/v1" {
		t.Fatalf("empty update should keep previous config: %+v", state)
	}
}

func TestChatAuditLog(t *testing.T) {
	service := chatTestService(t, config.AIOpsConfig{})
	// 不 panic、不报错（fakeStore 审计成功路径）。
	service.AuditChat(context.Background(), "session-x", 123*time.Millisecond, 42, TokenUsage{PromptTokens: 10, CompletionTokens: 20}, nil)
	service.AuditChat(context.Background(), "session-x", 55*time.Millisecond, 40, TokenUsage{}, errors.New("boom"))
}
