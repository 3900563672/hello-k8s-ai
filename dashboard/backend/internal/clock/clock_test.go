package clock

import (
	"testing"
	"time"
)

func TestStateReportsActualAuthoritativeClock(t *testing.T) {
	fixed := time.Date(2026, time.August, 12, 12, 30, 0, 123, time.UTC)
	clock := New()
	clock.actualAnchor = fixed.Add(-time.Minute)
	clock.logicalAnchor = fixed.Add(-time.Minute)
	clock.now = func() time.Time { return fixed }

	state := clock.State()
	if state.ServerTime != fixed || state.LogicalTime != fixed || state.ActualTime != fixed {
		t.Fatalf("clock times do not use the authoritative source: %#v", state)
	}
	if !state.Authoritative || state.Rate != 1 || state.State != "running" {
		t.Fatalf("unexpected authoritative clock state: %#v", state)
	}
	if state.Capabilities.SimulatorAcceleration || state.Capabilities.CanSetRate || state.Capabilities.CanPause {
		t.Fatalf("actual clock must not claim simulator controls: %#v", state.Capabilities)
	}
}
