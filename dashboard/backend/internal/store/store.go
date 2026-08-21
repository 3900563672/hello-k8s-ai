package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/model"
)

var ErrUnavailable = errors.New("persistent store is unavailable")

// 切面（Segment）相关记录类型复用 model 包定义，store 层只做持久化。
type SegmentRecord = model.SegmentRecord
type SegmentEvent = model.SegmentEvent
type MetricBucket = model.MetricBucket

type SnapshotRecord struct {
	ID             string
	CapturedAt     time.Time
	LogicalTime    time.Time
	SourceVersions map[string]string
	Payload        json.RawMessage
}

type IdempotencyRecord struct {
	Key         string
	RequestHash string
	State       string
	StatusCode  int
	Response    json.RawMessage
	CreatedAt   time.Time
	ExpiresAt   time.Time
}

// StoreStatus 描述持久化存储的健康与数据量，用于启动日志与健康接口。
type StoreStatus struct {
	MigrationsApplied int
	ResourceEvents    int
	ResourceSnapshots int
	ResourceStates    int
}

// ResourceStateRecord 是"某个资源的最新聚合状态"，由快照循环周期性 upsert。
type ResourceStateRecord struct {
	Kind            string          `json:"kind"`
	Namespace       string          `json:"namespace"`
	Name            string          `json:"name"`
	UID             string          `json:"uid,omitempty"`
	ResourceVersion string          `json:"resourceVersion,omitempty"`
	Generation      int64           `json:"generation,omitempty"`
	CapturedAt      time.Time       `json:"capturedAt"`
	Payload         json.RawMessage `json:"payload"`
}

type Store interface {
	Migrate(context.Context) error
	Health(context.Context) error
	Status(context.Context) (StoreStatus, error)
	RecordResourceChange(context.Context, model.ResourceChange) error
	SaveSnapshot(context.Context, SnapshotRecord) error
	SnapshotAt(context.Context, time.Time) (*model.StoredSnapshot, error)
	ListTimeline(context.Context, int, *time.Time) ([]model.TimelineItem, error)
	RecordAudit(context.Context, model.AuditRecord) error
	ReserveIdempotency(context.Context, string, string, time.Time) (*IdempotencyRecord, bool, error)
	CompleteIdempotency(context.Context, string, string, int, json.RawMessage) error
	ReleaseIdempotency(context.Context, string, string) error
	IndexTraces(context.Context, []model.TraceSummary) error
	UpsertResourceStates(context.Context, []ResourceStateRecord) error
	PruneResourceStates(context.Context, []ResourceStateRecord) error
	ListResourceStates(context.Context, string, string, int) ([]ResourceStateRecord, error)
	ListResourceEvents(context.Context, time.Time, int) ([]model.ResourceChange, error)
	CreateSegment(context.Context, SegmentRecord) error
	UpdateSegmentLifecycle(context.Context, string, string, string, json.RawMessage, json.RawMessage) error
	ListSegments(context.Context, int, string) ([]SegmentRecord, error)
	GetSegment(context.Context, string) (*SegmentRecord, error)
	RecordSegmentEvent(context.Context, SegmentEvent) error
	AppendSegmentMetrics(context.Context, string, []MetricBucket) error
	LinkSegmentTraces(context.Context, string, []string) error
	ListSegmentEvents(context.Context, string, int) ([]SegmentEvent, error)
	ListSegmentMetrics(context.Context, string, int) ([]MetricBucket, error)
	ListSegmentTraces(context.Context, string) ([]model.TraceSummary, error)
	CreateAIOpsAnalysis(context.Context, model.AIOpsAnalysis) error
	ClaimAIOpsAnalysis(context.Context, string) (bool, error)
	RequeueStaleAIOpsAnalyses(context.Context, time.Time) (int, error)
	UpdateAIOpsAnalysisProgress(context.Context, string, string, int, int, string) error
	CompleteAIOpsAnalysis(context.Context, string, json.RawMessage, json.RawMessage) error
	FailAIOpsAnalysis(context.Context, string, string) error
	GetAIOpsAnalysis(context.Context, string) (*model.AIOpsAnalysis, error)
	GetAIOpsAnalysisBySegment(context.Context, string) (*model.AIOpsAnalysis, error)
	ListAIOpsAnalyses(context.Context, int, string) ([]model.AIOpsAnalysis, error)
	UpsertAIOpsEntitySummaries(context.Context, string, []model.AIOpsEntitySummary) error
	ListAIOpsEntitySummaries(context.Context, string) ([]model.AIOpsEntitySummary, error)
	CreateAIOpsCommand(context.Context, model.AIOpsCommand) error
	GetAIOpsCommand(context.Context, string) (*model.AIOpsCommand, error)
	UpdateAIOpsCommand(context.Context, string, string, json.RawMessage, string) error
	UpsertAIOpsWindowSummary(context.Context, model.AIOpsWindowSummary) error
	ListAIOpsWindowSummaries(context.Context, string, int) ([]model.AIOpsWindowSummary, error)
	ListAIOpsAnalysesInWindow(context.Context, time.Time, time.Time) ([]model.AIOpsAnalysis, error)
	CreateAIOpsAlert(context.Context, model.AIOpsAlert) error
	ListAIOpsAlerts(context.Context, int) ([]model.AIOpsAlert, error)
	CreateAIOpsAuditLog(context.Context, model.AIOpsAuditLog) error
	Prune(context.Context, time.Time) error
	Close()
	Available() bool
}

type Disabled struct{}

func (Disabled) Migrate(context.Context) error { return nil }
func (Disabled) Health(context.Context) error  { return ErrUnavailable }
func (Disabled) Status(context.Context) (StoreStatus, error) {
	return StoreStatus{}, ErrUnavailable
}
func (Disabled) RecordResourceChange(context.Context, model.ResourceChange) error {
	return ErrUnavailable
}
func (Disabled) SaveSnapshot(context.Context, SnapshotRecord) error { return ErrUnavailable }
func (Disabled) SnapshotAt(context.Context, time.Time) (*model.StoredSnapshot, error) {
	return nil, ErrUnavailable
}
func (Disabled) ListTimeline(context.Context, int, *time.Time) ([]model.TimelineItem, error) {
	return nil, ErrUnavailable
}
func (Disabled) RecordAudit(context.Context, model.AuditRecord) error { return ErrUnavailable }
func (Disabled) ReserveIdempotency(context.Context, string, string, time.Time) (*IdempotencyRecord, bool, error) {
	return nil, false, ErrUnavailable
}
func (Disabled) CompleteIdempotency(context.Context, string, string, int, json.RawMessage) error {
	return ErrUnavailable
}
func (Disabled) ReleaseIdempotency(context.Context, string, string) error { return ErrUnavailable }
func (Disabled) IndexTraces(context.Context, []model.TraceSummary) error  { return ErrUnavailable }
func (Disabled) UpsertResourceStates(context.Context, []ResourceStateRecord) error {
	return ErrUnavailable
}
func (Disabled) PruneResourceStates(context.Context, []ResourceStateRecord) error {
	return ErrUnavailable
}
func (Disabled) ListResourceStates(context.Context, string, string, int) ([]ResourceStateRecord, error) {
	return nil, ErrUnavailable
}
func (Disabled) ListResourceEvents(context.Context, time.Time, int) ([]model.ResourceChange, error) {
	return nil, ErrUnavailable
}
func (Disabled) CreateSegment(context.Context, SegmentRecord) error { return ErrUnavailable }
func (Disabled) UpdateSegmentLifecycle(context.Context, string, string, string, json.RawMessage, json.RawMessage) error {
	return ErrUnavailable
}
func (Disabled) ListSegments(context.Context, int, string) ([]SegmentRecord, error) {
	return nil, ErrUnavailable
}
func (Disabled) GetSegment(context.Context, string) (*SegmentRecord, error) {
	return nil, ErrUnavailable
}
func (Disabled) RecordSegmentEvent(context.Context, SegmentEvent) error { return ErrUnavailable }
func (Disabled) AppendSegmentMetrics(context.Context, string, []MetricBucket) error {
	return ErrUnavailable
}
func (Disabled) LinkSegmentTraces(context.Context, string, []string) error { return ErrUnavailable }
func (Disabled) ListSegmentEvents(context.Context, string, int) ([]SegmentEvent, error) {
	return nil, ErrUnavailable
}
func (Disabled) ListSegmentMetrics(context.Context, string, int) ([]MetricBucket, error) {
	return nil, ErrUnavailable
}
func (Disabled) ListSegmentTraces(context.Context, string) ([]model.TraceSummary, error) {
	return nil, ErrUnavailable
}
func (Disabled) CreateAIOpsAnalysis(context.Context, model.AIOpsAnalysis) error {
	return ErrUnavailable
}
func (Disabled) ClaimAIOpsAnalysis(context.Context, string) (bool, error) {
	return false, ErrUnavailable
}
func (Disabled) RequeueStaleAIOpsAnalyses(context.Context, time.Time) (int, error) {
	return 0, ErrUnavailable
}
func (Disabled) UpdateAIOpsAnalysisProgress(context.Context, string, string, int, int, string) error {
	return ErrUnavailable
}
func (Disabled) CompleteAIOpsAnalysis(context.Context, string, json.RawMessage, json.RawMessage) error {
	return ErrUnavailable
}
func (Disabled) FailAIOpsAnalysis(context.Context, string, string) error { return ErrUnavailable }
func (Disabled) GetAIOpsAnalysis(context.Context, string) (*model.AIOpsAnalysis, error) {
	return nil, ErrUnavailable
}
func (Disabled) GetAIOpsAnalysisBySegment(context.Context, string) (*model.AIOpsAnalysis, error) {
	return nil, ErrUnavailable
}
func (Disabled) ListAIOpsAnalyses(context.Context, int, string) ([]model.AIOpsAnalysis, error) {
	return nil, ErrUnavailable
}
func (Disabled) UpsertAIOpsEntitySummaries(context.Context, string, []model.AIOpsEntitySummary) error {
	return ErrUnavailable
}
func (Disabled) ListAIOpsEntitySummaries(context.Context, string) ([]model.AIOpsEntitySummary, error) {
	return nil, ErrUnavailable
}
func (Disabled) CreateAIOpsCommand(context.Context, model.AIOpsCommand) error {
	return ErrUnavailable
}
func (Disabled) GetAIOpsCommand(context.Context, string) (*model.AIOpsCommand, error) {
	return nil, ErrUnavailable
}
func (Disabled) UpdateAIOpsCommand(context.Context, string, string, json.RawMessage, string) error {
	return ErrUnavailable
}
func (Disabled) UpsertAIOpsWindowSummary(context.Context, model.AIOpsWindowSummary) error {
	return ErrUnavailable
}
func (Disabled) ListAIOpsWindowSummaries(context.Context, string, int) ([]model.AIOpsWindowSummary, error) {
	return nil, ErrUnavailable
}
func (Disabled) ListAIOpsAnalysesInWindow(context.Context, time.Time, time.Time) ([]model.AIOpsAnalysis, error) {
	return nil, ErrUnavailable
}
func (Disabled) CreateAIOpsAlert(context.Context, model.AIOpsAlert) error {
	return ErrUnavailable
}
func (Disabled) CreateAIOpsAuditLog(context.Context, model.AIOpsAuditLog) error {
	return ErrUnavailable
}
func (Disabled) ListAIOpsAlerts(context.Context, int) ([]model.AIOpsAlert, error) {
	return nil, ErrUnavailable
}
func (Disabled) Prune(context.Context, time.Time) error { return ErrUnavailable }
func (Disabled) Close()                                 {}
func (Disabled) Available() bool                        { return false }
