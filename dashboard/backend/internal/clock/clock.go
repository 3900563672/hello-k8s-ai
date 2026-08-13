package clock

import (
	"sync"
	"time"

	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/model"
)

// Clock 是 Backend 的墙钟时间源。当前 Controller 与 Simulator 都使用真实时间，
// 因此不会声明模拟加速能力。
type Clock struct {
	mu            sync.RWMutex
	actualAnchor  time.Time
	logicalAnchor time.Time
	now           func() time.Time
}

func New() *Clock {
	now := time.Now().UTC()
	return &Clock{actualAnchor: now, logicalAnchor: now, now: time.Now}
}

func (clock *Clock) State() model.ClockState {
	clock.mu.RLock()
	defer clock.mu.RUnlock()

	now := clock.now().UTC()
	return model.ClockState{
		ClockID:          "default",
		Mode:             "actual",
		State:            "running",
		ServerTime:       now,
		ActualTime:       now,
		LogicalTime:      now,
		SimulationTime:   nil,
		ActualAnchor:     clock.actualAnchor,
		LogicalAnchor:    clock.logicalAnchor,
		OffsetMs:         0,
		Rate:             1,
		Version:          "actual-v1",
		Authoritative:    true,
		MaxClientDriftMs: 2000,
		Capabilities: model.ClockCapabilities{
			CanSetRate:            false,
			CanPause:              false,
			CanSeek:               false,
			SimulatorAcceleration: false,
		},
	}
}
