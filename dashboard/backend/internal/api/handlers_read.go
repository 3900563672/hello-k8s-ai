package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/model"
	jaegerprovider "github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/providers/jaeger"
	prometheusprovider "github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/providers/prometheus"
	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/store"
)

func (server *Server) handleLive(writer http.ResponseWriter, request *http.Request) {
	writeData(writer, request, http.StatusOK, map[string]string{"status": "live"}, false, nil, nil)
}

func (server *Server) handleReady(writer http.ResponseWriter, request *http.Request) {
	checks := map[string]any{
		"kubernetesCache": map[string]any{
			"ready":    server.cache.Synced(),
			"syncedAt": server.cache.SyncedAt(),
		},
	}
	ready := server.cache.Synced()
	checkContext, cancel := context.WithTimeout(request.Context(), 1500*time.Millisecond)
	defer cancel()
	providers, _ := server.providerStates(checkContext)
	checks["providers"] = providers
	databaseDetails := map[string]any{"available": server.store.Available()}
	if server.store.Available() {
		statusContext, statusCancel := context.WithTimeout(request.Context(), 1500*time.Millisecond)
		status, statusErr := server.store.Status(statusContext)
		statusCancel()
		if statusErr != nil {
			databaseDetails["error"] = statusErr.Error()
		} else {
			databaseDetails["migrationsApplied"] = status.MigrationsApplied
			databaseDetails["resourceEvents"] = status.ResourceEvents
			databaseDetails["resourceSnapshots"] = status.ResourceSnapshots
			databaseDetails["resourceStates"] = status.ResourceStates
		}
	}
	checks["database"] = databaseDetails
	if server.config.Database.Required && providers["postgresql"].State != "ready" {
		ready = false
	}
	status := http.StatusOK
	if !ready {
		status = http.StatusServiceUnavailable
	}
	writeData(writer, request, status, map[string]any{"status": map[bool]string{true: "ready", false: "not-ready"}[ready], "checks": checks}, false, nil, sourceVersions(server.cache.SyncedAt()))
}

func (server *Server) handleCapabilities(writer http.ResponseWriter, request *http.Request) {
	commandsAvailable := server.store.Available()
	data := map[string]any{
		"sources": map[string]any{
			"kubernetes": map[string]any{"enabled": true, "currentState": true, "history": server.store.Available()},
			"prometheus": map[string]any{"enabled": server.prometheus.Enabled(), "metrics": server.prometheus.Catalog()},
			"jaeger":     map[string]any{"enabled": server.jaeger.Enabled()},
			"postgresql": map[string]any{"enabled": server.store.Available(), "role": "persistent-current-and-history"},
		},
		"commands": map[string]bool{
			"configurationApply":  commandsAvailable,
			"configurationDelete": commandsAvailable,
			"tenantQPS":           commandsAvailable,
			"idempotency":         commandsAvailable,
			"simulationRun":       false,
			"simulationRate":      commandsAvailable,
		},
		"clock": server.currentClockState().Capabilities,
		"replay": map[string]any{
			"kubernetesSnapshots":   server.store.Available(),
			"simulatorAcceleration": false,
		},
	}
	writeData(writer, request, http.StatusOK, data, false, nil, sourceVersions(server.cache.SyncedAt()))
}

func (server *Server) handleBootstrap(writer http.ResponseWriter, request *http.Request) {
	if !server.requireCache(writer, request) {
		return
	}
	now := server.currentClockState().ServerTime
	configuration := server.aggregator.Configuration(now)
	traffic := server.aggregator.Traffic(now)
	workloads := server.aggregator.Workloads(now)
	timeline, timelineErr := server.timeline(request.Context(), 300)
	providers, warnings := server.providerStates(request.Context())
	if timelineErr != nil {
		warnings = append(warnings, "Persistent timeline is unavailable: "+timelineErr.Error())
		timeline = []model.TimelineItem{}
	}
	readyNodes := 0
	for _, node := range workloads.Nodes {
		if node.Ready {
			readyNodes++
		}
	}
	data := model.Bootstrap{
		Cluster: model.ClusterInfo{
			Name:          server.config.ClusterName,
			Context:       server.cache.ContextName(),
			Version:       server.cache.ServerVersion(),
			Connected:     true,
			CacheSynced:   server.cache.Synced(),
			CacheSyncedAt: server.cache.SyncedAt(),
			NodeCount:     len(workloads.Nodes),
			ReadyNodes:    readyNodes,
		},
		Clock:     server.currentClockState(),
		Counts:    server.aggregator.Counts(configuration, traffic, workloads),
		Nodes:     workloads.Nodes,
		Providers: providers,
		Timeline:  timeline,
	}
	writeData(writer, request, http.StatusOK, data, len(warnings) > 0, warnings, sourceVersions(server.cache.SyncedAt()))
}

func (server *Server) handleConfiguration(writer http.ResponseWriter, request *http.Request) {
	snapshot, availability, _, err := server.snapshotFor(request)
	if err != nil {
		server.writeSnapshotError(writer, request, err)
		return
	}
	if availability != "available" {
		configuration := model.Configuration{
			AsOf: requestTimeOrNow(request), Availability: availability,
			Models: []model.PlatformResource{}, WorkerNodes: []model.PlatformResource{}, Tenants: []model.PlatformResource{},
			Policies:      model.PolicySet{TenantModel: []model.PlatformResource{}, TenantNode: []model.PlatformResource{}, ModelNode: []model.PlatformResource{}},
			Orchestrators: []model.PlatformResource{}, SimulationClocks: []model.PlatformResource{}, SimulatorInstances: []model.PlatformResource{}, TenantPerformance: []model.PlatformResource{}, TenantRuntimes: []model.PlatformResource{},
		}
		writeData(writer, request, http.StatusOK, configuration, true, []string{"No persisted Kubernetes snapshot exists at the requested time."}, sourceVersions(server.cache.SyncedAt()))
		return
	}
	writeData(writer, request, http.StatusOK, snapshot.Configuration, false, nil, sourceVersions(server.cache.SyncedAt()))
}

func (server *Server) handleTraffic(writer http.ResponseWriter, request *http.Request) {
	snapshot, availability, _, err := server.snapshotFor(request)
	if err != nil {
		server.writeSnapshotError(writer, request, err)
		return
	}
	if availability != "available" {
		writeData(writer, request, http.StatusOK, model.Traffic{AsOf: requestTimeOrNow(request), Tenants: []model.TenantTraffic{}}, true, []string{"No persisted Kubernetes snapshot exists at the requested time."}, sourceVersions(server.cache.SyncedAt()))
		return
	}
	traffic := snapshot.Traffic
	if tenant := strings.TrimSpace(request.URL.Query().Get("tenant")); tenant != "" {
		filtered := traffic.Tenants[:0]
		for _, item := range traffic.Tenants {
			if item.Tenant.Name == tenant {
				filtered = append(filtered, item)
			}
		}
		traffic.Tenants = filtered
	}
	writeData(writer, request, http.StatusOK, traffic, false, nil, sourceVersions(server.cache.SyncedAt()))
}

func (server *Server) handleMetrics(writer http.ResponseWriter, request *http.Request) {
	metricID := strings.TrimSpace(request.URL.Query().Get("metricId"))
	if metricID == "" {
		writeProblem(writer, request, http.StatusBadRequest, "MISSING_METRIC_ID", "metricId is required.", false, nil)
		return
	}
	end, err := queryTime(request, "end", time.Now().UTC())
	if err != nil {
		writeProblem(writer, request, http.StatusBadRequest, "INVALID_TIME", err.Error(), false, nil)
		return
	}
	start, err := queryTime(request, "start", end.Add(-15*time.Minute))
	if err != nil {
		writeProblem(writer, request, http.StatusBadRequest, "INVALID_TIME", err.Error(), false, nil)
		return
	}
	step, err := queryDuration(request, "step", 0)
	if err != nil {
		writeProblem(writer, request, http.StatusBadRequest, "INVALID_STEP", err.Error(), false, nil)
		return
	}
	result, err := server.prometheus.QueryRange(request.Context(), prometheusprovider.Query{
		MetricID: metricID,
		Start:    start,
		End:      end,
		Step:     step,
		Tenant:   request.URL.Query().Get("tenant"),
		Model:    request.URL.Query().Get("model"),
		Instance: request.URL.Query().Get("instance"),
		Node:     request.URL.Query().Get("node"),
	})
	if err != nil {
		status, code, retryable := http.StatusBadGateway, "PROMETHEUS_QUERY_FAILED", true
		if errors.Is(err, prometheusprovider.ErrInvalidQuery) {
			status, code, retryable = http.StatusBadRequest, "INVALID_METRIC_QUERY", false
		} else if errors.Is(err, prometheusprovider.ErrDisabled) {
			status, code, retryable = http.StatusServiceUnavailable, "PROMETHEUS_DISABLED", false
		}
		writeProblem(writer, request, status, code, err.Error(), retryable, nil)
		return
	}
	writeData(writer, request, http.StatusOK, result, false, result.Warnings, map[string]string{"prometheus": result.QueriedAt.Format(time.RFC3339Nano)})
}

func (server *Server) handleTraces(writer http.ResponseWriter, request *http.Request) {
	end, err := queryTime(request, "end", time.Now().UTC())
	if err != nil {
		writeProblem(writer, request, http.StatusBadRequest, "INVALID_TIME", err.Error(), false, nil)
		return
	}
	start, err := queryTime(request, "start", end.Add(-15*time.Minute))
	if err != nil {
		writeProblem(writer, request, http.StatusBadRequest, "INVALID_TIME", err.Error(), false, nil)
		return
	}
	limit := queryInteger(request, "limit", 20)
	minDuration, err := queryDuration(request, "minDuration", 0)
	if err != nil {
		writeProblem(writer, request, http.StatusBadRequest, "INVALID_DURATION", err.Error(), false, nil)
		return
	}
	maxDuration, err := queryDuration(request, "maxDuration", 0)
	if err != nil {
		writeProblem(writer, request, http.StatusBadRequest, "INVALID_DURATION", err.Error(), false, nil)
		return
	}
	traces, err := server.jaeger.Search(request.Context(), jaegerprovider.SearchRequest{
		Start: start, End: end,
		Service: request.URL.Query().Get("service"), Operation: request.URL.Query().Get("operation"),
		Tenant: request.URL.Query().Get("tenant"), Model: request.URL.Query().Get("model"), Instance: request.URL.Query().Get("instance"),
		MinDuration: minDuration, MaxDuration: maxDuration, Limit: limit,
	})
	if err != nil {
		server.writeJaegerError(writer, request, err)
		return
	}
	server.indexTraces(request.Context(), traces)
	writeData(writer, request, http.StatusOK, map[string]any{"items": traces, "page": map[string]any{"next": nil}}, false, nil, map[string]string{"jaeger": time.Now().UTC().Format(time.RFC3339Nano)})
}

func (server *Server) handleTraceDetail(writer http.ResponseWriter, request *http.Request) {
	detail, err := server.jaeger.Trace(request.Context(), request.PathValue("traceID"))
	if err != nil {
		server.writeJaegerError(writer, request, err)
		return
	}
	detail.EntityLinks = server.entityLinks(detail)
	writeData(writer, request, http.StatusOK, detail, false, nil, map[string]string{"jaeger": time.Now().UTC().Format(time.RFC3339Nano)})
}

func (server *Server) writeJaegerError(writer http.ResponseWriter, request *http.Request, err error) {
	status, code, retryable := http.StatusBadGateway, "JAEGER_QUERY_FAILED", true
	switch {
	case errors.Is(err, jaegerprovider.ErrInvalidQuery):
		status, code, retryable = http.StatusBadRequest, "INVALID_TRACE_QUERY", false
	case errors.Is(err, jaegerprovider.ErrTraceNotFound):
		status, code, retryable = http.StatusNotFound, "TRACE_NOT_FOUND", false
	case errors.Is(err, jaegerprovider.ErrDisabled):
		status, code, retryable = http.StatusServiceUnavailable, "JAEGER_DISABLED", false
	}
	writeProblem(writer, request, status, code, err.Error(), retryable, nil)
}

func (server *Server) handleEvents(writer http.ResponseWriter, request *http.Request) {
	if !server.requireCache(writer, request) {
		return
	}
	events := server.aggregator.Workloads(time.Now().UTC()).Events
	limit := queryInteger(request, "limit", 200)
	if limit < 1 || limit > 500 {
		writeProblem(writer, request, http.StatusBadRequest, "INVALID_LIMIT", "limit must be between 1 and 500.", false, nil)
		return
	}
	if len(events) > limit {
		events = events[:limit]
	}
	writeData(writer, request, http.StatusOK, map[string]any{"items": events, "page": map[string]any{"next": nil}}, false, nil, sourceVersions(server.cache.SyncedAt()))
}

func (server *Server) handleReplay(writer http.ResponseWriter, request *http.Request) {
	limit := queryInteger(request, "limit", 500)
	if limit < 1 || limit > 1000 {
		writeProblem(writer, request, http.StatusBadRequest, "INVALID_LIMIT", "limit must be between 1 and 1000.", false, nil)
		return
	}
	timeline, err := server.timeline(request.Context(), limit)
	if err != nil {
		writeProblem(writer, request, http.StatusServiceUnavailable, "TIMELINE_UNAVAILABLE", err.Error(), true, nil)
		return
	}
	writeData(writer, request, http.StatusOK, map[string]any{"clock": server.currentClockState(), "timeline": timeline}, false, nil, sourceVersions(server.cache.SyncedAt()))
}

func (server *Server) handleClock(writer http.ResponseWriter, request *http.Request) {
	writeData(writer, request, http.StatusOK, server.currentClockState(), false, nil, sourceVersions(server.cache.SyncedAt()))
}

func (server *Server) handleOverview(writer http.ResponseWriter, request *http.Request) {
	snapshot, availability, snapshotID, err := server.snapshotFor(request)
	if err != nil {
		server.writeSnapshotError(writer, request, err)
		return
	}
	asOf := snapshot.CapturedAt
	if asOf.IsZero() {
		asOf = requestTimeOrNow(request)
	}
	if availability != "available" {
		snapshot = emptySnapshot(asOf, availability)
	}
	overview := model.Overview{
		Availability:  availability,
		AsOf:          asOf,
		SnapshotID:    snapshotID,
		Clock:         server.currentClockState(),
		Configuration: snapshot.Configuration,
		Traffic:       snapshot.Traffic,
		Workloads:     snapshot.Workloads,
		Metrics:       map[string]model.MetricResult{},
		Traces:        []model.TraceSummary{},
		Freshness:     map[string]model.ProviderState{},
	}
	if availability != "available" {
		writeData(writer, request, http.StatusOK, overview, true, []string{"No persisted Kubernetes snapshot exists at the requested time."}, sourceVersions(server.cache.SyncedAt()))
		return
	}

	warnings := make([]string, 0)
	var mutex sync.Mutex
	var group sync.WaitGroup
	metricIDs := []string{"simulator.ttft", "simulator.queue", "simulator.qps", "simulator.errorRate", "simulator.tickLatency"}
	for _, metricID := range metricIDs {
		metricID := metricID
		group.Add(1)
		go func() {
			defer group.Done()
			result, queryErr := server.prometheus.QueryRange(request.Context(), prometheusprovider.Query{
				MetricID: metricID,
				Start:    asOf.Add(-15 * time.Minute),
				End:      asOf,
				Tenant:   request.URL.Query().Get("tenant"),
				Model:    request.URL.Query().Get("model"),
				Instance: request.URL.Query().Get("instance"),
				Node:     request.URL.Query().Get("node"),
			})
			mutex.Lock()
			defer mutex.Unlock()
			if queryErr != nil {
				warnings = append(warnings, metricID+": "+queryErr.Error())
				return
			}
			overview.Metrics[metricID] = result
		}()
	}
	group.Add(1)
	go func() {
		defer group.Done()
		traces, queryErr := server.jaeger.Search(request.Context(), jaegerprovider.SearchRequest{
			Start: asOf.Add(-15 * time.Minute), End: asOf, Limit: 20,
			Tenant: request.URL.Query().Get("tenant"), Model: request.URL.Query().Get("model"), Instance: request.URL.Query().Get("instance"),
		})
		mutex.Lock()
		defer mutex.Unlock()
		if queryErr != nil {
			warnings = append(warnings, "traces: "+queryErr.Error())
			return
		}
		overview.Traces = traces
	}()
	group.Wait()
	if len(overview.Traces) > 0 {
		server.indexTraces(request.Context(), overview.Traces)
	}
	providers, providerWarnings := server.providerStates(request.Context())
	overview.Freshness = providers
	warnings = append(warnings, providerWarnings...)
	writeData(writer, request, http.StatusOK, overview, len(warnings) > 0, warnings, sourceVersions(server.cache.SyncedAt()))
}

func emptySnapshot(at time.Time, availability string) model.CurrentSnapshot {
	return model.CurrentSnapshot{
		CapturedAt: at,
		Configuration: model.Configuration{
			AsOf: at, Availability: availability,
			Models: []model.PlatformResource{}, WorkerNodes: []model.PlatformResource{}, Tenants: []model.PlatformResource{},
			Policies: model.PolicySet{
				TenantModel: []model.PlatformResource{}, TenantNode: []model.PlatformResource{}, ModelNode: []model.PlatformResource{},
			},
			Orchestrators: []model.PlatformResource{}, SimulationClocks: []model.PlatformResource{}, SimulatorInstances: []model.PlatformResource{},
			TenantPerformance: []model.PlatformResource{}, TenantRuntimes: []model.PlatformResource{},
		},
		Traffic: model.Traffic{AsOf: at, Tenants: []model.TenantTraffic{}},
		Workloads: model.Workloads{
			Nodes: []model.ClusterNode{}, Pods: []model.Pod{}, Deployments: []model.Deployment{},
			Services: []model.Service{}, Leases: []model.Lease{}, Events: []model.Event{},
		},
	}
}

func (server *Server) snapshotFor(request *http.Request) (model.CurrentSnapshot, string, string, error) {
	requestedAtRaw := strings.TrimSpace(request.URL.Query().Get("at"))
	now := server.currentClockState().ServerTime
	if requestedAtRaw == "" {
		if snapshot, ok, _ := server.currentSnapshotFromStore(request.Context(), now); ok {
			return snapshot, "available", "", nil
		}
		if err := server.requireCacheError(); err != nil {
			return model.CurrentSnapshot{}, "unavailable", "", err
		}
		return server.aggregator.CurrentSnapshot(now), "available", "", nil
	}
	requestedAt, err := time.Parse(time.RFC3339Nano, requestedAtRaw)
	if err != nil {
		return model.CurrentSnapshot{}, "unavailable", "", fmt.Errorf("invalid at timestamp: %w", err)
	}
	requestedAt = requestedAt.UTC()
	if requestedAt.After(now.Add(-2 * time.Second)) {
		if err := server.requireCacheError(); err != nil {
			return model.CurrentSnapshot{}, "unavailable", "", err
		}
		return server.aggregator.CurrentSnapshot(now), "available", "", nil
	}
	if !server.store.Available() {
		return model.CurrentSnapshot{CapturedAt: requestedAt}, "unavailable", "", nil
	}
	stored, err := server.store.SnapshotAt(request.Context(), requestedAt)
	if err != nil {
		return model.CurrentSnapshot{}, "unavailable", "", err
	}
	if stored == nil {
		return model.CurrentSnapshot{CapturedAt: requestedAt}, "unavailable", "", nil
	}
	var snapshot model.CurrentSnapshot
	if err := json.Unmarshal(stored.Payload, &snapshot); err != nil {
		return model.CurrentSnapshot{}, "unavailable", "", fmt.Errorf("decode persisted snapshot %s: %w", stored.ID, err)
	}
	return snapshot, "available", stored.ID, nil
}

// handleResourceStates 暴露数据库中的当前态记录（resource_states），供前端与调试查询。
func (server *Server) handleResourceStates(writer http.ResponseWriter, request *http.Request) {
	kind := strings.TrimSpace(request.URL.Query().Get("kind"))
	namespace := strings.TrimSpace(request.URL.Query().Get("namespace"))
	limit := queryInteger(request, "limit", 100)
	if limit < 1 || limit > 1000 {
		writeProblem(writer, request, http.StatusBadRequest, "INVALID_LIMIT", "limit must be between 1 and 1000.", false, nil)
		return
	}
	if !server.store.Available() {
		writeProblem(writer, request, http.StatusServiceUnavailable, "PERSISTENT_STORE_UNAVAILABLE", store.ErrUnavailable.Error(), true, nil)
		return
	}
	records, err := server.store.ListResourceStates(request.Context(), kind, namespace, limit)
	if err != nil {
		writeProblem(writer, request, http.StatusServiceUnavailable, "RESOURCE_STATES_UNAVAILABLE", err.Error(), true, nil)
		return
	}
	writeData(writer, request, http.StatusOK, map[string]any{"states": records, "count": len(records)}, false, nil, sourceVersions(server.cache.SyncedAt()))
}

// currentSnapshotFromStore 优先从数据库当前态（resource_states）重建快照；
// 存储不可用、记录为空或查询失败时返回 ok=false，由调用方回退实时聚合。
func (server *Server) currentSnapshotFromStore(ctx context.Context, now time.Time) (model.CurrentSnapshot, bool, error) {
	if !server.store.Available() {
		return model.CurrentSnapshot{}, false, nil
	}
	records, err := server.store.ListResourceStates(ctx, "", "", 1000)
	if err != nil {
		server.logger.Error("Could not read current resource states from database", "error", err)
		return model.CurrentSnapshot{}, false, err
	}
	if len(records) == 0 {
		return model.CurrentSnapshot{}, false, nil
	}
	snapshot, ok := currentSnapshotFromRecords(records, now)
	return snapshot, ok, nil
}

// currentSnapshotFromRecords 把数据库当前态记录重组为聚合快照，供读接口返回；
// 无法反序列化的单条记录跳过，其余记录不受影响。
func currentSnapshotFromRecords(records []store.ResourceStateRecord, now time.Time) (model.CurrentSnapshot, bool) {
	if len(records) == 0 {
		return model.CurrentSnapshot{}, false
	}
	snapshot := emptySnapshot(now, "available")
	asOf := now
	seenCapturedAt := false
	for _, record := range records {
		if !record.CapturedAt.IsZero() && (!seenCapturedAt || record.CapturedAt.After(asOf)) {
			asOf = record.CapturedAt
			seenCapturedAt = true
		}
		switch record.Kind {
		case "Model":
			var resource model.PlatformResource
			if json.Unmarshal(record.Payload, &resource) == nil {
				snapshot.Configuration.Models = append(snapshot.Configuration.Models, resource)
			}
		case "WorkerNode":
			var resource model.PlatformResource
			if json.Unmarshal(record.Payload, &resource) == nil {
				snapshot.Configuration.WorkerNodes = append(snapshot.Configuration.WorkerNodes, resource)
			}
		case "Tenant":
			var resource model.PlatformResource
			if json.Unmarshal(record.Payload, &resource) == nil {
				snapshot.Configuration.Tenants = append(snapshot.Configuration.Tenants, resource)
			}
		case "TenantModelPolicy":
			var resource model.PlatformResource
			if json.Unmarshal(record.Payload, &resource) == nil {
				snapshot.Configuration.Policies.TenantModel = append(snapshot.Configuration.Policies.TenantModel, resource)
			}
		case "TenantNodePolicy":
			var resource model.PlatformResource
			if json.Unmarshal(record.Payload, &resource) == nil {
				snapshot.Configuration.Policies.TenantNode = append(snapshot.Configuration.Policies.TenantNode, resource)
			}
		case "ModelNodePolicy":
			var resource model.PlatformResource
			if json.Unmarshal(record.Payload, &resource) == nil {
				snapshot.Configuration.Policies.ModelNode = append(snapshot.Configuration.Policies.ModelNode, resource)
			}
		case "Orchestrator":
			var resource model.PlatformResource
			if json.Unmarshal(record.Payload, &resource) == nil {
				snapshot.Configuration.Orchestrators = append(snapshot.Configuration.Orchestrators, resource)
			}
		case "SimulationClock":
			var resource model.PlatformResource
			if json.Unmarshal(record.Payload, &resource) == nil {
				snapshot.Configuration.SimulationClocks = append(snapshot.Configuration.SimulationClocks, resource)
			}
		case "SimulatorInstance":
			var resource model.PlatformResource
			if json.Unmarshal(record.Payload, &resource) == nil {
				snapshot.Configuration.SimulatorInstances = append(snapshot.Configuration.SimulatorInstances, resource)
			}
		case "TenantPerformance":
			var resource model.PlatformResource
			if json.Unmarshal(record.Payload, &resource) == nil {
				snapshot.Configuration.TenantPerformance = append(snapshot.Configuration.TenantPerformance, resource)
			}
		case "TenantRuntime":
			var resource model.PlatformResource
			if json.Unmarshal(record.Payload, &resource) == nil {
				snapshot.Configuration.TenantRuntimes = append(snapshot.Configuration.TenantRuntimes, resource)
			}
		case "TenantTraffic":
			var traffic model.TenantTraffic
			if json.Unmarshal(record.Payload, &traffic) == nil {
				snapshot.Traffic.Tenants = append(snapshot.Traffic.Tenants, traffic)
			}
		case "Node":
			var node model.ClusterNode
			if json.Unmarshal(record.Payload, &node) == nil {
				snapshot.Workloads.Nodes = append(snapshot.Workloads.Nodes, node)
			}
		case "Deployment":
			var deployment model.Deployment
			if json.Unmarshal(record.Payload, &deployment) == nil {
				snapshot.Workloads.Deployments = append(snapshot.Workloads.Deployments, deployment)
			}
		case "Pod":
			var pod model.Pod
			if json.Unmarshal(record.Payload, &pod) == nil {
				snapshot.Workloads.Pods = append(snapshot.Workloads.Pods, pod)
			}
		}
	}
	snapshot.CapturedAt = asOf
	snapshot.Configuration.AsOf = asOf
	snapshot.Traffic.AsOf = asOf
	return snapshot, true
}

func (server *Server) providerStates(ctx context.Context) (map[string]model.ProviderState, []string) {
	states := map[string]model.ProviderState{
		"kubernetes": {State: map[bool]string{true: "ready", false: "not-ready"}[server.cache.Synced()], ObservedAt: time.Now().UTC()},
	}
	warnings := make([]string, 0)
	var mutex sync.Mutex
	var group sync.WaitGroup
	checks := []struct {
		name      string
		health    func(context.Context) error
		retention string
		storage   string
	}{
		{"prometheus", server.prometheus.Health, "provider-configured", "prometheus-tsdb"},
		{"jaeger", server.jaeger.Health, "runtime-configured", "jaeger"},
		{"postgresql", server.store.Health, server.config.Persistence.SnapshotRetention.String(), "persistent"},
	}
	for _, check := range checks {
		check := check
		group.Add(1)
		go func() {
			defer group.Done()
			checkContext, cancel := context.WithTimeout(ctx, 3*time.Second)
			defer cancel()
			err := check.health(checkContext)
			state := model.ProviderState{State: "ready", ObservedAt: time.Now().UTC(), Retention: check.retention, Storage: check.storage}
			if err != nil {
				state.State = "unavailable"
				state.Error = err.Error()
			}
			mutex.Lock()
			states[check.name] = state
			if err != nil {
				warnings = append(warnings, check.name+": "+err.Error())
			}
			mutex.Unlock()
		}()
	}
	group.Wait()
	return states, warnings
}

func (server *Server) timeline(ctx context.Context, limit int) ([]model.TimelineItem, error) {
	if !server.store.Available() {
		return []model.TimelineItem{}, store.ErrUnavailable
	}
	return server.store.ListTimeline(ctx, limit, nil)
}

func (server *Server) indexTraces(ctx context.Context, traces []model.TraceSummary) {
	if !server.store.Available() || len(traces) == 0 {
		return
	}
	indexContext, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := server.store.IndexTraces(indexContext, traces); err != nil {
		server.logger.Error("Could not persist Trace index", "error", err)
	}
}

func (server *Server) entityLinks(detail model.TraceDetail) []model.ResourceRef {
	seen := map[string]model.ResourceRef{}
	for _, span := range detail.Spans {
		for attribute, kind := range map[string]string{
			"platform.tenant.name":             "Tenant",
			"platform.model.name":              "Model",
			"platform.simulator_instance.name": "SimulatorInstance",
		} {
			name, _ := span.Attributes[attribute].(string)
			if name == "" {
				continue
			}
			if object, exists, _ := server.cache.GetPlatform(kind, name); exists {
				key := kind + "/" + name
				seen[key] = model.ResourceRef{APIVersion: "platform.study.com/v1", Kind: kind, Name: name, UID: string(object.GetUID())}
			}
		}
	}
	links := make([]model.ResourceRef, 0, len(seen))
	for _, link := range seen {
		links = append(links, link)
	}
	return links
}

func (server *Server) requireCache(writer http.ResponseWriter, request *http.Request) bool {
	if err := server.requireCacheError(); err != nil {
		writeProblem(writer, request, http.StatusServiceUnavailable, "KUBERNETES_CACHE_NOT_READY", err.Error(), true, nil)
		return false
	}
	return true
}

func (server *Server) requireCacheError() error {
	if !server.cache.Synced() {
		return errors.New("Kubernetes informer cache is not synchronized")
	}
	return nil
}

func (server *Server) writeSnapshotError(writer http.ResponseWriter, request *http.Request, err error) {
	status := http.StatusServiceUnavailable
	code := "SNAPSHOT_QUERY_FAILED"
	if strings.Contains(err.Error(), "invalid at timestamp") {
		status = http.StatusBadRequest
		code = "INVALID_TIME"
	}
	writeProblem(writer, request, status, code, err.Error(), status >= 500, nil)
}

func queryTime(request *http.Request, name string, fallback time.Time) (time.Time, error) {
	raw := strings.TrimSpace(request.URL.Query().Get(name))
	if raw == "" {
		return fallback.UTC(), nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("%s must be RFC3339: %w", name, err)
	}
	return parsed.UTC(), nil
}

func queryDuration(request *http.Request, name string, fallback time.Duration) (time.Duration, error) {
	raw := strings.TrimSpace(request.URL.Query().Get(name))
	if raw == "" {
		return fallback, nil
	}
	parsed, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("%s must be a duration: %w", name, err)
	}
	return parsed, nil
}

func queryInteger(request *http.Request, name string, fallback int) int {
	raw := strings.TrimSpace(request.URL.Query().Get(name))
	if raw == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}
	return parsed
}

func requestTimeOrNow(request *http.Request) time.Time {
	value, err := queryTime(request, "at", time.Now().UTC())
	if err != nil {
		return time.Now().UTC()
	}
	return value
}

func sourceVersions(syncedAt time.Time) map[string]string {
	if syncedAt.IsZero() {
		return map[string]string{}
	}
	return map[string]string{"kubernetes": syncedAt.UTC().Format(time.RFC3339Nano)}
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
