package endpointtest

// SQL-level tests for the 0023 trigger that keeps pins.good_spot_count in step
// with the good_spots table. These exercise the trigger, not the (still
// unwritten) reaction service, so every case drives the database directly.

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// seedTriggerPin inserts one visible pin owned by userID and removes it after
// the test. The geohash is unique per call so a leaked row is identifiable.
func seedTriggerPin(t *testing.T, pool *pgxpool.Pool, userID string) string {
	t.Helper()
	var id string
	err := pool.QueryRow(context.Background(), `
		INSERT INTO pins (user_id, location, geohash, caption, category_id)
		VALUES ($1, ST_SetSRID(ST_MakePoint(-122.4, 37.7), 4326), $2, 'good spot probe', 1)
		RETURNING id
	`, userID, fmt.Sprintf("gsp-%d", time.Now().UnixNano())).Scan(&id)
	if err != nil {
		t.Fatalf("seed pin: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM pins WHERE id = $1`, id)
	})
	return id
}

// TestDBGoodSpotTriggerCounts covers the trigger's arithmetic and the cascade
// behaviour that makes reaction rows orphan-free.
func TestDBGoodSpotTriggerCounts(t *testing.T) {
	pool := requireEndpointDB(t)
	ctx := context.Background()

	owner := seedResetUser(t, pool, fmt.Sprintf("gs-owner-%d@example.com", time.Now().UnixNano()))
	reactor := seedResetUser(t, pool, fmt.Sprintf("gs-reactor-%d@example.com", time.Now().UnixNano()))
	other := seedResetUser(t, pool, fmt.Sprintf("gs-other-%d@example.com", time.Now().UnixNano()))

	count := func(pinID string) int {
		t.Helper()
		var n int
		if err := pool.QueryRow(ctx, `SELECT good_spot_count FROM pins WHERE id = $1`, pinID).Scan(&n); err != nil {
			t.Fatalf("read good_spot_count: %v", err)
		}
		return n
	}
	// reactionRows counts the reaction rows for a pin directly, so a test can
	// compare the ground truth against the denormalised column.
	reactionRows := func(pinID string) int {
		t.Helper()
		var n int
		if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM good_spots WHERE pin_id = $1`, pinID).Scan(&n); err != nil {
			t.Fatalf("count reactions: %v", err)
		}
		return n
	}
	// assertConsistent is the invariant that must hold after every sequence:
	// the denormalised column equals the number of reaction rows.
	assertConsistent := func(pinIDs ...string) {
		t.Helper()
		for _, id := range pinIDs {
			if got, want := count(id), reactionRows(id); got != want {
				t.Fatalf("pin %s: good_spot_count = %d, but good_spots holds %d rows", id, got, want)
			}
		}
	}
	react := func(pinID, userID string) {
		t.Helper()
		if _, err := pool.Exec(ctx,
			`INSERT INTO good_spots (pin_id, user_id) VALUES ($1, $2)`, pinID, userID); err != nil {
			t.Fatalf("react: %v", err)
		}
	}
	unreact := func(pinID, userID string) {
		t.Helper()
		if _, err := pool.Exec(ctx,
			`DELETE FROM good_spots WHERE pin_id = $1 AND user_id = $2`, pinID, userID); err != nil {
			t.Fatalf("unreact: %v", err)
		}
	}

	t.Run("InsertIncrements", func(t *testing.T) {
		pin := seedTriggerPin(t, pool, owner)
		if got := count(pin); got != 0 {
			t.Fatalf("new pin count = %d, want 0", got)
		}
		react(pin, reactor)
		if got := count(pin); got != 1 {
			t.Fatalf("count = %d after one reaction, want 1", got)
		}
		react(pin, other)
		if got := count(pin); got != 2 {
			t.Fatalf("count = %d after two distinct reactors, want 2", got)
		}
		assertConsistent(pin)
	})

	// One reaction per user per pin is the primary key's job, not the
	// service's. Assert it here so a schema change cannot quietly allow
	// double-counting.
	t.Run("DuplicateReactionRejectedByPrimaryKey", func(t *testing.T) {
		pin := seedTriggerPin(t, pool, owner)
		react(pin, reactor)
		if _, err := pool.Exec(ctx,
			`INSERT INTO good_spots (pin_id, user_id) VALUES ($1, $2)`, pin, reactor); err == nil {
			t.Fatal("second identical reaction was accepted, want a primary key violation")
		}
		assertConsistent(pin)
	})

	t.Run("DeleteDecrements", func(t *testing.T) {
		pin := seedTriggerPin(t, pool, owner)
		react(pin, reactor)
		react(pin, other)
		unreact(pin, reactor)
		if got := count(pin); got != 1 {
			t.Fatalf("count = %d after one undo, want 1", got)
		}
		unreact(pin, other)
		if got := count(pin); got != 0 {
			t.Fatalf("count = %d after both undos, want 0", got)
		}
		assertConsistent(pin)
	})

	// GREATEST(count-1, 0) is the self-healing guard. A delete whose row exists
	// while the column already reads 0 (drift, or a trigger that ran twice) must
	// not push the count negative.
	t.Run("DecrementNeverGoesBelowZero", func(t *testing.T) {
		pin := seedTriggerPin(t, pool, owner)
		react(pin, reactor)
		// Simulate drift: the reaction row exists but the column says 0.
		if _, err := pool.Exec(ctx, `UPDATE pins SET good_spot_count = 0 WHERE id = $1`, pin); err != nil {
			t.Fatalf("force drift: %v", err)
		}
		unreact(pin, reactor)
		if got := count(pin); got != 0 {
			t.Fatalf("count = %d after an undo from a drifted 0, want 0 (never negative)", got)
		}
		assertConsistent(pin)
	})

	// Deleting a pin cascades its reaction rows, which fires the AFTER DELETE
	// trigger against a row that is being deleted. The trigger's UPDATE matches
	// nothing; what matters is that no reaction row survives.
	t.Run("PinDeleteLeavesNoReactionRows", func(t *testing.T) {
		pin := seedTriggerPin(t, pool, owner)
		react(pin, reactor)
		react(pin, other)
		if _, err := pool.Exec(ctx, `DELETE FROM pins WHERE id = $1`, pin); err != nil {
			t.Fatalf("delete pin: %v", err)
		}
		var left int
		if err := pool.QueryRow(ctx,
			`SELECT COUNT(*) FROM good_spots WHERE pin_id = $1`, pin).Scan(&left); err != nil {
			t.Fatalf("count surviving reactions: %v", err)
		}
		if left != 0 {
			t.Fatalf("%d reaction rows survived the pin delete, want 0", left)
		}
	})

	// Deleting a user cascades their reactions, so the counts of *other*
	// people's pins must fall by exactly what that user contributed.
	t.Run("UserDeleteDecrementsOtherPinsCounts", func(t *testing.T) {
		pinA := seedTriggerPin(t, pool, owner)
		pinB := seedTriggerPin(t, pool, other)
		react(pinA, reactor)
		react(pinA, other) // not the reactor, must survive
		react(pinB, reactor)
		if got := count(pinA); got != 2 {
			t.Fatalf("pinA = %d, want 2 before the delete", got)
		}
		if got := count(pinB); got != 1 {
			t.Fatalf("pinB = %d, want 1 before the delete", got)
		}

		if _, err := pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, reactor); err != nil {
			t.Fatalf("delete reactor: %v", err)
		}

		if got := count(pinA); got != 1 {
			t.Fatalf("pinA = %d after deleting the reactor, want 1", got)
		}
		if got := count(pinB); got != 0 {
			t.Fatalf("pinB = %d after deleting the reactor, want 0", got)
		}
		assertConsistent(pinA, pinB)
	})

	// The invariant after an arbitrary interleaving: the column always equals
	// the number of rows, whatever order the operations happened in.
	//
	// Fresh fixtures: the subtest above deletes `reactor`, and a closed-over id
	// from an earlier subtest would either 404 or trip a foreign key.
	t.Run("CountMatchesRowsAfterMixedSequence", func(t *testing.T) {
		pin := seedTriggerPin(t, pool, owner)
		reactorB := seedResetUser(t, pool, fmt.Sprintf("gs-reactor-b-%d@example.com", time.Now().UnixNano()))

		react(pin, other)
		react(pin, reactorB)
		unreact(pin, other)
		react(pin, other)
		unreact(pin, other)
		react(pin, owner) // the author may not react once the service lands;
		// this exercises the trigger only, so the insert is allowed here.
		unreact(pin, reactorB)
		react(pin, reactorB)
		react(pin, other)
		unreact(pin, owner)
		unreact(pin, owner) // no row, no trigger, count unchanged

		assertConsistent(pin)
		if got := count(pin); got != 2 {
			t.Fatalf("count = %d after the mixed sequence, want 2 (reactorB + other)", got)
		}
	})
}
