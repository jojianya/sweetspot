package endpointtest

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	users "github.com/jojianya/sweetspot247-backend/internal/modules/user"
)

// TestConcurrentOwnerDemotionKeepsAnOwner demotes two different
// owners at the same moment while exactly two owners exist. The count
// and the role write share a transaction that locks the owner rows, so
// the demotions serialize and at least one owner must remain. The
// interleaving is timing-dependent, so the scenario repeats to give
// the two transactions a chance to overlap.
func TestConcurrentOwnerDemotionKeepsAnOwner(t *testing.T) {
	pool := requireDB(t)
	defer pool.Close()

	repo := users.NewRepository(pool)
	ctx := context.Background()

	for attempt := 0; attempt < 25; attempt++ {
		raceDemotions(ctx, t, pool, repo, attempt)
	}
}

// raceDemotions runs one round: seed two owners, demote both at once,
// then assert one of them is still an owner. The seeded pair is
// deleted before returning (including on failure) so each round sees
// exactly two owners.
func raceDemotions(ctx context.Context, t *testing.T, pool *pgxpool.Pool, repo users.Repository, attempt int) {
	t.Helper()

	idA := seedOwner(ctx, t, pool, fmt.Sprintf("race-owner-a-%d@example.com", attempt))
	idB := seedOwner(ctx, t, pool, fmt.Sprintf("race-owner-b-%d@example.com", attempt))
	defer func() {
		if _, err := pool.Exec(ctx, `DELETE FROM users WHERE id = ANY($1)`, []string{idA, idB}); err != nil {
			t.Errorf("attempt %d: cleanup: %v", attempt, err)
		}
	}()

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

	// Count only the two raced users: the database may hold other
	// owners from earlier tests, and those would hide a lost race.
	var owners int
	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM users WHERE id = ANY($1) AND role = 'owner'
	`, []string{idA, idB}).Scan(&owners); err != nil {
		t.Fatalf("attempt %d: count seeded owners: %v", attempt, err)
	}
	if owners < 1 {
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