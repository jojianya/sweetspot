package endpointtest

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/jojianya/sweetspot247-backend/internal/modules/auth"
	"github.com/jojianya/sweetspot247-backend/pkg/password"
)

// TestDBResetCleanupDeletesOnlyDeadRows proves the janitor removes used and
// long-expired rows while a live token issued through the real path still
// consumes afterwards: valid credentials are never collected.
func TestDBResetCleanupDeletesOnlyDeadRows(t *testing.T) {
	pool := requireEndpointDB(t)
	email := fmt.Sprintf("janitor-%d@example.com", time.Now().UnixNano())
	uid := seedResetUser(t, pool, email)
	store := auth.NewResetStore(pool)
	ctx := context.Background()

	if _, err := pool.Exec(ctx, `
		INSERT INTO password_resets (user_id, token_hash, expires_at, used_at)
		SELECT $1, md5('janitor-used-' || g::text), now() - interval '2 hours', now() - interval '1 hour'
		FROM generate_series(1, 3) g
	`, uid); err != nil {
		t.Fatalf("seed used rows: %v", err)
	}
	if _, err := pool.Exec(ctx, `
		INSERT INTO password_resets (user_id, token_hash, expires_at, used_at)
		SELECT $1, md5('janitor-stale-' || g::text), now() - interval '2 days', NULL
		FROM generate_series(1, 4) g
	`, uid); err != nil {
		t.Fatalf("seed stale rows: %v", err)
	}
	raw, _, err := store.IssueToken(ctx, uid)
	if err != nil {
		t.Fatalf("issue live token: %v", err)
	}

	deleted, err := store.CleanupPasswordResets(ctx)
	if err != nil {
		t.Fatalf("cleanup: %v", err)
	}
	if deleted != 7 {
		t.Fatalf("deleted = %d, want 7 (3 used + 4 stale)", deleted)
	}

	// The live token must still work: the janitor never takes valid rows.
	newHash, err := password.Hash("brandnewpassword123")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if _, err := store.ConsumeToken(ctx, raw, newHash); err != nil {
		t.Fatalf("live token should still consume after cleanup: %v", err)
	}
}

// TestDBResetCleanupBatchesBigBackfills proves a backfill larger than one
// batch is fully collected across passes.
func TestDBResetCleanupBatchesBigBackfills(t *testing.T) {
	pool := requireEndpointDB(t)
	email := fmt.Sprintf("janitor-batch-%d@example.com", time.Now().UnixNano())
	uid := seedResetUser(t, pool, email)
	store := auth.NewResetStore(pool)
	ctx := context.Background()

	if _, err := pool.Exec(ctx, `
		INSERT INTO password_resets (user_id, token_hash, expires_at, used_at)
		SELECT $1, md5('janitor-batch-' || g::text), now() - interval '2 days', NULL
		FROM generate_series(1, 2500) g
	`, uid); err != nil {
		t.Fatalf("seed backfill: %v", err)
	}

	deleted, err := store.CleanupPasswordResets(ctx)
	if err != nil {
		t.Fatalf("cleanup: %v", err)
	}
	if deleted != 2500 {
		t.Fatalf("deleted = %d, want 2500 across batches", deleted)
	}
}

// TestResetCleanupStopsOnCancel proves a cancelled context ends the janitor
// at once with no error and no work: shutdown is not a failure.
func TestResetCleanupStopsOnCancel(t *testing.T) {
	store := auth.NewResetStore(nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	deleted, err := store.CleanupPasswordResets(ctx)
	if err != nil {
		t.Fatalf("cancelled cleanup must not error: %v", err)
	}
	if deleted != 0 {
		t.Fatalf("cancelled cleanup deleted = %d, want 0", deleted)
	}
}

// TestResetCleanupStopsOnCancelBetweenBatches proves a cancelled context ends
// the janitor between batches: the passes that ran are kept, the ones that
// never start are not, and shutdown is not reported as a failure.
//
// It cancels from AfterBatch, which runs immediately after a pass commits, so
// the next loop iteration sees the done context at its top. No timer is
// involved: batch 100 against 250 dead rows deletes exactly two full passes
// before the cancel lands, deterministically.
func TestResetCleanupStopsOnCancelBetweenBatches(t *testing.T) {
	pool := requireEndpointDB(t)
	email := fmt.Sprintf("janitor-batchcancel-%d@example.com", time.Now().UnixNano())
	uid := seedResetUser(t, pool, email)
	ctx, cancel := context.WithCancel(context.Background())

	if _, err := pool.Exec(ctx, `
		INSERT INTO password_resets (user_id, token_hash, expires_at, used_at)
		SELECT $1, md5('janitor-batchcancel-' || g::text), now() - interval '2 days', NULL
		FROM generate_series(1, 250) g
	`, uid); err != nil {
		t.Fatalf("seed rows: %v", err)
	}

	cancelled := 0
	store := auth.NewResetStore(pool)
	store.BatchSize = 100
	store.AfterBatch = func() {
		// Cancel after the second pass, so the loop stops before the third.
		cancelled++
		if cancelled >= 2 {
			cancel()
		}
	}

	deleted, err := store.CleanupPasswordResets(ctx)
	if err != nil {
		t.Fatalf("cancelled cleanup must not error: %v", err)
	}
	cancel()

	// ctx is done now, so verification queries use a fresh context.
	verify := context.Background()

	// Two full batches ran before the cancel landed.
	if deleted != 200 {
		t.Fatalf("deleted = %d, want 200 (two batches of 100)", deleted)
	}
	if deleted >= 250 {
		t.Fatalf("deleted = %d, want fewer than the 250 seeded", deleted)
	}
	// The 50 rows the janitor never reached are still there.
	var remaining int
	if err := pool.QueryRow(verify,
		`SELECT count(*) FROM password_resets WHERE user_id = $1`, uid).Scan(&remaining); err != nil {
		t.Fatalf("count remaining: %v", err)
	}
	if remaining != 50 {
		t.Fatalf("remaining = %d, want 50 rows the janitor never reached", remaining)
	}
}
