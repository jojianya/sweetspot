package app

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"
)

// TestServeReturnsNilWhenShutdownExternally covers the ListenAndServe branch:
// once Shutdown runs, ListenAndServe returns http.ErrServerClosed, which is a
// normal stop and must not be reported as an error. Previously this surfaced as
// a spurious "server error" log on every restart.
func TestServeReturnsNilWhenShutdownExternally(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	addr := ln.Addr().String()
	if err := ln.Close(); err != nil {
		t.Fatalf("close probe listener: %v", err)
	}

	srv := &http.Server{
		Addr:              addr,
		Handler:           http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}),
		ReadHeaderTimeout: readHeaderTimeout,
		IdleTimeout:       idleTimeout,
		MaxHeaderBytes:    maxHeaderBytes,
	}

	go func() {
		time.Sleep(150 * time.Millisecond)
		_ = srv.Shutdown(context.Background())
	}()

	done := make(chan error, 1)
	go func() { done <- serve(srv) }()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("expected nil after graceful shutdown, got %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("serve did not return after shutdown")
	}
}

// TestServeReturnsErrorOnBindFailure confirms a genuine failure is still
// surfaced, so the nil-on-shutdown path did not swallow real errors.
func TestServeReturnsErrorOnBindFailure(t *testing.T) {
	// Hold a port so the server below cannot bind it.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	srv := &http.Server{
		Addr:    ln.Addr().String(),
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}),
	}

	if err := serve(srv); err == nil {
		t.Fatal("expected an error when the port is already in use, got nil")
	}
}

// TestReadHeaderTimeoutIsBounded guards the Slowloris mitigation: a zero value
// here reintroduces the unbounded header read.
//
// The exact values are asserted, not just their sign, because they are a
// security boundary — a longer deadline means more connections held open. A
// previous revision quietly shipped 10s/120s instead of the agreed 5s/60s and
// nothing caught it.
func TestReadHeaderTimeoutIsBounded(t *testing.T) {
	if want := 5 * time.Second; readHeaderTimeout != want {
		t.Errorf("readHeaderTimeout = %v, want %v", readHeaderTimeout, want)
	}
	if want := 60 * time.Second; idleTimeout != want {
		t.Errorf("idleTimeout = %v, want %v", idleTimeout, want)
	}
	if want := 1 << 20; maxHeaderBytes != want {
		t.Errorf("maxHeaderBytes = %d, want %d", maxHeaderBytes, want)
	}
	// The header deadline must stay well under the idle deadline so a slow
	// reader is dropped before its keep-alive slot expires.
	if readHeaderTimeout >= idleTimeout {
		t.Errorf("readHeaderTimeout (%v) should be shorter than idleTimeout (%v)",
			readHeaderTimeout, idleTimeout)
	}
}
