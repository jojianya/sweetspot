package realtime

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	httpx "github.com/jojianya/sweetspot247-backend/internal/http/params"
	"github.com/jojianya/sweetspot247-backend/internal/http/response"
	"github.com/jojianya/sweetspot247-backend/internal/modules/pins"
)

// sseWriteTimeout bounds a single write to a stream client.
//
// The stream is intentionally unbounded in total duration, so the server-wide
// WriteTimeout is left unset and each write carries its own deadline instead.
// That way a peer that stops reading is detected and its goroutine released,
// while an active-but-quiet stream is never cut off.
const sseWriteTimeout = 10 * time.Second

type Handler struct {
	broker *Broker
}

// setWriteDeadline arms a per-write deadline on the underlying
// http.ResponseWriter. gin.ResponseWriter implements http.ResponseWriter's
// Unwrap convention, so ResponseController can reach the real writer.
func setWriteDeadline(w http.ResponseWriter, timeout time.Duration) {
	rc := http.NewResponseController(w)
	// Unsupported is fine: it only means the ResponseWriter in this chain does
	// not expose a deadline, in which case the stream still works.
	_ = rc.SetWriteDeadline(time.Now().Add(timeout))
}

func NewHandler(broker *Broker) *Handler {
	return &Handler{broker: broker}
}

// Stream is a Server-Sent Events endpoint streaming newly created pins.
//
//	GET /events?bbox=minLat,minLng,maxLat,maxLng&category=3
//
// Events are `event: pin` messages whose data is a pins.Event JSON payload.
// The connection lives until the client disconnects; EventSource clients
// reconnect automatically.
func (h *Handler) Stream(c *gin.Context) {
	var bbox *[4]float64
	if raw := strings.TrimSpace(c.Query("bbox")); raw != "" {
		b, ok := httpx.ParseBbox(c)
		if !ok {
			return
		}
		bbox = &b
	}

	var category *int
	if raw := strings.TrimSpace(c.Query("category")); raw != "" {
		id, err := strconv.Atoi(raw)
		if err != nil {
			response.BadRequest(c, "category must be an integer")
			return
		}
		category = &id
	}

	ctx := c.Request.Context()
	sub := h.broker.Subscribe(ctx)
	defer sub.Close()

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	// Arm a deadline before the first flush too, so a client that connects and
	// never reads cannot hold the handler open.
	setWriteDeadline(c.Writer, sseWriteTimeout)
	c.Writer.WriteHeader(http.StatusOK)
	c.Writer.Flush()

	ch := sub.Channel()
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
			// Re-arm per write: the previous deadline has likely already
			// passed, and each event gets a fresh window.
			setWriteDeadline(c.Writer, sseWriteTimeout)
			if _, err := fmt.Fprintf(c.Writer, "event: pin\ndata: %s\n\n", msg.Payload); err != nil {
				return
			}
			c.Writer.Flush()
		case <-ctx.Done():
			return
		}
	}
}

// matches applies the optional bbox/category filters to an event.
func matches(ev pins.Event, bbox *[4]float64, category *int) bool {
	if category != nil && ev.CategoryID != *category {
		return false
	}
	if bbox == nil {
		return true
	}

	var lng, lat float64
	if _, err := fmt.Sscanf(ev.Location, "POINT(%f %f)", &lng, &lat); err != nil {
		// Malformed location: only pass through when no bbox filter is set.
		return false
	}
	return lat >= bbox[0] && lat <= bbox[2] && lng >= bbox[1] && lng <= bbox[3]
}
