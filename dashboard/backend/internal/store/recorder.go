package store

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/model"
)

// Recorder 将数据库 I/O 移出 informer 回调 goroutine。缓冲区满时记录日志和丢弃计数，
// 避免无限阻塞 Kubernetes Watch。
type Recorder struct {
	store   Store
	logger  *slog.Logger
	changes chan model.ResourceChange
	dropped atomic.Uint64
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
				recorder.logger.Error(
					"Could not persist Kubernetes resource change",
					"kind", change.Ref.Kind,
					"name", change.Ref.Name,
					"operation", change.Operation,
					"error", err,
				)
			}
		}
	}
}

func (recorder *Recorder) Dropped() uint64 {
	return recorder.dropped.Load()
}
