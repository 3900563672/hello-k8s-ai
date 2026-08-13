package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

func (server *Server) handleStream(writer http.ResponseWriter, request *http.Request) {
	flusher, ok := writer.(http.Flusher)
	if !ok {
		writeProblem(writer, request, http.StatusInternalServerError, "STREAM_UNSUPPORTED", "Streaming is not supported by this HTTP server.", false, nil)
		return
	}
	writer.Header().Set("Content-Type", "text/event-stream")
	writer.Header().Set("Cache-Control", "no-cache, no-transform")
	writer.Header().Set("Connection", "keep-alive")
	writer.Header().Set("X-Accel-Buffering", "no")
	_, _ = fmt.Fprint(writer, "retry: 3000\n\n")
	flusher.Flush()

	if request.Header.Get("Last-Event-ID") != "" {
		payload, _ := json.Marshal(map[string]string{"reason": "event-buffer-is-not-a-history-store"})
		_, _ = fmt.Fprintf(writer, "event: resync-required\ndata: %s\n\n", payload)
		flusher.Flush()
	}

	events, unsubscribe := server.events.Subscribe()
	defer unsubscribe()
	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case <-request.Context().Done():
			return
		case <-heartbeat.C:
			_, _ = fmt.Fprintf(writer, ": heartbeat %s\n\n", time.Now().UTC().Format(time.RFC3339Nano))
			flusher.Flush()
		case event, open := <-events:
			if !open {
				return
			}
			payload, err := json.Marshal(event)
			if err != nil {
				continue
			}
			_, _ = fmt.Fprintf(writer, "id: %s\nevent: %s\ndata: %s\n\n", event.ID, event.Type, payload)
			flusher.Flush()
		}
	}
}
