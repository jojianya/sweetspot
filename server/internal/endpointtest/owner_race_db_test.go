package endpointtest

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	users "github.com/jojianya/sweetspot247-backend/internal/modules/user"
)

// TestConcurrentOwnerDemotionKeepsAnOwner demotes two different owners at
// the same moment. The count and the role write share a transaction that locks
// the owner rows, so the demotions serialize and the database never ends up
// without an owner. The interleaving is timing-dependent, so the scenario
// repeats to give the two transactions a chance to overlap.
func TestConcurrentOwnerDemotionKeepsAnOwner(t *testing.T) {
	pool := requireDB(t)
	defer pool.Close()

	repo := users.NewRepository(pool)
	ctx := context.Background()

	for attempt := 0; attempt < 25; attempt++ {
		raceDemotions(ctx, t, pool, repo, attempt)
	}
}

// raceDemotions runs one round: seed two owners, demote both at once, then
// assert the database still has an owner. The seeded pair is deleted before
// returning (including on failure) so no round leaks owners into the next.
func raceDemotions(ctx context.Context, t *testing.T, pool *pgxpool.Pool, repo users.Repository, attempt int) {
	t.Helper()

	idA := seedOwner(ctx, t, pool, fmt.Sprintf("race-owner-a-%d@example.com", attempt))
	idB := seedOwner(ctx, t, pool, fmt.Sprintf("race-owner-b-%d@example.com", attempt))
	defer func() {
		if _, err := pool.Exec(ctx, `DELETE FROM users WHERE id = ANY($1)`, []string{idA, idB}); err != nil {
			t.Errorf("attempt %d: cleanup: %v", attempt, err)
		}
	}()

	// Owners already present before this race. On a shared development
	// database other owners exist and both demotions may legitimately
	// succeed; when the seeded pair is all there is, the guard has to refuse
	// exactly one of them.
	var ownersBefore int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM users WHERE role = 'owner'`).Scan(&ownersBefore); err != nil {
		t.Fatalf("attempt %d: count owners before race: %v", attempt, err)
	}

	errs := make([]error, 2)
	var wg sync.WaitGroup
	for i, id := range []string{idA, idB} {
		wg.Add(1)
		go func(i int, id string) {
			defer wg.Done()
			_, errs[i] = repo.UpdateRole(ctx, id, users.RoleUser)
		}(i, id)
	}
	wg.Wait()

	// No demotion may fail for a reason other than the guard. Without this the
	// test would also pass when every demotion errors out and nothing is
	// demoted at all, which is how a broken lock query slipped through: the
	// loser has to see ErrCannotDemoteLastOwner, not a database error.
	var succeeded, refused int
	for _, err := range errs {
		switch {
		case err == nil:
			succeeded++
		case errors.Is(err, users.ErrCannotDemoteLastOwner):
			refused++
		default:
			t.Fatalf("attempt %d: unexpected demotion error: %v", attempt, err)
		}
	}

	// With exactly the two seeded owners the guard must reject one demotion,
	// so the pair can never both lose their role.
	if ownersBefore == 2 && (succeeded != 1 || refused != 1) {
		t.Fatalf("attempt %d: %d demotions succeeded and %d were refused, want 1 and 1 (errs: %v)",
			attempt, succeeded, refused, errs)
	}

	// The invariant itself: every owner in the database, not just the raced
	// pair, because a shared database may hold owners the test did not seed
	// and those legitimately absorb both demotions.
	var ownersAfter int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM users WHERE role = 'owner'`).Scan(&ownersAfter); err != nil {
		t.Fatalf("attempt %d: count owners after race: %v", attempt, err)
	}
	if ownersAfter < 1 {
		t.Fatalf("attempt %d: concurrent demotions left zero owners (errs: %v)", attempt, errs)
	}
}

func seedOwner(ctx context.Context, t *testing.T, pool *pgxpool.Pool, email string) string {
	t.Helper()
	var id string
	err := pool.QueryRow(ctx, `
		INSERT INTO users (id, email, username, password_hash, role)
		VALUES (gen_random_uuid(), $1, $2, 'not-a-real-hash', 'owner')
		RETURNING id
	`, email, email).Scan(&id)
	if err != nil {
		t.Fatalf("seed owner %s: %v", email, err)
	}
	return id
}
