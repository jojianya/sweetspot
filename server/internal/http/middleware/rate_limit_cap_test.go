package middleware

import (
	"fmt"
	"testing"
	"time"
)

// The map must never grow past the cap, even when every key is still live:
// the old code only swept expired entries, so 10k+ live keys grew it
// unboundedly for the whole window.
func TestLimiterCapsLiveKeys(t *testing.T) {
	l := New(10, time.Hour)
	for i := 0; i < 2*maxKeysBounds+500; i++ {
		if !l.AllowKey(fmt.Sprintf("key-%d", i)) {
			t.Fatalf("first hit for key-%d rejected", i)
		}
	}
	l.mu.Lock()
	n := len(l.byKey)
	l.mu.Unlock()
	if n > maxKeysBounds {
		t.Fatalf("tracked keys = %d, want at most %d", n, maxKeysBounds)
	}
}

// Evictions elsewhere must not disturb per-key limiting: a fresh key still
// gets exactly `limit` allows, then 429s.
func TestLimiterLimitsFreshKeyAfterEvictions(t *testing.T) {
	const limit = 5
	l := New(limit, time.Hour)
	for i := 0; i < 2*maxKeysBounds; i++ {
		l.AllowKey(fmt.Sprintf("filler-%d", i))
	}
	for i := 0; i < limit; i++ {
		if !l.AllowKey("victim") {
			t.Fatalf("hit %d for victim rejected, want allowed", i+1)
		}
	}
	if l.AllowKey("victim") {
		t.Fatal("hit over the limit allowed, want rejected")
	}
}
