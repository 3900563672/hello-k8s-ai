package clock

import (
	"testing"
	"time"
)

type fakeRateSource struct {
	desired      int
	applied      int
	synchronized int
	total        int
	version      string
	ready        bool
	found        bool
}

func (source fakeRateSource) SimulationRate() (int, int, int, int, string, bool, bool) {
	return source.desired,
		source.applied,
		source.synchronized,
		source.total,
		source.version,
		source.ready,
		source.found
}

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

func TestStateExposesSimulatorRateWithoutChangingLogicalTime(t *testing.T) {
	fixed := time.Date(2026, time.August, 14, 12, 30, 0, 0, time.UTC)
	clock := New(fakeRateSource{
		desired:      10,
		applied:      5,
		synchronized: 2,
		total:        3,
		version:      "42",
		ready:        false,
		found:        true,
	})
	clock.now = func() time.Time { return fixed }

	state := clock.State()
	if state.ServerTime != fixed || state.LogicalTime != fixed || state.ActualTime != fixed {
		t.Fatalf("Simulator 倍速改变了 Backend 墙钟: %#v", state)
	}
	if state.Rate != 10 || state.AppliedRate != 5 || state.ResourceVersion != "42" {
		t.Fatalf("unexpected rate state: %#v", state)
	}
	if state.Converged || state.SynchronizedInstances != 2 || state.TotalInstances != 3 {
		t.Fatalf("unexpected convergence state: %#v", state)
	}
	if !state.Capabilities.CanSetRate || !state.Capabilities.SimulatorAcceleration {
		t.Fatalf("rate source capabilities are missing: %#v", state.Capabilities)
	}
	if state.Capabilities.CanPause || state.Capabilities.CanSeek {
		t.Fatalf("unsupported time controls were advertised: %#v", state.Capabilities)
	}
}
