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

// openGapDB 打开测试库并迁移（TEST_DATABASE_URL 门控，与既有集成测试同规约）。
func openGapDB(t *testing.T) *Postgres {
	t.Helper()
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
	t.Cleanup(database.Close)
	if err := database.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return database
}

func gapSuffix() string { return fmt.Sprintf("%d", time.Now().UnixNano()) }

// TestGapSegmentCRUD 覆盖 segments 生命周期 + 事件/指标/Trace 子表（#142 store 门禁）。
func TestGapSegmentCRUD(t *testing.T) {
	database := openGapDB(t)
	ctx := context.Background()
	suffix := gapSuffix()
	segmentID := "gap-seg-" + suffix

	record := SegmentRecord{
		SegmentID: segmentID,
		Tenant:    "default",
		Name:      "gap-segment",
		Status:    string(model.SegmentPending),
	}
	if err := database.CreateSegment(ctx, record); err != nil {
		t.Fatalf("create segment: %v", err)
	}
	if err := database.UpdateSegmentLifecycle(ctx, segmentID, string(model.SegmentRunning), "", nil, nil); err != nil {
		t.Fatalf("start segment: %v", err)
	}
	got, err := database.GetSegment(ctx, segmentID)
	if err != nil {
		t.Fatalf("get segment: %v", err)
	}
	if got.Status != string(model.SegmentRunning) || got.StartedAt == nil {
		t.Fatalf("segment should be running with started_at: %+v", got)
	}
	// 幂等再跑 running：start_snapshot 不被覆盖。
	if err := database.UpdateSegmentLifecycle(ctx, segmentID, string(model.SegmentRunning), "", nil, nil); err != nil {
		t.Fatalf("re-start segment: %v", err)
	}
	if err := database.UpdateSegmentLifecycle(ctx, segmentID, string(model.SegmentCompleted), "gap-done",
		json.RawMessage(`{"stable":true}`), json.RawMessage(`{"summary":"ok"}`)); err != nil {
		t.Fatalf("complete segment: %v", err)
	}
	got, err = database.GetSegment(ctx, segmentID)
	if err != nil {
		t.Fatalf("get completed segment: %v", err)
	}
	if got.Status != string(model.SegmentCompleted) || got.EndedAt == nil || got.Summary == nil {
		t.Fatalf("segment should be completed: %+v", got)
	}
	listed, err := database.ListSegments(ctx, 100, string(model.SegmentCompleted))
	if err != nil {
		t.Fatalf("list segments: %v", err)
	}
	found := false
	for _, item := range listed {
		if item.SegmentID == segmentID {
			found = true
		}
	}
	if !found {
		t.Fatalf("completed segment %s not listed", segmentID)
	}

	now := time.Now().UTC()
	event := SegmentEvent{
		EventID:    "gap-ev-" + suffix,
		SegmentID:  segmentID,
		EventType:  "decision",
		OccurredAt: now,
		Entity:     "node-1",
		Severity:   "info",
		Payload:    json.RawMessage(`{"verdict":"attention"}`),
	}
	if err := database.RecordSegmentEvent(ctx, event); err != nil {
		t.Fatalf("record segment event: %v", err)
	}
	events, err := database.ListSegmentEvents(ctx, segmentID, 10)
	if err != nil {
		t.Fatalf("list segment events: %v", err)
	}
	if len(events) != 1 || events[0].EventID != event.EventID {
		t.Fatalf("unexpected segment events: %+v", events)
	}

	bucketStart := now.Truncate(time.Minute)
	buckets := []MetricBucket{{
		MetricName:  "qps",
		BucketStart: bucketStart,
		BucketEnd:   bucketStart.Add(time.Minute),
		Min:         1, Max: 5, Avg: 3, P95: 4.5,
	}}
	if err := database.AppendSegmentMetrics(ctx, segmentID, buckets); err != nil {
		t.Fatalf("append metrics: %v", err)
	}
	metrics, err := database.ListSegmentMetrics(ctx, segmentID, 10)
	if err != nil {
		t.Fatalf("list metrics: %v", err)
	}
	if len(metrics) != 1 || metrics[0].MetricName != "qps" || metrics[0].P95 != 4.5 {
		t.Fatalf("unexpected metrics: %+v", metrics)
	}

	traceID := "gap-trace-" + suffix
	// LinkSegmentTraces 只更新已存在的 trace_index 行：先索引再挂链。
	if err := database.IndexTraces(ctx, []model.TraceSummary{{
		TraceID: traceID, RootService: "simulator", RootOperation: "tick",
		StartTime: now, DurationMs: 12.5, SpanCount: 2, ErrorSpanCount: 0,
		Entities: map[string]string{"tenant": "default", "model": "gpt-4o"},
	}}); err != nil {
		t.Fatalf("index traces: %v", err)
	}
	if err := database.LinkSegmentTraces(ctx, segmentID, []string{traceID}); err != nil {
		t.Fatalf("link traces: %v", err)
	}
	traces, err := database.ListSegmentTraces(ctx, segmentID)
	if err != nil {
		t.Fatalf("list segment traces: %v", err)
	}
	if len(traces) != 1 || traces[0].TraceID != traceID {
		t.Fatalf("unexpected segment traces: %+v", traces)
	}

	// 业务资源事件进入时间线（Tenant 属于 businessKinds）。
	// 先清掉本测试历史残留，避免 ASC+LIMIT 把新行挤出。
	if _, err := database.pool.Exec(ctx, `DELETE FROM resource_events WHERE event_id LIKE 'gap-change-%'`); err != nil {
		t.Fatalf("clean resource events: %v", err)
	}
	if _, err := database.pool.Exec(ctx, `DELETE FROM resource_snapshots WHERE snapshot_id LIKE 'gap-snap-%'`); err != nil {
		t.Fatalf("clean snapshots: %v", err)
	}
	change := model.ResourceChange{
		EventID:    "gap-change-" + suffix,
		OccurredAt: now,
		Operation:  "create",
		Ref:        model.ResourceRef{Kind: "Tenant", Namespace: "default", Name: "tenant-gap"},
	}
	if err := database.RecordResourceChange(ctx, change); err != nil {
		t.Fatalf("record resource change: %v", err)
	}
	resourceEvents, err := database.ListResourceEvents(ctx, now.Add(-time.Hour), 100)
	if err != nil {
		t.Fatalf("list resource events: %v", err)
	}
	foundChange := false
	for _, item := range resourceEvents {
		if item.EventID == change.EventID {
			foundChange = true
		}
	}
	if !foundChange {
		t.Fatalf("resource change not listed: %+v", resourceEvents)
	}
	timeline, err := database.ListTimeline(ctx, 100, nil)
	if err != nil {
		t.Fatalf("list timeline: %v", err)
	}
	foundTimeline := false
	for _, item := range timeline {
		if item.ID == change.EventID {
			foundTimeline = true
		}
	}
	if !foundTimeline {
		t.Fatalf("resource change not in timeline: %+v", timeline)
	}
}

// TestGapSnapshotIdempotencyAudit 覆盖快照回放/幂等/审计/健康（#142 store 门禁）。
func TestGapSnapshotIdempotencyAudit(t *testing.T) {
	database := openGapDB(t)
	ctx := context.Background()
	suffix := gapSuffix()
	now := time.Now().UTC()

	if err := database.Health(ctx); err != nil {
		t.Fatalf("health: %v", err)
	}
	if !database.Available() {
		t.Fatal("available should be true")
	}

	snapshotID := "gap-snap-" + suffix
	if err := database.SaveSnapshot(ctx, SnapshotRecord{
		ID:             snapshotID,
		CapturedAt:     now,
		LogicalTime:    now,
		SourceVersions: map[string]string{"api": "v1"},
		Payload:        json.RawMessage(`{"configuration":{"models":[]}}`),
	}); err != nil {
		t.Fatalf("save snapshot: %v", err)
	}
	snapshot, err := database.SnapshotAt(ctx, now.Add(time.Hour))
	if err != nil {
		t.Fatalf("snapshot at: %v", err)
	}
	if snapshot == nil || snapshot.ID != snapshotID {
		t.Fatalf("unexpected snapshot: %+v", snapshot)
	}
	if got, err := database.SnapshotAt(ctx, now.Add(-time.Hour)); err != nil || got != nil {
		t.Fatalf("snapshot before start should be nil: %+v %v", got, err)
	}

	key := "gap-idem-" + suffix
	record, owned, err := database.ReserveIdempotency(ctx, key, "hash-a", now.Add(time.Hour))
	if err != nil {
		t.Fatalf("reserve idempotency: %v", err)
	}
	if !owned || record.State != "pending" || record.Key != key {
		t.Fatalf("unexpected reservation: owned=%v record=%+v", owned, record)
	}
	// 未过期重复保留：不拥有。
	_, owned, err = database.ReserveIdempotency(ctx, key, "hash-a", now.Add(time.Hour))
	if err != nil {
		t.Fatalf("re-reserve idempotency: %v", err)
	}
	if owned {
		t.Fatal("re-reserve should not be owned while pending")
	}
	if err := database.CompleteIdempotency(ctx, key, "hash-a", 200, json.RawMessage(`{"ok":true}`)); err != nil {
		t.Fatalf("complete idempotency: %v", err)
	}
	if err := database.CompleteIdempotency(ctx, key, "hash-a", 200, json.RawMessage(`{"ok":true}`)); err == nil {
		t.Fatal("complete twice should fail (no longer pending)")
	}
	if err := database.ReleaseIdempotency(ctx, key, "hash-a"); err != nil {
		t.Fatalf("release idempotency: %v", err)
	}
	// 过期 key 可重新拥有。
	expiredKey := "gap-idem-expired-" + suffix
	if _, _, err := database.ReserveIdempotency(ctx, expiredKey, "hash-b", now.Add(-time.Minute)); err != nil {
		t.Fatalf("reserve expired key: %v", err)
	}
	record, owned, err = database.ReserveIdempotency(ctx, expiredKey, "hash-b2", now.Add(time.Hour))
	if err != nil {
		t.Fatalf("re-reserve expired key: %v", err)
	}
	if !owned || record.RequestHash != "hash-b2" {
		t.Fatalf("expired key should be re-owned with new hash: owned=%v record=%+v", owned, record)
	}

	audit := model.AuditRecord{
		OperationID: "gap-audit-" + suffix,
		OccurredAt:  now,
		Actor:       "tester",
		Action:      "create",
		Ref:         model.ResourceRef{Kind: "Tenant", Namespace: "default", Name: "tenant-gap"},
		Outcome:     "success",
		RequestID:   "req-1",
		Details:     json.RawMessage(`{"source":"gap-test"}`),
	}
	if err := database.RecordAudit(ctx, audit); err != nil {
		t.Fatalf("record audit: %v", err)
	}
	if err := database.Prune(ctx, now.Add(-time.Hour)); err != nil {
		t.Fatalf("prune: %v", err)
	}
}

// TestGapAIOpsExtras 覆盖 AIOps 命令列表/窗口分析/审计日志/聊天/任务队列（#142 store 门禁）。
func TestGapAIOpsExtras(t *testing.T) {
	database := openGapDB(t)
	ctx := context.Background()
	suffix := gapSuffix()
	now := time.Now().UTC()

	segmentID := "gap-aiops-seg-" + suffix
	if err := database.CreateSegment(ctx, SegmentRecord{
		SegmentID: segmentID, Tenant: "default", Name: "gap-aiops", Status: string(model.SegmentCompleted),
	}); err != nil {
		t.Fatalf("create aiops segment: %v", err)
	}

	analysisID := "gap-analysis-" + suffix
	if err := database.CreateAIOpsAnalysis(ctx, model.AIOpsAnalysis{
		AnalysisID: analysisID, SegmentID: segmentID, Status: string(model.AIOpsPending),
	}); err != nil {
		t.Fatalf("create analysis: %v", err)
	}
	claimed, err := database.ClaimAIOpsAnalysis(ctx, analysisID)
	if err != nil || !claimed {
		t.Fatalf("claim analysis: claimed=%v err=%v", claimed, err)
	}
	requeued, err := database.RequeueStaleAIOpsAnalyses(ctx, now.Add(time.Minute))
	if err != nil {
		t.Fatalf("requeue stale analyses: %v", err)
	}
	if requeued < 1 {
		t.Fatalf("expected >=1 stale analysis requeued, got %d", requeued)
	}
	if err := database.FailAIOpsAnalysis(ctx, analysisID, "gap-fail"); err != nil {
		t.Fatalf("fail analysis: %v", err)
	}
	failed, err := database.GetAIOpsAnalysis(ctx, analysisID)
	if err != nil {
		t.Fatalf("get failed analysis: %v", err)
	}
	if failed.Status != string(model.AIOpsFailed) || failed.Error != "gap-fail" {
		t.Fatalf("analysis should be failed: %+v", failed)
	}

	// 窗口内已完成分析：先完成一个，再按窗口查询（CreateAIOpsAnalysis 按 segment 幂等，需独立 segment）。
	windowSegmentID := "gap-window-seg-" + suffix
	if err := database.CreateSegment(ctx, SegmentRecord{
		SegmentID: windowSegmentID, Tenant: "default", Name: "gap-window", Status: string(model.SegmentCompleted),
	}); err != nil {
		t.Fatalf("create window segment: %v", err)
	}
	windowAnalysisID := "gap-window-analysis-" + suffix
	if err := database.CreateAIOpsAnalysis(ctx, model.AIOpsAnalysis{
		AnalysisID: windowAnalysisID, SegmentID: windowSegmentID, Status: string(model.AIOpsPending),
	}); err != nil {
		t.Fatalf("create window analysis: %v", err)
	}
	if err := database.CompleteAIOpsAnalysis(ctx, windowAnalysisID,
		json.RawMessage(`{"overall":70}`), json.RawMessage(`{"entityTotal":1}`)); err != nil {
		t.Fatalf("complete window analysis: %v", err)
	}
	inWindow, err := database.ListAIOpsAnalysesInWindow(ctx, now.Add(-time.Hour), now.Add(time.Hour))
	if err != nil {
		t.Fatalf("list analyses in window: %v", err)
	}
	foundWindow := false
	for _, item := range inWindow {
		if item.AnalysisID == windowAnalysisID {
			foundWindow = true
		}
	}
	if !foundWindow {
		t.Fatalf("window analysis not found: %+v", inWindow)
	}

	commandID := "gap-cmd-" + suffix
	parsed, _ := json.Marshal(map[string]any{"sceneType": "潮汐流量"})
	if err := database.CreateAIOpsCommand(ctx, model.AIOpsCommand{
		CommandID: commandID, RawInput: "gap command", Parsed: parsed,
		Status: string(model.AIOpsCommandParsed), Steps: json.RawMessage(`[]`),
	}); err != nil {
		t.Fatalf("create command: %v", err)
	}
	commands, err := database.ListAIOpsCommands(ctx, 10)
	if err != nil {
		t.Fatalf("list commands: %v", err)
	}
	foundCommand := false
	for _, item := range commands {
		if item.CommandID == commandID {
			foundCommand = true
		}
	}
	if !foundCommand {
		t.Fatalf("command not listed: %+v", commands)
	}

	audit := model.AIOpsAuditLog{
		AuditID: "gap-audit-" + suffix, SessionID: "session-1", Kind: "chat",
		Model: "deepseek-v4-flash", DurationMS: 12, MessageLen: 3,
		PromptTokens: 10, CompletionTokens: 5, Status: "success",
	}
	if err := database.CreateAIOpsAuditLog(ctx, audit); err != nil {
		t.Fatalf("create audit log: %v", err)
	}
	calls, tokens, err := database.SumAIOpsUsageSince(ctx, now.Add(-time.Hour))
	if err != nil {
		t.Fatalf("sum usage: %v", err)
	}
	if calls < 1 || tokens < 15 {
		t.Fatalf("usage should include audit log: calls=%d tokens=%d", calls, tokens)
	}

	// 会话消息可重复运行：先清掉同会话历史。
	if _, err := database.pool.Exec(ctx, `DELETE FROM aiops_chat_messages WHERE session_id='session-1'`); err != nil {
		t.Fatalf("clean chat messages: %v", err)
	}
	message := model.AIOpsChatMessage{
		MessageID: "gap-msg-" + suffix, SessionID: "session-1", Role: "user",
		Content: "你好", WindowIDs: json.RawMessage(`[]`),
		AlertIDs: json.RawMessage(`[]`), CommandIDs: json.RawMessage(`[]`),
	}
	if err := database.CreateAIOpsChatMessage(ctx, message); err != nil {
		t.Fatalf("create chat message: %v", err)
	}
	messages, err := database.ListAIOpsChatMessages(ctx, "session-1", 10)
	if err != nil {
		t.Fatalf("list chat messages: %v", err)
	}
	if len(messages) != 1 || messages[0].Content != "你好" {
		t.Fatalf("unexpected chat messages: %+v", messages)
	}

	// 任务表跨运行残留会干扰断言：先清本测试历史。
	if _, err := database.pool.Exec(ctx, `DELETE FROM aiops_jobs WHERE job_id LIKE 'gap-job%'`); err != nil {
		t.Fatalf("clean jobs: %v", err)
	}
	jobID := "gap-job-" + suffix
	if err := database.CreateAIOpsJob(ctx, model.AIOpsJob{
		JobID: jobID, SegmentID: segmentID, Kind: "analysis", Status: "pending", MaxAttempts: 2,
	}); err != nil {
		t.Fatalf("create job: %v", err)
	}
	job, ok, err := database.ClaimNextAIOpsJob(ctx)
	if err != nil {
		t.Fatalf("claim job: %v", err)
	}
	if !ok || job.JobID != jobID {
		t.Fatalf("unexpected claimed job: ok=%v job=%+v", ok, job)
	}
	// 已认领不可再认领。
	if _, ok, err := database.ClaimNextAIOpsJob(ctx); err != nil || ok {
		t.Fatalf("second claim should fail: ok=%v err=%v", ok, err)
	}
	if err := database.CompleteAIOpsJob(ctx, jobID, "done", ""); err != nil {
		t.Fatalf("complete job: %v", err)
	}
	jobs, err := database.ListAIOpsJobs(ctx, 10, "done")
	if err != nil {
		t.Fatalf("list jobs: %v", err)
	}
	if len(jobs) != 1 || jobs[0].JobID != jobID {
		t.Fatalf("unexpected jobs: %+v", jobs)
	}
	// 崩溃遗留 running 任务回收（job2 需要独立 segment，FK 约束）。
	job2SegmentID := "gap-job2-seg-" + suffix
	if err := database.CreateSegment(ctx, SegmentRecord{
		SegmentID: job2SegmentID, Tenant: "default", Name: "gap-job2", Status: string(model.SegmentCompleted),
	}); err != nil {
		t.Fatalf("create job2 segment: %v", err)
	}
	job2ID := "gap-job2-" + suffix
	if err := database.CreateAIOpsJob(ctx, model.AIOpsJob{
		JobID: job2ID, SegmentID: job2SegmentID, Kind: "analysis", Status: "pending", MaxAttempts: 2,
	}); err != nil {
		t.Fatalf("create job2: %v", err)
	}
	if _, ok, err := database.ClaimNextAIOpsJob(ctx); err != nil || !ok {
		t.Fatalf("claim job2: ok=%v err=%v", ok, err)
	}
	staleJobs, err := database.RequeueStaleAIOpsJobs(ctx, now.Add(time.Minute))
	if err != nil {
		t.Fatalf("requeue stale jobs: %v", err)
	}
	if staleJobs < 1 {
		t.Fatalf("expected >=1 stale job requeued, got %d", staleJobs)
	}
}
