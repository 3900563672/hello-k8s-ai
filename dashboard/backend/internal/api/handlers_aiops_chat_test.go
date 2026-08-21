package api

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/aiops"
	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/config"
	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/model"
	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/store"
)

// chatStoreStub 只实现对话历史读写；其余方法走 Disabled 兜底。
type chatStoreStub struct {
	store.Disabled
	mu       sync.Mutex
	messages []model.AIOpsChatMessage
}

func (stub *chatStoreStub) Available() bool { return true }

func (stub *chatStoreStub) CreateAIOpsChatMessage(_ context.Context, message model.AIOpsChatMessage) error {
	stub.mu.Lock()
	defer stub.mu.Unlock()
	stub.messages = append(stub.messages, message)
	return nil
}

func (stub *chatStoreStub) ListAIOpsChatMessages(_ context.Context, sessionID string, limit int) ([]model.AIOpsChatMessage, error) {
	stub.mu.Lock()
	defer stub.mu.Unlock()
	var messages []model.AIOpsChatMessage
	for _, message := range stub.messages {
		if message.SessionID == sessionID {
			messages = append(messages, message)
		}
	}
	if len(messages) > limit {
		messages = messages[len(messages)-limit:]
	}
	return messages, nil
}

func newChatHistoryTestServer(t *testing.T, stub *chatStoreStub, withAIOps bool) *Server {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := &Server{logger: logger, store: stub}
	if withAIOps {
		server.aiops = aiops.NewService(config.AIOpsConfig{}, stub, nil, logger)
	}
	return server
}

func performChatHistoryRequest(t *testing.T, server *Server, path string) *httptest.ResponseRecorder {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, path, nil)
	recorder := httptest.NewRecorder()
	server.handleListAIOpsChatMessages(recorder, request)
	return recorder
}

func TestListAIOpsChatMessages(t *testing.T) {
	stub := &chatStoreStub{}
	server := newChatHistoryTestServer(t, stub, true)
	service := server.aiops
	service.ChatRecord(context.Background(), "session-a", "问题一", "回答一", aiops.ChatContextRefs{})
	service.ChatRecord(context.Background(), "session-a", "问题二", "回答二", aiops.ChatContextRefs{})
	service.ChatRecord(context.Background(), "session-b", "别处问题", "别处回答", aiops.ChatContextRefs{})

	recorder := performChatHistoryRequest(t, server, "/api/v1/aiops/chat/messages?sessionId=session-a&limit=10")
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}
	var envelope struct {
		Data []model.AIOpsChatMessage `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response: %v\nbody=%s", err, recorder.Body.String())
	}
	if len(envelope.Data) != 4 {
		t.Fatalf("messages = %d, want 4", len(envelope.Data))
	}
	if envelope.Data[0].Role != "user" || envelope.Data[0].Content != "问题一" ||
		envelope.Data[3].Role != "assistant" || envelope.Data[3].Content != "回答二" {
		t.Fatalf("unexpected history: %+v", envelope.Data)
	}

	// 无消息会话返回空数组（非 null）。
	empty := performChatHistoryRequest(t, server, "/api/v1/aiops/chat/messages?sessionId=no-such-session")
	if empty.Code != http.StatusOK {
		t.Fatalf("empty session status = %d, want 200", empty.Code)
	}
	var emptyEnvelope struct {
		Data []model.AIOpsChatMessage `json:"data"`
	}
	if err := json.Unmarshal(empty.Body.Bytes(), &emptyEnvelope); err != nil {
		t.Fatalf("decode empty response: %v\nbody=%s", err, empty.Body.String())
	}
	if emptyEnvelope.Data == nil {
		t.Fatal("empty session data should be [] not null")
	}
}

func TestListAIOpsChatMessagesRejectsInvalidInput(t *testing.T) {
	server := newChatHistoryTestServer(t, &chatStoreStub{}, true)
	cases := []struct {
		name string
		path string
	}{
		{name: "missing session id", path: "/api/v1/aiops/chat/messages"},
		{name: "limit zero", path: "/api/v1/aiops/chat/messages?sessionId=a&limit=0"},
		{name: "limit too large", path: "/api/v1/aiops/chat/messages?sessionId=a&limit=201"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			recorder := performChatHistoryRequest(t, server, test.path)
			if recorder.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400; body=%s", recorder.Code, recorder.Body.String())
			}
		})
	}
}

func TestListAIOpsChatMessagesDisabled(t *testing.T) {
	server := newChatHistoryTestServer(t, &chatStoreStub{}, false)
	recorder := performChatHistoryRequest(t, server, "/api/v1/aiops/chat/messages?sessionId=a")
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body=%s", recorder.Code, recorder.Body.String())
	}
}
