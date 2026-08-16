package api

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/kubernetes"
	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/model"
	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/store"
)

type resourceStatesStub struct {
	store.Disabled
	available    bool
	records      []store.ResourceStateRecord
	err          error
	gotKind      string
	gotNamespace string
	gotLimit     int
}

func (stub *resourceStatesStub) Available() bool { return stub.available }

func (stub *resourceStatesStub) ListResourceStates(_ context.Context, kind, namespace string, limit int) ([]store.ResourceStateRecord, error) {
	stub.gotKind = kind
	stub.gotNamespace = namespace
	stub.gotLimit = limit
	return stub.records, stub.err
}

func platformRecord(kind, name string, capturedAt time.Time) store.ResourceStateRecord {
	payload, err := json.Marshal(model.PlatformResource{
		Ref: model.ResourceRef{Name: name},
	})
	if err != nil {
		panic(err)
	}
	return store.ResourceStateRecord{Kind: kind, Name: name, CapturedAt: capturedAt, Payload: payload}
}

func TestCurrentSnapshotFromRecords(t *testing.T) {
	now := time.Date(2026, time.August, 16, 4, 0, 0, 0, time.UTC)
	capturedAt := now.Add(-30 * time.Second)
	records := []store.ResourceStateRecord{
		platformRecord("Model", "gpt-4o", capturedAt),
		platformRecord("WorkerNode", "node-a", capturedAt),
		platformRecord("Tenant", "acme", capturedAt),
		platformRecord("TenantModelPolicy", "acme-gpt-4o", capturedAt),
		platformRecord("TenantNodePolicy", "acme-node-a", capturedAt),
		platformRecord("ModelNodePolicy", "gpt-4o-node-a", capturedAt),
		platformRecord("Orchestrator", "orchestrator", capturedAt),
		platformRecord("SimulationClock", "clock", capturedAt),
		platformRecord("SimulatorInstance", "instance", capturedAt),
		platformRecord("TenantPerformance", "acme-perf", capturedAt),
		platformRecord("TenantRuntime", "acme-runtime", capturedAt),
	}
	trafficPayload, err := json.Marshal(model.TenantTraffic{Tenant: model.ResourceRef{Name: "acme"}})
	if err != nil {
		t.Fatalf("marshal traffic: %v", err)
	}
	records = append(records, store.ResourceStateRecord{Kind: "TenantTraffic", Name: "acme", CapturedAt: capturedAt, Payload: trafficPayload})
	nodePayload, err := json.Marshal(model.ClusterNode{Ref: model.ResourceRef{Name: "node-a"}})
	if err != nil {
		t.Fatalf("marshal node: %v", err)
	}
	records = append(records, store.ResourceStateRecord{Kind: "Node", Name: "node-a", CapturedAt: capturedAt, Payload: nodePayload})
	deploymentPayload, err := json.Marshal(model.Deployment{Ref: model.ResourceRef{Name: "deploy-a"}})
	if err != nil {
		t.Fatalf("marshal deployment: %v", err)
	}
	records = append(records, store.ResourceStateRecord{Kind: "Deployment", Name: "deploy-a", CapturedAt: capturedAt, Payload: deploymentPayload})
	podPayload, err := json.Marshal(model.Pod{Ref: model.ResourceRef{Name: "pod-a"}})
	if err != nil {
		t.Fatalf("marshal pod: %v", err)
	}
	records = append(records, store.ResourceStateRecord{Kind: "Pod", Name: "pod-a", CapturedAt: capturedAt, Payload: podPayload})

	snapshot, ok := currentSnapshotFromRecords(records, now)
	if !ok {
		t.Fatal("expected ok=true for non-empty records")
	}
	if len(snapshot.Configuration.Models) != 1 || snapshot.Configuration.Models[0].Ref.Name != "gpt-4o" {
		t.Fatalf("unexpected models: %#v", snapshot.Configuration.Models)
	}
	if len(snapshot.Configuration.WorkerNodes) != 1 || snapshot.Configuration.WorkerNodes[0].Ref.Name != "node-a" {
		t.Fatalf("unexpected worker nodes: %#v", snapshot.Configuration.WorkerNodes)
	}
	if len(snapshot.Configuration.Tenants) != 1 || snapshot.Configuration.Tenants[0].Ref.Name != "acme" {
		t.Fatalf("unexpected tenants: %#v", snapshot.Configuration.Tenants)
	}
	if len(snapshot.Configuration.Policies.TenantModel) != 1 || snapshot.Configuration.Policies.TenantModel[0].Ref.Name != "acme-gpt-4o" {
		t.Fatalf("unexpected tenantModel policies: %#v", snapshot.Configuration.Policies.TenantModel)
	}
	if len(snapshot.Configuration.Policies.TenantNode) != 1 || snapshot.Configuration.Policies.TenantNode[0].Ref.Name != "acme-node-a" {
		t.Fatalf("unexpected tenantNode policies: %#v", snapshot.Configuration.Policies.TenantNode)
	}
	if len(snapshot.Configuration.Policies.ModelNode) != 1 || snapshot.Configuration.Policies.ModelNode[0].Ref.Name != "gpt-4o-node-a" {
		t.Fatalf("unexpected modelNode policies: %#v", snapshot.Configuration.Policies.ModelNode)
	}
	if len(snapshot.Configuration.Orchestrators) != 1 || len(snapshot.Configuration.SimulationClocks) != 1 ||
		len(snapshot.Configuration.SimulatorInstances) != 1 || len(snapshot.Configuration.TenantPerformance) != 1 ||
		len(snapshot.Configuration.TenantRuntimes) != 1 {
		t.Fatalf("unexpected configuration counts: %#v", snapshot.Configuration)
	}
	if len(snapshot.Traffic.Tenants) != 1 || snapshot.Traffic.Tenants[0].Tenant.Name != "acme" {
		t.Fatalf("unexpected traffic: %#v", snapshot.Traffic)
	}
	if len(snapshot.Workloads.Nodes) != 1 || snapshot.Workloads.Nodes[0].Ref.Name != "node-a" {
		t.Fatalf("unexpected nodes: %#v", snapshot.Workloads.Nodes)
	}
	if len(snapshot.Workloads.Deployments) != 1 || snapshot.Workloads.Deployments[0].Ref.Name != "deploy-a" {
		t.Fatalf("unexpected deployments: %#v", snapshot.Workloads.Deployments)
	}
	if len(snapshot.Workloads.Pods) != 1 || snapshot.Workloads.Pods[0].Ref.Name != "pod-a" {
		t.Fatalf("unexpected pods: %#v", snapshot.Workloads.Pods)
	}
	if !snapshot.Configuration.AsOf.Equal(capturedAt) || !snapshot.Traffic.AsOf.Equal(capturedAt) {
		t.Fatalf("asOf not derived from latest capturedAt: configuration=%s traffic=%s", snapshot.Configuration.AsOf, snapshot.Traffic.AsOf)
	}
}

func TestCurrentSnapshotFromRecordsEmpty(t *testing.T) {
	if _, ok := currentSnapshotFromRecords(nil, time.Now()); ok {
		t.Fatal("expected ok=false for empty records")
	}
}

func TestCurrentSnapshotFromRecordsSkipsBrokenPayload(t *testing.T) {
	now := time.Now().UTC()
	records := []store.ResourceStateRecord{
		{Kind: "Model", Name: "broken", CapturedAt: now, Payload: json.RawMessage(`{"ref":`)},
		platformRecord("Model", "gpt-4o", now),
	}
	snapshot, ok := currentSnapshotFromRecords(records, now)
	if !ok {
		t.Fatal("expected ok=true despite broken record")
	}
	if len(snapshot.Configuration.Models) != 1 || snapshot.Configuration.Models[0].Ref.Name != "gpt-4o" {
		t.Fatalf("broken record should be skipped, got %#v", snapshot.Configuration.Models)
	}
}

func TestHandleResourceStates(t *testing.T) {
	stub := &resourceStatesStub{
		available: true,
		records: []store.ResourceStateRecord{
			platformRecord("Model", "gpt-4o", time.Now().UTC()),
		},
	}
	server := &Server{
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		cache:  &kubernetes.Cache{},
		store:  stub,
	}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/resources?kind=Model&namespace=default&limit=5", nil)
	recorder := httptest.NewRecorder()
	server.handleResourceStates(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	if stub.gotKind != "Model" || stub.gotNamespace != "default" || stub.gotLimit != 5 {
		t.Fatalf("filters not passed through: kind=%q namespace=%q limit=%d", stub.gotKind, stub.gotNamespace, stub.gotLimit)
	}
	var envelope struct {
		Data struct {
			Count  int                         `json:"count"`
			States []store.ResourceStateRecord `json:"states"`
		} `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if envelope.Data.Count != 1 || len(envelope.Data.States) != 1 || envelope.Data.States[0].Kind != "Model" {
		t.Fatalf("unexpected response: %s", recorder.Body.String())
	}
}

func TestHandleResourceStatesUnavailable(t *testing.T) {
	server := &Server{
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		cache:  &kubernetes.Cache{},
		store:  &resourceStatesStub{available: false},
	}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/resources", nil)
	recorder := httptest.NewRecorder()
	server.handleResourceStates(recorder, request)
	if recorder.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d: %s", recorder.Code, recorder.Body.String())
	}
}

func TestHandleResourceStatesInvalidLimit(t *testing.T) {
	server := &Server{
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		cache:  &kubernetes.Cache{},
		store:  &resourceStatesStub{available: true},
	}
	for _, limit := range []string{"0", "1001", "-3"} {
		request := httptest.NewRequest(http.MethodGet, "/api/v1/resources?limit="+limit, nil)
		recorder := httptest.NewRecorder()
		server.handleResourceStates(recorder, request)
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("limit=%s expected 400, got %d: %s", limit, recorder.Code, recorder.Body.String())
		}
	}
}
