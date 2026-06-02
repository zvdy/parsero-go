package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// handleEvents streams scan progress as Server-Sent Events. Progress is read
// from Redis, so the stream works no matter which instance ran the job. It emits
// `progress` events until the scan reaches a terminal state, then a final
// `done` or `failed` event and closes.
func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	sc, err := s.loadOwnedScan(r)
	if err != nil {
		s.writeScanLoadErr(w, err)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeErr(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	emit := func(event string, payload any) {
		b, _ := json.Marshal(payload)
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, b)
		flusher.Flush()
	}

	ctx := r.Context()
	scanID := sc.ID
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cur, err := s.store.GetScan(ctx, scanID)
			if err != nil {
				emit("failed", map[string]string{"error": "scan unavailable"})
				return
			}
			switch cur.Status {
			case "done":
				emit("done", toScanResponse(cur, false))
				return
			case "failed":
				emit("failed", map[string]string{"error": cur.ErrorMessage})
				return
			default:
				done, total, _ := s.cache.GetProgress(ctx, scanID)
				emit("progress", map[string]any{
					"status": cur.Status, "done": done, "total": total,
				})
			}
		}
	}
}
