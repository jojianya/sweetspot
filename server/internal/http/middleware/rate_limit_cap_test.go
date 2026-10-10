package middleware

import (
	"bytes"
	"fmt"
	"log/slog"
	"strings"
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

// captureLogs routes the default slog output into buf for the duration of
// the test, restoring the previous default afterwards.
func captureLogs(t *testing.T, buf *bytes.Buffer) {
	t.Helper()
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })
}

// Filling past the cap must evict a live entry and log it once, with the
// count; further evictions inside the same minute stay silent.
func TestLimiterLogsLiveEvictionsAtCap(t *testing.T) {
	var buf bytes.Buffer
	captureLogs(t, &buf)

	l := New(1<<30, time.Hour)
	for i := 0; i < maxKeysBounds; i++ {
		l.AllowKey(fmt.Sprintf("key-%d", i))
	}
	if buf.Len() != 0 {
		t.Fatalf("at-cap fill logged %q, want silence", buf.String())
	}

	l.AllowKey("one-more")
	if got := buf.String(); !strings.Contains(got, "evicted_since_last_log=1") {
		t.Fatalf("expected one eviction log line, got %q", got)
	}

	buf.Reset()
	l.AllowKey("another")
	if buf.Len() != 0 {
		t.Fatalf("second eviction inside the minute logged %q, want silence", buf.String())
	}
}

// Below the cap nothing is ever evicted, so nothing is logged.
func TestLimiterSilentBelowCap(t *testing.T) {
	var buf bytes.Buffer
	captureLogs(t, &buf)

	l := New(10, time.Hour)
	for i := 0; i < 100; i++ {
		l.AllowKey(fmt.Sprintf("key-%d", i))
	}
	if buf.Len() != 0 {
		t.Fatalf("below-cap use logged %q, want silence", buf.String())
	}
}

// loginLockKeySpent burns the whole per-(IP, identifier) login budget, the way
// repeated wrong passwords do, and reports whether that pair is now locked.
func loginLockKeySpent(l *Limiter, key string, budget int) bool {
	for i := 0; i < budget; i++ {
		l.AllowKey(key)
	}
	return l.Locked(key)
}

// A known limitation, pinned so it cannot regress silently: the cap is one map
// with no exemptions, so a flood of distinct keys evicts the oldest entry —
// which can be a live login-lockout counter, releasing an in-progress lockout.
//
// The alternative, a second map that never evicts, was rejected: it is
// unbounded by design, so the same flood that exhausts this map would grow
// that one without limit, and an attacker controls its growth. So the cap is
// allowed to evict anything, and the eviction is logged and counted instead
// (see TestLimiterLogsLiveEvictionsAtCap). If that tradeoff ever changes, this
// test is the thing that must change with it.
func TestLimiterCanEvictALiveLockoutCounter(t *testing.T) {
	const budget = 5
	l := New(budget, time.Minute)
	const lockKey = "127.0.0.1|user@example.test"

	if !loginLockKeySpent(l, lockKey, budget) {
		t.Fatal("pair not locked after the budget was spent, want locked")
	}

	var buf bytes.Buffer
	captureLogs(t, &buf)
	// One distinct key more than the cap holds pushes the oldest entry out.
	for i := 0; i < maxKeysBounds; i++ {
		l.AllowKey(fmt.Sprintf("flood-%d", i))
	}

	if l.Locked(lockKey) {
		t.Fatal("lockout counter survived a full-map flood, want it evicted — the cap has no exemptions")
	}
	if got := buf.String(); !strings.Contains(got, "evicted_since_last_log=1") {
		t.Fatalf("the eviction of a live lockout counter was not logged, got %q", got)
	}
}
