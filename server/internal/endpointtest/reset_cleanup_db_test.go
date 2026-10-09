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

// TestResetCleanupStopsOnCancelBetweenBatches proves a cancelled context
// ends the janitor between batches: some rows are deleted, but not all.
// It seeds rows larger than the batch size, starts the cleanup, cancels after
// the first batch completes, and asserts that only a batch's worth were deleted.
func TestResetCleanupStopsOnCancelBetweenBatches(t *testing.T) {
	pool := requireEndpointDB(t)
	email := fmt.Sprintf("janitor-batchcancel-%d@example.com", time.Now().UnixNano())
	uid := seedResetUser(t, pool, email)
	store := auth.NewResetStore(pool)
	ctx, cancel := context.WithCancel(context.Background())

	// Insert stale rows explicitly so the cleanup has work to do.
	if _, err := pool.Exec(ctx, `
		INSERT INTO password_resets (user_id, token_hash, expires_at, used_at)
		SELECT $1, md5('janitor-batchcancel-' || g::text), now() - interval '2 days', NULL
		FROM generate_series(1, 1001) g
	`, uid); err != nil {
		t.Fatalf("seed rows: %v", err)
	}

	// Run cleanup in a goroutine so we can cancel between batches.
	var deleted int
	done := make(chan struct{})
	go func() {
		deleted, _ = store.CleanupPasswordResets(ctx)
		close(done)
	}()

	// Cancel after a very short delay to catch the loop between iterations.
	// The loop does: select, Exec, check n < batch, loop back to select.
	// With 1001 rows and batch=1000: pass 1 deletes 1000, loop continues,
	// pass 2 deletes 1, n=1<1000, returns.
	select {
	case <-time.After(5 * time.Millisecond):
		// Cancel between passes.
	case <-done:
		// Completed instantly; cancel anyway.
	}
	cancel()

	// Wait for the goroutine to exit (it will on context cancel).
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("cleanup goroutine did not finish")
	}

	// Some rows were deleted, but not all (since we cancelled between batches).
	if deleted == 0 {
		t.Fatalf("expected some rows deleted, got 0")
	}
	if deleted >= 1001 {
		t.Fatalf("expected fewer than 1001 deleted, got %d", deleted)
	}
}
