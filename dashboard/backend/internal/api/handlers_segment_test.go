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

	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/config"
	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/kubernetes"
	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/model"
	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/store"
)

func TestParseSegmentWindow(t *testing.T) {
	now := time.Date(2026, time.August, 17, 8, 0, 0, 0, time.UTC)
	tests := []struct {
		name    string
		query   string
		wantErr bool
	}{
		{name: "valid", query: "start=2026-08-17T07:00:00Z&end=2026-08-17T08:00:00Z", wantErr: false},
		{name: "missing params", query: "start=2026-08-17T07:00:00Z", wantErr: true},
		{name: "empty params", query: "", wantErr: true},
		{name: "invalid start", query: "start=not-a-time&end=2026-08-17T08:00:00Z", wantErr: true},
		{name: "invalid end", query: "start=2026-08-17T07:00:00Z&end=not-a-time", wantErr: true},
		{name: "start after end", query: "start=2026-08-17T08:00:00Z&end=2026-08-17T07:00:00Z", wantErr: true},
		{name: "start equals end", query: "start=2026-08-17T07:00:00Z&end=2026-08-17T07:00:00Z", wantErr: true},
		{name: "window too large", query: "start=2026-08-16T07:00:00Z&end=2026-08-17T08:00:00Z", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "/api/v1/segment?"+test.query, nil)
			start, end, err := parseSegmentWindow(request)
			if test.wantErr && err == nil {
				t.Fatalf("expected error, got start=%s end=%s", start, end)
			}
			if !test.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
	// 校验归一化：入参带纳秒与偏移，返回 UTC 且保持大小关系
	request := httptest.NewRequest(http.MethodGet,
		"/api/v1/segment?start=2026-08-17T07:00:00.123456789%2B08:00&end=2026-08-17T08:00:00Z", nil)
	start, end, err := parseSegmentWindow(request)
	if err != nil {
		t.Fatalf("parse with offset: %v", err)
	}
	if start.Location() != time.UTC || end.Location() != time.UTC {
		t.Fatalf("expected UTC, got %s / %s", start.Location(), end.Location())
	}
	if !start.Before(end) {
		t.Fatalf("offset parsing broke ordering: %s !< %s", start, end)
	}
	if now.Before(start) {
		t.Fatalf("unexpected start after test reference: %s", start)
	}
}

func TestSegmentCoverageWarnings(t *testing.T) {
	now := time.Date(2026, time.August, 17, 12, 0, 0, 0, time.UTC)
	server := Server{config: config.Config{
		Prometheus: config.ProviderConfig{Retention: 24 * time.Hour},
		Jaeger:     config.ProviderConfig{Retention: 0},
	}}
	tests := []struct {
		name  string
		start time.Time
		end   time.Time
		want  int
	}{
		{name: "live window", start: now.Add(-5 * time.Minute), end: now, want: 0},
		{name: "end beyond jaeger memory", start: now.Add(-30 * time.Minute), end: now.Add(-20 * time.Minute), want: 1},
		{name: "start before prometheus retention", start: now.Add(-25 * time.Hour), end: now.Add(-24 * time.Hour), want: 2},
		{name: "whole segment expired", start: now.Add(-30 * time.Hour), end: now.Add(-28 * time.Hour), want: 2},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			warnings := server.segmentCoverageWarnings(test.start, test.end, now)
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
	if warnings := configured.segmentCoverageWarnings(now.Add(-3*time.Hour), now.Add(-time.Hour), now); len(warnings) != 0 {
		t.Fatalf("within configured retentions warnings = %v, want 0", warnings)
	}
	if warnings := configured.segmentCoverageWarnings(now.Add(-3*time.Hour), now.Add(-3*time.Hour).Add(time.Minute), now); len(warnings) != 1 {
		t.Fatalf("end beyond jaeger retention warnings = %v, want 1", warnings)
	}
}

// segmentStoreStub 模拟持久化快照：start 前与 end 前各有一个快照。
type segmentStoreStub struct {
	store.Disabled
	available bool
	startAt   time.Time
	endAt     time.Time
}

func (stub *segmentStoreStub) Available() bool { return stub.available }

func (stub *segmentStoreStub) SnapshotAt(_ context.Context, at time.Time) (*model.StoredSnapshot, error) {
	if !stub.available {
		return nil, store.ErrUnavailable
	}
	if at.Before(stub.startAt) {
		return nil, nil
	}
	capturedAt := stub.startAt
	if at.After(stub.endAt) {
		capturedAt = stub.endAt
	}
	payload, err := json.Marshal(model.CurrentSnapshot{
		CapturedAt: capturedAt,
		Configuration: model.Configuration{
			AsOf: capturedAt, Availability: "available",
			Models: []model.PlatformResource{{}},
		},
		Traffic:   model.Traffic{AsOf: capturedAt, Tenants: []model.TenantTraffic{}},
		Workloads: model.Workloads{Nodes: []model.ClusterNode{}},
	})
	if err != nil {
		return nil, err
	}
	return &model.StoredSnapshot{
		ID:         "snapshot-" + capturedAt.Format("150405"),
		CapturedAt: capturedAt,
		Payload:    payload,
	}, nil
}

func TestHandleSegmentUnavailableWithoutSnapshots(t *testing.T) {
	now := time.Date(2026, time.August, 17, 12, 0, 0, 0, time.UTC)
	server := &Server{
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		cache:  &kubernetes.Cache{},
		store:  &segmentStoreStub{available: true, startAt: now.Add(-time.Hour), endAt: now.Add(-time.Minute)},
		config: config.Config{},
	}
	request := httptest.NewRequest(http.MethodGet,
		"/api/v1/segment?start="+now.Add(-3*time.Hour).Format(time.RFC3339)+"&end="+now.Add(-2*time.Hour).Format(time.RFC3339), nil)
	recorder := httptest.NewRecorder()
	server.handleSegment(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var envelope struct {
		Data  model.SegmentOverview `json:"data"`
		Error struct {
			Warnings []string `json:"warnings"`
		} `json:"meta"`
	}
	_ = json.Unmarshal(recorder.Body.Bytes(), &envelope)
	if envelope.Data.Availability != "unavailable" {
		t.Fatalf("expected unavailable, got %q", envelope.Data.Availability)
	}
	if envelope.Data.StartSnapshot != nil || envelope.Data.EndSnapshot != nil {
		t.Fatalf("expected no snapshots, got %+v / %+v", envelope.Data.StartSnapshot, envelope.Data.EndSnapshot)
	}
}

func TestHandleSegmentStoreDisabled(t *testing.T) {
	now := time.Date(2026, time.August, 17, 12, 0, 0, 0, time.UTC)
	server := &Server{
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		cache:  &kubernetes.Cache{},
		store:  &segmentStoreStub{available: false},
		config: config.Config{},
	}
	request := httptest.NewRequest(http.MethodGet,
		"/api/v1/segment?start="+now.Add(-time.Hour).Format(time.RFC3339)+"&end="+now.Format(time.RFC3339), nil)
	recorder := httptest.NewRecorder()
	server.handleSegment(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", recorder.Code, recorder.Body.String())
	}
	var envelope struct {
		Data model.SegmentOverview `json:"data"`
	}
	_ = json.Unmarshal(recorder.Body.Bytes(), &envelope)
	if envelope.Data.Availability != "unavailable" {
		t.Fatalf("expected unavailable, got %q", envelope.Data.Availability)
	}
}

func TestHandleSegmentInvalidWindow(t *testing.T) {
	server := &Server{
		logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		cache:  &kubernetes.Cache{},
		store:  &segmentStoreStub{available: true},
		config: config.Config{},
	}
	for _, query := range []string{
		"start=2026-08-17T07:00:00Z",
		"start=not-a-time&end=2026-08-17T08:00:00Z",
		"start=2026-08-17T08:00:00Z&end=2026-08-17T07:00:00Z",
	} {
		request := httptest.NewRequest(http.MethodGet, "/api/v1/segment?"+query, nil)
		recorder := httptest.NewRecorder()
		server.handleSegment(recorder, request)
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("query=%q expected 400, got %d: %s", query, recorder.Code, recorder.Body.String())
		}
	}
}
