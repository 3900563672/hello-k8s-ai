package segment

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/model"
	prometheusprovider "github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/providers/prometheus"
	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/store"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestClassifyResourceEvent(t *testing.T) {
	now := time.Date(2026, time.August, 18, 8, 0, 0, 0, time.UTC)
	state := &activeSegment{lastDecisions: map[string]string{}}

	// TimelineGap → gap，关键事件
	gap := model.ResourceChange{
		EventID: "gap-1", OccurredAt: now, Operation: "gap",
		Ref:     model.ResourceRef{Kind: "TimelineGap", Name: "recorder"},
		Payload: json.RawMessage(`{"dropped":1}`),
	}
	event, critical := classifyResourceEvent(gap, state)
	if event == nil || event.EventType != model.SegmentEventGap || event.Severity != severityWarning || !critical {
		t.Fatalf("gap event = %#v critical=%v", event, critical)
	}

	// Orchestrator lastScaling → decision；相同指纹去重
	scaling := model.ResourceChange{
		EventID: "evt-1", OccurredAt: now, Operation: "update",
		Ref:     model.ResourceRef{Kind: "Orchestrator", Name: "tenant-a"},
		Payload: json.RawMessage(`{"status":{"lastScaling":{"action":"ScaleUp","instanceName":"tenant-a-model-a","oldReplicas":2,"newReplicas":6}}}`),
	}
	event, critical = classifyResourceEvent(scaling, state)
	if event == nil || event.EventType != model.SegmentEventDecision || !critical {
		t.Fatalf("scaling decision = %#v critical=%v", event, critical)
	}
	if again, _ := classifyResourceEvent(scaling, state); again != nil {
		t.Fatalf("duplicate scaling decision was not deduplicated: %#v", again)
	}

	// Orchestrator 没有扩缩记录 → 不产生决策事件
	idle := model.ResourceChange{
		EventID: "evt-2", OccurredAt: now, Operation: "update",
		Ref:     model.ResourceRef{Kind: "Orchestrator", Name: "tenant-a"},
		Payload: json.RawMessage(`{"status":{"conditions":[]}}`),
	}
	if event, _ := classifyResourceEvent(idle, state); event != nil {
		t.Fatalf("orchestrator without lastScaling produced event: %#v", event)
	}

	// SimulatorInstance spec 变化 → decision；纯 status 更新不算
	specChange := model.ResourceChange{
		EventID: "evt-3", OccurredAt: now, Operation: "update",
		Ref:     model.ResourceRef{Kind: "SimulatorInstance", Name: "tenant-a-model-a"},
		Payload: json.RawMessage(`{"spec":{"replicas":6,"traffic":{"qps":20}}}`),
	}
	event, _ = classifyResourceEvent(specChange, state)
	if event == nil || event.EventType != model.SegmentEventDecision {
		t.Fatalf("spec decision = %#v", event)
	}
	if again, _ := classifyResourceEvent(specChange, state); again != nil {
		t.Fatalf("duplicate spec decision was not deduplicated: %#v", again)
	}
	statusOnly := model.ResourceChange{
		EventID: "evt-4", OccurredAt: now, Operation: "update",
		Ref:     model.ResourceRef{Kind: "SimulatorInstance", Name: "tenant-a-model-a"},
		Payload: json.RawMessage(`{"status":{"phase":"Running"}}`),
	}
	if event, _ := classifyResourceEvent(statusOnly, state); event != nil {
		t.Fatalf("status-only update produced decision: %#v", event)
	}

	// 无关资源不产生事件
	irrelevant := model.ResourceChange{
		EventID: "evt-5", OccurredAt: now, Operation: "add",
		Ref: model.ResourceRef{Kind: "Tenant", Name: "tenant-b"},
	}
	if event, _ := classifyResourceEvent(irrelevant, state); event != nil {
		t.Fatalf("irrelevant resource produced event: %#v", event)
	}
}

func TestBucketAccumulatorDedupesAndComputesStats(t *testing.T) {
	acc := &bucketAccumulator{
		metricName: "simulator.ttft",
		start:      time.Date(2026, time.August, 18, 8, 0, 0, 0, time.UTC),
		end:        time.Date(2026, time.August, 18, 8, 1, 0, 0, time.UTC),
		seen:       map[int64]struct{}{},
	}
	base := acc.start
	acc.add(10, base.Add(5*time.Second))
	acc.add(10, base.Add(5*time.Second)) // 同一秒去重
	acc.add(20, base.Add(10*time.Second))
	acc.add(30, base.Add(15*time.Second))
	acc.add(40, base.Add(20*time.Second))
	acc.add(1000, base.Add(25*time.Second))
	if len(acc.values) != 5 {
		t.Fatalf("values = %v, want 5 after dedupe", acc.values)
	}
	bucket := acc.snapshot()
	if bucket.Min != 10 || bucket.Max != 1000 || bucket.Avg != 220 {
		t.Fatalf("bucket stats = %#v", bucket)
	}
	// 5 个值排序 [10,20,30,40,1000]，p95 取第 ceil(4.75)=5 个
	if bucket.P95 != 1000 {
		t.Fatalf("p95 = %v, want 1000", bucket.P95)
	}
	if !acc.complete(acc.end) || acc.complete(acc.end.Add(-time.Second)) {
		t.Fatal("bucket completion boundary is wrong")
	}
}

func TestDueSwitchesBetweenBaselineAndBurst(t *testing.T) {
	config := Config{BaselineInterval: 30 * time.Second, QuiescenceWindow: 60 * time.Second}
	now := time.Date(2026, time.August, 18, 8, 0, 0, 0, time.UTC)
	state := &activeSegment{lastCritical: now.Add(-10 * time.Second), lastSample: now}
	if !state.due(now, config) {
		t.Fatal("segment in burst window must be due every tick")
	}
	state.lastCritical = now.Add(-120 * time.Second)
	state.lastSample = now.Add(-29 * time.Second)
	if state.due(now, config) {
		t.Fatal("segment outside burst window must wait for baseline interval")
	}
	state.lastSample = now.Add(-31 * time.Second)
	if !state.due(now, config) {
		t.Fatal("segment past baseline interval must be due")
	}
}

// fakeSegmentStore 只实现采样器用到的 store 方法。
type fakeSegmentStore struct {
	store.Disabled
	mu         sync.Mutex
	segments   []model.SegmentRecord
	changes    []model.ResourceChange
	events     []model.SegmentEvent
	buckets    []store.MetricBucket
	flushCalls int
}

func (fake *fakeSegmentStore) Available() bool { return true }

func (fake *fakeSegmentStore) ListSegments(_ context.Context, _ int, status string) ([]model.SegmentRecord, error) {
	fake.mu.Lock()
	defer fake.mu.Unlock()
	var records []model.SegmentRecord
	for _, record := range fake.segments {
		if record.Status == status {
			records = append(records, record)
		}
	}
	return records, nil
}

func (fake *fakeSegmentStore) ListResourceEvents(_ context.Context, since time.Time, _ int) ([]model.ResourceChange, error) {
	fake.mu.Lock()
	defer fake.mu.Unlock()
	var changes []model.ResourceChange
	for _, change := range fake.changes {
		if !change.OccurredAt.Before(since) {
			changes = append(changes, change)
		}
	}
	return changes, nil
}

func (fake *fakeSegmentStore) RecordSegmentEvent(_ context.Context, event model.SegmentEvent) error {
	fake.mu.Lock()
	defer fake.mu.Unlock()
	fake.events = append(fake.events, event)
	return nil
}

func (fake *fakeSegmentStore) AppendSegmentMetrics(_ context.Context, _ string, buckets []store.MetricBucket) error {
	fake.mu.Lock()
	defer fake.mu.Unlock()
	fake.buckets = append(fake.buckets, buckets...)
	return nil
}

// fakeMetricSource 按 query 区间过滤返回指标点，模拟 Prometheus 行为。
type fakeMetricSource struct {
	series map[string][]model.MetricPoint
}

func (fake *fakeMetricSource) QueryRange(_ context.Context, query prometheusprovider.Query) (model.MetricResult, error) {
	result := model.MetricResult{MetricID: query.MetricID, Start: query.Start, End: query.End}
	for _, point := range fake.series[query.MetricID] {
		if !point.Time.Before(query.Start) && !point.Time.After(query.End) {
			result.Series = append(result.Series, model.MetricSeries{Points: []model.MetricPoint{point}})
		}
	}
	return result, nil
}

// fakeSnapshotSource 返回指定实例的目标副本数。
type fakeSnapshotSource struct {
	replicas map[string]int
}

func (fake *fakeSnapshotSource) CurrentSnapshot(time.Time) model.CurrentSnapshot {
	snapshot := model.CurrentSnapshot{}
	tenant := model.TenantTraffic{}
	for name, replicas := range fake.replicas {
		tenant.Instances = append(tenant.Instances, model.TrafficInstance{
			Name: name, DesiredReplicas: replicas,
		})
	}
	snapshot.Traffic.Tenants = []model.TenantTraffic{tenant}
	return snapshot
}

func TestSamplerTickRecordsEventsMetricsAndFlushes(t *testing.T) {
	now := time.Date(2026, time.August, 18, 8, 0, 30, 0, time.UTC)
	ctx := context.Background()
	segmentID := "segment-1"
	database := &fakeSegmentStore{
		segments: []model.SegmentRecord{{
			SegmentID: segmentID, Tenant: "tenant-a", Name: "实验一",
			Status: string(model.SegmentRunning),
		}},
		changes: []model.ResourceChange{{
			EventID: "evt-1", OccurredAt: now.Add(-20 * time.Second), Operation: "update",
			Ref:     model.ResourceRef{Kind: "Orchestrator", Name: "tenant-a"},
			Payload: json.RawMessage(`{"status":{"lastScaling":{"action":"ScaleUp","instanceName":"tenant-a-model-a","oldReplicas":2,"newReplicas":6}}}`),
		}},
	}
	previousMinute := now.Add(-time.Minute).Truncate(time.Minute)
	metricSource := &fakeMetricSource{series: map[string][]model.MetricPoint{
		"simulator.ttft": {
			{Time: previousMinute.Add(15 * time.Second), Value: 100},
			{Time: previousMinute.Add(30 * time.Second), Value: 200},
		},
		"simulator.errorRate": {
			{Time: previousMinute.Add(15 * time.Second), Value: 0.01},
			{Time: previousMinute.Add(30 * time.Second), Value: 0.02},
		},
	}}
	snapshotSource := &fakeSnapshotSource{replicas: map[string]int{"tenant-a-model-a": 2}}
	sampler := New(Config{
		BaselineInterval: 30 * time.Second, BurstInterval: 5 * time.Second,
		QuiescenceWindow: 60 * time.Second, BurstReplicaDelta: 5,
		ErrorRateThreshold: 0.05, TTFTThresholdMS: 2000,
	}, database, metricSource, snapshotSource, testLogger())

	// 第一次采样：建立副本基线 + 决策事件 + 完整分钟桶
	sampler.syncActive(ctx, database.segments, now)
	state := sampler.active[segmentID]
	sampler.sample(ctx, state, now)
	if state.lastCritical.IsZero() {
		t.Fatal("decision event must enter burst window")
	}

	// 第二次采样：副本 2→8 触发 phase_change + burst
	snapshotSource.replicas["tenant-a-model-a"] = 8
	sampler.sample(ctx, state, now.Add(5*time.Second))

	database.mu.Lock()
	events := append([]model.SegmentEvent(nil), database.events...)
	buckets := append([]store.MetricBucket(nil), database.buckets...)
	database.mu.Unlock()

	eventTypes := map[string]int{}
	for _, event := range events {
		eventTypes[event.EventType]++
	}
	if eventTypes[model.SegmentEventDecision] != 1 {
		t.Fatalf("decision events = %d, want 1", eventTypes[model.SegmentEventDecision])
	}
	if eventTypes[model.SegmentEventPhaseChange] != 1 {
		t.Fatalf("phase change events = %d, want 1", eventTypes[model.SegmentEventPhaseChange])
	}
	if eventTypes[model.SegmentEventBurst] != 1 {
		t.Fatalf("burst events = %d, want 1", eventTypes[model.SegmentEventBurst])
	}
	if len(buckets) != 2 {
		t.Fatalf("metric buckets = %d, want 2 (ttft + errorRate)", len(buckets))
	}
	for _, bucket := range buckets {
		if bucket.BucketEnd.After(now) {
			t.Fatalf("incomplete bucket was flushed: %#v", bucket)
		}
	}

	// 切面离开 running：终态冲刷并停止跟踪
	database.mu.Lock()
	database.segments = nil
	database.mu.Unlock()
	sampler.syncActive(ctx, nil, now.Add(10*time.Second))
	if sampler.Active() != 0 {
		t.Fatalf("active segments = %d, want 0", sampler.Active())
	}
}

func TestSamplerThresholdEventsTriggerAlerts(t *testing.T) {
	now := time.Date(2026, time.August, 18, 8, 0, 30, 0, time.UTC)
	ctx := context.Background()
	segmentID := "segment-2"
	database := &fakeSegmentStore{
		segments: []model.SegmentRecord{{
			SegmentID: segmentID, Tenant: "tenant-a", Name: "实验二",
			Status: string(model.SegmentRunning),
		}},
	}
	previousMinute := now.Add(-time.Minute).Truncate(time.Minute)
	metricSource := &fakeMetricSource{series: map[string][]model.MetricPoint{
		"simulator.errorRate": {
			{Time: previousMinute.Add(30 * time.Second), Value: 0.2},
			{Time: previousMinute.Add(45 * time.Second), Value: 0.3},
		},
		"simulator.ttft": {
			{Time: previousMinute.Add(30 * time.Second), Value: 2500},
			{Time: previousMinute.Add(45 * time.Second), Value: 3000},
		},
	}}
	sampler := New(Config{
		BaselineInterval: 30 * time.Second, BurstInterval: 5 * time.Second,
		QuiescenceWindow: 60 * time.Second, BurstReplicaDelta: 5,
		ErrorRateThreshold: 0.05, TTFTThresholdMS: 2000,
	}, database, metricSource, &fakeSnapshotSource{}, testLogger())

	sampler.syncActive(ctx, database.segments, now)
	sampler.sample(ctx, sampler.active[segmentID], now)

	database.mu.Lock()
	defer database.mu.Unlock()
	eventTypes := map[string]int{}
	for _, event := range database.events {
		eventTypes[event.EventType]++
	}
	if eventTypes[model.SegmentEventError] != 1 {
		t.Fatalf("error events = %d, want 1", eventTypes[model.SegmentEventError])
	}
	if eventTypes[model.SegmentEventAlert] != 1 {
		t.Fatalf("alert events = %d, want 1", eventTypes[model.SegmentEventAlert])
	}
}
