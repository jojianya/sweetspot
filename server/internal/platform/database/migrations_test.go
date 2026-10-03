package database

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
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

// appliedMigrations returns the set of filenames currently recorded in
// schema_migrations.
//
// The test reads this instead of asking RollbackLastMigration what it did,
// because RollbackLastMigration returns only an error and reports the migration
// it undid on stdout. Diffing the table around the call is both quieter and
// more accurate: RollbackLastMigration skips migrations that have no down file
// and keeps walking, so the migration it removes is not always simply the
// highest one applied beforehand.
func appliedMigrations(t *testing.T, pool *pgxpool.Pool) map[string]bool {
	t.Helper()

	rows, err := pool.Query(context.Background(), "SELECT filename FROM schema_migrations")
	if err != nil {
		t.Fatalf("read schema_migrations: %v", err)
	}
	defer rows.Close()

	applied := make(map[string]bool)
	for rows.Next() {
		var filename string
		if err := rows.Scan(&filename); err != nil {
			t.Fatalf("scan schema_migrations: %v", err)
		}
		applied[filename] = true
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate schema_migrations: %v", err)
	}
	return applied
}

// removedFrom returns the single entry present in before but absent from
// after, which is the migration the rollback undid.
func removedFrom(t *testing.T, before, after map[string]bool) (string, bool) {
	t.Helper()

	var removed []string
	for filename := range before {
		if !after[filename] {
			removed = append(removed, filename)
		}
	}
	if len(removed) != 1 {
		sort.Strings(removed)
		t.Fatalf("expected the rollback to remove exactly 1 migration, it removed %d: %v", len(removed), removed)
	}
	return removed[0], true
}

// migrationFilesOnDisk lists the runnable migrations: the top level of
// migrations/, excluding the down/ subdirectory. RunMigrations reads them the
// same way.
func migrationFilesOnDisk(t *testing.T, dir string) []string {
	t.Helper()

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read migrations dir: %v", err)
	}

	var files []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		files = append(files, e.Name())
	}
	sort.Strings(files)
	return files
}

// assertAllApplied checks that every migration on disk has a row, so the end of
// the cycle is verified against the files rather than against a hard-coded
// count or the last few numbers. Adding 0018 needs no change here.
func assertAllApplied(t *testing.T, pool *pgxpool.Pool, dir string) {
	t.Helper()

	applied := appliedMigrations(t, pool)
	for _, name := range migrationFilesOnDisk(t, dir) {
		if !applied[name] {
			t.Errorf("after up: %s is on disk but absent from schema_migrations", name)
		}
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

// pinViewsTable reports whether pin_views exists with its (pin_id, user_id)
// primary key and both foreign keys.
func pinViewsTable(t *testing.T, pool *pgxpool.Pool) (exists, pk, fks bool) {
	t.Helper()
	ctx := context.Background()
	if err := pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pg_tables WHERE tablename = 'pin_views')`).Scan(&exists); err != nil {
		t.Fatalf("check pin_views: %v", err)
	}
	if !exists {
		return false, false, false
	}
	var pkName string
	if err := pool.QueryRow(ctx, `
		SELECT conname FROM pg_constraint
		WHERE conrelid = 'pin_views'::regclass AND contype = 'p'
	`).Scan(&pkName); err != nil {
		t.Fatalf("read pin_views pk: %v", err)
	}
	var fkCount int
	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM pg_constraint
		WHERE conrelid = 'pin_views'::regclass AND contype = 'f'
	`).Scan(&fkCount); err != nil {
		t.Fatalf("count pin_views fks: %v", err)
	}
	return true, pkName != "", fkCount == 2
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
		if exists, pk, fks := pinViewsTable(t, pool); !exists || !pk || !fks {
			t.Errorf("after up: pin_views should exist with pk+fks (exists=%v pk=%v fks=%v)", exists, pk, fks)
		}
		assertAllApplied(t, pool, dir)
	})

	// rollbackUntil rolls back one migration at a time until undoMigration has
	// been undone, checking the schema after every step.
	//
	// The loop is driven by name, not by how many migrations happen to sit at
	// the top of the table. The previous version called RollbackLastMigration
	// twice and assumed the two highest were 0016 and 0015, which was true when
	// 0016 was the last file written and stopped being true the moment 0017
	// landed: the first rollback undid 0017 instead, so the trigger was still
	// present and both subtests failed. 0018 would have broken it the same way.
	//
	// Each iteration asserts the state that belongs to the migration just
	// removed, and additionally asserts that migrations still applied are intact,
	// which is what catches a down script reaching further than it should.
	rollbackUntil := func(t *testing.T, undoMigration string) {
		t.Helper()

		for {
			before := appliedMigrations(t, pool)
			if !before[undoMigration] {
				// Already undone. Checked before rolling back so the loop
				// terminates on the target rather than on a count.
				return
			}

			if err := RollbackLastMigration(pool, dir); err != nil {
				t.Fatalf("RollbackLastMigration (targeting %s): %v", undoMigration, err)
			}
			after := appliedMigrations(t, pool)
			undone, _ := removedFrom(t, before, after)

			// A newer migration that stays applied across this step is not a
			// bug: RollbackLastMigration walks the applied list newest-first
			// and skips any migration with no down script, so a data-only
			// migration with no authored rollback is stepped over rather than
			// blocking the ones below it. So the invariant is that nothing was
			// lost, not that only the newest was removed.
			for name := range before {
				if after[name] {
					continue
				}
				if name == undone {
					continue
				}
				t.Errorf("after rolling back %s: %s also disappeared from schema_migrations", undone, name)
			}

			switch undone {
			case "0016_pins_updated_at_trigger.sql":
				if pinsUpdatedAtTriggerExists(t, pool) {
					t.Error("after rolling back 0016: pins_updated_at trigger should be gone")
				}
				if thumbnailNullable(t, pool) {
					t.Error("after rolling back 0016: 0015's NOT NULL constraint must survive")
				}

			case "0015_pin_photo_thumbnail_not_null.sql":
				if !thumbnailNullable(t, pool) {
					t.Error("after rolling back 0015: pin_photos.thumbnail_url should accept NULL again")
				}
				if pinsUpdatedAtTriggerExists(t, pool) {
					t.Error("after rolling back 0015: 0016's trigger must not be resurrected")
				}

			case "0021_pin_views.sql":
				if exists, _, _ := pinViewsTable(t, pool); exists {
					t.Error("after rolling back 0021: pin_views should be gone")
				}

			default:
				// Newer migrations are rolled back on the way to the target and
				// have no schema assertion attached here, but the loop is what
				// makes them reversible at all: each one must have a down script
				// for RollbackLastMigration to reach 0015 at all.
				t.Logf("rolled back %s on the way to %s", undone, undoMigration)
			}
		}
	}

	t.Run("DownToThumbnail", func(t *testing.T) {
		rollbackUntil(t, "0015_pin_photo_thumbnail_not_null.sql")

		if thumbnailNullable(t, pool) != true {
			t.Error("0015 was rolled back, so pin_photos.thumbnail_url should accept NULL")
		}
		if pinsUpdatedAtTriggerExists(t, pool) {
			t.Error("0016 is older than 0015, so its trigger must be gone too")
		}
		if err := RollbackLastMigration(pool, dir); err == nil {
			t.Error("expected no further rollback: 0014 and earlier have no down scripts")
		}
	})

	t.Run("UpFinal", func(t *testing.T) {
		// Everything rolled back above is pending again, so this re-applies the
		// tail of the sequence in filename order.
		if err := RunMigrations(pool, dir); err != nil {
			t.Fatalf("RunMigrations (final up): %v", err)
		}
		if thumbnailNullable(t, pool) {
			t.Error("after final up: pin_photos.thumbnail_url should be NOT NULL")
		}
		if !pinsUpdatedAtTriggerExists(t, pool) {
			t.Error("after final up: pins_updated_at trigger should exist")
		}
		if exists, pk, fks := pinViewsTable(t, pool); !exists || !pk || !fks {
			t.Errorf("after final up: pin_views should exist with pk+fks (exists=%v pk=%v fks=%v)", exists, pk, fks)
		}
		assertAllApplied(t, pool, dir)
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
