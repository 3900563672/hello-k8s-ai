package clock

import (
	"sync"
	"time"

	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/model"
)

// RateSource 提供 Kubernetes 中保存的 Simulator 倍速与收敛状态。
type RateSource interface {
	SimulationRate() (
		desiredRate int,
		appliedRate int,
		synchronizedInstances int,
		totalInstances int,
		resourceVersion string,
		ready bool,
		found bool,
	)
}

// Clock 保持 Backend 逻辑时间与墙钟一致，同时读取独立的 Simulator 引擎倍速。
// 倍速不会改变历史查询、数据新鲜度或 Controller 冷却使用的时间。
type Clock struct {
	mu            sync.RWMutex
	actualAnchor  time.Time
	logicalAnchor time.Time
	now           func() time.Time
	rateSource    RateSource
}

func New(sources ...RateSource) *Clock {
	now := time.Now().UTC()
	result := &Clock{actualAnchor: now, logicalAnchor: now, now: time.Now}
	if len(sources) > 0 {
		result.rateSource = sources[0]
	}
	return result
}

func (clock *Clock) State() model.ClockState {
	clock.mu.RLock()
	defer clock.mu.RUnlock()

	now := clock.now().UTC()
	desiredRate := 1
	appliedRate := 1
	synchronizedInstances := 0
	totalInstances := 0
	resourceVersion := ""
	converged := false
	if clock.rateSource != nil {
		var found bool
		desiredRate, appliedRate, synchronizedInstances, totalInstances, resourceVersion, converged, found = clock.rateSource.SimulationRate()
		if !found {
			desiredRate = 1
			appliedRate = 1
			converged = false
		}
	}
	version := "actual-v2"
	if resourceVersion != "" {
		version = resourceVersion
	}
	return model.ClockState{
		ClockID:               "default",
		Mode:                  "actual-with-simulator-acceleration",
		State:                 "running",
		ServerTime:            now,
		ActualTime:            now,
		LogicalTime:           now,
		SimulationTime:        nil,
		ActualAnchor:          clock.actualAnchor,
		LogicalAnchor:         clock.logicalAnchor,
		OffsetMs:              0,
		Rate:                  float64(desiredRate),
		AppliedRate:           float64(appliedRate),
		ResourceVersion:       resourceVersion,
		Converged:             converged,
		SynchronizedInstances: synchronizedInstances,
		TotalInstances:        totalInstances,
		Version:               version,
		Authoritative:         true,
		MaxClientDriftMs:      2000,
		Capabilities: model.ClockCapabilities{
			CanSetRate:            clock.rateSource != nil,
			CanPause:              false,
			CanSeek:               false,
			SimulatorAcceleration: clock.rateSource != nil,
		},
	}
}
