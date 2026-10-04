package endpointtest

import (
	"context"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	users "github.com/jojianya/sweetspot247-backend/internal/modules/user"
)

// TestSearchUsersTotalCountsMatches checks that a search reports the
// number of matches rather than the number of registered users, and
// that paging with limit/offset walks the matches exactly once each.
func TestSearchUsersTotalCountsMatches(t *testing.T) {
	pool := requireDB(t)
	defer pool.Close()

	repo := users.NewRepository(pool)
	ctx := context.Background()

	// Ten users, three of whose usernames contain the needle.
	const needle = "paginated"
	seeded := make([]string, 0, 10)
	for i := 0; i < 10; i++ {
		username := fmt.Sprintf("nomatch-%02d-elsewhere", i)
		if i < 3 {
			username = fmt.Sprintf("hit-%02d-%s", i, needle)
		}
		seeded = append(seeded, seedUsernameUser(ctx, t, pool, username))
	}
	defer func() {
		if _, err := pool.Exec(ctx, `DELETE FROM users WHERE id = ANY($1)`, seeded); err != nil {
			t.Errorf("cleanup: %v", err)
		}
	}()

	t.Run("TotalIsMatchCount", func(t *testing.T) {
		found, total, err := repo.SearchUsers(ctx, needle, 50, 0)
		if err != nil {
			t.Fatalf("SearchUsers: %v", err)
		}
		if total != 3 {
			t.Fatalf("expected total 3 matches, got %d", total)
		}
		if len(found) != 3 {
			t.Fatalf("expected 3 users, got %d", len(found))
		}
		for _, u := range found {
			if u.Username == "" {
				t.Fatal("empty username in results")
			}
		}
	})

	t.Run("PagesDoNotOverlap", func(t *testing.T) {
		seen := map[string]bool{}
		page := 0
		for {
			found, total, err := repo.SearchUsers(ctx, needle, 2, page*2)
			if err != nil {
				t.Fatalf("page %d: SearchUsers: %v", page, err)
			}
			if total != 3 {
				t.Fatalf("page %d: expected total 3, got %d", page, total)
			}
			if len(found) == 0 {
				break
			}
			if len(found) > 2 {
				t.Fatalf("page %d: page larger than limit: %d rows", page, len(found))
			}
			for _, u := range found {
				if seen[u.ID] {
					t.Fatalf("page %d: %s returned twice across pages", page, u.Username)
				}
				seen[u.ID] = true
			}
			page++
			if page > 5 {
				t.Fatal("paging did not terminate")
			}
		}
		if page != 2 {
			t.Fatalf("expected 2 pages of limit 2 for 3 matches, got %d", page)
		}
		if len(seen) != 3 {
			t.Fatalf("expected 3 distinct users across pages, got %d", len(seen))
		}
	})

	t.Run("EmptyPageKeepsTotal", func(t *testing.T) {
		found, total, err := repo.SearchUsers(ctx, needle, 2, 100)
		if err != nil {
			t.Fatalf("SearchUsers past the end: %v", err)
		}
		if len(found) != 0 {
			t.Fatalf("expected no users past the end, got %d", len(found))
		}
		if total != 3 {
			t.Fatalf("expected total 3 on an empty page, got %d", total)
		}
	})
}

func seedUsernameUser(ctx context.Context, t *testing.T, pool *pgxpool.Pool, username string) string {
	t.Helper()
	var id string
	err := pool.QueryRow(ctx, `
		INSERT INTO users (id, email, username, password_hash, role)
		VALUES (gen_random_uuid(), $1, $2, 'not-a-real-hash', 'user')
		RETURNING id
	`, username+"@example.com", username).Scan(&id)
	if err != nil {
		t.Fatalf("seed user %s: %v", username, err)
	}
	return id
}
