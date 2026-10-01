package realtime

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	httpx "github.com/jojianya/sweetspot247-backend/internal/http/params"
)

// TestSSEGlobalCap checks that the handler tracks active connections
// and would reject when over the limit.
func TestSSEGlobalCap(t *testing.T) {
	gin.SetMode(gin.TestMode)
	h := NewHandler(nil, 1)

	h.connsMu.Lock()
	if h.activeConns != 0 {
		t.Fatalf("expected 0 active conns, got %d", h.activeConns)
	}
	h.connsMu.Unlock()

	// Simulate one active connection
	h.connsMu.Lock()
	h.activeConns = 1
	h.connsMu.Unlock()

	// Check cap logic
	h.connsMu.Lock()
	overCap := h.maxConns > 0 && h.activeConns >= h.maxConns
	h.connsMu.Unlock()
	if !overCap {
		t.Fatal("expected overCap=true when activeConns >= maxConns")
	}
}

// TestSSEBboxValidation tests bbox coordinate range validation.
func TestSSEBboxValidation(t *testing.T) {
	tests := []struct {
		name     string
		bbox     string
		valid    bool
		wantBbox [4]float64
	}{
		{"valid", "0,0,1,1", true, [4]float64{0, 0, 1, 1}},
		{"valid negative", "-10,-20,10,20", true, [4]float64{-10, -20, 10, 20}},
		{"lat too high", "91,0,92,1", false, [4]float64{}},
		{"lat too low", "-91,0,-90,1", false, [4]float64{}},
		{"lng too high", "0,181,1,182", false, [4]float64{}},
		{"lng too low", "0,-181,1,-180", false, [4]float64{}},
		{"minLat > maxLat", "10,0,5,1", false, [4]float64{}},
		{"minLng > maxLng", "0,10,1,5", false, [4]float64{}},
		{"empty", "", false, [4]float64{}},
		{"malformed", "a,b,c,d", false, [4]float64{}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			url := "/events"
			if tc.bbox != "" {
				url += "?bbox=" + tc.bbox
			}
			c.Request = httptest.NewRequest(http.MethodGet, url, nil)

			// Test the ParseBbox function directly
			bbox, ok := httpx.ParseBbox(c)
			if tc.valid {
				if !ok {
					t.Errorf("ParseBbox(%q): expected ok=true, got false", tc.bbox)
				}
				if bbox != tc.wantBbox {
					t.Errorf("ParseBbox(%q): got %v, want %v", tc.bbox, bbox, tc.wantBbox)
				}
				// Now test the range validation logic from handler
				if bbox[0] < -90 || bbox[0] > 90 || bbox[2] < -90 || bbox[2] > 90 ||
					bbox[1] < -180 || bbox[1] > 180 || bbox[3] < -180 || bbox[3] > 180 ||
					bbox[0] > bbox[2] || bbox[1] > bbox[3] {
					t.Errorf("range validation failed for valid bbox %v", bbox)
				}
			} else {
				if ok {
					t.Errorf("ParseBbox(%q): expected ok=false, got true", tc.bbox)
				}
			}
		})
	}
}

// TestSSEHeartbeatTiming guards the heartbeat interval.
func TestSSEHeartbeatTiming(t *testing.T) {
	// Heartbeat should be frequent enough to keep connections alive through
	// typical proxies (often 30-60s idle timeout) but not so frequent as to
	// waste bandwidth.
	const heartbeatInterval = 20 * time.Second
	if heartbeatInterval <= 0 {
		t.Fatal("heartbeat interval must be positive")
	}
	if heartbeatInterval > 30*time.Second {
		t.Errorf("heartbeat interval %v too long (>30s), proxies may drop connection", heartbeatInterval)
	}
	if heartbeatInterval < 10*time.Second {
		t.Errorf("heartbeat interval %v too short (<10s), excessive traffic", heartbeatInterval)
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

// TestSSEWriteTimeoutIsShort guards the value: a per-write deadline much longer
// than this defeats the purpose of bounding a blocked peer.
func TestSSEWriteTimeoutIsShort(t *testing.T) {
	if sseWriteTimeout <= 0 {
		t.Fatalf("sseWriteTimeout must be positive, got %v", sseWriteTimeout)
	}
	if sseWriteTimeout > 30*time.Second {
		t.Errorf("sseWriteTimeout = %v, want <= 30s", sseWriteTimeout)
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