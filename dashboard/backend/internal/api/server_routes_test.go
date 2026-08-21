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

// routesStoreStub 提供路由注册所需的最小存储能力。
type routesStoreStub struct {
	store.Disabled
}

func (stub *routesStoreStub) Available() bool { return true }

func (stub *routesStoreStub) ListAIOpsJobs(_ context.Context, _ int, _ string) ([]model.AIOpsJob, error) {
	return []model.AIOpsJob{}, nil
}

// TestAIOpsSettingsReachableWhileDisabled 回归：开关关闭后 settings 读写必须仍可用，
// 否则面板无法重新打开（enabled 是恢复逃生通道，不走 requireAIOps 门禁）。
// 直接调用 handler 而非完整 HTTP 链：聚焦 settings 自身的门禁行为，不受幂等中间件影响。
func TestAIOpsSettingsReachableWhileDisabled(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stub := &routesStoreStub{}
	server := &Server{logger: logger, store: stub}
	server.aiops = aiops.NewService(config.AIOpsConfig{}, stub, nil, logger)
	server.aiops.SetEnabled(false)

	getRequest := httptest.NewRequest(http.MethodGet, "/api/v1/aiops/settings", nil)
	getRecorder := httptest.NewRecorder()
	server.handleGetAIOpsSettings(getRecorder, getRequest)
	if getRecorder.Code != http.StatusOK {
		t.Fatalf("GET settings with switch off = %d, want 200; body=%s",
			getRecorder.Code, getRecorder.Body.String())
	}

	postRequest := httptest.NewRequest(http.MethodPost, "/api/v1/aiops/settings", nil)
	postRecorder := httptest.NewRecorder()
	server.handleUpdateAIOpsSettings(postRecorder, postRequest)
	if postRecorder.Code != http.StatusBadRequest {
		t.Fatalf("POST settings（空 body）with switch off = %d, want 400（至少一个字段）; body=%s",
			postRecorder.Code, postRecorder.Body.String())
	}

	enableBody := strings.NewReader(`{"enabled":true}`)
	enableRequest := httptest.NewRequest(http.MethodPost, "/api/v1/aiops/settings", enableBody)
	enableRecorder := httptest.NewRecorder()
	server.handleUpdateAIOpsSettings(enableRecorder, enableRequest)
	if enableRecorder.Code != http.StatusOK {
		t.Fatalf("POST settings（enabled=true）with switch off = %d, want 200; body=%s",
			enableRecorder.Code, enableRecorder.Body.String())
	}
	if !server.aiops.Enabled() {
		t.Fatal("POST /aiops/settings 携带 enabled=true 后开关应恢复为 true")
	}
}

// TestAIOpsJobsDisabledReturns404 回归：面板运行时开关关闭后，
// jobs 等 AIOps 路由应返回 404（前端据此提示「AIOps 未启用」），
// 且不影响服务自身注册（不再 panic）。
func TestAIOpsJobsDisabledReturns404(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stub := &routesStoreStub{}
	server := &Server{logger: logger, store: stub}
	server.aiops = aiops.NewService(config.AIOpsConfig{}, stub, nil, logger)
	server.aiops.SetEnabled(false)

	request := httptest.NewRequest(http.MethodGet, "/api/v1/aiops/jobs", nil)
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("GET /api/v1/aiops/jobs with switch off = %d, want 404; body=%s",
			recorder.Code, recorder.Body.String())
	}
}

// TestHandlerRegistersAIOpsRoutesOnce 回归：#113 合并时曾残留
// GET /api/v1/aiops/jobs 的重复注册（一个指向 analyses、一个指向 jobs），
// Go 1.22+ ServeMux 对相同 pattern 直接 panic；AIOps 启用路径此前未被
// 任何测试覆盖，部署开启后才暴露。此测试确保完整注册不再 panic 且
// jobs 路由按新语义返回 200。
func TestHandlerRegistersAIOpsRoutesOnce(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stub := &routesStoreStub{}
	server := &Server{logger: logger, store: stub}
	server.aiops = aiops.NewService(config.AIOpsConfig{}, stub, nil, logger)

	request := httptest.NewRequest(http.MethodGet, "/api/v1/aiops/jobs", nil)
	recorder := httptest.NewRecorder()
	server.Handler().ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/aiops/jobs = %d, want 200; body=%s",
			recorder.Code, recorder.Body.String())
	}
}
