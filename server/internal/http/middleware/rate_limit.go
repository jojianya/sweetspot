package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
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
}

// maxKeysBounds caps the number of tracked keys. Once the limit is reached,
// expired entries are swept on the next AllowKey call so the map cannot grow
// unbounded with one-time visitors.
const maxKeysBounds = 10000

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

	if len(l.byKey) >= maxKeysBounds {
		l.sweepExpired(now)
	}

	e, ok := l.byKey[key]
	if !ok || now.After(e.resetAt) {
		l.byKey[key] = &entry{count: 1, resetAt: now.Add(l.window)}
		return true
	}

	e.count++
	return e.count <= l.limit
}

func (l *Limiter) sweepExpired(now time.Time) {
	for k, e := range l.byKey {
		if now.After(e.resetAt) {
			delete(l.byKey, k)
		}
	}
}

// Allow reports whether the key is within its window's request limit.
// It increments the counter for the key and returns true if the result is within limit.
func (l *Limiter) Allow(key string) bool {
	return l.AllowKey(key)
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

func (l *Limiter) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if !l.Allow(c.ClientIP()) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "too many requests, try again later"})
			return
		}
		c.Next()
	}
}

func (l *Limiter) MiddlewareKeyed(keyFn func(*gin.Context) string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !l.AllowKey(keyFn(c)) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "too many requests, try again later"})
			return
		}
		c.Next()
	}
}
