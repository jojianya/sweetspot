package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jojianya/sweetspot247-backend/internal/http/response"
)

type entry struct {
	count   int
	resetAt time.Time
}

type Limiter struct {
	mu     sync.Mutex
	limit  int
	window time.Duration
	byKey  map[string]*entry
	// order records window starts oldest-first so a full map can evict the
	// oldest one in amortized O(1). Entries go stale (reset, swept, deleted)
	// and are skipped lazily; resetAt disambiguates a re-queued key.
	order []queuedKey
	head  int
	// pendingSweep counts new-key inserts since the last full sweep, so the
	// O(n) sweep runs at most once per batch instead of on every request
	// while the map sits at the cap.
	pendingSweep int
}

type queuedKey struct {
	key     string
	resetAt time.Time
}

// maxKeysBounds caps the number of tracked keys. Once the limit is reached,
// expired entries are swept on the next AllowKey call so the map cannot grow
// unbounded with one-time visitors.
const maxKeysBounds = 10000

// sweepBatch bounds how often the O(n) expired-key sweep runs while the map
// sits at the cap: at most once per this many new-key inserts. Between sweeps
// a full map evicts the oldest window-start instead, so no request pays a
// scan and no request is rejected merely because the map is full.
const sweepBatch = 1024

func New(limit int, window time.Duration) *Limiter {
	return &Limiter{
		limit:  limit,
		window: window,
		byKey:  make(map[string]*entry),
	}
}

func (l *Limiter) AllowKey(key string) bool {
	now := time.Now()

	l.mu.Lock()
	defer l.mu.Unlock()

	e, ok := l.byKey[key]
	if ok && !now.After(e.resetAt) {
		e.count++
		return e.count <= l.limit
	}

	if ok {
		// Expired entry for a known key: refresh in place. The map size is
		// unchanged, so no room needs making.
		e.count = 1
		e.resetAt = now.Add(l.window)
		l.enqueue(key, e.resetAt)
		return true
	}

	if len(l.byKey) >= maxKeysBounds {
		if l.pendingSweep >= sweepBatch {
			l.sweepExpired(now)
			l.pendingSweep = 0
		}
		if len(l.byKey) >= maxKeysBounds {
			l.evictOldest()
		}
		l.pendingSweep++
	}

	resetAt := now.Add(l.window)
	l.byKey[key] = &entry{count: 1, resetAt: resetAt}
	l.enqueue(key, resetAt)
	return true
}

func (l *Limiter) sweepExpired(now time.Time) {
	for k, e := range l.byKey {
		if now.After(e.resetAt) {
			delete(l.byKey, k)
		}
	}
}

// enqueue records a window start for key. Call with l.mu held.
func (l *Limiter) enqueue(key string, resetAt time.Time) {
	l.order = append(l.order, queuedKey{key: key, resetAt: resetAt})
}

// evictOldest drops the oldest window-start entry still present, making room
// for one new key. Stale queue entries (keys since reset, swept or deleted,
// or re-queued under a newer window) are skipped. Call with l.mu held.
func (l *Limiter) evictOldest() {
	for l.head < len(l.order) {
		q := l.order[l.head]
		l.head++
		if e, ok := l.byKey[q.key]; ok && e.resetAt.Equal(q.resetAt) {
			delete(l.byKey, q.key)
			break
		}
	}
	// Compact the consumed prefix so the queue itself stays bounded.
	if l.head > 4096 && l.head > len(l.order)/2 {
		l.order = append([]queuedKey(nil), l.order[l.head:]...)
		l.head = 0
	}
}

// Locked reports whether the key is currently blocked: at least `limit`
// attempts have been recorded within the active window. Call Reset after
// success to release the lockout.
func (l *Limiter) Locked(key string) bool {
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()
	e, ok := l.byKey[key]
	return ok && !now.After(e.resetAt) && e.count >= l.limit
}

// Reset removes all state for the given key, releasing any lockout.
func (l *Limiter) Reset(key string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.byKey, key)
}

// Middleware rate-limits per client IP. It is MiddlewareKeyed with the
// ClientIP key; because the counters live in process memory the limits are
// per-replica and do not hold up across multiple server instances.
func (l *Limiter) Middleware() gin.HandlerFunc {
	return l.MiddlewareKeyed(func(c *gin.Context) string {
		return c.ClientIP()
	})
}

func (l *Limiter) MiddlewareKeyed(keyFn func(*gin.Context) string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !l.AllowKey(keyFn(c)) {
			response.AbortError(c, http.StatusTooManyRequests, "too many requests, try again later")
			return
		}
		c.Next()
	}
}
