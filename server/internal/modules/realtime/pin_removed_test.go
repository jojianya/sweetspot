package realtime

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jojianya/sweetspot247-backend/internal/modules/pins"
	"github.com/redis/go-redis/v9"
)

func init() { gin.SetMode(gin.TestMode) }

// TestRunStreamLoopRemovalEmitsPinRemovedOnly proves a removal can never
// produce `event: pin`: a payload arriving on the removed channel is framed
// exclusively as `event: pin_removed`.
func TestRunStreamLoopRemovalEmitsPinRemovedOnly(t *testing.T) {
	msgCh := make(chan *redis.Message, 1) // no pin creations
	removedCh := make(chan *redis.Message, 1)
	removedCh <- &redis.Message{Payload: `{"id":"11111111-1111-1111-1111-111111111111","location":"POINT(0 0)"}`}

	rec := httptest.NewRecorder()
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	runStreamLoop(ctx, rec, msgCh, 50*time.Millisecond, nil, nil, removedCh)

	body := rec.Body.String()
	if !strings.Contains(body, "event: pin_removed") {
		t.Fatalf("expected 'event: pin_removed' in body, got: %s", body)
	}
	if strings.Contains(body, "event: pin\n") {
		t.Fatalf("removal must not produce 'event: pin', got: %s", body)
	}
}

// TestRunStreamLoopRemovalBboxFilter proves removals honor the bbox filter:
// inside-bbox removals are forwarded, outside-bbox removals are dropped, and
// neither path can emit `event: pin`.
func TestRunStreamLoopRemovalBboxFilter(t *testing.T) {
	bbox := [4]float64{10, 20, 30, 40} // minLat, minLng, maxLat, maxLng

	msgCh := make(chan *redis.Message, 1)
	removedCh := make(chan *redis.Message, 2)
	// POINT(lng lat): inside = lng 25, lat 15; outside = lng 100, lat 100.
	removedCh <- &redis.Message{Payload: `{"id":"22222222-2222-2222-2222-222222222222","location":"POINT(25 15)"}`}
	removedCh <- &redis.Message{Payload: `{"id":"33333333-3333-3333-3333-333333333333","location":"POINT(100 100)"}`}

	rec := httptest.NewRecorder()
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	runStreamLoop(ctx, rec, msgCh, 50*time.Millisecond, &bbox, nil, removedCh)

	body := rec.Body.String()
	if !strings.Contains(body, "22222222-2222-2222-2222-222222222222") {
		t.Errorf("expected inside-bbox removal in body, got: %s", body)
	}
	if strings.Contains(body, "33333333-3333-3333-3333-333333333333") {
		t.Errorf("expected outside-bbox removal to be filtered, got: %s", body)
	}
	if strings.Contains(body, "event: pin\n") {
		t.Errorf("removal must not produce 'event: pin', got: %s", body)
	}
}

// TestMatchesRemoved covers the bbox predicate directly, including the
// malformed-location rule (dropped when a bbox is set, passed when unset).
func TestMatchesRemoved(t *testing.T) {
	bbox := [4]float64{10, 20, 30, 40}
	inside := pins.PinRemoved{ID: "x", Location: "POINT(25 15)"}
	outside := pins.PinRemoved{ID: "x", Location: "POINT(100 100)"}
	bad := pins.PinRemoved{ID: "x", Location: "not-a-point"}

	if !matchesRemoved(inside, &bbox) {
		t.Errorf("expected inside bbox to match")
	}
	if matchesRemoved(outside, &bbox) {
		t.Errorf("expected outside bbox to not match")
	}
	if matchesRemoved(bad, &bbox) {
		t.Errorf("expected malformed location to not match with bbox set")
	}
	if !matchesRemoved(bad, nil) {
		t.Errorf("expected malformed location to match when bbox is unset")
	}
}

// signalingRecorder wraps httptest.ResponseRecorder and signals when headers are first written.
type signalingRecorder struct {
	*httptest.ResponseRecorder
	headersWritten chan struct{}
	once           sync.Once
}

func newSignalingRecorder() *signalingRecorder {
	return &signalingRecorder{
		ResponseRecorder: httptest.NewRecorder(),
		headersWritten:   make(chan struct{}),
	}
}

func (sr *signalingRecorder) WriteHeader(code int) {
	sr.ResponseRecorder.WriteHeader(code)
	// Signal that headers are written (WriteHeader is called after headers are set)
	sr.once.Do(func() { close(sr.headersWritten) })
}

func (sr *signalingRecorder) Header() http.Header {
	return sr.ResponseRecorder.Header()
}

// TestStreamSetsNoTransformCacheControl proves the SSE response opts out of
// compression at every hop: proxies (Next rewrites, nginx gzip) must
// not buffer the stream, or browsers never receive events live.
// Needs a reachable Redis (same bar as the DB-backed endpoint tests).
func TestStreamSetsNoTransformCacheControl(t *testing.T) {
	broker := NewBroker("127.0.0.1:6379", "")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := broker.Ping(ctx); err != nil {
		t.Skipf("redis not reachable, skipping: %v", err)
	}

	h := NewHandler(broker, 0)

	rec := newSignalingRecorder()
	c, _ := gin.CreateTestContext(rec)
	reqCtx, stop := context.WithCancel(context.Background())
	defer stop()
	c.Request = httptest.NewRequest("GET", "/events?bbox=17.3,78.3,17.5,78.6", nil).WithContext(reqCtx)

	done := make(chan struct{})
	go func() {
		defer close(done)
		h.Stream(c)
	}()

	// Wait for handler to write headers (signaled by WriteHeader) to avoid data race
	select {
	case <-rec.headersWritten:
	case <-time.After(3 * time.Second):
		t.Fatal("timeout waiting for headers to be written")
	}

	deadline := time.Now().Add(3 * time.Second)
	for rec.Header().Get("Cache-Control") == "" && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	stop()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Stream did not return after context cancel")
	}

	cc := rec.Header().Get("Cache-Control")
	if !contains(cc, "no-transform") {
		t.Errorf("expected Cache-Control to contain no-transform, got %q", cc)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || contains(s[1:], substr)))
}
