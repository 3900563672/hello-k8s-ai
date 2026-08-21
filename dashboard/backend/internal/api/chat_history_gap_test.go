package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHandleListAIOpsChatMessagesValidation(t *testing.T) {
	stub := &chatStoreStub{}
	server := newChatHistoryTestServer(t, stub, true)
	missingSession := httptest.NewRequest(http.MethodGet, "/api/v1/aiops/chat/messages", nil)
	recorder := httptest.NewRecorder()
	server.handleListAIOpsChatMessages(recorder, missingSession)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("missing sessionId = %d, want 400", recorder.Code)
	}
	badLimit := httptest.NewRequest(http.MethodGet, "/api/v1/aiops/chat/messages?sessionId=s-1&limit=999", nil)
	recorder = httptest.NewRecorder()
	server.handleListAIOpsChatMessages(recorder, badLimit)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("bad limit = %d, want 400", recorder.Code)
	}
}
