package realtime

import (
	"log/slog"
	"strings"
	"testing"

	"github.com/jojianya/sweetspot247-backend/internal/modules/pins"
)

// captureSlog redirects the default logger to a buffer for the duration of fn
// and returns what was written.
func captureSlog(t *testing.T, fn func()) string {
	t.Helper()
	var buf strings.Builder
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })
	fn()
	return buf.String()
}

// resetMalformedLocationCounters puts both counters back to zero so a test does
// not inherit another test's drops or rate-limit window.
func resetMalformedLocationCounters() {
	malformedLocationDrops.Store(0)
	malformedLocationLastLog.Store(0)
}

// A malformed location must still be dropped from a bbox-filtered stream — the
// delivery behaviour is unchanged — but the drop is now counted and reported, so
// a publisher that emits the wrong WKT dialect cannot make pins silently stop
// leaving other viewports.
func TestMatchesRemovedReportsMalformedLocation(t *testing.T) {
	resetMalformedLocationCounters()
	bbox := [4]float64{10, 20, 30, 40}
	bad := pins.PinRemoved{ID: "pin-malformed", Location: "0101000020E6100000"}

	logged := captureSlog(t, func() {
		if matchesRemoved(bad, &bbox) {
			t.Error("a malformed location must still be dropped when a bbox is set")
		}
	})

	if got := malformedLocationDrops.Load(); got != 1 {
		t.Errorf("drop counter = %d, want 1", got)
	}
	if !strings.Contains(logged, "unparsable location") {
		t.Errorf("expected a warning about the unparsable location, got %q", logged)
	}
	if !strings.Contains(logged, "pin-malformed") {
		t.Errorf("expected the event id in the warning, got %q", logged)
	}
	if !strings.Contains(logged, "drops_since_last_log") {
		t.Errorf("expected a running counter in the warning, got %q", logged)
	}
}

// A well-formed location is not a problem to report: it matches when inside the
// bbox and does not count.
func TestMatchesRemovedDoesNotCountWellFormedLocations(t *testing.T) {
	resetMalformedLocationCounters()
	bbox := [4]float64{10, 20, 30, 40}
	inside := pins.PinRemoved{ID: "pin-inside", Location: "POINT(25 15)"}
	outside := pins.PinRemoved{ID: "pin-outside", Location: "POINT(100 100)"}

	logged := captureSlog(t, func() {
		if !matchesRemoved(inside, &bbox) {
			t.Error("expected an inside-bbox removal to match")
		}
		if matchesRemoved(outside, &bbox) {
			t.Error("expected an outside-bbox removal to be filtered")
		}
	})

	if got := malformedLocationDrops.Load(); got != 0 {
		t.Errorf("drop counter = %d, want 0 for well-formed locations", got)
	}
	if logged != "" {
		t.Errorf("expected no warnings for well-formed locations, got %q", logged)
	}
}

// With no bbox filter a malformed location is passed through, exactly as before.
// It is not counted either: nothing was dropped.
func TestMatchesRemovedWithoutBboxIsNotCounted(t *testing.T) {
	resetMalformedLocationCounters()
	bad := pins.PinRemoved{ID: "pin-malformed", Location: "not-a-point"}

	logged := captureSlog(t, func() {
		if !matchesRemoved(bad, nil) {
			t.Error("a malformed location must pass when no bbox is set")
		}
	})

	if got := malformedLocationDrops.Load(); got != 0 {
		t.Errorf("drop counter = %d, want 0 when nothing was dropped", got)
	}
	if logged != "" {
		t.Errorf("expected no warnings when nothing was dropped, got %q", logged)
	}
}

// A burst of bad events must not become a burst of log lines: the counter keeps
// climbing so the operator still sees the true total, but only one line is
// written per interval.
func TestMalformedLocationWarningIsRateLimited(t *testing.T) {
	resetMalformedLocationCounters()
	bbox := [4]float64{10, 20, 30, 40}

	// The first drop opens the window; the burst below lands inside it.
	captureSlog(t, func() {
		matchesRemoved(pins.PinRemoved{ID: "pin-first", Location: "bad"}, &bbox)
	})
	malformedLocationDrops.Store(0)

	logged := captureSlog(t, func() {
		for i := 0; i < 50; i++ {
			matchesRemoved(pins.PinRemoved{ID: "pin", Location: "bad"}, &bbox)
		}
	})

	if got := malformedLocationDrops.Load(); got != 50 {
		t.Errorf("drop counter = %d, want 50 (every drop still counts)", got)
	}
	if logged != "" {
		t.Errorf("expected the burst to be rate limited to silence, got %q", logged)
	}
}

// The counter is shared across callers and monotonic, which is what makes a
// recurrence visible rather than anecdotal.
func TestMalformedLocationCounterAccumulates(t *testing.T) {
	resetMalformedLocationCounters()
	bbox := [4]float64{10, 20, 30, 40}

	before := malformedLocationDrops.Load()
	for i := 0; i < 3; i++ {
		matchesRemoved(pins.PinRemoved{ID: "pin", Location: "bad"}, &bbox)
	}
	if got := malformedLocationDrops.Load() - before; got != 3 {
		t.Errorf("counter advanced by %d, want 3", got)
	}
}

// --- pin_created (matches) tests mirror the pin_removed ones ---

func TestMatchesReportsMalformedLocation(t *testing.T) {
	resetMalformedLocationCounters()
	bbox := [4]float64{10, 20, 30, 40}
	bad := pins.Event{ID: "pin-malformed", Location: "0101000020E6100000"}

	logged := captureSlog(t, func() {
		if matches(bad, &bbox, nil) {
			t.Error("a malformed location must still be dropped when a bbox is set")
		}
	})

	if got := malformedLocationDrops.Load(); got != 1 {
		t.Errorf("drop counter = %d, want 1", got)
	}
	if !strings.Contains(logged, "unparsable location") {
		t.Errorf("expected a warning about the unparsable location, got %q", logged)
	}
	if !strings.Contains(logged, "pin-malformed") {
		t.Errorf("expected the event id in the warning, got %q", logged)
	}
	if !strings.Contains(logged, "drops_since_last_log") {
		t.Errorf("expected a running counter in the warning, got %q", logged)
	}
	if !strings.Contains(logged, "pin_created") {
		t.Errorf("expected the event kind in the warning, got %q", logged)
	}
}

func TestMatchesDoesNotCountWellFormedLocations(t *testing.T) {
	resetMalformedLocationCounters()
	bbox := [4]float64{10, 20, 30, 40}
	inside := pins.Event{ID: "pin-inside", Location: "POINT(25 15)"}
	outside := pins.Event{ID: "pin-outside", Location: "POINT(100 100)"}

	logged := captureSlog(t, func() {
		if !matches(inside, &bbox, nil) {
			t.Error("expected an inside-bbox event to match")
		}
		if matches(outside, &bbox, nil) {
			t.Error("expected an outside-bbox event to be filtered")
		}
	})

	if got := malformedLocationDrops.Load(); got != 0 {
		t.Errorf("drop counter = %d, want 0 for well-formed locations", got)
	}
	if logged != "" {
		t.Errorf("expected no warnings for well-formed locations, got %q", logged)
	}
}

func TestMatchesWithoutBboxIsNotCounted(t *testing.T) {
	resetMalformedLocationCounters()
	bad := pins.Event{ID: "pin-malformed", Location: "not-a-point"}

	logged := captureSlog(t, func() {
		if !matches(bad, nil, nil) {
			t.Error("a malformed location must pass when no bbox is set")
		}
	})

	if got := malformedLocationDrops.Load(); got != 0 {
		t.Errorf("drop counter = %d, want 0 when nothing was dropped", got)
	}
	if logged != "" {
		t.Errorf("expected no warnings when nothing was dropped, got %q", logged)
	}
}
