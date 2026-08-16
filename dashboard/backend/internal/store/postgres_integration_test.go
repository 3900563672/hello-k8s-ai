package store

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/config"
)

// TestPostgresLifecycle 验证数据库生命周期：
// 自动迁移 → 写入快照/当前态 → 重启连接（模拟服务重启）→ 历史数据仍存在、迁移幂等。
// 需要 TEST_DATABASE_URL 指向可用的 PostgreSQL（推荐：docker 起 postgres:17-alpine）。
func TestPostgresLifecycle(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping PostgreSQL integration test")
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	cfg := config.DatabaseConfig{
		URL:            databaseURL,
		Required:       true,
		ConnectTimeout: 10 * time.Second,
		MaxConnections: 5,
		MinConnections: 1,
	}

	ctx := context.Background()
	database, err := OpenPostgres(ctx, cfg, logger)
	if err != nil {
		t.Fatalf("first connect: %v", err)
	}
	if err := database.Migrate(ctx); err != nil {
		t.Fatalf("first migrate: %v", err)
	}
	status, err := database.Status(ctx)
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if status.MigrationsApplied < 3 {
		t.Fatalf("expected at least 3 migrations, got %d", status.MigrationsApplied)
	}

	now := time.Now().UTC()
	if err := database.SaveSnapshot(ctx, SnapshotRecord{
		ID:          fmt.Sprintf("snapshot-itest-%d", now.UnixNano()),
		CapturedAt:  now,
		LogicalTime: now,
		SourceVersions: map[string]string{"test": "1"},
		Payload:        json.RawMessage(`{"configuration":{"models":[]}}`),
	}); err != nil {
		t.Fatalf("save snapshot: %v", err)
	}
	if err := database.UpsertResourceStates(ctx, []ResourceStateRecord{
		{Kind: "Model", Name: "gpt-4o", CapturedAt: now, Payload: json.RawMessage(`{"ref":{"kind":"Model","name":"gpt-4o"}}`)},
		{Kind: "Tenant", Name: "tenant-a", CapturedAt: now, Payload: json.RawMessage(`{"ref":{"kind":"Tenant","name":"tenant-a"}}`)},
	}); err != nil {
		t.Fatalf("upsert states: %v", err)
	}
	// 再次 upsert 同一资源：应更新而不是新增（幂等）。
	if err := database.UpsertResourceStates(ctx, []ResourceStateRecord{
		{Kind: "Model", Name: "gpt-4o", CapturedAt: now.Add(time.Second), Payload: json.RawMessage(`{"ref":{"kind":"Model","name":"gpt-4o"},"updated":true}`)},
	}); err != nil {
		t.Fatalf("upsert states again: %v", err)
	}
	// 写入完成后的状态作为重启基线。
	baseline, err := database.Status(ctx)
	if err != nil {
		t.Fatalf("status after write: %v", err)
	}
	database.Close()

	// 模拟服务重启：重新连接 → 迁移幂等 → 历史数据仍存在。
	restarted, err := OpenPostgres(ctx, cfg, logger)
	if err != nil {
		t.Fatalf("reconnect after restart: %v", err)
	}
	defer restarted.Close()
	if err := restarted.Migrate(ctx); err != nil {
		t.Fatalf("migrate after restart: %v", err)
	}
	statusAfter, err := restarted.Status(ctx)
	if err != nil {
		t.Fatalf("status after restart: %v", err)
	}
	if statusAfter.ResourceEvents != baseline.ResourceEvents {
		t.Fatalf("resource events changed across restart: %d -> %d", baseline.ResourceEvents, statusAfter.ResourceEvents)
	}
	if statusAfter.ResourceSnapshots != baseline.ResourceSnapshots {
		t.Fatalf("snapshots changed across restart: %d -> %d", baseline.ResourceSnapshots, statusAfter.ResourceSnapshots)
	}
	if statusAfter.ResourceStates != 2 {
		t.Fatalf("expected 2 resource states after restart, got %d", statusAfter.ResourceStates)
	}
	states, err := restarted.ListResourceStates(ctx, "Model", "", 10)
	if err != nil {
		t.Fatalf("list states: %v", err)
	}
	if len(states) != 1 || states[0].Name != "gpt-4o" {
		t.Fatalf("unexpected model states: %#v", states)
	}
}