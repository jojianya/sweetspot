package realtime

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jojianya/sweetspot247-backend/internal/modules/pins"
	"github.com/redis/go-redis/v9"
)

func init() { gin.SetMode(gin.TestMode) }

// TestRunStreamLoopRemovalEmitsPinRemovedOnly proves a removal can never
// produce `event: pin`: a payload arriving on the removed channel is framed
// exclusively as `event: pin_removed`.
func TestRunStreamLoopRemovalEmitsPinRemovedOnly(t *testing.T) {
	msgCh := make(chan *redis.Message, 1) // no pin creations
	removedCh := make(chan *redis.Message, 1)
	removedCh <- &redis.Message{Payload: `{"id":"11111111-1111-1111-1111-111111111111","location":"POINT(0 0)"}`}

	rec := httptest.NewRecorder()
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	runStreamLoop(ctx, rec, msgCh, 50*time.Millisecond, nil, nil, removedCh)

	body := rec.Body.String()
	if !strings.Contains(body, "event: pin_removed") {
		t.Fatalf("expected 'event: pin_removed' in body, got: %s", body)
	}
	if strings.Contains(body, "event: pin\n") {
		t.Fatalf("removal must not produce 'event: pin', got: %s", body)
	}
}

// TestRunStreamLoopRemovalBboxFilter proves removals honor the bbox filter:
// inside-bbox removals are forwarded, outside-bbox removals are dropped, and
// neither path can emit `event: pin`.
func TestRunStreamLoopRemovalBboxFilter(t *testing.T) {
	bbox := [4]float64{10, 20, 30, 40} // minLat, minLng, maxLat, maxLng

	msgCh := make(chan *redis.Message, 1)
	removedCh := make(chan *redis.Message, 2)
	// POINT(lng lat): inside = lng 25, lat 15; outside = lng 100, lat 100.
	removedCh <- &redis.Message{Payload: `{"id":"22222222-2222-2222-2222-222222222222","location":"POINT(25 15)"}`}
	removedCh <- &redis.Message{Payload: `{"id":"33333333-3333-3333-3333-333333333333","location":"POINT(100 100)"}`}

	rec := httptest.NewRecorder()
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	runStreamLoop(ctx, rec, msgCh, 50*time.Millisecond, &bbox, nil, removedCh)

	body := rec.Body.String()
	if !strings.Contains(body, "22222222-2222-2222-2222-222222222222") {
		t.Errorf("expected inside-bbox removal in body, got: %s", body)
	}
	if strings.Contains(body, "33333333-3333-3333-3333-333333333333") {
		t.Errorf("expected outside-bbox removal to be filtered, got: %s", body)
	}
	if strings.Contains(body, "event: pin\n") {
		t.Errorf("removal must not produce 'event: pin', got: %s", body)
	}
}

// TestMatchesRemoved covers the bbox predicate directly, including the
// malformed-location rule (dropped when a bbox is set, passed when unset).
func TestMatchesRemoved(t *testing.T) {
	bbox := [4]float64{10, 20, 30, 40}
	inside := pins.PinRemoved{ID: "x", Location: "POINT(25 15)"}
	outside := pins.PinRemoved{ID: "x", Location: "POINT(100 100)"}
	bad := pins.PinRemoved{ID: "x", Location: "not-a-point"}

	if !matchesRemoved(inside, &bbox) {
		t.Error("expected inside-bbox removal to match")
	}
	if matchesRemoved(outside, &bbox) {
		t.Error("expected outside-bbox removal to be filtered")
	}
	if matchesRemoved(bad, &bbox) {
		t.Error("expected malformed location to be filtered when bbox is set")
	}
	if !matchesRemoved(bad, nil) {
		t.Error("expected malformed location to pass when no bbox is set")
	}
	if !matchesRemoved(outside, nil) {
		t.Error("expected removal to pass when no bbox is set")
	}
}
