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

	suffix := fmt.Sprintf("-%d", time.Now().UnixNano())
	segment := SegmentRecord{
		SegmentID: "aiops-test-segment" + suffix,
		Tenant:    "default",
		Name:      "aiops-integration",
		Status:    string(model.SegmentCompleted),
	}
	if err := database.CreateSegment(ctx, segment); err != nil {
		t.Fatalf("create segment: %v", err)
	}

	first := model.AIOpsAnalysis{
		AnalysisID: "aiops-test-1" + suffix,
		SegmentID:  segment.SegmentID,
		Status:     string(model.AIOpsPending),
	}
	if err := database.CreateAIOpsAnalysis(ctx, first); err != nil {
		t.Fatalf("create analysis: %v", err)
	}
	// 同切面重复入队幂等：不产生第二条。
	if err := database.CreateAIOpsAnalysis(ctx, model.AIOpsAnalysis{
		AnalysisID: "aiops-test-dup" + suffix,
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
	// 认领后 attempts 计数为 1（重试语义：#110 阶段一）。
	claimedRow, err := database.GetAIOpsAnalysis(ctx, first.AnalysisID)
	if err != nil {
		t.Fatalf("get analysis after claim: %v", err)
	}
	if claimedRow.Attempts != 1 {
		t.Fatalf("attempts after first claim = %d, want 1", claimedRow.Attempts)
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
		{SummaryID: "aiops-sum-1" + suffix, AnalysisID: first.AnalysisID, EntityKind: "Pod", EntityName: "pod-a",
			Classification: string(model.AIOpsProblem), Phenomenon: "restart", IssueFlag: true, Conclusion: "异常"},
		{SummaryID: "aiops-sum-2" + suffix, AnalysisID: first.AnalysisID, EntityKind: "Node", EntityName: "node-1",
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

	// 失败重试流转：attempts(1) < maxAttempts(2) → 回 pending；再认领后 attempts=2，
	// 失败达上限 → failed 终态。
	retried, err := database.FailOrRetryAIOpsAnalysis(ctx, first.AnalysisID, "boom once", 2)
	if err != nil {
		t.Fatalf("fail or retry: %v", err)
	}
	if !retried {
		t.Fatalf("first failure should retry (attempts=1 < max=2)")
	}
	requeuedRow, err := database.GetAIOpsAnalysis(ctx, first.AnalysisID)
	if err != nil {
		t.Fatalf("get analysis after retry: %v", err)
	}
	if requeuedRow.Status != string(model.AIOpsPending) || requeuedRow.Error != "boom once" {
		t.Fatalf("analysis should be pending with error kept: %+v", requeuedRow)
	}
	if claimed, err = database.ClaimAIOpsAnalysis(ctx, first.AnalysisID); err != nil || !claimed {
		t.Fatalf("re-claim after retry: claimed=%v err=%v", claimed, err)
	}
	if retried, err = database.FailOrRetryAIOpsAnalysis(ctx, first.AnalysisID, "boom twice", 2); err != nil {
		t.Fatalf("fail or retry second: %v", err)
	}
	if retried {
		t.Fatalf("second failure should be final (attempts=2 >= max=2)")
	}
	failedRow, err := database.GetAIOpsAnalysis(ctx, first.AnalysisID)
	if err != nil {
		t.Fatalf("get analysis after final fail: %v", err)
	}
	if failedRow.Status != string(model.AIOpsFailed) || failedRow.Attempts != 2 || failedRow.Error != "boom twice" {
		t.Fatalf("analysis should be failed with attempts=2: %+v", failedRow)
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

// TestAIOpsCommandStoreLifecycle 验证 aiops_commands 表：创建 → 状态推进 → 读取。
// 与 TestAIOpsStoreLifecycle 同 DB 门控。
func TestAIOpsCommandStoreLifecycle(t *testing.T) {
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

	parsed, _ := json.Marshal(map[string]any{"sceneType": "突发流量高峰"})
	steps, _ := json.Marshal([]map[string]string{{"step": "set-traffic", "status": "done"}})
	command := model.AIOpsCommand{
		CommandID: "cmd-test-1-" + fmt.Sprintf("%d", time.Now().UnixNano()),
		RawInput:  "美国时间 9 点开始，持续 2 小时，突发流量高峰",
		Parsed:    parsed,
		Status:    string(model.AIOpsCommandParsed),
		Steps:     steps,
	}
	if err := database.CreateAIOpsCommand(ctx, command); err != nil {
		t.Fatalf("create command: %v", err)
	}
	if err := database.UpdateAIOpsCommand(ctx, command.CommandID, string(model.AIOpsCommandDone), steps, ""); err != nil {
		t.Fatalf("update command: %v", err)
	}
	loaded, err := database.GetAIOpsCommand(ctx, command.CommandID)
	if err != nil {
		t.Fatalf("get command: %v", err)
	}
	if loaded.Status != string(model.AIOpsCommandDone) || loaded.RawInput != command.RawInput {
		t.Fatalf("unexpected command: %+v", loaded)
	}
	t.Cleanup(func() {
		_, _ = database.pool.Exec(ctx, `DELETE FROM aiops_commands WHERE command_id=$1`, command.CommandID)
	})
}

// TestAIOpsWindowAlertStoreLifecycle 验证 aiops_window_summaries 与 aiops_alerts 读写（同 DB 门控）。
func TestAIOpsWindowAlertStoreLifecycle(t *testing.T) {
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

	now := time.Now().UTC()
	// 清掉历史 L3-test 残留，保证重复运行可查（ORDER BY window_start DESC LIMIT 10）。
	if _, err := database.pool.Exec(ctx, `DELETE FROM aiops_window_summaries WHERE window_id LIKE 'L3-test-%'`); err != nil {
		t.Fatalf("clean window leftovers: %v", err)
	}
	windowStart := now.Truncate(2 * time.Hour)
	window := model.AIOpsWindowSummary{
		WindowID:    "L3-test-" + fmt.Sprintf("%d", now.UnixNano()),
		Level:       string(model.AIOpsWindowL3),
		WindowStart: windowStart,
		WindowEnd:   windowStart.Add(2 * time.Hour),
		Scores:      json.RawMessage(`{"overall":60,"trend":"stable"}`),
	}
	if err := database.UpsertAIOpsWindowSummary(ctx, window); err != nil {
		t.Fatalf("upsert window: %v", err)
	}
	// 幂等重写。
	if err := database.UpsertAIOpsWindowSummary(ctx, window); err != nil {
		t.Fatalf("re-upsert window: %v", err)
	}
	windows, err := database.ListAIOpsWindowSummaries(ctx, string(model.AIOpsWindowL3), 10)
	if err != nil {
		t.Fatalf("list windows: %v", err)
	}
	found := false
	for _, item := range windows {
		if item.WindowID == window.WindowID {
			found = true
		}
	}
	if !found {
		t.Fatalf("window %s not found", window.WindowID)
	}

	analysisID := "window-alert-analysis-" + fmt.Sprintf("%d", time.Now().UnixNano())
	// alerts 通过 FK 引用 aiops_analyses（后者再经 segments FK），先建前置数据。
	alertSegment := SegmentRecord{
		SegmentID: "window-alert-segment-" + fmt.Sprintf("%d", time.Now().UnixNano()),
		Tenant:    "default",
		Name:      "window-alert-integration",
		Status:    string(model.SegmentCompleted),
	}
	if err := database.CreateSegment(ctx, alertSegment); err != nil {
		t.Fatalf("create alert segment: %v", err)
	}
	if err := database.CreateAIOpsAnalysis(ctx, model.AIOpsAnalysis{
		AnalysisID: analysisID,
		SegmentID:  alertSegment.SegmentID,
		Status:     string(model.AIOpsCompleted),
	}); err != nil {
		t.Fatalf("create alert analysis: %v", err)
	}
	alert := model.AIOpsAlert{
		AlertID:        "alert-test-" + now.Format("150405"),
		Rule:           "consecutive-low-score",
		Severity:       "warning",
		TriggeredAt:    now,
		AnalysisID:     &analysisID,
		Interpretation: json.RawMessage(`{"summary":"低分"}`),
	}
	if err := database.CreateAIOpsAlert(ctx, alert); err != nil {
		t.Fatalf("create alert: %v", err)
	}
	alerts, err := database.ListAIOpsAlerts(ctx, 10)
	if err != nil {
		t.Fatalf("list alerts: %v", err)
	}
	if len(alerts) == 0 || alerts[0].AnalysisID == nil || *alerts[0].AnalysisID != analysisID {
		t.Fatalf("unexpected alerts: %+v", alerts)
	}
	t.Cleanup(func() {
		_, _ = database.pool.Exec(ctx, `DELETE FROM aiops_window_summaries WHERE window_id=$1`, window.WindowID)
		_, _ = database.pool.Exec(ctx, `DELETE FROM aiops_alerts WHERE alert_id=$1`, alert.AlertID)
	})
}
