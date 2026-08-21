package aiops

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/config"
	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/model"
	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/store"
)

// m3FakeStore 实现 M3 需要的 store 方法（窗口/告警/窗口内分析）。
type m3FakeStore struct {
	store.Disabled
	mu       sync.Mutex
	analyses []model.AIOpsAnalysis
	windows  []model.AIOpsWindowSummary
	alerts   []model.AIOpsAlert
	upserted []string
}

func (store *m3FakeStore) Available() bool { return true }
func (store *m3FakeStore) ListAIOpsAnalysesInWindow(_ context.Context, _, _ time.Time) ([]model.AIOpsAnalysis, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	return store.analyses, nil
}
func (store *m3FakeStore) ListAIOpsWindowSummaries(_ context.Context, level string, _ int) ([]model.AIOpsWindowSummary, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	var result []model.AIOpsWindowSummary
	for _, window := range store.windows {
		if window.Level == level {
			result = append(result, window)
		}
	}
	return result, nil
}
func (store *m3FakeStore) UpsertAIOpsWindowSummary(_ context.Context, summary model.AIOpsWindowSummary) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.upserted = append(store.upserted, summary.WindowID)
	store.windows = append(store.windows, summary)
	return nil
}
func (store *m3FakeStore) CreateAIOpsAlert(_ context.Context, alert model.AIOpsAlert) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.alerts = append(store.alerts, alert)
	return nil
}

func m3TestService(store store.Store, llm LLM) *Service {
	return NewService(config.AIOpsConfig{
		Enabled:           true,
		MaxTokensPerCall:  1024,
		WindowGranularity: 2 * time.Hour,
		AlertThreshold:    40,
		AlertConsecutive:  3,
	}, store, llm, testLogger())
}

func TestRunWindowAggregation(t *testing.T) {
	fake := &m3FakeStore{}
	scores, _ := json.Marshal(map[string]any{"overall": 55, "verdict": "attention", "reason": "TTFT 升高"})
	fake.analyses = []model.AIOpsAnalysis{
		{AnalysisID: "a1", Status: "completed", Scores: scores, CreatedAt: time.Now().UTC().Add(-30 * time.Minute)},
		{AnalysisID: "a2", Status: "completed", Scores: scores, CreatedAt: time.Now().UTC().Add(-20 * time.Minute)},
	}
	service := NewService(config.AIOpsConfig{
		Enabled: true, MaxTokensPerCall: 1024, WindowGranularity: 2 * time.Hour,
	}, fake, fakeCommandLLM{err: errFakeUpstream}, testLogger())
	if err := service.runWindowAggregation(context.Background()); err != nil {
		t.Fatalf("runWindowAggregation() error = %v", err)
	}
	if len(fake.upserted) == 0 {
		t.Fatal("expected at least one L3 window upsert")
	}
}

func TestEvaluateAlerts(t *testing.T) {
	t.Run("连续低分触发", func(t *testing.T) {
		fake := &m3FakeStore{}
		now := time.Now().UTC()
		for i := 0; i < 4; i++ {
			scores, _ := json.Marshal(map[string]any{"overall": 30})
			fake.analyses = append(fake.analyses, model.AIOpsAnalysis{
				AnalysisID: "low-" + string(rune('a'+i)), Status: "completed",
				Scores: scores, CreatedAt: now.Add(-time.Duration(4-i) * 10 * time.Minute),
			})
		}
		service := NewService(config.AIOpsConfig{
			Enabled: true, MaxTokensPerCall: 1024, WindowGranularity: 2 * time.Hour,
			AlertThreshold: 40, AlertConsecutive: 3,
		}, fake, fakeCommandLLM{err: errFakeUpstream}, testLogger())
		if err := service.evaluateAlerts(context.Background()); err != nil {
			t.Fatalf("evaluateAlerts() error = %v", err)
		}
		if len(fake.alerts) == 0 {
			t.Fatal("expected alert for consecutive low scores")
		}
		if fake.alerts[0].Rule != alertRuleConsecutiveLow {
			t.Fatalf("unexpected rule: %s", fake.alerts[0].Rule)
		}
	})
	t.Run("分数正常不触发", func(t *testing.T) {
		fake := &m3FakeStore{}
		now := time.Now().UTC()
		for i := 0; i < 3; i++ {
			scores, _ := json.Marshal(map[string]any{"overall": 80})
			fake.analyses = append(fake.analyses, model.AIOpsAnalysis{
				AnalysisID: "ok-" + string(rune('a'+i)), Status: "completed",
				Scores: scores, CreatedAt: now.Add(-time.Duration(3-i) * 10 * time.Minute),
			})
		}
		service := NewService(config.AIOpsConfig{
			Enabled: true, MaxTokensPerCall: 1024, WindowGranularity: 2 * time.Hour,
			AlertThreshold: 40, AlertConsecutive: 3,
		}, fake, fakeCommandLLM{err: errFakeUpstream}, testLogger())
		if err := service.evaluateAlerts(context.Background()); err != nil {
			t.Fatalf("evaluateAlerts() error = %v", err)
		}
		if len(fake.alerts) != 0 {
			t.Fatalf("expected no alert, got %d", len(fake.alerts))
		}
	})
}

var errFakeUpstream = errors.New("upstream unavailable")

func TestConsecutiveLowTail(t *testing.T) {
	points := []scorePoint{
		{overall: 80}, {overall: 30}, {overall: 35}, {overall: 20},
	}
	tail := consecutiveLowTail(points, 40, 3)
	if len(tail) != 3 {
		t.Fatalf("expected tail of 3, got %d: %+v", len(tail), tail)
	}
	if tail[0].overall != 30 {
		t.Fatalf("expected first tail point 30, got %d", tail[0].overall)
	}
}
