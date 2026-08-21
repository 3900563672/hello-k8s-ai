package aiops

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/model"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, nil))
}

func newTestClient(baseURL string) *OpenAI {
	client := &OpenAI{
		baseURL: baseURL,
		apiKey:  "test-key",
		model:   "test-model",
		timeout: 5 * time.Second,
		client:  &http.Client{Timeout: 5 * time.Second},
		logger:  testLogger(),
	}
	client.maxTokens = 1000
	return client
}

// TestOpenAICompleteJSON 验证正常响应解析。
func TestOpenAICompleteJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/chat/completions" {
			t.Errorf("path = %s", request.URL.Path)
		}
		if request.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("missing authorization header")
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"choices":[{"message":{"content":"{\"ok\":true}"}}]}`))
	}))
	defer server.Close()
	client := newTestClient(server.URL)
	completion, err := client.CompleteJSON(context.Background(), "sys", "user", 500, 0)
	if err != nil {
		t.Fatalf("CompleteJSON: %v", err)
	}
	var decoded map[string]bool
	if err := json.Unmarshal([]byte(completion.Content), &decoded); err != nil || !decoded["ok"] {
		t.Fatalf("unexpected content: %s", completion.Content)
	}
}

// TestOpenAIRetry 验证 500 后重试成功。
func TestOpenAIRetry(t *testing.T) {
	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if calls.Add(1) == 1 {
			http.Error(writer, "boom", http.StatusInternalServerError)
			return
		}
		_, _ = writer.Write([]byte(`{"choices":[{"message":{"content":"{}"}}]}`))
	}))
	defer server.Close()
	client := newTestClient(server.URL)
	start := time.Now()
	completion, err := client.CompleteJSON(context.Background(), "sys", "user", 500, 0)
	if err != nil {
		t.Fatalf("CompleteJSON after retry: %v", err)
	}
	if completion.Content != "{}" {
		t.Fatalf("unexpected content: %s", completion.Content)
	}
	if calls.Load() < 2 {
		t.Fatalf("expected at least 2 calls, got %d", calls.Load())
	}
	if time.Since(start) < 400*time.Millisecond {
		t.Fatalf("retry backoff too short: %s", time.Since(start))
	}
}

// TestOpenAIErrorResponse 验证 400 直接失败不重试。
func TestOpenAIErrorResponse(t *testing.T) {
	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		calls.Add(1)
		http.Error(writer, `{"error":{"message":"bad request"}}`, http.StatusBadRequest)
	}))
	defer server.Close()
	client := newTestClient(server.URL)
	if _, err := client.CompleteJSON(context.Background(), "sys", "user", 500, 0); err == nil {
		t.Fatalf("expected error")
	}
	if calls.Load() != 1 {
		t.Fatalf("expected single call, got %d", calls.Load())
	}
}

// TestNormalizeScores 验证分数钳制与 verdict 规范化。
func TestNormalizeScores(t *testing.T) {
	scores := normalizeScores(model.AIOpsScores{Goal: 120, Stability: -5, Efficiency: 50, Anomaly: 80, Overall: 999, Verdict: "weird"})
	if scores.Goal != 100 || scores.Stability != 0 || scores.Overall != 100 {
		t.Fatalf("unexpected normalized scores: %+v", scores)
	}
	if scores.Verdict != "attention" {
		t.Fatalf("verdict = %s, want attention", scores.Verdict)
	}
}

// TestOpenAIStreamComplete 验证流式响应：逐 chunk 回调增量，[DONE] 正常结束。
func TestOpenAIStreamComplete(t *testing.T) {
	streamBody := "data: {\"choices\":[{\"delta\":{\"content\":\"你\"}}]}\n\n" +
		"data: {\"choices\":[{\"delta\":{\"content\":\"好\"}}]}\n\n" +
		"data: {\"choices\":[{\"delta\":{\"content\":\"！\"}}]}\n\n" +
		"data: {\"usage\":{\"prompt_tokens\":11,\"completion_tokens\":3}}\n\n" +
		"data: [DONE]\n\n"
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/chat/completions" {
			t.Errorf("path = %s", request.URL.Path)
		}
		writer.Header().Set("Content-Type", "text/event-stream")
		_, _ = writer.Write([]byte(streamBody))
	}))
	defer server.Close()
	client := newTestClient(server.URL)
	var deltas []string
	var gotUsage TokenUsage
	if err := client.StreamComplete(context.Background(), "sys", "user", 500, 0, func(delta string) {
		deltas = append(deltas, delta)
	}, func(usage TokenUsage) { gotUsage = usage }); err != nil {
		t.Fatalf("StreamComplete: %v", err)
	}
	joined := ""
	for _, delta := range deltas {
		joined += delta
	}
	if joined != "你好！" {
		t.Fatalf("streamed content = %q, want 你好！", joined)
	}
	if gotUsage.PromptTokens != 11 || gotUsage.CompletionTokens != 3 {
		t.Fatalf("usage = %+v, want prompt=11 completion=3", gotUsage)
	}
}

// TestOpenAIStreamCompleteHTTPError 验证非 200 时返回错误。
func TestOpenAIStreamCompleteHTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		http.Error(writer, "boom", http.StatusInternalServerError)
	}))
	defer server.Close()
	client := newTestClient(server.URL)
	if err := client.StreamComplete(context.Background(), "sys", "user", 500, 0, func(string) {}, nil); err == nil {
		t.Fatal("stream should fail on HTTP error")
	}
}

// TestOpenAICompleteJSONUsage 验证非流式响应解析 usage（#112 token 记录）。
func TestOpenAICompleteJSONUsage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"choices":[{"message":{"content":"{}"}}],"usage":{"prompt_tokens":12,"completion_tokens":7}}`))
	}))
	defer server.Close()
	client := newTestClient(server.URL)
	completion, err := client.CompleteJSON(context.Background(), "sys", "user", 500, 0)
	if err != nil {
		t.Fatalf("CompleteJSON: %v", err)
	}
	if completion.Content != "{}" {
		t.Fatalf("unexpected content: %s", completion.Content)
	}
	if completion.Usage.PromptTokens != 12 || completion.Usage.CompletionTokens != 7 {
		t.Fatalf("unexpected usage: %+v", completion.Usage)
	}
}

// TestOpenAICompleteJSONNoUsage 验证服务端不返回 usage 时零值兜底。
func TestOpenAICompleteJSONNoUsage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"choices":[{"message":{"content":"{}"}}]}`))
	}))
	defer server.Close()
	client := newTestClient(server.URL)
	completion, err := client.CompleteJSON(context.Background(), "sys", "user", 500, 0)
	if err != nil {
		t.Fatalf("CompleteJSON: %v", err)
	}
	if completion.Usage.PromptTokens != 0 || completion.Usage.CompletionTokens != 0 {
		t.Fatalf("expected zero usage, got %+v", completion.Usage)
	}
}

// TestOptionalTemperature 验证温度参数：0 不发送，非 0 发送指针。
func TestOptionalTemperature(t *testing.T) {
	if optionalTemperature(0) != nil {
		t.Fatalf("temperature 0 should be nil")
	}
	value := optionalTemperature(0.1)
	if value == nil || *value != 0.1 {
		t.Fatalf("temperature 0.1 should be pointer, got %v", value)
	}
}

// TestOpenAITemperaturePayload 验证 temperature 分层真实进入请求体：
// 分析层 0.1 显式发送；0 省略（服务端默认），不覆盖服务端配置。
func TestOpenAITemperaturePayload(t *testing.T) {
	var payloads []map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			t.Errorf("decode payload: %v", err)
		}
		payloads = append(payloads, payload)
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"choices":[{"message":{"content":"{}"}}]}`))
	}))
	defer server.Close()
	client := newTestClient(server.URL)
	if _, err := client.CompleteJSON(context.Background(), "sys", "user", 500, 0.1); err != nil {
		t.Fatalf("CompleteJSON: %v", err)
	}
	if len(payloads) != 1 {
		t.Fatalf("want 1 call, got %d", len(payloads))
	}
	temperature, ok := payloads[0]["temperature"].(float64)
	if !ok || temperature != 0.1 {
		t.Fatalf("temperature = %v, want 0.1", payloads[0]["temperature"])
	}
	if _, err := client.CompleteJSON(context.Background(), "sys", "user", 500, 0); err != nil {
		t.Fatalf("CompleteJSON with zero temperature: %v", err)
	}
	if _, exists := payloads[1]["temperature"]; exists {
		t.Fatalf("temperature should be omitted when 0, got %v", payloads[1]["temperature"])
	}
}

// TestOpenAIStreamTemperaturePayload 验证流式调用同样携带 temperature（对话层 0.5）。
func TestOpenAIStreamTemperaturePayload(t *testing.T) {
	var got float64
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var payload struct {
			Temperature *float64 `json:"temperature"`
		}
		_ = json.NewDecoder(request.Body).Decode(&payload)
		if payload.Temperature != nil {
			got = *payload.Temperature
		}
		writer.Header().Set("Content-Type", "text/event-stream")
		_, _ = writer.Write([]byte("data: {\"choices\":[{\"delta\":{\"content\":\"好\"}}]}\n\ndata: [DONE]\n\n"))
	}))
	defer server.Close()
	client := newTestClient(server.URL)
	if err := client.StreamComplete(context.Background(), "sys", "user", 500, 0.5, func(string) {}, func(TokenUsage) {}); err != nil {
		t.Fatalf("StreamComplete: %v", err)
	}
	if got != 0.5 {
		t.Fatalf("stream temperature = %v, want 0.5", got)
	}
}
