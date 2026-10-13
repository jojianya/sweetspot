package realtime

import (
	"context"
	"sync"
)

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
