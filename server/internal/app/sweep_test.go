package app

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

// TestSweepLoopStopsOnCancel proves cancelling the loop context ends the
// goroutine and no further passes run: shutdown leaves nothing behind.
func TestSweepLoopStopsOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	var calls atomic.Int64
	done := make(chan struct{})
	go func() {
		defer close(done)
		sweepLoop(ctx, 10*time.Millisecond, func(context.Context) { calls.Add(1) })
	}()

	deadline := time.Now().Add(5 * time.Second)
	for calls.Load() == 0 {
		if time.Now().After(deadline) {
			t.Fatal("loop never ran a pass")
		}
		time.Sleep(5 * time.Millisecond)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("loop did not stop after cancel")
	}
	atCancel := calls.Load()
	time.Sleep(100 * time.Millisecond)
	if calls.Load() != atCancel {
		t.Fatalf("passes ran after cancel: %d -> %d", atCancel, calls.Load())
	}
}

// TestSweepLoopInFlightHonorsCancel proves an in-flight pass does not wait
// out the run timeout: when the parent context is cancelled, the derived
// run context from sweepLoop is cancelled too, so a shutdown-aware pass
// returns promptly.
func TestSweepLoopInFlightHonorsCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	started := make(chan struct{})
	var once bool
	done := make(chan struct{})
	go func() {
		defer close(done)
		sweepLoop(ctx, 10*time.Millisecond, func(runCtx context.Context) {
			if !once {
				once = true
				close(started)
			}
			<-runCtx.Done()
		})
	}()

	select {
	case <-started:
	case <-time.After(5 * time.Second):
		t.Fatal("loop never started a pass")
	}
	t0 := time.Now()
	cancel()
	select {
	case <-done:
		if elapsed := time.Since(t0); elapsed > 5*time.Second {
			t.Fatalf("in-flight pass took %v to stop, want well under the 30s run timeout", elapsed)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("loop did not stop after cancel")
	}
}
