package store

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/model"
)

type recordingStore struct {
	Disabled
	records []model.ResourceChange
	fail    bool
}

func (fake *recordingStore) RecordResourceChange(_ context.Context, change model.ResourceChange) error {
	if fake.fail {
		return errors.New("database unavailable")
	}
	fake.records = append(fake.records, change)
	return nil
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestRecorderDropsWhenBufferFullAndRecordsGap(t *testing.T) {
	fake := &recordingStore{}
	recorder := NewRecorder(fake, 1, discardLogger())
	recorder.Publish(model.ResourceChange{EventID: "e1", Ref: model.ResourceRef{Kind: "Model", Name: "a"}})
	recorder.Publish(model.ResourceChange{EventID: "e2", Ref: model.ResourceRef{Kind: "Model", Name: "b"}})
	if recorder.Dropped() != 1 {
		t.Fatalf("expected 1 dropped event, got %d", recorder.Dropped())
	}

	recorder.recordGapIfNeeded(context.Background())
	if len(fake.records) != 1 || fake.records[0].Ref.Kind != "TimelineGap" {
		t.Fatalf("expected one TimelineGap record, got %#v", fake.records)
	}
	var payload map[string]uint64
	if err := json.Unmarshal(fake.records[0].Payload, &payload); err != nil {
		t.Fatalf("unmarshal gap payload: %v", err)
	}
	if payload["dropped"] != 1 || payload["writeFailures"] != 0 {
		t.Fatalf("unexpected gap payload: %#v", payload)
	}

	// 水位已前进：没有新丢失时不再重复写 gap 记录。
	recorder.recordGapIfNeeded(context.Background())
	if len(fake.records) != 1 {
		t.Fatalf("expected no duplicate gap record, got %d", len(fake.records))
	}
}

func TestRecorderGapWriteFailureDoesNotAdvanceWatermark(t *testing.T) {
	fake := &recordingStore{fail: true}
	recorder := NewRecorder(fake, 1, discardLogger())
	recorder.Publish(model.ResourceChange{EventID: "e1", Ref: model.ResourceRef{Kind: "Tenant", Name: "t"}})
	recorder.Publish(model.ResourceChange{EventID: "e2", Ref: model.ResourceRef{Kind: "Tenant", Name: "t"}})

	recorder.recordGapIfNeeded(context.Background())
	recorder.recordGapIfNeeded(context.Background())
	if len(fake.records) != 0 {
		t.Fatalf("gap records should not be stored while database is down, got %#v", fake.records)
	}

	fake.fail = false
	recorder.recordGapIfNeeded(context.Background())
	if len(fake.records) != 1 || fake.records[0].Ref.Kind != "TimelineGap" {
		t.Fatalf("expected gap record after database recovered, got %#v", fake.records)
	}
	var payload map[string]uint64
	if err := json.Unmarshal(fake.records[0].Payload, &payload); err != nil {
		t.Fatalf("unmarshal gap payload: %v", err)
	}
	if payload["dropped"] != 1 {
		t.Fatalf("expected dropped delta 1 after recovery, got %#v", payload)
	}
}

func TestTimelineItemMarksGapAsAttention(t *testing.T) {
	item := timelineItem("gap-1", time.Now(), "gap", "TimelineGap", "", "recorder")
	if item.Domain != "runtime" || item.Severity != "attention" {
		t.Fatalf("unexpected TimelineGap classification: %#v", item)
	}
	if item.Weight < 6 || item.Source != "postgresql/gap" {
		t.Fatalf("unexpected TimelineGap weight or source: %#v", item)
	}
}
