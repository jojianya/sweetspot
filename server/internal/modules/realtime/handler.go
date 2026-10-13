package realtime

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	httpx "github.com/jojianya/sweetspot247-backend/internal/http/params"
	"github.com/jojianya/sweetspot247-backend/internal/http/response"
)

// sseWriteTimeout bounds a single write to a stream client.
//
// The stream is intentionally unbounded in total duration, so the server-wide
// WriteTimeout is left unset and each write carries its own deadline instead.
// That way a peer that stops reading is detected and its goroutine released,
// while an active-but-quiet stream is never cut off.
const sseWriteTimeout = 10 * time.Second

// sseHeartbeatInterval is the interval between SSE heartbeat comments.
const sseHeartbeatInterval = 20 * time.Second

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
		// ParseBbox is the single source of truth for bbox validation: it
		// rejects out-of-range, inverted and zero-area boxes itself.
		b, ok := httpx.ParseBbox(c)
		if !ok {
			return
		}
		bbox = &b
	}

	category, ok := httpx.ParseCategoryID(c)
	if !ok {
		return
	}

	sub, err := h.broker.Subscribe(ctx)
	if err != nil {
		// Redis is unreachable or refused the subscription. Answering 200 here
		// would give the client a stream that stays open and never delivers a
		// pin, and every retry would do the same while holding a limiter slot.
		// A 503 with Retry-After lets the browser's EventSource back off and
		// reconnect, so the stream recovers on its own when Redis does.
		slog.Error("realtime: subscribe failed", "error", err.Error())
		c.Header("Retry-After", "5")
		response.Error(c, http.StatusServiceUnavailable, "realtime stream unavailable, try again later")
		return
	}
	defer sub.Close()

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache, no-transform")
	c.Header("Connection", "keep-alive")
	c.Header("X-Accel-Buffering", "no")

	// Arm a deadline before the first flush too, so a client that connects and
	// never reads cannot hold the handler open.
	setWriteDeadline(c.Writer, sseWriteTimeout)
	c.Writer.WriteHeader(http.StatusOK)
	c.Writer.Flush()

	ch := sub.Channel()

	// Also subscribe to pin removed channel
	removedSub, err := h.broker.SubscribeRemoved(ctx)
	if err != nil {
		slog.Error("realtime: subscribe removed failed", "error", err.Error())
		c.Header("Retry-After", "5")
		response.Error(c, http.StatusServiceUnavailable, "realtime stream unavailable, try again later")
		return
	}
	defer removedSub.Close()
	removedCh := removedSub.Channel()

	runStreamLoop(ctx, c.Writer, ch, h.heartbeatInterval, bbox, category, removedCh)
}
