package aiops

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/config"
	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/model"
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

func TestCheckDailyQuota(t *testing.T) {
	database := newFakeStore(nil)
	database.usageCalls = 5
	database.usageTokens = 500
	service := NewService(config.AIOpsConfig{DailyMaxCalls: 10, DailyMaxTokens: 1000}, database, newFakeLLM(nil, nil), testLogger())
	if err := service.CheckDailyQuota(context.Background()); err != nil {
		t.Fatalf("within quota should pass: %v", err)
	}
	database.usageCalls = 10
	if err := service.CheckDailyQuota(context.Background()); err == nil {
		t.Fatal("call quota exceeded should fail")
	}
	database.usageCalls = 0
	database.usageTokens = 1000
	if err := service.CheckDailyQuota(context.Background()); err == nil {
		t.Fatal("token quota exceeded should fail")
	}
	service.config.DailyMaxCalls = 0
	service.config.DailyMaxTokens = 0
	if err := service.CheckDailyQuota(context.Background()); err != nil {
		t.Fatalf("quota disabled should pass: %v", err)
	}
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
	chatContext, err := service.ChatBuildContext(context.Background())
	if err != nil {
		t.Fatalf("build context: %v", err)
	}
	if !strings.Contains(chatContext.Text, "recentAnalyses") {
		t.Fatalf("context should contain recentAnalyses key: %s", chatContext.Text)
	}
	if len(chatContext.Refs.WindowIDs) != 0 || len(chatContext.Refs.AlertIDs) != 0 || len(chatContext.Refs.CommandIDs) != 0 {
		t.Fatalf("empty store should produce empty refs, got %+v", chatContext.Refs)
	}
	// 有数据时：refs 收集窗口总结 / 警戒 / 意图命令 ID（与文本截断无关）。
	fake := service.database.(*fakeStore)
	fake.mu.Lock()
	fake.windowSummaries = []model.AIOpsWindowSummary{
		{WindowID: "w-1", Level: string(model.AIOpsWindowL3)},
		{WindowID: "w-2", Level: string(model.AIOpsWindowL3)},
	}
	fake.alerts = []model.AIOpsAlert{{AlertID: "a-1"}, {AlertID: "a-2"}}
	fake.commands = []model.AIOpsCommand{{CommandID: "c-1"}, {CommandID: "c-2"}}
	fake.mu.Unlock()

	chatContext, err = service.ChatBuildContext(context.Background())
	if err != nil {
		t.Fatalf("build context with data: %v", err)
	}
	wantWindows := []string{"w-1", "w-2"}
	if len(chatContext.Refs.WindowIDs) != len(wantWindows) {
		t.Fatalf("window refs = %v, want %v", chatContext.Refs.WindowIDs, wantWindows)
	}
	for i, want := range wantWindows {
		if chatContext.Refs.WindowIDs[i] != want {
			t.Fatalf("window refs = %v, want %v", chatContext.Refs.WindowIDs, wantWindows)
		}
	}
	wantAlerts := []string{"a-1", "a-2"}
	if len(chatContext.Refs.AlertIDs) != len(wantAlerts) {
		t.Fatalf("alert refs = %v, want %v", chatContext.Refs.AlertIDs, wantAlerts)
	}
	for i, want := range wantAlerts {
		if chatContext.Refs.AlertIDs[i] != want {
			t.Fatalf("alert refs = %v, want %v", chatContext.Refs.AlertIDs, wantAlerts)
		}
	}
	wantCommands := []string{"c-1", "c-2"}
	if len(chatContext.Refs.CommandIDs) != len(wantCommands) {
		t.Fatalf("command refs = %v, want %v", chatContext.Refs.CommandIDs, wantCommands)
	}
	for i, want := range wantCommands {
		if chatContext.Refs.CommandIDs[i] != want {
			t.Fatalf("command refs = %v, want %v", chatContext.Refs.CommandIDs, wantCommands)
		}
	}
}

func TestChatStreamEvents(t *testing.T) {
	service := chatTestService(t, config.AIOpsConfig{})
	var tools []string
	var deltas []string
	var usage TokenUsage
	_, err := service.ChatStream(context.Background(), "当前集群什么情况？",
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

func TestChatRecord(t *testing.T) {
	service := chatTestService(t, config.AIOpsConfig{})
	service.ChatRecord(context.Background(), "session-rec", "当前集群什么情况？", "总体稳定。", ChatContextRefs{
		WindowIDs:  []string{"w-1", "w-2"},
		AlertIDs:   []string{"a-1"},
		CommandIDs: []string{"c-1"},
	})
	fake := service.database.(*fakeStore)
	fake.mu.Lock()
	messages := append([]model.AIOpsChatMessage(nil), fake.chatMessages...)
	fake.mu.Unlock()
	if len(messages) != 2 {
		t.Fatalf("want 2 persisted messages (user+assistant), got %d", len(messages))
	}
	if messages[0].Role != "user" || messages[0].Content != "当前集群什么情况？" {
		t.Fatalf("user message = %+v", messages[0])
	}
	if messages[1].Role != "assistant" || messages[1].Content != "总体稳定。" {
		t.Fatalf("assistant message = %+v", messages[1])
	}
	if messages[0].SessionID != "session-rec" || messages[1].SessionID != "session-rec" {
		t.Fatalf("session id not persisted: %+v", messages)
	}
	// 引用 ID 只落在 assistant 消息；user 消息为空数组。
	var windowIDs, alertIDs, commandIDs []string
	if err := json.Unmarshal(messages[1].WindowIDs, &windowIDs); err != nil {
		t.Fatalf("unmarshal assistant window ids: %v", err)
	}
	if err := json.Unmarshal(messages[1].AlertIDs, &alertIDs); err != nil {
		t.Fatalf("unmarshal assistant alert ids: %v", err)
	}
	if err := json.Unmarshal(messages[1].CommandIDs, &commandIDs); err != nil {
		t.Fatalf("unmarshal assistant command ids: %v", err)
	}
	if len(windowIDs) != 2 || windowIDs[0] != "w-1" || windowIDs[1] != "w-2" {
		t.Fatalf("assistant window ids = %v", windowIDs)
	}
	if len(alertIDs) != 1 || alertIDs[0] != "a-1" {
		t.Fatalf("assistant alert ids = %v", alertIDs)
	}
	if len(commandIDs) != 1 || commandIDs[0] != "c-1" {
		t.Fatalf("assistant command ids = %v", commandIDs)
	}
	if string(messages[0].WindowIDs) != "[]" || string(messages[0].AlertIDs) != "[]" || string(messages[0].CommandIDs) != "[]" {
		t.Fatalf("user message refs should be empty arrays, got %+v", messages[0])
	}
	// 匿名会话回退。
	service.ChatRecord(context.Background(), "", "问题", "回答", ChatContextRefs{})
	fake.mu.Lock()
	messages = append([]model.AIOpsChatMessage(nil), fake.chatMessages...)
	fake.mu.Unlock()
	if len(messages) != 4 {
		t.Fatalf("want 4 messages after anonymous record, got %d", len(messages))
	}
	if messages[2].SessionID != "anonymous" || messages[3].SessionID != "anonymous" {
		t.Fatalf("anonymous session fallback failed: %+v", messages[2:])
	}
}
