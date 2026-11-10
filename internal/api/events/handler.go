package events

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	domainevents "git.nexacloud.dev/MrSnakeDoc/tuby/internal/events"
)

const heartbeatInterval = 25 * time.Second

type handler struct{ updates *domainevents.Hub }

func (h *handler) stream(writer http.ResponseWriter, request *http.Request) {
	if h.updates == nil {
		http.Error(writer, "live updates unavailable", http.StatusServiceUnavailable)
		return
	}
	flusher, ok := writer.(http.Flusher)
	if !ok {
		http.Error(writer, "streaming unavailable", http.StatusInternalServerError)
		return
	}
	writer.Header().Set("Content-Type", "text/event-stream")
	writer.Header().Set("Cache-Control", "no-cache, no-store")
	writer.Header().Set("Connection", "keep-alive")
	writer.Header().Set("X-Accel-Buffering", "no")
	_ = http.NewResponseController(writer).SetWriteDeadline(time.Time{})

	updates, unsubscribe := h.updates.Subscribe()
	defer unsubscribe()
	if err := writeUpdate(writer, h.updates.SyncUpdate()); err != nil {
		return
	}
	flusher.Flush()
	heartbeat := time.NewTicker(heartbeatInterval)
	defer heartbeat.Stop()
	for {
		select {
		case <-request.Context().Done():
			return
		case update, open := <-updates:
			if !open || writeUpdate(writer, update) != nil {
				return
			}
			flusher.Flush()
		case <-heartbeat.C:
			if _, err := fmt.Fprint(writer, ": heartbeat\n\n"); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func writeUpdate(writer http.ResponseWriter, update domainevents.Update) error {
	payload, err := json.Marshal(update)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(writer, "id: %d\nevent: update\ndata: %s\n\n", update.Revision, payload)
	return err
}
