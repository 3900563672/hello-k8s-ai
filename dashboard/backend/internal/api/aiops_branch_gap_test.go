package api

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/aiops"
	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/config"
	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/model"
	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/store"
)

// unavailableStoreStub store 不可用场景。
type unavailableStoreStub struct {
	store.Disabled
}

func (stub *unavailableStoreStub) Available() bool { return false }

// noFlusherWriter 不实现 http.Flusher，触发 SSE_UNSUPPORTED。
type noFlusherWriter struct {
	http.ResponseWriter
}

func TestRequireAIOpsStoreUnavailable(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stub := &unavailableStoreStub{}
	server := &Server{logger: logger, store: stub}
	server.aiops = aiops.NewService(config.AIOpsConfig{}, stub, &apiFakeLLM{}, logger)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/aiops/analyses", nil)
	recorder := httptest.NewRecorder()
	server.handleListAIOpsAnalyses(recorder, request)
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("store unavailable = %d, want 503; body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestHandleListAIOpsJobsBranches(t *testing.T) {
	stub := &commandStoreStub{commands: map[string]model.AIOpsCommand{}}
	server := newCommandTestServer(stub)
	badStatus := httptest.NewRequest(http.MethodGet, "/api/v1/aiops/jobs?status=weird", nil)
	recorder := httptest.NewRecorder()
	server.handleListAIOpsJobs(recorder, badStatus)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("invalid job status = %d, want 400", recorder.Code)
	}
	ok := httptest.NewRequest(http.MethodGet, "/api/v1/aiops/jobs", nil)
	recorder = httptest.NewRecorder()
	server.handleListAIOpsJobs(recorder, ok)
	if recorder.Code != http.StatusOK {
		t.Fatalf("jobs = %d, want 200", recorder.Code)
	}
}

func TestStreamChatSSEUnsupported(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stub := &chatStoreStub{}
	server := &Server{logger: logger, store: stub}
	server.aiops = aiops.NewService(config.AIOpsConfig{}, stub, &apiFakeLLM{}, logger)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/aiops/chat", strings.NewReader(`{"message":"hi","sessionId":"s-1"}`))
	recorder := httptest.NewRecorder()
	server.streamChat(&noFlusherWriter{ResponseWriter: recorder}, request, "s-1", "hi")
	if recorder.Code != http.StatusInternalServerError || !strings.Contains(recorder.Body.String(), "SSE_UNSUPPORTED") {
		t.Fatalf("no flusher = %d %s", recorder.Code, recorder.Body.String())
	}
}

// aiopsErrorStore 让 ListAIOpsEntitySummaries 返回错误，覆盖 writeAIOpsAnalysis 错误分支。
type aiopsErrorStore struct {
	aiopsStoreStub
}

func (stub *aiopsErrorStore) ListAIOpsEntitySummaries(context.Context, string) ([]model.AIOpsEntitySummary, error) {
	return nil, io.ErrUnexpectedEOF
}

func TestWriteAIOpsAnalysisSummariesError(t *testing.T) {
	stub := &aiopsErrorStore{}
	stub.aiopsStoreStub.byID = map[string]*model.AIOpsAnalysis{"a-1": {AnalysisID: "a-1"}}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := &Server{logger: logger, store: stub}
	server.aiops = aiops.NewService(config.AIOpsConfig{}, stub, &apiFakeLLM{}, logger)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/aiops/analyses/a-1", nil)
	request.SetPathValue("id", "a-1")
	recorder := httptest.NewRecorder()
	server.handleGetAIOpsAnalysis(recorder, request)
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("summaries error = %d, want 503; body=%s", recorder.Code, recorder.Body.String())
	}
}
