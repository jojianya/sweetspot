package realtime

import "sync"

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
