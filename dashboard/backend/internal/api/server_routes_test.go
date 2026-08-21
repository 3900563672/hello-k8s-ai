package api

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
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
