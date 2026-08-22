package api

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/aiops"
	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/config"
	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/store"
)

// streamingChatStore 组合对话历史 stub 与幂等三件套，验证中间件对 SSE 请求透传 writer。
type streamingChatStore struct {
	chatStoreStub
	completed []string
}

func (stub *streamingChatStore) ReserveIdempotency(
	_ context.Context, key string, _ string, _ time.Time,
) (*store.IdempotencyRecord, bool, error) {
	return &store.IdempotencyRecord{Key: key, State: "pending"}, true, nil
}

func (stub *streamingChatStore) CompleteIdempotency(_ context.Context, key string, _ string, _ int, _ json.RawMessage) error {
	stub.completed = append(stub.completed, key)
	return nil
}

func (stub *streamingChatStore) ReleaseIdempotency(context.Context, string, string) error { return nil }

// TestIdempotencyMiddlewareStreamsChatWithoutBuffering 回归：#110 chat SSE 被
// idempotencyMiddleware 的 bufferedResponse 吞掉后 Flusher 断言失败（SSE_UNSUPPORTED），
// 线上 AIOps 对话不可用。修复后流式请求应透传 writer 且幂等记录照常完成。
func TestIdempotencyMiddlewareStreamsChatWithoutBuffering(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stub := &streamingChatStore{}
	server := &Server{logger: logger, store: stub}
	server.aiops = aiops.NewService(config.AIOpsConfig{ChatMaxMessageLen: 4000, ChatRatePerMinute: 6}, stub, &apiFakeLLM{}, logger)

	handler := idempotencyMiddleware(stub, 1<<20, logger, http.HandlerFunc(server.handleAIOpsChat))
	body := strings.NewReader(`{"message":"你好","sessionId":"s-stream"}`)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/aiops/chat", body)
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "text/event-stream")
	request.Header.Set("Idempotency-Key", "stream-1")
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("chat via middleware = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}
	output := recorder.Body.String()
	if strings.Contains(output, "SSE_UNSUPPORTED") {
		t.Fatalf("stream was blocked by buffered writer: %s", output)
	}
	for _, fragment := range []string{`"type":"lifecycle"`, `"type":"text"`, `"phase":"end"`} {
		if !strings.Contains(output, fragment) {
			t.Fatalf("stream 缺少 %q: %s", fragment, output)
		}
	}
	if len(stub.completed) != 1 || stub.completed[0] != "stream-1" {
		t.Fatalf("idempotency completed = %v, want [stream-1]", stub.completed)
	}

	// 非流式写请求仍走缓冲 + 完整幂等记录。
	stub.completed = nil
	regular := httptest.NewRequest(http.MethodPost, "/api/v1/aiops/chat", strings.NewReader(`{"message":"hi","sessionId":"s-2"}`))
	regular.Header.Set("Content-Type", "application/json")
	regular.Header.Set("Idempotency-Key", "regular-1")
	recorder = httptest.NewRecorder()
	handler.ServeHTTP(recorder, regular)
	if recorder.Code != http.StatusOK {
		t.Fatalf("regular chat = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}
	if len(stub.completed) != 1 || stub.completed[0] != "regular-1" {
		t.Fatalf("regular idempotency completed = %v, want [regular-1]", stub.completed)
	}
}
