package ratelimit

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
	if !ok || now.After(e.resetAt) {
		l.byKey[key] = &entry{count: 1, resetAt: now.Add(l.window)}
		return true
	}

	e.count++
	return e.count <= l.limit
}

func (l *Limiter) Allow(ip string) bool {
	return l.AllowKey(ip)
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