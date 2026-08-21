package api

import (
	"log/slog"
	"net/http"

	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/aiops"
	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/clock"
	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/config"
	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/kubernetes"
	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/model"
	jaegerprovider "github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/providers/jaeger"
	prometheusprovider "github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/providers/prometheus"
	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/readmodel"
	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/store"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Dependencies struct {
	Config     config.Config
	Logger     *slog.Logger
	Cache      *kubernetes.Cache
	Aggregator *readmodel.Aggregator
	Gateway    *kubernetes.Gateway
	Store      store.Store
	Prometheus *prometheusprovider.Client
	Jaeger     *jaegerprovider.Client
	Grafana    config.ProviderConfig
	Clock      *clock.Clock
	Events     *EventBus
	AIOps      *aiops.Service
}

type Server struct {
	config     config.Config
	logger     *slog.Logger
	cache      *kubernetes.Cache
	aggregator *readmodel.Aggregator
	gateway    *kubernetes.Gateway
	store      store.Store
	prometheus *prometheusprovider.Client
	jaeger     *jaegerprovider.Client
	grafana    config.ProviderConfig
	clock      *clock.Clock
	events     *EventBus
	aiops      *aiops.Service
}

func NewServer(dependencies Dependencies) *Server {
	return &Server{
		config:     dependencies.Config,
		logger:     dependencies.Logger,
		cache:      dependencies.Cache,
		aggregator: dependencies.Aggregator,
		gateway:    dependencies.Gateway,
		store:      dependencies.Store,
		prometheus: dependencies.Prometheus,
		jaeger:     dependencies.Jaeger,
		grafana:    dependencies.Grafana,
		clock:      dependencies.Clock,
		events:     dependencies.Events,
		aiops:      dependencies.AIOps,
	}
}

func (server *Server) currentClockState() model.ClockState {
	state := server.clock.State()
	// 所有写命令都依赖审计与幂等存储；存储不可用时前端必须禁用写入口。
	state.Capabilities.CanSetRate = state.Capabilities.CanSetRate && server.store.Available()
	return state
}

func (server *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/health/live", server.handleLive)
	mux.HandleFunc("GET /api/v1/health/ready", server.handleReady)
	// 自描述指标供 Prometheus 抓取，只读，不参与写认证。
	mux.Handle("GET /metrics", promhttp.Handler())
	mux.HandleFunc("GET /api/v1/capabilities", server.handleCapabilities)
	mux.HandleFunc("GET /api/v1/bootstrap", server.handleBootstrap)
	mux.HandleFunc("GET /api/v1/configuration", server.handleConfiguration)
	mux.HandleFunc("POST /api/v1/configuration:apply", server.handleApplyConfiguration)
	mux.HandleFunc("DELETE /api/v1/configuration/{kind}/{name}", server.handleDeleteConfiguration)
	mux.HandleFunc("GET /api/v1/traffic", server.handleTraffic)
	mux.HandleFunc("PATCH /api/v1/tenants/{name}/traffic", server.handleTenantTraffic)
	mux.HandleFunc("GET /api/v1/metrics", server.handleMetrics)
	mux.HandleFunc("GET /api/v1/metrics/query", server.handleMetrics)
	mux.HandleFunc("GET /api/v1/traces", server.handleTraces)
	mux.HandleFunc("GET /api/v1/traces/{traceID}", server.handleTraceDetail)
	mux.HandleFunc("GET /api/v1/events", server.handleEvents)
	mux.HandleFunc("GET /api/v1/resources", server.handleResourceStates)
	mux.HandleFunc("GET /api/v1/replay", server.handleReplay)
	mux.HandleFunc("GET /api/v1/replay/frame", server.handleOverview)
	mux.HandleFunc("GET /api/v1/overview", server.handleOverview)
	mux.HandleFunc("GET /api/v1/segment", server.handleSegment)
	mux.HandleFunc("POST /api/v1/experiments", server.handleCreateExperiment)
	mux.HandleFunc("POST /api/v1/experiments/{id}/start", server.handleStartExperiment)
	mux.HandleFunc("POST /api/v1/experiments/{id}/complete", server.handleCompleteExperiment)
	mux.HandleFunc("POST /api/v1/experiments/{id}/fail", server.handleFailExperiment)
	mux.HandleFunc("GET /api/v1/experiments", server.handleListExperiments)
	mux.HandleFunc("GET /api/v1/experiments/{id}", server.handleExperimentDetail)
	if server.aiops != nil {
		mux.HandleFunc("GET /api/v1/aiops/analyses", server.handleListAIOpsAnalyses)
		mux.HandleFunc("GET /api/v1/aiops/jobs", server.handleListAIOpsAnalyses)
		mux.HandleFunc("GET /api/v1/aiops/analyses/{id}", server.handleGetAIOpsAnalysis)
		mux.HandleFunc("GET /api/v1/aiops/templates", server.handleListAIOpsTemplates)
		mux.HandleFunc("POST /api/v1/aiops/commands", server.handleCreateAIOpsCommand)
		mux.HandleFunc("GET /api/v1/aiops/commands/{id}", server.handleGetAIOpsCommand)
		mux.HandleFunc("POST /api/v1/aiops/commands/{id}/confirm", server.handleConfirmAIOpsCommand)
		mux.HandleFunc("GET /api/v1/aiops/windows", server.handleListAIOpsWindows)
		mux.HandleFunc("GET /api/v1/aiops/alerts", server.handleListAIOpsAlerts)
		mux.HandleFunc("POST /api/v1/aiops/chat", server.handleAIOpsChat)
		mux.HandleFunc("GET /api/v1/aiops/chat/messages", server.handleListAIOpsChatMessages)
		mux.HandleFunc("GET /api/v1/aiops/settings", server.handleGetAIOpsSettings)
		mux.HandleFunc("GET /api/v1/aiops/jobs", server.handleListAIOpsJobs)
		mux.HandleFunc("POST /api/v1/aiops/settings", server.handleUpdateAIOpsSettings)
	}
	mux.HandleFunc("GET /api/v1/clock", server.handleClock)
	mux.HandleFunc("PATCH /api/v1/clock/rate", server.handleSimulationRate)
	mux.HandleFunc("GET /api/v1/stream", server.handleStream)
	if server.grafana.Enabled {
		// Grafana 面板以 sub-path 部署，经 Backend 反代后前端只需相对路径 /grafana/。
		mux.Handle("/grafana/", newGrafanaProxy(server.logger, server.grafana))
	}

	var handler http.Handler = mux
	handler = idempotencyMiddleware(server.store, server.config.HTTP.MaxBodyBytes, server.logger, handler)
	// 认证在最外层写链路上生效：未认证的写请求不会进入幂等存储。
	handler = authMiddleware(server.config.HTTP, server.config.Environment, server.logger, handler)
	handler = requestTimeoutMiddleware(server.config.HTTP.WriteTimeout, handler)
	handler = corsMiddleware(server.config.HTTP.AllowedOrigins, handler)
	handler = securityHeadersMiddleware(handler)
	handler = loggingMiddleware(server.logger, handler)
	handler = recoveryMiddleware(server.logger, handler)
	handler = requestIDMiddleware(handler)
	return handler
}
