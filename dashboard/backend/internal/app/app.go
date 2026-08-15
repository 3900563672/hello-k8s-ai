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
		Jaeger: jaegerClient, Clock: clockState, Events: eventBus,
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
}

func openDatabase(ctx context.Context, cfg config.Config, logger *slog.Logger) (store.Store, error) {
	database, err := store.OpenPostgres(ctx, cfg.Database, logger)
	if err != nil {
		if cfg.Database.Required {
			return nil, err
		}
		logger.Warn("PostgreSQL is unavailable; historical Dashboard features are disabled", "error", err)
		return store.Disabled{}, nil
	}
	migrationContext, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := database.Migrate(migrationContext); err != nil {
		database.Close()
		if cfg.Database.Required {
			return nil, fmt.Errorf("migrate PostgreSQL: %w", err)
		}
		logger.Warn("PostgreSQL migrations failed; historical Dashboard features are disabled", "error", err)
		return store.Disabled{}, nil
	}
	return database, nil
}
