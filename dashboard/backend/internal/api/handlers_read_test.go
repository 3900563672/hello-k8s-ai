package api

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"
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
