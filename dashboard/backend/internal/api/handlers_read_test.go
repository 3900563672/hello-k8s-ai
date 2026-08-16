package api

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/config"
)

func TestEmptySnapshotSerializesCollectionsAsArrays(t *testing.T) {
	snapshot := emptySnapshot(time.Date(2026, time.August, 12, 12, 0, 0, 0, time.UTC), "unavailable")
	payload, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatalf("marshal empty snapshot: %v", err)
	}
	if bytes.Contains(payload, []byte(":null")) {
		t.Fatalf("empty snapshot contains a null collection: %s", payload)
	}
}

func TestHistoryCoverageWarnings(t *testing.T) {
	now := time.Date(2026, time.August, 16, 12, 0, 0, 0, time.UTC)
	server := Server{config: config.Config{
		Prometheus: config.ProviderConfig{Retention: 24 * time.Hour},
		Jaeger:     config.ProviderConfig{Retention: 0},
	}}
	tests := []struct {
		name string
		asOf time.Time
		want int
	}{
		{name: "live window", asOf: now.Add(-5 * time.Minute), want: 0},
		{name: "jaeger in-memory beyond query window", asOf: now.Add(-20 * time.Minute), want: 1},
		{name: "within prometheus retention", asOf: now.Add(-23 * time.Hour), want: 1},
		{name: "before prometheus retention", asOf: now.Add(-25 * time.Hour), want: 2},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			warnings := server.historyCoverageWarnings(test.asOf, now)
			if len(warnings) != test.want {
				t.Fatalf("warnings = %v, want %d", warnings, test.want)
			}
		})
	}

	// 配置了 Jaeger 保留窗口时，按窗口告警而不是内存模式提示
	configured := Server{config: config.Config{
		Prometheus: config.ProviderConfig{Retention: 24 * time.Hour},
		Jaeger:     config.ProviderConfig{Retention: 2 * time.Hour},
	}}
	if warnings := configured.historyCoverageWarnings(now.Add(-3*time.Hour), now); len(warnings) != 1 {
		t.Fatalf("configured jaeger warnings = %v, want 1", warnings)
	}
	if warnings := configured.historyCoverageWarnings(now.Add(-time.Hour), now); len(warnings) != 0 {
		t.Fatalf("within jaeger retention warnings = %v, want 0", warnings)
	}
	if warnings := configured.historyCoverageWarnings(now.Add(-25*time.Hour), now); len(warnings) != 2 {
		t.Fatalf("beyond both retentions warnings = %v, want 2", warnings)
	}
}

func TestJaegerRetentionLabel(t *testing.T) {
	if got := jaegerRetentionLabel(0); got != "in-memory（进程生命周期）" {
		t.Fatalf("label for 0 = %q", got)
	}
	if got := jaegerRetentionLabel(2 * time.Hour); got != "2h0m0s" {
		t.Fatalf("label for 2h = %q", got)
	}
}
