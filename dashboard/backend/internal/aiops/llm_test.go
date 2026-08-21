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
	content, err := client.CompleteJSON(context.Background(), "sys", "user", 500)
	if err != nil {
		t.Fatalf("CompleteJSON: %v", err)
	}
	var decoded map[string]bool
	if err := json.Unmarshal([]byte(content), &decoded); err != nil || !decoded["ok"] {
		t.Fatalf("unexpected content: %s", content)
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
	content, err := client.CompleteJSON(context.Background(), "sys", "user", 500)
	if err != nil {
		t.Fatalf("CompleteJSON after retry: %v", err)
	}
	if content != "{}" {
		t.Fatalf("unexpected content: %s", content)
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
	if _, err := client.CompleteJSON(context.Background(), "sys", "user", 500); err == nil {
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
