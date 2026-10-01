package database

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// testPool connects to the database named by MIGRATION_TEST_DB.
//
// This test deliberately does NOT fall back to DB_NAME the way
// TestRunMigrationsWithConcurrently does. It writes rows and flips constraints,
// and the migrations it touches apply process-wide to whatever database they
// run against; pointing it at the development database by accident would
// leave that database with a rolled-back schema. Requiring an explicitly named
// disposable database means the test either runs somewhere safe or skips.
func testPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	name := os.Getenv("MIGRATION_TEST_DB")
	if name == "" {
		t.Skip("MIGRATION_TEST_DB not set; set it to a disposable database to run")
	}
	if os.Getenv("DB_PASSWORD") == "" {
		t.Skip("DB_PASSWORD not set, skipping database test")
	}

	ctx := context.Background()
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s",
		os.Getenv("DB_USER"), os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_HOST"), os.Getenv("DB_PORT"), name)

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect to %s: %v", name, err)
	}
	t.Cleanup(pool.Close)

	return pool
}

// migrationsPath resolves MigrationsDir for a test.
//
// MigrationsDir is relative to the server module root because that is the
// working directory the binary runs in, but `go test` runs each test with the
// package directory as its working directory. Walking up to the module root
// keeps the test honest about which files it applies rather than silently
// finding nothing.
func migrationsPath(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return filepath.Join(dir, MigrationsDir)
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not find module root above %s", dir)
		}
		dir = parent
	}
}

// thumbnailNullable reports whether pin_photos.thumbnail_url currently accepts NULL.
func thumbnailNullable(t *testing.T, pool *pgxpool.Pool) bool {
	t.Helper()
	var nullable string
	err := pool.QueryRow(context.Background(), `
		SELECT is_nullable FROM information_schema.columns
		WHERE table_name = 'pin_photos' AND column_name = 'thumbnail_url'
	`).Scan(&nullable)
	if err != nil {
		t.Fatalf("read thumbnail_url nullability: %v", err)
	}
	return nullable == "YES"
}

func pinsUpdatedAtTriggerExists(t *testing.T, pool *pgxpool.Pool) bool {
	t.Helper()
	var exists bool
	err := pool.QueryRow(context.Background(), `
		SELECT EXISTS (
			SELECT 1 FROM pg_trigger
			WHERE tgrelid = 'pins'::regclass AND tgname = 'pins_updated_at' AND NOT tgisinternal
		)
	`).Scan(&exists)
	if err != nil {
		t.Fatalf("check pins_updated_at trigger: %v", err)
	}
	return exists
}

// seedPin creates a throwaway user, category and pin with one photo, and
// removes all of them when the test ends.
func seedPin(t *testing.T, pool *pgxpool.Pool) (pinID string) {
	t.Helper()
	ctx := context.Background()

	if err := pool.QueryRow(ctx,
		"INSERT INTO users (email, password_hash, username) VALUES ($1, $2, $3) RETURNING id",
		"trigger-test@example.com", "not-a-real-hash", "trigger-test-user").Scan(&pinID); err != nil {
		t.Fatalf("seed user: %v", err)
	}
	var userID string
	if err := pool.QueryRow(ctx,
		"SELECT id FROM users WHERE email = $1", "trigger-test@example.com").Scan(&userID); err != nil {
		t.Fatalf("read seeded user: %v", err)
	}
	t.Cleanup(func() {
		// pin_photos and pins cascade from the user; deleting the user is enough.
		if _, err := pool.Exec(ctx, "DELETE FROM users WHERE email = $1", "trigger-test@example.com"); err != nil {
			t.Errorf("cleanup seeded user: %v", err)
		}
	})

	var categoryID int
	if err := pool.QueryRow(ctx, "SELECT id FROM categories LIMIT 1").Scan(&categoryID); err != nil {
		t.Fatalf("read a category: %v", err)
	}

	err := pool.QueryRow(ctx, `
		INSERT INTO pins (user_id, location, geohash, caption, category_id)
		VALUES ($1, ST_SetSRID(ST_MakePoint(4.9, 52.4), 4326)::geography, 'u4pr', $2, $3)
		RETURNING id
	`, userID, "trigger test pin", categoryID).Scan(&pinID)
	if err != nil {
		t.Fatalf("seed pin: %v", err)
	}

	return pinID
}

func TestMigrationUpDownUp(t *testing.T) {
	pool := testPool(t)
	ctx := context.Background()
	dir := migrationsPath(t)

	t.Run("Up", func(t *testing.T) {
		if err := RunMigrations(pool, dir); err != nil {
			t.Fatalf("RunMigrations: %v", err)
		}

		if thumbnailNullable(t, pool) {
			t.Error("after up: pin_photos.thumbnail_url should be NOT NULL")
		}
		if !pinsUpdatedAtTriggerExists(t, pool) {
			t.Error("after up: pins_updated_at trigger should exist")
		}
	})

	t.Run("DownTrigger", func(t *testing.T) {
		// Rollback is one-at-a-time from the highest applied migration, so
		// reaching 0015 means rolling 0016 back and leaving it that way.
		if err := RollbackLastMigration(pool, dir); err != nil {
			t.Fatalf("RollbackLastMigration: %v", err)
		}
		if pinsUpdatedAtTriggerExists(t, pool) {
			t.Error("after down: pins_updated_at trigger should be gone")
		}
		if thumbnailNullable(t, pool) {
			t.Error("after down: rolling back 0016 must not drop 0015's NOT NULL constraint")
		}
	})

	t.Run("DownThumbnail", func(t *testing.T) {
		if err := RollbackLastMigration(pool, dir); err != nil {
			t.Fatalf("RollbackLastMigration: %v", err)
		}
		if !thumbnailNullable(t, pool) {
			t.Error("after down: pin_photos.thumbnail_url should accept NULL again")
		}
		if pinsUpdatedAtTriggerExists(t, pool) {
			t.Error("after down: 0015's rollback must not resurrect 0016's trigger")
		}
	})

	t.Run("UpFinal", func(t *testing.T) {
		// Both migrations are pending again, so this re-applies 0015 then 0016.
		if err := RunMigrations(pool, dir); err != nil {
			t.Fatalf("RunMigrations (final up): %v", err)
		}
		if thumbnailNullable(t, pool) {
			t.Error("after final up: pin_photos.thumbnail_url should be NOT NULL")
		}
		if !pinsUpdatedAtTriggerExists(t, pool) {
			t.Error("after final up: pins_updated_at trigger should exist")
		}
	})

	t.Run("ThumbnailConstraintRejectsNull", func(t *testing.T) {
		pinID := seedPin(t, pool)
		_, err := pool.Exec(ctx, `
			INSERT INTO pin_photos (pin_id, photo_url, thumbnail_url, position)
			VALUES ($1, 'https://example.test/full.jpg', NULL, 99)
		`, pinID)
		if err == nil {
			t.Fatal("expected insert with NULL thumbnail_url to be rejected")
		}
	})

	t.Run("TriggerSkipsViewIncrement", func(t *testing.T) {
		pinID := seedPin(t, pool)

		var before time.Time
		if err := pool.QueryRow(ctx,
			"SELECT updated_at FROM pins WHERE id = $1", pinID).Scan(&before); err != nil {
			t.Fatalf("read updated_at before: %v", err)
		}

		// Same shape as RegisterView in pins/repository.go.
		if _, err := pool.Exec(ctx,
			"UPDATE pins SET views = views + 1 WHERE id = $1", pinID); err != nil {
			t.Fatalf("view increment: %v", err)
		}

		var after time.Time
		var views int64
		if err := pool.QueryRow(ctx,
			"SELECT updated_at, views FROM pins WHERE id = $1", pinID).Scan(&after, &views); err != nil {
			t.Fatalf("read updated_at after: %v", err)
		}
		if views != 1 {
			t.Fatalf("expected views to be 1, got %d", views)
		}
		if !after.Equal(before) {
			t.Errorf("view increment must not touch updated_at: before %v, after %v", before, after)
		}
	})

	t.Run("TriggerBumpsOnEdit", func(t *testing.T) {
		pinID := seedPin(t, pool)

		var before time.Time
		if err := pool.QueryRow(ctx,
			"SELECT updated_at FROM pins WHERE id = $1", pinID).Scan(&before); err != nil {
			t.Fatalf("read updated_at before: %v", err)
		}

		var categoryID int
		if err := pool.QueryRow(ctx, "SELECT id FROM categories LIMIT 1").Scan(&categoryID); err != nil {
			t.Fatalf("read a category: %v", err)
		}
		// Same shape as UpdatePin, minus its explicit updated_at = now(), so
		// this proves the trigger alone is what maintains the column.
		if _, err := pool.Exec(ctx,
			"UPDATE pins SET caption = $1 WHERE id = $2", "edited caption", pinID); err != nil {
			t.Fatalf("edit pin: %v", err)
		}

		var after time.Time
		if err := pool.QueryRow(ctx,
			"SELECT updated_at FROM pins WHERE id = $1", pinID).Scan(&after); err != nil {
			t.Fatalf("read updated_at after: %v", err)
		}
		if !after.After(before) {
			t.Errorf("edit must advance updated_at: before %v, after %v", before, after)
		}
	})

	t.Run("TriggerBumpsOnSoftDelete", func(t *testing.T) {
		pinID := seedPin(t, pool)

		var before time.Time
		if err := pool.QueryRow(ctx,
			"SELECT updated_at FROM pins WHERE id = $1", pinID).Scan(&before); err != nil {
			t.Fatalf("read updated_at before: %v", err)
		}

		// Same shape as DeletePin.
		if _, err := pool.Exec(ctx,
			"UPDATE pins SET is_hidden = true WHERE id = $1", pinID); err != nil {
			t.Fatalf("soft delete: %v", err)
		}

		var after time.Time
		if err := pool.QueryRow(ctx,
			"SELECT updated_at FROM pins WHERE id = $1", pinID).Scan(&after); err != nil {
			t.Fatalf("read updated_at after: %v", err)
		}
		if !after.After(before) {
			t.Errorf("soft delete must advance updated_at: before %v, after %v", before, after)
		}
	})
}
