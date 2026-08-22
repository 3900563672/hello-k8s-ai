package app

import (
	"context"
	"io"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/config"
	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/model"
	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/store"
)

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func platformResource(name string) model.PlatformResource {
	return model.PlatformResource{
		Ref: model.ResourceRef{Kind: "Model", Name: name},
		Metadata: model.ResourceMetadata{
			ResourceVersion: "1",
			Generation:      1,
		},
	}
}

func TestSnapshotHasBusinessData(t *testing.T) {
	if snapshotHasBusinessData(model.CurrentSnapshot{}) {
		t.Fatal("empty snapshot should have no business data")
	}
	cases := []struct {
		name   string
		mutate func(*model.Configuration)
	}{
		{"Models", func(c *model.Configuration) { c.Models = []model.PlatformResource{platformResource("m")} }},
		{"WorkerNodes", func(c *model.Configuration) { c.WorkerNodes = []model.PlatformResource{platformResource("w")} }},
		{"Tenants", func(c *model.Configuration) { c.Tenants = []model.PlatformResource{platformResource("t")} }},
		{"TenantModel", func(c *model.Configuration) {
			c.Policies.TenantModel = []model.PlatformResource{platformResource("tm")}
		}},
		{"TenantNode", func(c *model.Configuration) { c.Policies.TenantNode = []model.PlatformResource{platformResource("tn")} }},
		{"ModelNode", func(c *model.Configuration) { c.Policies.ModelNode = []model.PlatformResource{platformResource("mn")} }},
		{"Orchestrators", func(c *model.Configuration) { c.Orchestrators = []model.PlatformResource{platformResource("o")} }},
		{"SimulatorInstances", func(c *model.Configuration) { c.SimulatorInstances = []model.PlatformResource{platformResource("s")} }},
		{"TenantPerformance", func(c *model.Configuration) { c.TenantPerformance = []model.PlatformResource{platformResource("tp")} }},
		{"TenantRuntimes", func(c *model.Configuration) { c.TenantRuntimes = []model.PlatformResource{platformResource("tr")} }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			snapshot := model.CurrentSnapshot{}
			tc.mutate(&snapshot.Configuration)
			if !snapshotHasBusinessData(snapshot) {
				t.Fatalf("%s should count as business data", tc.name)
			}
		})
	}
}

func TestResourceStateRecords(t *testing.T) {
	snapshot := model.CurrentSnapshot{
		CapturedAt: time.Now().UTC(),
	}
	snapshot.Configuration.Models = []model.PlatformResource{platformResource("model-a")}
	snapshot.Configuration.Orchestrators = []model.PlatformResource{platformResource("orch-a")}
	snapshot.Configuration.TenantRuntimes = []model.PlatformResource{platformResource("rt-a")}
	snapshot.Configuration.Policies.TenantModel = []model.PlatformResource{platformResource("policy-a")}
	snapshot.Workloads.Nodes = []model.ClusterNode{
		{Ref: model.ResourceRef{Kind: "Node", Name: "node-1"}},
	}
	snapshot.Workloads.Pods = []model.Pod{
		{Ref: model.ResourceRef{Kind: "Pod", Namespace: "default", Name: "pod-1"}},
	}
	snapshot.Workloads.Deployments = []model.Deployment{
		{Ref: model.ResourceRef{Kind: "Deployment", Namespace: "default", Name: "dep-1"}},
	}
	snapshot.Configuration.WorkerNodes = []model.PlatformResource{platformResource("wn-a")}
	snapshot.Configuration.Tenants = []model.PlatformResource{platformResource("tenant-a")}
	snapshot.Configuration.SimulationClocks = []model.PlatformResource{platformResource("clock-a")}
	snapshot.Configuration.SimulatorInstances = []model.PlatformResource{platformResource("sim-a")}
	snapshot.Configuration.TenantPerformance = []model.PlatformResource{platformResource("perf-a")}
	snapshot.Configuration.Policies.TenantNode = []model.PlatformResource{platformResource("tn-a")}
	snapshot.Configuration.Policies.ModelNode = []model.PlatformResource{platformResource("mn-a")}
	snapshot.Traffic.Tenants = []model.TenantTraffic{
		{Tenant: model.ResourceRef{Kind: "Tenant", Namespace: "default", Name: "tenant-a"}},
	}

	records := resourceStateRecords(snapshot, time.Now().UTC())
	byKind := map[string]int{}
	for _, record := range records {
		byKind[record.Kind]++
		if record.Payload == nil {
			t.Fatalf("%s %s payload is nil", record.Kind, record.Name)
		}
	}
	want := map[string]int{
		"Model": 1, "WorkerNode": 1, "Tenant": 1, "SimulationClock": 1,
		"SimulatorInstance": 1, "TenantPerformance": 1, "TenantRuntime": 1,
		"TenantModelPolicy": 1, "TenantNodePolicy": 1, "ModelNodePolicy": 1,
		"Node": 1, "Pod": 1, "Deployment": 1, "TenantTraffic": 1,
	}
	for kind, count := range want {
		if byKind[kind] != count {
			t.Fatalf("records[%s] = %d, want %d (all=%v)", kind, byKind[kind], count, byKind)
		}
	}
	// Pod 记录带命名空间与名称
	for _, record := range records {
		if record.Kind == "Pod" && (record.Name != "pod-1" || record.Namespace != "default") {
			t.Fatalf("Pod record = %s/%s", record.Namespace, record.Name)
		}
	}
}

func TestOpenDatabaseWithRealPostgres(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	cfg := config.Config{Database: config.DatabaseConfig{
		URL:            url,
		Required:       true,
		ConnectTimeout: 5 * time.Second,
		StartupRetries: 0,
		MaxConnections: 10,
		MinConnections: 1,
	}}
	database, err := openDatabase(context.Background(), cfg, discardLogger())
	if err != nil {
		t.Fatalf("openDatabase with real postgres: %v", err)
	}
	defer database.Close()
	if _, ok := database.(store.Disabled); ok {
		t.Fatal("openDatabase returned Disabled despite reachable database")
	}
}

func TestOpenDatabaseNotRequiredReturnsDisabled(t *testing.T) {
	cfg := config.Config{Database: config.DatabaseConfig{
		URL:            "postgres://user:pass@127.0.0.1:1/none?sslmode=disable",
		Required:       false,
		ConnectTimeout: 200 * time.Millisecond,
		StartupRetries: 0,
	}}
	database, err := openDatabase(context.Background(), cfg, discardLogger())
	if err != nil {
		t.Fatalf("openDatabase should not error when not required: %v", err)
	}
	if _, ok := database.(store.Disabled); !ok {
		t.Fatalf("database type = %T, want store.Disabled", database)
	}
}

func TestOpenDatabaseRequiredReturnsError(t *testing.T) {
	cfg := config.Config{Database: config.DatabaseConfig{
		URL:            "postgres://user:pass@127.0.0.1:1/none?sslmode=disable",
		Required:       true,
		ConnectTimeout: 200 * time.Millisecond,
		StartupRetries: 0,
	}}
	if _, err := openDatabase(context.Background(), cfg, discardLogger()); err == nil {
		t.Fatal("openDatabase should error when required and unreachable")
	}
}

func TestOpenDatabaseRetriesThenFails(t *testing.T) {
	cfg := config.Config{Database: config.DatabaseConfig{
		URL:            "postgres://user:pass@127.0.0.1:1/none?sslmode=disable",
		Required:       true,
		ConnectTimeout: 150 * time.Millisecond,
		StartupRetries: 1,
		StartupBackoff: 10 * time.Millisecond,
	}}
	start := time.Now()
	if _, err := openDatabase(context.Background(), cfg, discardLogger()); err == nil {
		t.Fatal("openDatabase should error after retries")
	}
	if elapsed := time.Since(start); elapsed < 10*time.Millisecond {
		t.Fatalf("retry backoff not honored, elapsed %v", elapsed)
	}
}

func TestOpenDatabaseNotRequiredWithRetries(t *testing.T) {
	cfg := config.Config{Database: config.DatabaseConfig{
		URL:            "postgres://user:pass@127.0.0.1:1/none?sslmode=disable",
		Required:       false,
		ConnectTimeout: 150 * time.Millisecond,
		StartupRetries: 1,
		StartupBackoff: 10 * time.Millisecond,
	}}
	database, err := openDatabase(context.Background(), cfg, discardLogger())
	if err != nil {
		t.Fatalf("openDatabase should degrade to Disabled: %v", err)
	}
	if _, ok := database.(store.Disabled); !ok {
		t.Fatalf("database type = %T, want store.Disabled", database)
	}
}

func TestOpenDatabaseNegativeRetriesClampsToZero(t *testing.T) {
	cfg := config.Config{Database: config.DatabaseConfig{
		URL:            "postgres://user:pass@127.0.0.1:1/none?sslmode=disable",
		Required:       true,
		ConnectTimeout: 150 * time.Millisecond,
		StartupRetries: -1,
	}}
	if _, err := openDatabase(context.Background(), cfg, discardLogger()); err == nil {
		t.Fatal("openDatabase should error with clamped retries")
	}
}

func TestOpenDatabaseCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cfg := config.Config{Database: config.DatabaseConfig{
		URL:            "postgres://user:pass@127.0.0.1:1/none?sslmode=disable",
		Required:       true,
		ConnectTimeout: 500 * time.Millisecond,
		StartupRetries: 5,
		StartupBackoff: 500 * time.Millisecond,
	}}
	go func() {
		time.Sleep(30 * time.Millisecond)
		cancel()
	}()
	if _, err := openDatabase(ctx, cfg, discardLogger()); err != context.Canceled {
		t.Fatalf("openDatabase err = %v, want context.Canceled", err)
	}
}
