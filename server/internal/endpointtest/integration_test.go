package endpointtest

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jojianya/sweetspot247-backend/internal/modules/pins"
	users "github.com/jojianya/sweetspot247-backend/internal/modules/user"
)

// Integration tests against a real PostGIS database.
//
// They run through requireDB, which is requireEndpointDB: the disposable
// ENDPOINT_TEST_DB, migrated on demand, skipped when that variable is unset.
// These tests used to build a DSN from DB_NAME — the development database —
// and seed fixtures straight into it with no cleanup, so every local
// `go test ./...` leaked rows into a real database. They now share the one
// disposable database the rest of the package uses.

func requireDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	return requireEndpointDB(t)
}

func TestRealDBTrendingAndBbox(t *testing.T) {
	pool := requireDB(t)

	repo := pins.NewRepository(pool)

	// Seed some pins and comments for a realistic test
	ctx := context.Background()
	userID := seedUser(t, ctx, pool, "trending-test@example.com")

	// Create pins in a small area so they all match the bbox.
	//
	// bbox is [minLat, minLng, maxLat, maxLng] - the order the handler's
	// ParseBbox produces and the order ListTrending/ListPins index it as. The
	// pins below are created around 37.77 N, -122.43 W, so the latitudes go
	// first. Swapping the pairs silently asks for longitude 37.75 and latitude
	// -122.45, which is a point in the Indian Ocean: the query then returns
	// nothing for the right-looking-but-wrong reason, and the test fails on an
	// envelope that never covered the data it seeded.
	bbox := [4]float64{37.75, -122.45, 37.80, -122.40}
	pinIDs := make([]string, 0, 5)
	for i := 0; i < 5; i++ {
		pin, err := repo.CreatePin(ctx, pins.NewPin{
			UserID:     userID,
			Lat:        37.77 + float64(i)*0.001,
			Lng:        -122.43 + float64(i)*0.001,
			Caption:    strPtr(fmt.Sprintf("Trending pin %d", i)),
			CategoryID: 1,
		})
		if err != nil {
			t.Fatalf("create pin %d: %v", i, err)
		}
		pinIDs = append(pinIDs, pin.ID)
	}

	// Debug: check what pins exist and their locations
	var debugCount int
	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM pins 
		WHERE ST_Intersects(location, ST_MakeEnvelope(-122.45, 37.75, -122.40, 37.80, 4326))
		AND is_hidden = false
	`).Scan(&debugCount); err != nil {
		t.Fatalf("debug count: %v", err)
	}
	t.Logf("Pins in bbox: %d", debugCount)

	// Add comments to some pins
	for i, pid := range pinIDs {
		if i < 3 {
			for j := 0; j <= i; j++ {
				if _, err := pool.Exec(ctx, `
					INSERT INTO comments (id, pin_id, user_id, body, is_hidden)
					VALUES (gen_random_uuid(), $1, $2, 'comment ' || $3, false)
				`, pid, userID, fmt.Sprintf("%d", j)); err != nil {
					t.Fatalf("add comment: %v", err)
				}
			}
		}
	}

	// ListTrending with bbox - should use the LATERAL join and comments_pin_idx
	trending, err := repo.ListTrending(ctx, bbox, 10)
	if err != nil {
		t.Fatalf("ListTrending: %v", err)
	}
	if len(trending) == 0 {
		t.Fatal("expected trending pins, got none")
	}
	t.Logf("ListTrending returned %d pins", len(trending))

	// Verify comment counts match expectations
	for _, tp := range trending {
		t.Logf("  pin %s: comments=%d score=%.2f", tp.Pin.ID, tp.CommentCount, tp.Score)
	}

	// ListPins with bbox - should use pins_location_idx
	pinsList, err := repo.ListPins(ctx, bbox, nil, 10)
	if err != nil {
		t.Fatalf("ListPins: %v", err)
	}
	if len(pinsList) == 0 {
		t.Fatal("expected pins, got none")
	}
	t.Logf("ListPins returned %d pins", len(pinsList))

	// ListPins with a wide regional bbox - same envelope path as above but with
	// a much larger span.
	//
	// Deliberately regional, not global. PostGIS ST_Intersects on a geography
	// column against a near-global envelope gives the wrong answer regardless
	// of the data: measured on this database, an envelope of lng +/-179.9 and
	// lat +/-85 contains the seeded pins yet matches 0 of 55, while the same
	// envelope cast to geometry matches all 55. The UI cannot reach that state
	// (MapView clamps to minZoom 5, so the widest bbox it can emit is about
	// 11 degrees), which makes it a latent API issue rather than a user-visible
	// one. Do not "fix" this by widening the box - the assertion would then be
	// asserting broken behaviour.
	wideBbox := [4]float64{30, -130, 45, -110}
	pinsAll, err := repo.ListPins(ctx, wideBbox, nil, 10)
	if err != nil {
		t.Fatalf("ListPins wide: %v", err)
	}
	// The test seeds 5 pins itself, so a box covering the seeded region must see
	// them. An empty result means the envelope missed the rows, which is
	// exactly what the swapped pair produced.
	if len(pinsAll) < len(pinIDs) {
		t.Fatalf("expected the wide bbox to cover the %d seeded pins, got %d", len(pinIDs), len(pinsAll))
	}
	t.Logf("ListPins (wide) returned %d pins", len(pinsAll))

	// Verify ordering (first page should be newest)
	if len(pinsAll) < 2 {
		t.Fatalf("need at least 2 pins to check ordering, got %d", len(pinsAll))
	}
	if pinsAll[0].CreatedAt.Before(pinsAll[1].CreatedAt) {
		t.Fatalf("pins not sorted by created_at DESC: first=%v second=%v",
			pinsAll[0].CreatedAt, pinsAll[1].CreatedAt)
	}
}

func TestRealDBListUsersWithCount(t *testing.T) {
	pool := requireDB(t)

	repo := users.NewRepository(pool)

	// Seed one fixture account so the assertion below means "the query sees the
	// row we just inserted" rather than "some other test happened to run
	// first". The disposable database starts empty, and the old version of this
	// test relied on leftovers in the development database.
	seedUser(t, context.Background(), pool, "list-users-test@example.com")

	// Test with data
	usersPage, total, err := repo.ListUsers(context.Background(), 5, 0)
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	if total == 0 {
		t.Fatal("expected users, got empty table")
	}
	if len(usersPage) == 0 {
		t.Fatal("expected users in first page")
	}
	if total < len(usersPage) {
		t.Fatalf("total (%d) less than page size (%d)", total, len(usersPage))
	}
	t.Logf("ListUsers: page=%d total=%d", len(usersPage), total)

	// Test empty page fallback
	_, total2, err := repo.ListUsers(context.Background(), 5, 10000)
	if err != nil {
		t.Fatalf("ListUsers empty page: %v", err)
	}
	if total2 != total {
		t.Fatalf("empty page total (%d) differs from first page total (%d)", total2, total)
	}
	t.Logf("Empty page fallback total=%d", total2)
}

// Helper to get env or default
func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// seedUser inserts a fixture account and registers a cleanup that removes it
// and everything it owns, so a DB-backed test leaves the database as it found
// it. It takes t so the cleanup is tied to this test's lifetime.
//
// The username is derived from the email rather than hardcoded, because
// users.username is UNIQUE (0002_users.sql) while ON CONFLICT (email) only
// covers an email collision — two tests seeding 'testuser' would collide on the
// username and the second would fail.
//
// The pins are deleted before the user, not after: pins.user_id is
// ON DELETE SET NULL (0003_pins.sql), so deleting the user alone would leave
// the fixture pins behind with a NULL author. Deleting the pins first also
// takes their comments with them, since comments.pin_id is ON DELETE CASCADE.
//
// Cleanup ordering: requireEndpointDB registers pool.Close as a t.Cleanup
// before this one runs, and cleanups fire last-registered-first, so this
// cleanup runs while the pool is still open. Callers must therefore NOT also
// `defer pool.Close()` — that would close the pool first and every cleanup
// after it would fail.
func seedUser(t *testing.T, ctx context.Context, pool *pgxpool.Pool, email string) string {
	t.Helper()
	username := "tu-" + strings.SplitN(email, "@", 2)[0]
	var id string
	err := pool.QueryRow(ctx, `
		INSERT INTO users (id, email, username, password_hash, role)
		VALUES (gen_random_uuid(), $1, $2, 'hash', 'user')
		ON CONFLICT (email) DO UPDATE SET id = users.id
		RETURNING id
	`, email, username).Scan(&id)
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	t.Cleanup(func() {
		// A fresh context: the test's own ctx may already be cancelled by the
		// time cleanups run, and a cleanup that cannot connect would leak.
		cleanup := context.Background()
		if _, err := pool.Exec(cleanup, `DELETE FROM pins WHERE user_id = $1`, id); err != nil {
			t.Errorf("cleanup test pins: %v", err)
		}
		if _, err := pool.Exec(cleanup, `DELETE FROM users WHERE id = $1`, id); err != nil {
			t.Errorf("cleanup test user: %v", err)
		}
	})
	return id
}
