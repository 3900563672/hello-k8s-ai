package store

import (
	"testing"
	"time"
)

func TestTimelineItemForSnapshotIsReplayable(t *testing.T) {
	timestamp := time.Date(2026, time.August, 12, 12, 0, 0, 0, time.UTC)
	item := timelineItem("snapshot-1", timestamp, "capture", "Snapshot", "", "snapshot-1")
	if item.ID != "snapshot-1" || item.Trigger != "time" || item.Source != "postgresql/snapshot" {
		t.Fatalf("unexpected snapshot timeline item: %#v", item)
	}
	if item.Timestamp != timestamp || item.Impact["changes"] != 0 {
		t.Fatalf("snapshot timeline time or impact is wrong: %#v", item)
	}
}

func TestTimelineItemClassifiesScalingResource(t *testing.T) {
	item := timelineItem("event-1", time.Now(), "update", "Orchestrator", "", "tenant-a")
	if item.Domain != "scheduler" || item.Impact["tenants"] != 1 || item.Impact["models"] != 1 {
		t.Fatalf("unexpected Orchestrator timeline classification: %#v", item)
	}
}
