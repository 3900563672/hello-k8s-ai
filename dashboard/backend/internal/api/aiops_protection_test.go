package api

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/model"
)

// aiopsListErrorStore 让列表类 AIOps handler 的 store 查询返回错误，覆盖 writeExperimentStoreError 分支。
type aiopsListErrorStore struct {
	aiopsStoreStub
	windowsErr error
	alertsErr  error
	jobsErr    error
}

func (stub *aiopsListErrorStore) ListAIOpsWindowSummaries(context.Context, string, int) ([]model.AIOpsWindowSummary, error) {
	return nil, stub.windowsErr
}

func (stub *aiopsListErrorStore) ListAIOpsAlerts(context.Context, int) ([]model.AIOpsAlert, error) {
	return nil, stub.alertsErr
}

func (stub *aiopsListErrorStore) ListAIOpsJobs(context.Context, int, string) ([]model.AIOpsJob, error) {
	return nil, stub.jobsErr
}

func TestHandleListAIOpsStoreErrorBranches(t *testing.T) {
	stub := &aiopsListErrorStore{
		windowsErr: io.ErrUnexpectedEOF,
		alertsErr:  io.ErrUnexpectedEOF,
		jobsErr:    io.ErrUnexpectedEOF,
	}
	logger := newAIOpsHandlerServer(&stub.aiopsStoreStub).logger
	server := &Server{logger: logger, store: stub}
	server.aiops = newAIOpsHandlerServer(&stub.aiopsStoreStub).aiops

	cases := []struct {
		name    string
		target  string
		handler func(http.ResponseWriter, *http.Request)
	}{
		{"windows", "/api/v1/aiops/windows", server.handleListAIOpsWindows},
		{"alerts", "/api/v1/aiops/alerts", server.handleListAIOpsAlerts},
		{"jobs", "/api/v1/aiops/jobs?status=running", server.handleListAIOpsJobs},
	}
	for _, test := range cases {
		request := httptest.NewRequest(http.MethodGet, test.target, nil)
		recorder := httptest.NewRecorder()
		test.handler(recorder, request)
		if recorder.Code != http.StatusServiceUnavailable {
			t.Fatalf("%s store error = %d, want 503; body=%s", test.name, recorder.Code, recorder.Body.String())
		}
	}

	badLimit := httptest.NewRequest(http.MethodGet, "/api/v1/aiops/windows?level=L3&limit=0", nil)
	recorder := httptest.NewRecorder()
	server.handleListAIOpsWindows(recorder, badLimit)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("windows bad limit = %d, want 400", recorder.Code)
	}
	badAlertLimit := httptest.NewRequest(http.MethodGet, "/api/v1/aiops/alerts?limit=999", nil)
	recorder = httptest.NewRecorder()
	server.handleListAIOpsAlerts(recorder, badAlertLimit)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("alerts bad limit = %d, want 400", recorder.Code)
	}
}

func TestNonNilAIOpsHelpersReturnEmptyForNil(t *testing.T) {
	if got := nonNilAIOpsAnalyses(nil); got == nil || len(got) != 0 {
		t.Fatalf("nonNilAIOpsAnalyses(nil) = %#v, want empty non-nil", got)
	}
	if got := nonNilAIOpsEntitySummaries(nil); got == nil || len(got) != 0 {
		t.Fatalf("nonNilAIOpsEntitySummaries(nil) = %#v, want empty non-nil", got)
	}
	if got := nonNilAIOpsWindowSummaries(nil); got == nil || len(got) != 0 {
		t.Fatalf("nonNilAIOpsWindowSummaries(nil) = %#v, want empty non-nil", got)
	}
	if got := nonNilAIOpsJobs(nil); got == nil || len(got) != 0 {
		t.Fatalf("nonNilAIOpsJobs(nil) = %#v, want empty non-nil", got)
	}
	if got := nonNilAIOpsAlerts(nil); got == nil || len(got) != 0 {
		t.Fatalf("nonNilAIOpsAlerts(nil) = %#v, want empty non-nil", got)
	}
}
