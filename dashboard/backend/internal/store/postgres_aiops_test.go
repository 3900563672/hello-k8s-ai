package store

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/config"
	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/model"
)

// TestAIOpsStoreLifecycle 验证 aiops_* 表的迁移与读写：
// 入队幂等 → 认领 → 进度 → L1 upsert → 完成 → 查询（按切面/列表/实体）。
// 需要 TEST_DATABASE_URL 指向可用 PostgreSQL（与 TestPostgresLifecycle 同门控）。
func TestAIOpsStoreLifecycle(t *testing.T) {
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
		t.Fatalf("open postgres: %v", err)
	}
	defer database.Close()
	if err := database.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	segment := SegmentRecord{
		SegmentID: "aiops-test-segment",
		Tenant:    "default",
		Name:      "aiops-integration",
		Status:    string(model.SegmentCompleted),
	}
	if err := database.CreateSegment(ctx, segment); err != nil {
		t.Fatalf("create segment: %v", err)
	}

	first := model.AIOpsAnalysis{
		AnalysisID: "aiops-test-1",
		SegmentID:  segment.SegmentID,
		Status:     string(model.AIOpsPending),
	}
	if err := database.CreateAIOpsAnalysis(ctx, first); err != nil {
		t.Fatalf("create analysis: %v", err)
	}
	// 同切面重复入队幂等：不产生第二条。
	if err := database.CreateAIOpsAnalysis(ctx, model.AIOpsAnalysis{
		AnalysisID: "aiops-test-dup",
		SegmentID:  segment.SegmentID,
		Status:     string(model.AIOpsPending),
	}); err != nil {
		t.Fatalf("create duplicate analysis: %v", err)
	}
	list, err := database.ListAIOpsAnalyses(ctx, 10, "")
	if err != nil {
		t.Fatalf("list analyses: %v", err)
	}
	found := 0
	for _, analysis := range list {
		if analysis.SegmentID == segment.SegmentID {
			found++
		}
	}
	if found != 1 {
		t.Fatalf("duplicate enqueue created %d rows, want 1", found)
	}

	claimed, err := database.ClaimAIOpsAnalysis(ctx, first.AnalysisID)
	if err != nil || !claimed {
		t.Fatalf("claim analysis: claimed=%v err=%v", claimed, err)
	}
	// 二次认领失败（已 running）。
	claimed, err = database.ClaimAIOpsAnalysis(ctx, first.AnalysisID)
	if err != nil || claimed {
		t.Fatalf("second claim should fail: claimed=%v err=%v", claimed, err)
	}

	if err := database.UpdateAIOpsAnalysisProgress(ctx, first.AnalysisID, string(model.AIOpsAggregating), 3, 3, ""); err != nil {
		t.Fatalf("update progress: %v", err)
	}
	summaries := []model.AIOpsEntitySummary{
		{SummaryID: "aiops-sum-1", AnalysisID: first.AnalysisID, EntityKind: "Pod", EntityName: "pod-a",
			Classification: string(model.AIOpsProblem), Phenomenon: "restart", IssueFlag: true, Conclusion: "异常"},
		{SummaryID: "aiops-sum-2", AnalysisID: first.AnalysisID, EntityKind: "Node", EntityName: "node-1",
			Classification: string(model.AIOpsHealthy), Conclusion: "正常"},
	}
	if err := database.UpsertAIOpsEntitySummaries(ctx, first.AnalysisID, summaries); err != nil {
		t.Fatalf("upsert summaries: %v", err)
	}
	scores, _ := json.Marshal(map[string]any{"overall": 60, "verdict": "attention"})
	summaryPayload, _ := json.Marshal(map[string]any{"entityTotal": 3})
	if err := database.CompleteAIOpsAnalysis(ctx, first.AnalysisID, scores, summaryPayload); err != nil {
		t.Fatalf("complete analysis: %v", err)
	}

	bySegment, err := database.GetAIOpsAnalysisBySegment(ctx, segment.SegmentID)
	if err != nil {
		t.Fatalf("get by segment: %v", err)
	}
	if bySegment.Status != string(model.AIOpsCompleted) || bySegment.L1Done != 3 {
		t.Fatalf("unexpected analysis: %+v", bySegment)
	}
	entities, err := database.ListAIOpsEntitySummaries(ctx, first.AnalysisID)
	if err != nil {
		t.Fatalf("list entities: %v", err)
	}
	if len(entities) != 2 || entities[0].EntityName != "pod-a" {
		t.Fatalf("unexpected entities: %+v", entities)
	}

	// 清理测试数据（幂等可重跑）。
	cleanup := func() {
		_, _ = database.pool.Exec(ctx, `DELETE FROM segments WHERE segment_id=$1`, segment.SegmentID)
	}
	t.Cleanup(cleanup)
}
