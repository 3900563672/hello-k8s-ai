package store

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/model"
)

var ErrUnavailable = errors.New("persistent store is unavailable")

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

type Store interface {
	Migrate(context.Context) error
	Health(context.Context) error
	RecordResourceChange(context.Context, model.ResourceChange) error
	SaveSnapshot(context.Context, SnapshotRecord) error
	SnapshotAt(context.Context, time.Time) (*model.StoredSnapshot, error)
	ListTimeline(context.Context, int, *time.Time) ([]model.TimelineItem, error)
	RecordAudit(context.Context, model.AuditRecord) error
	ReserveIdempotency(context.Context, string, string, time.Time) (*IdempotencyRecord, bool, error)
	CompleteIdempotency(context.Context, string, string, int, json.RawMessage) error
	ReleaseIdempotency(context.Context, string, string) error
	IndexTraces(context.Context, []model.TraceSummary) error
	Prune(context.Context, time.Time) error
	Close()
	Available() bool
}

type Disabled struct{}

func (Disabled) Migrate(context.Context) error { return nil }
func (Disabled) Health(context.Context) error  { return ErrUnavailable }
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
func (Disabled) Prune(context.Context, time.Time) error                   { return ErrUnavailable }
func (Disabled) Close()                                                   {}
func (Disabled) Available() bool                                          { return false }
