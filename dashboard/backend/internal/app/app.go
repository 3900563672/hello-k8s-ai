package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/api"
	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/clock"
	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/config"
	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/kubernetes"
	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/model"
	jaegerprovider "github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/providers/jaeger"
	prometheusprovider "github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/providers/prometheus"
	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/readmodel"
	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/store"
)

type App struct {
	config     config.Config
	logger     *slog.Logger
	database   store.Store
	recorder   *store.Recorder
	cache      *kubernetes.Cache
	aggregator *readmodel.Aggregator
	clock      *clock.Clock
	httpServer *http.Server
}

func New(ctx context.Context, cfg config.Config, logger *slog.Logger) (*App, error) {
	database, err := openDatabase(ctx, cfg, logger)
	if err != nil {
		return nil, err
	}
	recorder := store.NewRecorder(database, cfg.Persistence.EventBuffer, logger)
	eventBus := api.NewEventBus()

	clients, err := kubernetes.NewClients(cfg.Kubernetes)
	if err != nil {
		database.Close()
		return nil, err
	}
	cacheState, err := kubernetes.NewCache(clients, cfg.Kubernetes, logger, func(change model.ResourceChange) {
		eventBus.Publish(change)
		if database.Available() {
			recorder.Publish(change)
		}
	})
	if err != nil {
		database.Close()
		return nil, fmt.Errorf("create Kubernetes informer cache: %w", err)
	}

	prometheusClient, err := prometheusprovider.New(cfg.Prometheus)
	if err != nil {
		database.Close()
		return nil, err
	}
	jaegerClient, err := jaegerprovider.New(cfg.Jaeger)
	if err != nil {
		database.Close()
		return nil, err
	}
	clockState := clock.New(cacheState)
	aggregator := readmodel.NewAggregator(cacheState)
	gateway := kubernetes.NewGateway(cacheState)
	apiServer := api.NewServer(api.Dependencies{
		Config: cfg, Logger: logger, Cache: cacheState, Aggregator: aggregator,
		Gateway: gateway, Store: database, Prometheus: prometheusClient,
		Jaeger: jaegerClient, Grafana: cfg.Grafana, Clock: clockState, Events: eventBus,
	})
	httpServer := &http.Server{
		Addr:              cfg.HTTP.Address,
		Handler:           apiServer.Handler(),
		ReadTimeout:       cfg.HTTP.ReadTimeout,
		ReadHeaderTimeout: cfg.HTTP.ReadHeaderTimeout,
		WriteTimeout:      0, // SSE responses are long-lived; provider calls have their own deadlines.
		IdleTimeout:       cfg.HTTP.IdleTimeout,
		MaxHeaderBytes:    1 << 20,
	}
	return &App{
		config: cfg, logger: logger, database: database, recorder: recorder,
		cache: cacheState, aggregator: aggregator, clock: clockState, httpServer: httpServer,
	}, nil
}

func (application *App) Run(ctx context.Context) error {
	errorsChannel := make(chan error, 4)
	go func() {
		if err := application.cache.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
			errorsChannel <- fmt.Errorf("run Kubernetes cache: %w", err)
		}
	}()
	if application.database.Available() {
		go func() {
			if err := application.recorder.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
				errorsChannel <- fmt.Errorf("run persistence recorder: %w", err)
			}
		}()
		go application.runSnapshots(ctx)
	}
	go func() {
		application.logger.Info("Dashboard Backend HTTP server started", "address", application.httpServer.Addr)
		if err := application.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errorsChannel <- fmt.Errorf("serve Dashboard API: %w", err)
		}
	}()

	var runErr error
	select {
	case <-ctx.Done():
		runErr = nil
	case runErr = <-errorsChannel:
	}

	shutdownContext, cancel := context.WithTimeout(context.Background(), application.config.HTTP.ShutdownTimeout)
	defer cancel()
	if err := application.httpServer.Shutdown(shutdownContext); err != nil {
		application.logger.Error("Dashboard Backend HTTP shutdown failed", "error", err)
		if runErr == nil {
			runErr = err
		}
	}
	application.database.Close()
	return runErr
}

func (application *App) runSnapshots(ctx context.Context) {
	waitContext, cancel := context.WithTimeout(ctx, application.config.Kubernetes.CacheSyncTimeout+time.Second)
	defer cancel()
	if err := application.cache.WaitUntilSynced(waitContext); err != nil {
		application.logger.Error("Could not start Dashboard snapshot loop", "error", err)
		return
	}
	application.persistSnapshot(ctx)
	snapshotTicker := time.NewTicker(application.config.Persistence.SnapshotInterval)
	defer snapshotTicker.Stop()
	pruneTicker := time.NewTicker(24 * time.Hour)
	defer pruneTicker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-snapshotTicker.C:
			application.persistSnapshot(ctx)
		case <-pruneTicker.C:
			pruneContext, cancel := context.WithTimeout(ctx, 30*time.Second)
			err := application.database.Prune(pruneContext, time.Now().UTC().Add(-application.config.Persistence.SnapshotRetention))
			cancel()
			if err != nil {
				application.logger.Error("Could not prune Dashboard history", "error", err)
			}
		}
	}
}

func (application *App) persistSnapshot(ctx context.Context) {
	now := application.clock.State().LogicalTime
	snapshot := application.aggregator.CurrentSnapshot(now)
	if !snapshotHasBusinessData(snapshot) {
		application.logger.Debug("跳过空配置快照：没有用户业务资源", "capturedAt", now)
		return
	}
	payload, err := json.Marshal(snapshot)
	if err != nil {
		application.logger.Error("Could not serialize Dashboard resource snapshot", "error", err)
		return
	}
	writeContext, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	record := store.SnapshotRecord{
		ID:          fmt.Sprintf("snapshot-%d", now.UnixNano()),
		CapturedAt:  now,
		LogicalTime: now,
		SourceVersions: map[string]string{
			"kubernetesCacheSyncedAt": application.cache.SyncedAt().Format(time.RFC3339Nano),
		},
		Payload: payload,
	}
	if err := application.database.SaveSnapshot(writeContext, record); err != nil {
		application.logger.Error("Could not persist Dashboard resource snapshot", "error", err)
	}
	if err := application.database.UpsertResourceStates(writeContext, resourceStateRecords(snapshot, now)); err != nil {
		application.logger.Error("Could not persist current resource states", "error", err)
	}
}

// snapshotHasBusinessData 判断快照中是否存在用户业务资源。
// SimulationClock/default 与集群基础工作负载（系统 Pod、物理节点）不构成业务历史，
// 没有业务资源时不写快照，保证"没有运行就没有历史切面"。
func snapshotHasBusinessData(snapshot model.CurrentSnapshot) bool {
	configuration := snapshot.Configuration
	return len(configuration.Models) > 0 ||
		len(configuration.WorkerNodes) > 0 ||
		len(configuration.Tenants) > 0 ||
		len(configuration.Policies.TenantModel) > 0 ||
		len(configuration.Policies.TenantNode) > 0 ||
		len(configuration.Policies.ModelNode) > 0 ||
		len(configuration.Orchestrators) > 0 ||
		len(configuration.SimulatorInstances) > 0 ||
		len(configuration.TenantPerformance) > 0 ||
		len(configuration.TenantRuntimes) > 0
}

// resourceStateRecords 把聚合快照中的每个资源拆成一行"最新状态"，供数据库统一查询与恢复。
func resourceStateRecords(snapshot model.CurrentSnapshot, now time.Time) []store.ResourceStateRecord {
	records := make([]store.ResourceStateRecord, 0, 64)
	appendPlatform := func(kind string, resources []model.PlatformResource) {
		for _, resource := range resources {
			payload, err := json.Marshal(resource)
			if err != nil {
				continue
			}
			records = append(records, store.ResourceStateRecord{
				Kind:            kind,
				Namespace:       resource.Ref.Namespace,
				Name:            resource.Ref.Name,
				UID:             resource.Ref.UID,
				ResourceVersion: resource.Metadata.ResourceVersion,
				Generation:      resource.Metadata.Generation,
				CapturedAt:      now,
				Payload:         payload,
			})
		}
	}
	appendPlatform("Model", snapshot.Configuration.Models)
	appendPlatform("WorkerNode", snapshot.Configuration.WorkerNodes)
	appendPlatform("Tenant", snapshot.Configuration.Tenants)
	appendPlatform("TenantModelPolicy", snapshot.Configuration.Policies.TenantModel)
	appendPlatform("TenantNodePolicy", snapshot.Configuration.Policies.TenantNode)
	appendPlatform("ModelNodePolicy", snapshot.Configuration.Policies.ModelNode)
	appendPlatform("Orchestrator", snapshot.Configuration.Orchestrators)
	appendPlatform("SimulationClock", snapshot.Configuration.SimulationClocks)
	appendPlatform("SimulatorInstance", snapshot.Configuration.SimulatorInstances)
	appendPlatform("TenantPerformance", snapshot.Configuration.TenantPerformance)
	appendPlatform("TenantRuntime", snapshot.Configuration.TenantRuntimes)
	for _, tenant := range snapshot.Traffic.Tenants {
		payload, err := json.Marshal(tenant)
		if err != nil {
			continue
		}
		records = append(records, store.ResourceStateRecord{
			Kind:       "TenantTraffic",
			Namespace:  tenant.Tenant.Namespace,
			Name:       tenant.Tenant.Name,
			UID:        tenant.Tenant.UID,
			CapturedAt: now,
			Payload:    payload,
		})
	}
	for _, node := range snapshot.Workloads.Nodes {
		payload, err := json.Marshal(node)
		if err != nil {
			continue
		}
		records = append(records, store.ResourceStateRecord{
			Kind:       "Node",
			Namespace:  node.Ref.Namespace,
			Name:       node.Ref.Name,
			UID:        node.Ref.UID,
			CapturedAt: now,
			Payload:    payload,
		})
	}
	for _, deployment := range snapshot.Workloads.Deployments {
		payload, err := json.Marshal(deployment)
		if err != nil {
			continue
		}
		records = append(records, store.ResourceStateRecord{
			Kind:       "Deployment",
			Namespace:  deployment.Ref.Namespace,
			Name:       deployment.Ref.Name,
			UID:        deployment.Ref.UID,
			CapturedAt: now,
			Payload:    payload,
		})
	}
	for _, pod := range snapshot.Workloads.Pods {
		payload, err := json.Marshal(pod)
		if err != nil {
			continue
		}
		records = append(records, store.ResourceStateRecord{
			Kind:       "Pod",
			Namespace:  pod.Ref.Namespace,
			Name:       pod.Ref.Name,
			UID:        pod.Ref.UID,
			CapturedAt: now,
			Payload:    payload,
		})
	}
	return records
}

func openDatabase(ctx context.Context, cfg config.Config, logger *slog.Logger) (store.Store, error) {
	var lastErr error
	retries := cfg.Database.StartupRetries
	if retries < 0 {
		retries = 0
	}
	for attempt := 0; attempt <= retries; attempt++ {
		if attempt > 0 {
			logger.Warn("PostgreSQL not ready; retrying startup",
				"attempt", attempt, "max", retries, "backoff", cfg.Database.StartupBackoff, "error", lastErr)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(cfg.Database.StartupBackoff):
			}
		}
		database, err := store.OpenPostgres(ctx, cfg.Database, logger)
		if err != nil {
			lastErr = err
			continue
		}
		migrationContext, cancel := context.WithTimeout(ctx, 30*time.Second)
		err = database.Migrate(migrationContext)
		cancel()
		if err != nil {
			database.Close()
			lastErr = fmt.Errorf("migrate PostgreSQL: %w", err)
			continue
		}
		status, statusErr := database.Status(ctx)
		if statusErr != nil {
			logger.Warn("Could not read PostgreSQL store status", "error", statusErr)
		} else {
			logger.Info("PostgreSQL ready; history is persistent",
				"migrationsApplied", status.MigrationsApplied,
				"resourceEvents", status.ResourceEvents,
				"resourceSnapshots", status.ResourceSnapshots,
				"resourceStates", status.ResourceStates,
				"historyRecovered", status.ResourceEvents > 0 || status.ResourceSnapshots > 0 || status.ResourceStates > 0,
			)
		}
		return database, nil
	}
	if cfg.Database.Required {
		return nil, fmt.Errorf("PostgreSQL unavailable after %d attempts: %w", retries+1, lastErr)
	}
	logger.Warn("PostgreSQL is unavailable; historical Dashboard features are disabled", "error", lastErr)
	return store.Disabled{}, nil
}
