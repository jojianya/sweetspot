package realtime

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// TestValidateBbox tests the pure bbox validation function.
func TestValidateBbox(t *testing.T) {
	tests := []struct {
		name    string
		bbox    [4]float64
		wantErr bool
	}{
		{"valid", [4]float64{14.4, 120.9, 14.8, 121.1}, false},
		{"valid negative", [4]float64{-10, -20, 10, 20}, false},
		{"lat too high", [4]float64{91, 0, 92, 1}, true},
		{"lat too low", [4]float64{-91, 0, -90, 1}, true},
		{"lng too high", [4]float64{0, 181, 1, 182}, true},
		{"lng too low", [4]float64{0, -181, 1, -180}, true},
		{"minLat == maxLat", [4]float64{10, 0, 10, 1}, true},
		{"minLat > maxLat", [4]float64{10, 0, 5, 1}, true},
		{"minLng > maxLng", [4]float64{0, 10, 1, 5}, true},
		{"zero area", [4]float64{0, 0, 0, 0}, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateBbox(tc.bbox)
			if tc.wantErr {
				if err == nil {
					t.Errorf("validateBbox(%v): expected error, got nil", tc.bbox)
				}
			} else {
				if err != nil {
					t.Errorf("validateBbox(%v): unexpected error: %v", tc.bbox, err)
				}
			}
		})
	}
}

// TestConnectionLimiter tests the connection limiter acquire/release logic.
func TestConnectionLimiter(t *testing.T) {
	// Test nil limiter (no limit)
	nilLimiter := (*ConnectionLimiter)(nil)
	if !nilLimiter.TryAcquire() {
		t.Fatal("nil limiter should always allow acquire")
	}
	nilLimiter.Release() // Should not panic

	// Test limiter with cap 2
	l := NewConnectionLimiter(2)

	// Acquire first
	if !l.TryAcquire() {
		t.Fatal("first acquire should succeed")
	}
	// Acquire second
	if !l.TryAcquire() {
		t.Fatal("second acquire should succeed")
	}
	// Third should fail
	if l.TryAcquire() {
		t.Fatal("third acquire should fail at cap")
	}

	// Release one, should allow another
	l.Release()
	if !l.TryAcquire() {
		t.Fatal("acquire after release should succeed")
	}

	// Release both
	l.Release()
	l.Release()

	// Should be able to acquire again
	if !l.TryAcquire() {
		t.Fatal("acquire after full release should succeed")
	}
}

// TestConnectionLimiterZeroCap tests that a limiter with cap <= 0 allows all.
func TestConnectionLimiterZeroCap(t *testing.T) {
	for _, cap := range []int{0, -1, -5} {
		l := NewConnectionLimiter(cap)
		if l != nil {
			t.Errorf("NewConnectionLimiter(%d) should return nil", cap)
		}
	}
}

// TestSetWriteDeadlineToleratesUnsupportedWriter confirms the helper is safe on
// writers that cannot express a deadline. Stream must not fail on them.
func TestSetWriteDeadlineToleratesUnsupportedWriter(t *testing.T) {
	rec := httptest.NewRecorder()
	setWriteDeadline(rec, time.Second) // must not panic or block
	if rec.Code != 200 && rec.Code != 0 {
		t.Fatalf("unexpected recorder state: %d", rec.Code)
	}
}

// TestSSEWriteDeadlineFiresOnBlockedPeer is the point of the per-write deadline:
// a client that stops reading must not pin the handler goroutine forever.
func TestSSEWriteDeadlineFiresOnBlockedPeer(t *testing.T) {
	const deadline = 200 * time.Millisecond

	writeErr := make(chan error, 1)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	srv := &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			setWriteDeadline(w, deadline)
			w.WriteHeader(http.StatusOK)
			w.(http.Flusher).Flush()

			chunk := make([]byte, 64*1024)
			for {
				if _, err := w.Write(chunk); err != nil {
					writeErr <- err
					return
				}
				// Push each chunk to the socket so the buffer actually fills.
				w.(http.Flusher).Flush()
			}
		}),
	}
	go func() { _ = srv.Serve(ln) }()
	defer srv.Close()

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	// Send the request line and headers, then deliberately never read the
	// response body, so the socket buffer fills and the handler's writes block
	// until the deadline fires.
	if _, err := conn.Write([]byte("GET /events HTTP/1.1\r\nHost: test\r\n\r\n")); err != nil {
		t.Fatalf("write request: %v", err)
	}

	select {
	case err := <-writeErr:
		if err == nil {
			t.Fatal("expected a write error once the deadline fired, got nil")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("write deadline never fired; a blocked peer would pin the goroutine")
	}
}

// TestRunStreamLoop tests the extracted stream loop function.
// It sends one message, waits past the heartbeat interval, cancels the context,
// and verifies both the event and heartbeat are written.
func TestRunStreamLoop(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create a channel that will receive one message, then wait for heartbeat,
	// then close
	msgCh := make(chan *redis.Message, 1)
	msgCh <- &redis.Message{Payload: `{"id":"1","user_id":"u1","location":"POINT(0 0)","caption":null,"category_id":1,"cover_url":"","created_at":"2024-01-01T00:00:00Z"}`}

	rec := httptest.NewRecorder()

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	// Run the stream loop with a very short heartbeat interval
	runStreamLoop(ctx, rec, msgCh, 50*time.Millisecond, nil, nil)

	// Read body after goroutine exits
	body := rec.Body.String()

	// Should contain the event
	if !strings.Contains(body, "event: pin") {
		t.Errorf("expected 'event: pin' in body, got: %s", body)
	}
	// Should contain heartbeat comment
	if !strings.Contains(body, ": heartbeat") {
		t.Errorf("expected ': heartbeat' in body, got: %s", body)
	}
}

// TestRunStreamLoopWithFilters tests that bbox and category filters work.
// Only matching events are written; non-matching are silently dropped.
func TestRunStreamLoopWithFilters(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Bbox: minLat=10, minLng=20, maxLat=30, maxLng=40
	// Category: 5
	bbox := [4]float64{10, 20, 30, 40}
	category := 5

	// Two events: one matching (inside bbox, category 5), one non-matching (outside bbox, category 99)
	// Channel is NOT closed; context timeout will stop the loop
	msgCh := make(chan *redis.Message, 2)
	msgCh <- &redis.Message{Payload: `{"id":"match","user_id":"u1","location":"POINT(25 15)","caption":null,"category_id":5,"cover_url":"","created_at":"2024-01-01T00:00:00Z"}`}
	msgCh <- &redis.Message{Payload: `{"id":"nomatch","user_id":"u1","location":"POINT(100 100)","caption":null,"category_id":99,"cover_url":"","created_at":"2024-01-01T00:00:00Z"}`}

	rec := httptest.NewRecorder()

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	runStreamLoop(ctx, rec, msgCh, 50*time.Millisecond, &bbox, &category)

	body := rec.Body.String()

	// Should contain the matching event
	if !strings.Contains(body, `"id":"match"`) {
		t.Errorf("expected matching event in body, got: %s", body)
	}
	// Should NOT contain the non-matching event
	if strings.Contains(body, `"id":"nomatch"`) {
		t.Errorf("expected non-matching event to be filtered out, got: %s", body)
	}
	// Should contain heartbeat (context timeout > heartbeat interval)
	if !strings.Contains(body, ": heartbeat") {
		t.Errorf("expected heartbeat in body, got: %s", body)
	}
}

// TestRunStreamLoopFilterIsolation tests that each filter is applied independently.
// An event matching category but outside bbox is dropped.
// An event inside bbox but with wrong category is dropped.
func TestRunStreamLoopFilterIsolation(t *testing.T) {
	gin.SetMode(gin.TestMode)

	bbox := [4]float64{10, 20, 30, 40}
	category := 5

	// Event 1: right category (5), outside bbox (lat 100, lng 100)
	// Event 2: inside bbox (lat 25, lng 15), wrong category (99)
	msgCh := make(chan *redis.Message, 2)
	msgCh <- &redis.Message{Payload: `{"id":"cat_ok_bbox_bad","user_id":"u1","location":"POINT(100 100)","caption":null,"category_id":5,"cover_url":"","created_at":"2024-01-01T00:00:00Z"}`}
	msgCh <- &redis.Message{Payload: `{"id":"bbox_ok_cat_bad","user_id":"u1","location":"POINT(25 15)","caption":null,"category_id":99,"cover_url":"","created_at":"2024-01-01T00:00:00Z"}`}

	rec := httptest.NewRecorder()

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	runStreamLoop(ctx, rec, msgCh, 50*time.Millisecond, &bbox, &category)

	body := rec.Body.String()

	// Neither event should be written
	if strings.Contains(body, `"id":"cat_ok_bbox_bad"`) {
		t.Errorf("expected event with right category but outside bbox to be filtered, got: %s", body)
	}
	if strings.Contains(body, `"id":"bbox_ok_cat_bad"`) {
		t.Errorf("expected event with right bbox but wrong category to be filtered, got: %s", body)
	}
	// Heartbeat should still fire
	if !strings.Contains(body, ": heartbeat") {
		t.Errorf("expected heartbeat in body, got: %s", body)
	}
}

// TestRunStreamLoopNoFilters tests that without filters all events pass through.
func TestRunStreamLoopNoFilters(t *testing.T) {
	gin.SetMode(gin.TestMode)

	msgCh := make(chan *redis.Message, 2)
	msgCh <- &redis.Message{Payload: `{"id":"a","user_id":"u1","location":"POINT(0 0)","caption":null,"category_id":1,"cover_url":"","created_at":"2024-01-01T00:00:00Z"}`}
	msgCh <- &redis.Message{Payload: `{"id":"b","user_id":"u1","location":"POINT(100 100)","caption":null,"category_id":99,"cover_url":"","created_at":"2024-01-01T00:00:00Z"}`}

	rec := httptest.NewRecorder()

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	runStreamLoop(ctx, rec, msgCh, 50*time.Millisecond, nil, nil)

	body := rec.Body.String()

	// Both events should be written
	if !strings.Contains(body, `"id":"a"`) {
		t.Errorf("expected event a, got: %s", body)
	}
	if !strings.Contains(body, `"id":"b"`) {
		t.Errorf("expected event b, got: %s", body)
	}
	if !strings.Contains(body, ": heartbeat") {
		t.Errorf("expected heartbeat in body, got: %s", body)
	}
}

// TestStreamReturns503WhenLimiterFull tests that Stream returns 503 with Retry-After
// when the connection limiter is at capacity. The limiter is pre-acquired so the
// 503 path runs before the broker is touched.
func TestStreamReturns503WhenLimiterFull(t *testing.T) {
	gin.SetMode(gin.TestMode)

	// Create a handler with cap=1 limiter
	h := NewHandler(nil, 1)

	// Pre-acquire the single slot so the next request will hit the cap
	if !h.limiter.TryAcquire() {
		t.Fatal("failed to pre-acquire limiter slot")
	}
	defer h.limiter.Release()

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	c.Request = httptest.NewRequest(http.MethodGet, "/events", nil)

	// Call Stream directly
	h.Stream(c)

	// Should return 503
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", rec.Code)
	}
	// Should have Retry-After header
	retryAfter := rec.Header().Get("Retry-After")
	if retryAfter == "" {
		t.Error("expected Retry-After header on 503")
	}
	// Body should contain the error message
	body := rec.Body.String()
	if !strings.Contains(body, "too many SSE connections") {
		t.Errorf("expected error message in body, got: %s", body)
	}
}
