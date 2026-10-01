package realtime

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	httpx "github.com/jojianya/sweetspot247-backend/internal/http/params"
	"github.com/jojianya/sweetspot247-backend/internal/http/response"
	"github.com/jojianya/sweetspot247-backend/internal/modules/pins"
	"github.com/redis/go-redis/v9"
)

// validateBbox checks that a parsed bbox [minLat, minLng, maxLat, maxLng] has
// valid coordinate ranges and non-zero area. Returns nil if valid.
func validateBbox(b [4]float64) error {
	if b[0] < -90 || b[0] > 90 || b[2] < -90 || b[2] > 90 {
		return fmt.Errorf("latitudes must be between -90 and 90")
	}
	if b[1] < -180 || b[1] > 180 || b[3] < -180 || b[3] > 180 {
		return fmt.Errorf("longitudes must be between -180 and 180")
	}
	if b[0] >= b[2] || b[1] >= b[3] {
		return fmt.Errorf("bbox min must be less than max (minLat < maxLat and minLng < maxLng)")
	}
	return nil
}

// sseTracker tracks active SSE connections so graceful shutdown can notify
// them to close rather than waiting for the full shutdown timeout.
type sseTracker struct {
	mu    sync.Mutex
	conns map[context.Context]context.CancelFunc
}

var tracker = &sseTracker{conns: make(map[context.Context]context.CancelFunc)}

// track registers an SSE connection for graceful shutdown notification.
func track(ctx context.Context, cancel context.CancelFunc) {
	tracker.mu.Lock()
	defer tracker.mu.Unlock()
	tracker.conns[ctx] = cancel
}

// untrack removes an SSE connection from tracking.
func untrack(ctx context.Context) {
	tracker.mu.Lock()
	defer tracker.mu.Unlock()
	delete(tracker.conns, ctx)
}

// CloseAllSSE signals all active SSE connections to close. Returns the
// number of connections that were closed.
func CloseAllSSE() int {
	tracker.mu.Lock()
	defer tracker.mu.Unlock()
	count := len(tracker.conns)
	for _, cancel := range tracker.conns {
		cancel()
	}
	clear(tracker.conns)
	return count
}

// sseWriteTimeout bounds a single write to a stream client.
//
// The stream is intentionally unbounded in total duration, so the server-wide
// WriteTimeout is left unset and each write carries its own deadline instead.
// That way a peer that stops reading is detected and its goroutine released,
// while an active-but-quiet stream is never cut off.
const sseWriteTimeout = 10 * time.Second

// sseHeartbeatInterval is the interval between SSE heartbeat comments.
const sseHeartbeatInterval = 20 * time.Second

// ConnectionLimiter limits concurrent SSE connections with a global cap.
// A nil limiter allows unlimited connections.
type ConnectionLimiter struct {
	mu     sync.Mutex
	max    int
	active int
}

func NewConnectionLimiter(max int) *ConnectionLimiter {
	if max <= 0 {
		return nil
	}
	return &ConnectionLimiter{max: max}
}

// TryAcquire attempts to acquire a connection slot. Returns true if acquired,
// false if at capacity. Must be followed by Release() on success.
func (l *ConnectionLimiter) TryAcquire() bool {
	if l == nil {
		return true
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.active >= l.max {
		return false
	}
	l.active++
	return true
}

// Release releases a previously acquired connection slot.
func (l *ConnectionLimiter) Release() {
	if l == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.active > 0 {
		l.active--
	}
}

type Handler struct {
	broker            *Broker
	limiter           *ConnectionLimiter
	heartbeatInterval time.Duration
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

func NewHandler(broker *Broker, maxConns int) *Handler {
	return &Handler{
		broker:            broker,
		limiter:           NewConnectionLimiter(maxConns),
		heartbeatInterval: sseHeartbeatInterval,
	}
}

// Stream is a Server-Sent Events endpoint streaming newly created pins.
//
//	GET /events?bbox=minLat,minLng,maxLat,maxLng&category=3
//
// Events are `event: pin` messages whose data is a pins.Event JSON payload.
// The connection lives until the client disconnects; EventSource clients
// reconnect automatically.
func (h *Handler) Stream(c *gin.Context) {
	// Global connection cap.
	if !h.limiter.TryAcquire() {
		c.Header("Retry-After", "5")
		response.Error(c, http.StatusServiceUnavailable, "too many SSE connections, try again later")
		return
	}

	// Track this connection so graceful shutdown can cancel it instead of
	// waiting for the full shutdown timeout.
	ctx, cancel := context.WithCancel(c.Request.Context())
	defer cancel()
	track(ctx, cancel)
	defer untrack(ctx)

	// Release connection slot on exit.
	defer h.limiter.Release()

	var bbox *[4]float64
	if raw := strings.TrimSpace(c.Query("bbox")); raw != "" {
		b, ok := httpx.ParseBbox(c)
		if !ok {
			return
		}
		// Validate bbox coordinate ranges.
		if err := validateBbox(b); err != nil {
			response.BadRequest(c, err.Error())
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
	runStreamLoop(ctx, c.Writer, ch, h.heartbeatInterval, bbox, category)
}

// runStreamLoop runs the SSE event/heartbeat loop. Extracted for testing.
func runStreamLoop(ctx context.Context, w http.ResponseWriter, ch <-chan *redis.Message, interval time.Duration, bbox *[4]float64, category *int) {
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
