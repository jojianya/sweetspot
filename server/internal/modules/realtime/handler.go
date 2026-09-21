package realtime

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jojianya/sweetspot247-backend/internal/http/response"
	"github.com/jojianya/sweetspot247-backend/internal/modules/pins"
)

type Handler struct {
	broker *Broker
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
		parts := strings.Split(raw, ",")
		if len(parts) != 4 {
			response.BadRequest(c, "bbox must be 4 comma-separated floats (minLat,minLng,maxLat,maxLng)")
			return
		}
		var b [4]float64
		for i, p := range parts {
			v, err := strconv.ParseFloat(strings.TrimSpace(p), 64)
			if err != nil {
				response.BadRequest(c, "bbox must be 4 comma-separated floats")
				return
			}
			b[i] = v
		}
		if b[0] > b[2] || b[1] > b[3] {
			response.BadRequest(c, "bbox min must not exceed max (minLat,minLng,maxLat,maxLng)")
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
