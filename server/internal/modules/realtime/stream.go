package realtime

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/jojianya/sweetspot247-backend/internal/modules/pins"
	"github.com/redis/go-redis/v9"
)

// runStreamLoop runs the SSE event/heartbeat loop. Extracted for testing.
func runStreamLoop(ctx context.Context, w http.ResponseWriter, ch <-chan *redis.Message, interval time.Duration, bbox *[4]float64, category *int, removedCh <-chan *redis.Message) {
	heartbeat := time.NewTicker(interval)
	defer heartbeat.Stop()

	for {
		select {
		case msg, ok := <-ch:
			if !ok {
				return
			}
			var ev pins.Event
			if err := json.Unmarshal([]byte(msg.Payload), &ev); err != nil {
				continue
			}
			if !matches(ev, bbox, category) {
				continue
			}
			setWriteDeadline(w, sseWriteTimeout)
			if _, err := fmt.Fprintf(w, "event: pin\ndata: %s\n\n", msg.Payload); err != nil {
				return
			}
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
		case msg, ok := <-removedCh:
			if !ok {
				return
			}
			// Forward pin_removed events directly (they contain pin_id and location for filtering)
			var ev pins.PinRemoved
			if err := json.Unmarshal([]byte(msg.Payload), &ev); err != nil {
				continue
			}
			if !matchesRemoved(ev, bbox) {
				continue
			}
			setWriteDeadline(w, sseWriteTimeout)
			if _, err := fmt.Fprintf(w, "event: pin_removed\ndata: %s\n\n", msg.Payload); err != nil {
				return
			}
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
		case <-heartbeat.C:
			setWriteDeadline(w, sseWriteTimeout)
			if _, err := fmt.Fprint(w, ": heartbeat\n\n"); err != nil {
				return
			}
			if flusher, ok := w.(http.Flusher); ok {
				flusher.Flush()
			}
		case <-ctx.Done():
			return
		}
	}
}
