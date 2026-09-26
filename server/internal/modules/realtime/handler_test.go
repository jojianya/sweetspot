package realtime

import (
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestSSEWriteDeadlineFiresOnBlockedPeer is the point of the per-write deadline:
// a client that stops reading must not pin the handler goroutine forever.
//
// A real listener and a real client socket are used because
// http.ResponseController only reports ErrNotSupported against
// httptest.ResponseRecorder, so a recorder cannot exercise the mechanism.
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
