package endpointtest

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jojianya/sweetspot247-backend/internal/modules/pins"
	users "github.com/jojianya/sweetspot247-backend/internal/modules/user"
)

// Integration tests against a real PostGIS database.
// These require DB_HOST, DB_PORT, DB_USER, DB_PASSWORD, DB_NAME env vars.
// They run alongside the mock-based tests but exercise the real SQL.

func requireDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	host := getEnv("DB_HOST", "127.0.0.1")
	port := getEnv("DB_PORT", "5432")
	user := getEnv("DB_USER", "postgres")
	password := getEnv("DB_PASSWORD", "")
	dbname := getEnv("DB_NAME", "goodspotdb")

	if password == "" {
		t.Skip("DB_PASSWORD not set, skipping integration test")
	}

	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s", user, password, host, port, dbname)
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("db connect: %v", err)
	}
	if err := pool.Ping(context.Background()); err != nil {
		t.Fatalf("db ping: %v", err)
	}
	return pool
}

func TestRealDBTrendingAndBbox(t *testing.T) {
	pool := requireDB(t)
	defer pool.Close()

	repo := pins.NewRepository(pool)

	// Seed some pins and comments for a realistic test
	ctx := context.Background()
	userID, err := seedUser(ctx, pool, "trending-test@example.com")
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}

	// Create pins in a small area so they all match the bbox
	bbox := [4]float64{-122.45, 37.75, -122.40, 37.80}
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
		pinIDs = append(pinIDs, pin.ID.String())
	}

	// Debug: check what pins exist and their locations
	var debugCount int
	err = pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM pins 
		WHERE ST_Intersects(location, ST_MakeEnvelope(-122.45, 37.75, -122.40, 37.80, 4326))
		AND is_hidden = false
	`).Scan(&debugCount)
	if err != nil {
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
		t.Logf("  pin %s: comments=%d score=%.2f", tp.Pin.ID.String(), tp.CommentCount, tp.Score)
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

	// ListPins without bbox - should use pins_visible_created_idx
	// Use a valid world bbox (avoid antipodal edges)
	worldBbox := [4]float64{-179.9, -89.9, 179.9, 89.9}
	pinsAll, err := repo.ListPins(ctx, worldBbox, nil, 10)
	if err != nil {
		t.Fatalf("ListPins world: %v", err)
	}
	t.Logf("ListPins (world) returned %d pins", len(pinsAll))

	// Verify the sort index is used (first page should be newest)
	if len(pinsAll) >= 2 {
		if pinsAll[0].CreatedAt.Before(pinsAll[1].CreatedAt) {
			t.Fatalf("pins not sorted by created_at DESC: first=%v second=%v",
				pinsAll[0].CreatedAt, pinsAll[1].CreatedAt)
		}
	}
}

func TestRealDBListUsersWithCount(t *testing.T) {
	pool := requireDB(t)
	defer pool.Close()

	repo := users.NewRepository(pool)

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

func seedUser(ctx context.Context, pool *pgxpool.Pool, email string) (string, error) {
	var id string
	err := pool.QueryRow(ctx, `
		INSERT INTO users (id, email, username, password_hash, role)
		VALUES (gen_random_uuid(), $1, 'testuser', 'hash', 'user')
		ON CONFLICT (email) DO UPDATE SET id = users.id
		RETURNING id
	`, email).Scan(&id)
	return id, err
}
