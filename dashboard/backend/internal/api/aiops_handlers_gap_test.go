package api

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/aiops"
	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/clock"
	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/config"
	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/model"
	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/store"
	"github.com/jackc/pgx/v5"
)

// aiopsStoreStub 实现 AIOps 读侧 handler 所需方法；其余走 Disabled 兜底。
type aiopsStoreStub struct {
	store.Disabled
	analyses  []model.AIOpsAnalysis
	byID      map[string]*model.AIOpsAnalysis
	summaries []model.AIOpsEntitySummary
	windows   []model.AIOpsWindowSummary
	alerts    []model.AIOpsAlert
}

func (stub *aiopsStoreStub) Available() bool { return true }

func (stub *aiopsStoreStub) ListAIOpsAnalyses(_ context.Context, _ int, _ string) ([]model.AIOpsAnalysis, error) {
	return stub.analyses, nil
}

func (stub *aiopsStoreStub) GetAIOpsAnalysis(_ context.Context, analysisID string) (*model.AIOpsAnalysis, error) {
	if analysis, ok := stub.byID[analysisID]; ok {
		return analysis, nil
	}
	return nil, pgx.ErrNoRows
}

func (stub *aiopsStoreStub) GetAIOpsAnalysisBySegment(_ context.Context, segmentID string) (*model.AIOpsAnalysis, error) {
	for _, analysis := range stub.byID {
		if analysis.SegmentID == segmentID {
			return analysis, nil
		}
	}
	return nil, pgx.ErrNoRows
}

func (stub *aiopsStoreStub) ListAIOpsEntitySummaries(_ context.Context, _ string) ([]model.AIOpsEntitySummary, error) {
	return stub.summaries, nil
}

func (stub *aiopsStoreStub) ListAIOpsWindowSummaries(_ context.Context, _ string, _ int) ([]model.AIOpsWindowSummary, error) {
	return stub.windows, nil
}

func (stub *aiopsStoreStub) ListAIOpsAlerts(_ context.Context, _ int) ([]model.AIOpsAlert, error) {
	return stub.alerts, nil
}

func newAIOpsHandlerServer(stub *aiopsStoreStub) *Server {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := &Server{logger: logger, store: stub}
	server.aiops = aiops.NewService(config.AIOpsConfig{}, stub, nil, logger)
	return server
}

func TestEventBusPublishSubscribe(t *testing.T) {
	bus := NewEventBus()
	events, unsubscribe := bus.Subscribe()
	change := model.ResourceChange{
		EventID: "evt-1", Operation: "update",
		Ref: model.ResourceRef{Kind: "Pod", Name: "pod-1"},
	}
	bus.Publish(change)
	select {
	case event := <-events:
		if event.ID != "evt-1" || event.Type != "resource.changed" || event.ResourceRef.Name != "pod-1" {
			t.Fatalf("event = %+v", event)
		}
	case <-time.After(time.Second):
		t.Fatal("subscriber did not receive published event")
	}
	unsubscribe()
	select {
	case _, open := <-events:
		if open {
			t.Fatal("unsubscribe 后 channel 应被关闭")
		}
	case <-time.After(time.Second):
		t.Fatal("unsubscribe 后 channel 应被关闭")
	}
}

func TestEventBusSlowConsumerDropsWithoutBlocking(t *testing.T) {
	bus := NewEventBus()
	channel, unsubscribe := bus.Subscribe()
	defer unsubscribe()
	done := make(chan struct{})
	go func() {
		// 填满 128 buffer 后再发布也不应阻塞。
		for index := 0; index < 200; index++ {
			bus.Publish(model.ResourceChange{EventID: "evt"})
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Publish blocked on slow subscriber")
	}
	_ = channel
}

func TestHandleListAIOpsAnalyses(t *testing.T) {
	stub := &aiopsStoreStub{analyses: []model.AIOpsAnalysis{{AnalysisID: "a-1"}}}
	server := newAIOpsHandlerServer(stub)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/aiops/analyses", nil)
	recorder := httptest.NewRecorder()
	server.handleListAIOpsAnalyses(recorder, request)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), "a-1") {
		t.Fatalf("list analyses = %d %s", recorder.Code, recorder.Body.String())
	}
	badStatus := httptest.NewRequest(http.MethodGet, "/api/v1/aiops/analyses?status=weird", nil)
	recorder = httptest.NewRecorder()
	server.handleListAIOpsAnalyses(recorder, badStatus)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("invalid status = %d, want 400", recorder.Code)
	}
	badLimit := httptest.NewRequest(http.MethodGet, "/api/v1/aiops/analyses?limit=999", nil)
	recorder = httptest.NewRecorder()
	server.handleListAIOpsAnalyses(recorder, badLimit)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("invalid limit = %d, want 400", recorder.Code)
	}
}

func TestHandleGetAIOpsAnalysis(t *testing.T) {
	stub := &aiopsStoreStub{byID: map[string]*model.AIOpsAnalysis{
		"a-1": {AnalysisID: "a-1", SegmentID: "seg-1"},
	}}
	server := newAIOpsHandlerServer(stub)
	byID := httptest.NewRequest(http.MethodGet, "/api/v1/aiops/analyses/a-1", nil)
	byID.SetPathValue("id", "a-1")
	recorder := httptest.NewRecorder()
	server.handleGetAIOpsAnalysis(recorder, byID)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), "seg-1") {
		t.Fatalf("get by id = %d %s", recorder.Code, recorder.Body.String())
	}
	bySegment := httptest.NewRequest(http.MethodGet, "/api/v1/aiops/analyses?segmentId=seg-1", nil)
	recorder = httptest.NewRecorder()
	server.handleGetAIOpsAnalysis(recorder, bySegment)
	if recorder.Code != http.StatusOK {
		t.Fatalf("get by segment = %d, want 200", recorder.Code)
	}
	missing := httptest.NewRequest(http.MethodGet, "/api/v1/aiops/analyses/nope", nil)
	missing.SetPathValue("id", "nope")
	recorder = httptest.NewRecorder()
	server.handleGetAIOpsAnalysis(recorder, missing)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("missing analysis = %d, want 404", recorder.Code)
	}
}

func TestHandleListAIOpsWindows(t *testing.T) {
	stub := &aiopsStoreStub{windows: []model.AIOpsWindowSummary{{WindowID: "L3-x"}}}
	server := newAIOpsHandlerServer(stub)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/aiops/windows", nil)
	recorder := httptest.NewRecorder()
	server.handleListAIOpsWindows(recorder, request)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), "L3-x") {
		t.Fatalf("list windows = %d %s", recorder.Code, recorder.Body.String())
	}
	badLevel := httptest.NewRequest(http.MethodGet, "/api/v1/aiops/windows?level=L5", nil)
	recorder = httptest.NewRecorder()
	server.handleListAIOpsWindows(recorder, badLevel)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("invalid level = %d, want 400", recorder.Code)
	}
}

func TestHandleListAIOpsAlerts(t *testing.T) {
	stub := &aiopsStoreStub{alerts: []model.AIOpsAlert{{AlertID: "al-1"}}}
	server := newAIOpsHandlerServer(stub)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/aiops/alerts", nil)
	recorder := httptest.NewRecorder()
	server.handleListAIOpsAlerts(recorder, request)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), "al-1") {
		t.Fatalf("list alerts = %d %s", recorder.Code, recorder.Body.String())
	}
	badLimit := httptest.NewRequest(http.MethodGet, "/api/v1/aiops/alerts?limit=0", nil)
	recorder = httptest.NewRecorder()
	server.handleListAIOpsAlerts(recorder, badLimit)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("invalid limit = %d, want 400", recorder.Code)
	}
}

func TestValidAIOpsStatus(t *testing.T) {
	for _, status := range []string{"pending", "running", "aggregating", "completed", "failed"} {
		if !validAIOpsStatus(status) {
			t.Fatalf("validAIOpsStatus(%q) = false", status)
		}
	}
	if validAIOpsStatus("weird") {
		t.Fatal("validAIOpsStatus(weird) = true")
	}
}

func TestNewServerAndClockState(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	clockInstance := clock.New()
	server := NewServer(Dependencies{Logger: logger, Clock: clockInstance, Store: &aiopsStoreStub{}})
	if server.clock != clockInstance {
		t.Fatal("NewServer did not wire clock")
	}
	server.currentClockState() // 存储可用时不应 panic/禁用写能力
}

func TestHandleStreamPublishesEvents(t *testing.T) {
	bus := NewEventBus()
	server := &Server{logger: slog.New(slog.NewTextHandler(io.Discard, nil)), events: bus}
	ctx, cancel := context.WithCancel(context.Background())
	request := httptest.NewRequest(http.MethodGet, "/api/v1/stream", nil).WithContext(ctx)
	recorder := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		server.handleStream(recorder, request)
		close(done)
	}()
	time.Sleep(20 * time.Millisecond)
	bus.Publish(model.ResourceChange{
		EventID: "evt-1", Operation: "update",
		Ref: model.ResourceRef{Kind: "Pod", Name: "pod-1"},
	})
	deadline := time.Now().Add(2 * time.Second)
	for !strings.Contains(recorder.Body.String(), "resource.changed") {
		if time.Now().After(deadline) {
			t.Fatal("stream did not emit published event")
		}
		time.Sleep(10 * time.Millisecond)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("handleStream did not stop after cancel")
	}
}

func TestCorsPreflight(t *testing.T) {
	next := http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusOK)
	})
	middleware := corsMiddleware([]string{"https://app.example.com"}, next)

	preflight := httptest.NewRequest(http.MethodOptions, "/api/v1/x", nil)
	preflight.Header.Set("Origin", "https://app.example.com")
	recorder := httptest.NewRecorder()
	middleware.ServeHTTP(recorder, preflight)
	if recorder.Code != http.StatusNoContent || recorder.Header().Get("Access-Control-Allow-Origin") != "https://app.example.com" {
		t.Fatalf("preflight = %d %v", recorder.Code, recorder.Header())
	}

	other := httptest.NewRequest(http.MethodGet, "/api/v1/x", nil)
	other.Header.Set("Origin", "https://evil.example.com")
	recorder = httptest.NewRecorder()
	middleware.ServeHTTP(recorder, other)
	if recorder.Header().Get("Access-Control-Allow-Origin") != "" {
		t.Fatal("不允许的 origin 不应获得 CORS 头")
	}
}

func TestRequestTimeoutSkipsStreamAndAppliesElsewhere(t *testing.T) {
	streamNext := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if _, ok := request.Context().Deadline(); ok {
			t.Fatal("stream 路径不应设置超时")
		}
		writer.WriteHeader(http.StatusOK)
	})
	request := httptest.NewRequest(http.MethodGet, "/api/v1/stream", nil)
	recorder := httptest.NewRecorder()
	requestTimeoutMiddleware(5*time.Second, streamNext).ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("stream = %d", recorder.Code)
	}

	timeoutNext := http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		deadline, ok := request.Context().Deadline()
		if !ok || time.Until(deadline) > 5*time.Second {
			t.Fatalf("普通路径应带 5s 超时: ok=%v deadline=%v", ok, deadline)
		}
		writer.WriteHeader(http.StatusOK)
	})
	other := httptest.NewRequest(http.MethodGet, "/api/v1/experiments", nil)
	recorder = httptest.NewRecorder()
	requestTimeoutMiddleware(5*time.Second, timeoutNext).ServeHTTP(recorder, other)
}

func TestStatusRecorderFlushUnwrap(t *testing.T) {
	underlying := httptest.NewRecorder()
	recorder := &statusRecorder{ResponseWriter: underlying}
	recorder.WriteHeader(http.StatusCreated)
	if recorder.status != http.StatusCreated {
		t.Fatalf("status = %d", recorder.status)
	}
	recorder.Flush() // 不应 panic
	if recorder.Unwrap() != underlying {
		t.Fatal("Unwrap 应返回底层 ResponseWriter")
	}
}
