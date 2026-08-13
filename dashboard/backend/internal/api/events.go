package api

import (
	"encoding/json"
	"sync"

	"github.com/3900563672/hello-k8s-ai/dashboard/backend/internal/model"
)

type StreamEvent struct {
	ID              string            `json:"id"`
	Type            string            `json:"type"`
	OccurredAt      string            `json:"occurredAt"`
	ResourceRef     model.ResourceRef `json:"resourceRef"`
	ResourceVersion string            `json:"resourceVersion,omitempty"`
	Payload         json.RawMessage   `json:"payload"`
}

type EventBus struct {
	mu          sync.RWMutex
	subscribers map[uint64]chan StreamEvent
	nextID      uint64
}

func NewEventBus() *EventBus {
	return &EventBus{subscribers: make(map[uint64]chan StreamEvent)}
}

func (bus *EventBus) Publish(change model.ResourceChange) {
	payload, _ := json.Marshal(map[string]any{
		"operation": change.Operation,
		"kind":      change.Ref.Kind,
		"name":      change.Ref.Name,
	})
	event := StreamEvent{
		ID:              change.EventID,
		Type:            "resource.changed",
		OccurredAt:      change.OccurredAt.UTC().Format("2006-01-02T15:04:05.000000000Z07:00"),
		ResourceRef:     change.Ref,
		ResourceVersion: change.ResourceVersion,
		Payload:         payload,
	}
	bus.mu.RLock()
	defer bus.mu.RUnlock()
	for _, subscriber := range bus.subscribers {
		select {
		case subscriber <- event:
		default:
			// 慢客户端丢失通知后，以 REST 重新同步的结果为准。
		}
	}
}

func (bus *EventBus) Subscribe() (<-chan StreamEvent, func()) {
	bus.mu.Lock()
	bus.nextID++
	id := bus.nextID
	channel := make(chan StreamEvent, 128)
	bus.subscribers[id] = channel
	bus.mu.Unlock()

	return channel, func() {
		bus.mu.Lock()
		if existing, ok := bus.subscribers[id]; ok {
			delete(bus.subscribers, id)
			close(existing)
		}
		bus.mu.Unlock()
	}
}
