package aiops

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/config"
	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/model"
	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/store"
)

// fakeStore 补充方法：窗口聚合与日聚合依赖（与 worker_test.go 同包共享）。
func (store *fakeStore) ListAIOpsAnalysesInWindow(_ context.Context, start, end time.Time) ([]model.AIOpsAnalysis, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	var analyses []model.AIOpsAnalysis
	for _, analysis := range store.analyses {
		if analysis.Status != string(model.AIOpsCompleted) {
			continue
		}
		if !analysis.CreatedAt.Before(start) && analysis.CreatedAt.Before(end) {
			analyses = append(analyses, analysis)
		}
	}
	return analyses, nil
}

func (store *fakeStore) UpsertAIOpsWindowSummary(_ context.Context, summary model.AIOpsWindowSummary) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	for index, existing := range store.windowSummaries {
		if existing.WindowID == summary.WindowID && existing.Level == summary.Level {
			store.windowSummaries[index] = summary
			return nil
		}
	}
	store.windowSummaries = append(store.windowSummaries, summary)
	return nil
}

func completedAnalysis(id string, at time.Time, overall int) model.AIOpsAnalysis {
	scores, _ := json.Marshal(model.AIOpsScores{Overall: overall, Verdict: "success"})
	return model.AIOpsAnalysis{
		AnalysisID: id, SegmentID: "seg-" + id, Status: string(model.AIOpsCompleted),
		Scores: scores, CreatedAt: at, UpdatedAt: at,
	}
}

func aggregationService(database *fakeStore, responses []string) *Service {
	return NewService(config.AIOpsConfig{WindowGranularity: 2 * time.Hour}, database, newFakeLLM(responses, nil), testLogger())
}

func TestNormalizeWindowAggregation(t *testing.T) {
	high := normalizeWindowAggregation(windowAggregation{
		Overall: 120, Trend: "weird", CommonIssues: []string{"a", "b", "c", "d"},
	})
	if high.Overall != 100 || high.Trend != "stable" || len(high.CommonIssues) != 3 {
		t.Fatalf("normalize(high) = %+v", high)
	}
	low := normalizeWindowAggregation(windowAggregation{Overall: -5, Trend: "degrading"})
	if low.Overall != 0 || low.Trend != "degrading" {
		t.Fatalf("normalize(low) = %+v", low)
	}
	good := normalizeWindowAggregation(windowAggregation{Overall: 70, Trend: "improving"})
	if good.Overall != 70 || good.Trend != "improving" {
		t.Fatalf("normalize(good) = %+v", good)
	}
}

func TestWindowIDFormats(t *testing.T) {
	start := time.Date(2026, time.August, 18, 8, 5, 0, 0, time.UTC)
	if got := l3WindowID(start); got != "L3-2026-08-18T08-05" {
		t.Fatalf("l3WindowID = %q", got)
	}
	day := time.Date(2026, time.August, 18, 0, 0, 0, 0, time.UTC)
	if got := l4WindowID(day); got != "L4-2026-08-18" {
		t.Fatalf("l4WindowID = %q", got)
	}
}

func TestChildCount(t *testing.T) {
	if got := childCount([]l3Child{{}, {}}); got != 2 {
		t.Fatalf("childCount(l3) = %d", got)
	}
	if got := childCount([]l4Child{{}}); got != 1 {
		t.Fatalf("childCount(l4) = %d", got)
	}
	if got := childCount("unexpected"); got != 0 {
		t.Fatalf("childCount(other) = %d", got)
	}
}

func TestRuleWindowAggregation(t *testing.T) {
	service := aggregationService(newFakeStore(nil), nil)
	l3 := service.ruleWindowAggregation([]l3Child{
		{Overall: 80, Verdict: "success", Reason: "ok"},
		{Overall: 40, Verdict: "degrading", Reason: "cpu 高"},
	}, nil)
	if l3.Overall != 60 || len(l3.CommonIssues) != 1 || l3.CommonIssues[0] != "cpu 高" {
		t.Fatalf("ruleWindowAggregation(l3) = %+v", l3)
	}
	l4 := service.ruleWindowAggregation([]l4Child{
		{Overall: 80}, {Overall: 40},
	}, nil)
	if l4.Overall != 60 || l4.Situation == "" || l4.Recommendation == "" {
		t.Fatalf("ruleWindowAggregation(l4) = %+v", l4)
	}
	empty := service.ruleWindowAggregation([]l3Child{}, nil)
	if empty.Overall != 0 || empty.Situation == "" {
		t.Fatalf("ruleWindowAggregation(empty) = %+v", empty)
	}
}

func TestToL3ChildrenSkipsBadScores(t *testing.T) {
	now := time.Now().UTC()
	analyses := []model.AIOpsAnalysis{
		completedAnalysis("a", now.Add(-time.Minute), 80),
		{AnalysisID: "b", SegmentID: "seg-b", Status: string(model.AIOpsCompleted),
			Scores: json.RawMessage(`not-json`), CreatedAt: now},
	}
	children := toL3Children(analyses)
	if len(children) != 1 || children[0].AnalysisID != "a" || children[0].Overall != 80 {
		t.Fatalf("toL3Children = %+v", children)
	}
}

func TestRunWindowAggregationProducesL3(t *testing.T) {
	now := time.Now().UTC()
	database := newFakeStore(nil)
	database.analyses["seg-a"] = completedAnalysis("a", now.Add(-30*time.Minute), 80)
	service := aggregationService(database, []string{
		`{"overall":80,"trend":"improving","commonIssues":[],"situation":"窗口稳定","recommendation":"继续观察"}`,
	})
	if err := service.runWindowAggregation(context.Background()); err != nil {
		t.Fatalf("runWindowAggregation: %v", err)
	}
	database.mu.Lock()
	defer database.mu.Unlock()
	var l3 []model.AIOpsWindowSummary
	for _, summary := range database.windowSummaries {
		if summary.Level == string(model.AIOpsWindowL3) {
			l3 = append(l3, summary)
		}
	}
	if len(l3) != 1 || l3[0].WindowID == "" {
		t.Fatalf("L3 summaries = %+v", l3)
	}
}

func TestRunDayAggregationSkipsExisting(t *testing.T) {
	now := time.Now().UTC()
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	database := newFakeStore(nil)
	database.windowSummaries = []model.AIOpsWindowSummary{{
		WindowID: l4WindowID(dayStart), Level: string(model.AIOpsWindowL4),
	}}
	service := aggregationService(database, nil)
	if err := service.runDayAggregation(context.Background()); err != nil {
		t.Fatalf("runDayAggregation: %v", err)
	}
	database.mu.Lock()
	defer database.mu.Unlock()
	if len(database.windowSummaries) != 1 {
		t.Fatalf("已有今日 L4 不应重复产出: %+v", database.windowSummaries)
	}
}

func TestRunDayAggregationProducesL4(t *testing.T) {
	now := time.Now().UTC()
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	database := newFakeStore(nil)
	database.windowSummaries = []model.AIOpsWindowSummary{{
		WindowID: l3WindowID(dayStart.Add(time.Hour)), Level: string(model.AIOpsWindowL3),
		WindowStart: dayStart.Add(time.Hour), WindowEnd: dayStart.Add(3 * time.Hour),
		Scores: json.RawMessage(`{"overall":70,"trend":"stable","situation":"稳定"}`),
	}}
	service := aggregationService(database, []string{
		`{"overall":70,"trend":"stable","commonIssues":[],"situation":"当日稳定","recommendation":"继续观察"}`,
	})
	if err := service.runDayAggregation(context.Background()); err != nil {
		t.Fatalf("runDayAggregation: %v", err)
	}
	database.mu.Lock()
	defer database.mu.Unlock()
	var l4 []model.AIOpsWindowSummary
	for _, summary := range database.windowSummaries {
		if summary.Level == string(model.AIOpsWindowL4) {
			l4 = append(l4, summary)
		}
	}
	if len(l4) != 1 || l4[0].WindowID != l4WindowID(dayStart) {
		t.Fatalf("L4 summaries = %+v", l4)
	}
}

func TestAggregateWindowsToleratesStoreErrors(t *testing.T) {
	service := NewService(config.AIOpsConfig{}, store.Disabled{}, newFakeLLM(nil, nil), testLogger())
	service.aggregateWindows(context.Background()) // 各环节失败只记日志，不应 panic
}
