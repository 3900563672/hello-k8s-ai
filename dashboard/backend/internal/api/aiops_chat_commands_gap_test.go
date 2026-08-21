package api

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/aiops"
	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/config"
	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/kubernetes"
	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/model"
	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/store"
	"github.com/jackc/pgx/v5"
)

// apiFakeLLM 实现 aiops.LLM：CompleteJSON 返回可解析的意图，StreamComplete 流式输出。
type apiFakeLLM struct{}

func (fake *apiFakeLLM) CompleteJSON(_ context.Context, _, _ string, _ int, _ float64) (aiops.Completion, error) {
	return aiops.Completion{Content: `{"sceneType":"潮汐流量","targetTenant":"preset-tenant-001","traffic":{"shape":"tidal","peakQps":50,"periodMinutes":30},"rate":20}`}, nil
}

func (fake *apiFakeLLM) StreamComplete(_ context.Context, _, _ string, _ int, _ float64, onDelta func(string), _ func(aiops.TokenUsage)) error {
	onDelta("这是 AIOps 回答")
	return nil
}

// commandStoreStub 实现意图命令读写；其余走 Disabled 兜底。
type commandStoreStub struct {
	store.Disabled
	mu       sync.Mutex
	commands map[string]model.AIOpsCommand
}

func (stub *commandStoreStub) Available() bool { return true }

func (stub *commandStoreStub) CreateAIOpsCommand(_ context.Context, command model.AIOpsCommand) error {
	stub.mu.Lock()
	defer stub.mu.Unlock()
	stub.commands[command.CommandID] = command
	return nil
}

func (stub *commandStoreStub) GetAIOpsCommand(_ context.Context, commandID string) (*model.AIOpsCommand, error) {
	stub.mu.Lock()
	defer stub.mu.Unlock()
	if command, ok := stub.commands[commandID]; ok {
		return &command, nil
	}
	return nil, pgx.ErrNoRows
}

func (stub *commandStoreStub) ListAIOpsJobs(_ context.Context, _ int, _ string) ([]model.AIOpsJob, error) {
	return []model.AIOpsJob{}, nil
}

func (stub *commandStoreStub) UpdateAIOpsCommand(_ context.Context, commandID, status string, steps json.RawMessage, errorText string) error {
	stub.mu.Lock()
	defer stub.mu.Unlock()
	command, ok := stub.commands[commandID]
	if !ok {
		return pgx.ErrNoRows
	}
	command.Status = status
	command.Steps = steps
	command.Error = errorText
	stub.commands[commandID] = command
	return nil
}

func (stub *commandStoreStub) ListAIOpsCommands(_ context.Context, _ int) ([]model.AIOpsCommand, error) {
	stub.mu.Lock()
	defer stub.mu.Unlock()
	commands := make([]model.AIOpsCommand, 0, len(stub.commands))
	for _, command := range stub.commands {
		commands = append(commands, command)
	}
	return commands, nil
}

func newCommandTestServer(stub *commandStoreStub) *Server {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	server := &Server{
		logger: logger,
		store:  stub,
		config: config.Config{HTTP: config.HTTPConfig{MaxBodyBytes: 1 << 20}},
	}
	server.aiops = aiops.NewService(config.AIOpsConfig{}, stub, &apiFakeLLM{}, logger)
	return server
}

func TestHandleAIOpsChatStreams(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stub := &chatStoreStub{}
	server := &Server{logger: logger, store: stub}
	server.aiops = aiops.NewService(config.AIOpsConfig{ChatMaxMessageLen: 4000, ChatRatePerMinute: 6}, stub, &apiFakeLLM{}, logger)

	body := strings.NewReader(`{"message":"你好","sessionId":"s-1"}`)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/aiops/chat", body)
	recorder := httptest.NewRecorder()
	server.handleAIOpsChat(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("chat = %d, want 200; body=%s", recorder.Code, recorder.Body.String())
	}
	output := recorder.Body.String()
	for _, fragment := range []string{`"type":"lifecycle"`, `"type":"text"`, "这是 AIOps 回答", `"phase":"end"`} {
		if !strings.Contains(output, fragment) {
			t.Fatalf("stream 缺少 %q: %s", fragment, output)
		}
	}

	badBody := httptest.NewRequest(http.MethodPost, "/api/v1/aiops/chat", strings.NewReader(`not-json`))
	recorder = httptest.NewRecorder()
	server.handleAIOpsChat(recorder, badBody)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("bad body = %d, want 400", recorder.Code)
	}

	empty := httptest.NewRequest(http.MethodPost, "/api/v1/aiops/chat", strings.NewReader(`{"message":"  ","sessionId":"s-1"}`))
	recorder = httptest.NewRecorder()
	server.handleAIOpsChat(recorder, empty)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("empty message = %d, want 400", recorder.Code)
	}
}

func TestHandleAIOpsChatRateLimited(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stub := &chatStoreStub{}
	cfg := config.AIOpsConfig{ChatRatePerMinute: 1, ChatMaxMessageLen: 4000}
	server := &Server{logger: logger, store: stub}
	server.aiops = aiops.NewService(cfg, stub, &apiFakeLLM{}, logger)

	request := httptest.NewRequest(http.MethodPost, "/api/v1/aiops/chat", strings.NewReader(`{"message":"hi","sessionId":"s-1"}`))
	recorder := httptest.NewRecorder()
	server.handleAIOpsChat(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("first chat = %d, want 200", recorder.Code)
	}
	request = httptest.NewRequest(http.MethodPost, "/api/v1/aiops/chat", strings.NewReader(`{"message":"again","sessionId":"s-1"}`))
	recorder = httptest.NewRecorder()
	server.handleAIOpsChat(recorder, request)
	if recorder.Code != http.StatusTooManyRequests {
		t.Fatalf("second chat = %d, want 429", recorder.Code)
	}
}

func TestHandleCreateAndGetAIOpsCommand(t *testing.T) {
	stub := &commandStoreStub{commands: map[string]model.AIOpsCommand{}}
	server := newCommandTestServer(stub)

	create := httptest.NewRequest(http.MethodPost, "/api/v1/aiops/commands", strings.NewReader(`{"rawInput":"2 小时潮汐流量"}`))
	recorder := httptest.NewRecorder()
	server.handleCreateAIOpsCommand(recorder, create)
	if recorder.Code != http.StatusCreated {
		t.Fatalf("create = %d, want 201; body=%s", recorder.Code, recorder.Body.String())
	}
	var created struct {
		Data model.AIOpsCommand `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if created.Data.CommandID == "" || created.Data.Status != string(model.AIOpsCommandParsed) {
		t.Fatalf("created command = %+v", created.Data)
	}
	if len(created.Data.Applied) == 0 {
		t.Fatal("create 响应应附带 applied 波形（attachAIOpsApplied）")
	}

	empty := httptest.NewRequest(http.MethodPost, "/api/v1/aiops/commands", strings.NewReader(`{"rawInput":"  "}`))
	recorder = httptest.NewRecorder()
	server.handleCreateAIOpsCommand(recorder, empty)
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("empty rawInput = %d, want 400", recorder.Code)
	}

	get := httptest.NewRequest(http.MethodGet, "/api/v1/aiops/commands/"+created.Data.CommandID, nil)
	get.SetPathValue("id", created.Data.CommandID)
	recorder = httptest.NewRecorder()
	server.handleGetAIOpsCommand(recorder, get)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), "潮汐") {
		t.Fatalf("get = %d %s", recorder.Code, recorder.Body.String())
	}

	missing := httptest.NewRequest(http.MethodGet, "/api/v1/aiops/commands/nope", nil)
	missing.SetPathValue("id", "nope")
	recorder = httptest.NewRecorder()
	server.handleGetAIOpsCommand(recorder, missing)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("missing = %d, want 404", recorder.Code)
	}
}

func TestHandleStopAIOpsCommandBranches(t *testing.T) {
	stub := &commandStoreStub{commands: map[string]model.AIOpsCommand{
		"cmd-1": {CommandID: "cmd-1", Status: string(model.AIOpsCommandParsed)},
	}}
	server := newCommandTestServer(stub)

	missing := httptest.NewRequest(http.MethodPost, "/api/v1/aiops/commands/nope/stop", nil)
	missing.SetPathValue("id", "nope")
	recorder := httptest.NewRecorder()
	server.handleStopAIOpsCommand(recorder, missing)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("missing stop = %d, want 404", recorder.Code)
	}

	notExecuting := httptest.NewRequest(http.MethodPost, "/api/v1/aiops/commands/cmd-1/stop", nil)
	notExecuting.SetPathValue("id", "cmd-1")
	recorder = httptest.NewRecorder()
	server.handleStopAIOpsCommand(recorder, notExecuting)
	if recorder.Code != http.StatusConflict {
		t.Fatalf("non-executing stop = %d, want 409", recorder.Code)
	}
}

func TestHandleConfirmAIOpsCommandCacheNotReady(t *testing.T) {
	stub := &commandStoreStub{commands: map[string]model.AIOpsCommand{}}
	server := newCommandTestServer(stub)
	server.cache = &kubernetes.Cache{} // 零值：Synced() 为 false
	request := httptest.NewRequest(http.MethodPost, "/api/v1/aiops/commands/x/confirm", nil)
	request.SetPathValue("id", "x")
	recorder := httptest.NewRecorder()
	server.handleConfirmAIOpsCommand(recorder, request)
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("confirm with unsynced cache = %d, want 503", recorder.Code)
	}
}

func TestHandleAIOpsLimitsAndTemplates(t *testing.T) {
	stub := &commandStoreStub{commands: map[string]model.AIOpsCommand{}}
	server := newCommandTestServer(stub)
	request := httptest.NewRequest(http.MethodGet, "/api/v1/aiops/limits", nil)
	recorder := httptest.NewRecorder()
	server.handleGetAIOpsLimits(recorder, request)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), "maxTrafficQPS") {
		t.Fatalf("limits = %d %s", recorder.Code, recorder.Body.String())
	}
	request = httptest.NewRequest(http.MethodGet, "/api/v1/aiops/templates", nil)
	recorder = httptest.NewRecorder()
	server.handleListAIOpsTemplates(recorder, request)
	if recorder.Code != http.StatusOK || !strings.Contains(recorder.Body.String(), "preset-model-001") {
		t.Fatalf("templates = %d %s", recorder.Code, recorder.Body.String())
	}
}
