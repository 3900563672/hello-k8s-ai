package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/aiops"
	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/config"
	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/model"
	"github.com/jackc/pgx/v5"
)

func (stub *commandStoreStub) SumAIOpsUsageSince(_ context.Context, _ time.Time) (int, int64, error) {
	return 3, 100, nil
}

func TestHandleGetAIOpsQuota(t *testing.T) {
	stub := &commandStoreStub{commands: map[string]model.AIOpsCommand{}}
	server := newCommandTestServer(stub)
	server.aiops = aiops.NewService(config.AIOpsConfig{DailyMaxCalls: 10, DailyMaxTokens: 1000}, stub, &apiFakeLLM{}, server.logger)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/aiops/quota", nil)
	recorder := httptest.NewRecorder()
	server.handleGetAIOpsQuota(recorder, request)
	if recorder.Code != 200 {
		t.Fatalf("quota = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}
}

func TestAIOpsExperimentNameAndHelpers(t *testing.T) {
	if got := aiopsExperimentName(&aiops.CommandIntent{SceneType: "潮汐流量"}); got != "潮汐流量" {
		t.Fatalf("aiopsExperimentName = %q", got)
	}
	long := string(make([]rune, 80))
	if got := aiopsExperimentName(&aiops.CommandIntent{SceneType: long}); len([]rune(got)) != 63 {
		t.Fatalf("aiopsExperimentName(long) 应截断到 63: %d", len([]rune(got)))
	}
	if got := aiopsExperimentName(&aiops.CommandIntent{}); got != "AI 意图实验" {
		t.Fatalf("aiopsExperimentName(empty) = %q", got)
	}
	raw := mustJSON([]string{"a"})
	if string(raw) != `["a"]` {
		t.Fatalf("mustJSON = %s", raw)
	}
	raw = mustJSON(make(chan int))
	if string(raw) != `[]` {
		t.Fatalf("mustJSON(error) = %s", raw)
	}
}

func TestFailAIOpsCommandPersistsError(t *testing.T) {
	stub := &commandStoreStub{commands: map[string]model.AIOpsCommand{
		"cmd-f": {CommandID: "cmd-f", Status: string(model.AIOpsCommandExecuting), Parsed: json.RawMessage(`{}`)},
	}}
	server := newCommandTestServer(stub)
	server.failAIOpsCommand(context.Background(), "cmd-f", "执行失败", pgx.ErrNoRows)
	stub.mu.Lock()
	command := stub.commands["cmd-f"]
	stub.mu.Unlock()
	if command.Status != string(model.AIOpsCommandFailed) || command.Error == "" {
		t.Fatalf("failAIOpsCommand 后 = %+v", command)
	}
}
