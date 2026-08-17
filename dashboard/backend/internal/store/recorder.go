package store

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/model"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// 普通 Counter 而非 CounterVec：Vec 在没有 label 实例时不会出现在 /metrics，
	// 会导致静默期看不到 0 值。丢掉的 kind 仍会写进日志与 TimelineGap payload。
	eventsDropped = promauto.NewCounter(prometheus.CounterOpts{
		Name: "hello_k8s_ai_dashboard_events_dropped_total",
		Help: "Dashboard history events dropped because the recorder buffer was full.",
	})
	eventsWriteFailures = promauto.NewCounter(prometheus.CounterOpts{
		Name: "hello_k8s_ai_dashboard_events_write_failures_total",
		Help: "Dashboard history events that failed to persist to PostgreSQL.",
	})
)

// Recorder 将数据库 I/O 移出 informer 回调 goroutine。缓冲区满时记录日志、指标
// 和时间线 gap 记录，避免无限阻塞 Kubernetes Watch。
type Recorder struct {
	store         Store
	logger        *slog.Logger
	changes       chan model.ResourceChange
	dropped       atomic.Uint64
	writeFailures atomic.Uint64
	// 以下水位只由 Run goroutine 读写，用于把进程内计数增量沉淀为时间线 gap 记录。
	recordedDropped       uint64
	recordedWriteFailures uint64
}

func NewRecorder(database Store, buffer int, logger *slog.Logger) *Recorder {
	return &Recorder{
		store:   database,
		logger:  logger,
		changes: make(chan model.ResourceChange, buffer),
	}
}

func (recorder *Recorder) Publish(change model.ResourceChange) {
	select {
	case recorder.changes <- change:
	default:
		dropped := recorder.dropped.Add(1)
		eventsDropped.Inc()
		recorder.logger.Error(
			"Dashboard history event buffer is full",
			"dropped", dropped,
			"kind", change.Ref.Kind,
			"name", change.Ref.Name,
		)
	}
}

func (recorder *Recorder) Run(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case change := <-recorder.changes:
			writeContext, cancel := context.WithTimeout(ctx, 5*time.Second)
			err := recorder.store.RecordResourceChange(writeContext, change)
			cancel()
			if err != nil {
				recorder.writeFailures.Add(1)
				eventsWriteFailures.Inc()
				recorder.logger.Error(
					"Could not persist Kubernetes resource change",
					"kind", change.Ref.Kind,
					"name", change.Ref.Name,
					"operation", change.Operation,
					"error", err,
				)
			}
			recorder.recordGapIfNeeded(ctx)
		}
	}
}

// recordGapIfNeeded 在丢弃或写失败计数相对上次沉淀发生变化时，向 resource_events
// 写入一条 TimelineGap 记录，让历史回放时间线明确标注缺口。写入失败不阻塞主循环，
// 水位不前进，下次循环继续尝试，直到数据库恢复。
func (recorder *Recorder) recordGapIfNeeded(ctx context.Context) {
	dropped := recorder.dropped.Load()
	writeFailures := recorder.writeFailures.Load()
	if dropped == recorder.recordedDropped && writeFailures == recorder.recordedWriteFailures {
		return
	}
	payload, _ := json.Marshal(map[string]uint64{
		"dropped":       dropped - recorder.recordedDropped,
		"writeFailures": writeFailures - recorder.recordedWriteFailures,
	})
	gap := model.ResourceChange{
		EventID:    newID("gap"),
		OccurredAt: time.Now().UTC(),
		Operation:  "gap",
		Ref:        model.ResourceRef{Kind: "TimelineGap", Name: "recorder"},
		Payload:    payload,
	}
	gapContext, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := recorder.store.RecordResourceChange(gapContext, gap); err != nil {
		recorder.logger.Error("Could not persist history timeline gap record", "error", err)
		return
	}
	recorder.recordedDropped = dropped
	recorder.recordedWriteFailures = writeFailures
}

func (recorder *Recorder) Dropped() uint64 {
	return recorder.dropped.Load()
}
