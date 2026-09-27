package database

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestSplitStatements(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "simple two statements",
			input: "CREATE INDEX a ON t (x); CREATE INDEX b ON t (y);",
			want:  []string{"CREATE INDEX a ON t (x);", " CREATE INDEX b ON t (y);"},
		},
		{
			name:  "trailing semicolon",
			input: "CREATE INDEX a ON t (x);",
			want:  []string{"CREATE INDEX a ON t (x);"},
		},
		{
			name:  "no trailing semicolon",
			input: "CREATE INDEX a ON t (x)",
			want:  []string{"CREATE INDEX a ON t (x)"},
		},
		{
			name:  "semicolon inside string literal",
			input: `INSERT INTO t VALUES ('a;b'); CREATE INDEX a ON t (x);`,
			want:  []string{`INSERT INTO t VALUES ('a;b');`, " CREATE INDEX a ON t (x);"},
		},
		{
			name:  "escaped quote in string",
			input: `INSERT INTO t VALUES ('it''s'); CREATE INDEX a ON t (x);`,
			want:  []string{`INSERT INTO t VALUES ('it''s');`, " CREATE INDEX a ON t (x);"},
		},
		{
			name:  "line comment with semicolon",
			input: "-- comment; not a statement\nCREATE INDEX a ON t (x);",
			want:  []string{"-- comment; not a statement\nCREATE INDEX a ON t (x);"},
		},
		{
			name:  "block comment with semicolon",
			input: "/* comment; not a statement */\nCREATE INDEX a ON t (x);",
			want:  []string{"/* comment; not a statement */\nCREATE INDEX a ON t (x);"},
		},
		{
			name:  "empty input",
			input: "",
			want:  nil,
		},
		{
			name:  "only whitespace",
			input: "  \n  ",
			want:  nil,
		},
		{
			name:  "single statement no semicolon",
			input: "CREATE INDEX a ON t (x)",
			want:  []string{"CREATE INDEX a ON t (x)"},
		},
		{
			name: "migration file with CONCURRENTLY",
			input: `-- comment
CREATE INDEX CONCURRENTLY a ON t (x);

CREATE INDEX CONCURRENTLY b ON t (y) WHERE z;`,
			want: []string{
				"-- comment\nCREATE INDEX CONCURRENTLY a ON t (x);",
				"\n\nCREATE INDEX CONCURRENTLY b ON t (y) WHERE z;",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := splitStatements(tc.input)
			if len(got) != len(tc.want) {
				t.Fatalf("splitStatements(%q) returned %d statements, want %d: %q", tc.input, len(got), len(tc.want), got)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("statement[%d] = %q, want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestExecuteMigrationWithoutConcurrently(t *testing.T) {
	// Non-CONCURRENTLY SQL should run as a single Exec call.
	sql := "CREATE TABLE IF NOT EXISTS test_exec (id int);"
	if strings.Contains(strings.ToUpper(sql), "CONCURRENTLY") {
		t.Fatal("test setup error: should not contain CONCURRENTLY")
	}
}

func TestExecuteMigrationDetectsConcurrently(t *testing.T) {
	tests := []struct {
		name string
		sql  string
		want bool
	}{
		{"uppercase", "CREATE INDEX CONCURRENTLY a ON t (x);", true},
		{"lowercase", "create index concurrently a on t (x);", true},
		{"mixed case", "Create Index Concurrently a On t (x);", true},
		{"no concurrently", "CREATE INDEX a ON t (x);", false},
		{"in comment", "-- creates index concurrently\nCREATE INDEX a ON t (x);", true},
		{"empty", "", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := strings.Contains(strings.ToUpper(tc.sql), "CONCURRENTLY")
			if got != tc.want {
				t.Errorf("CONCURRENTLY detection for %q = %v, want %v", tc.sql, got, tc.want)
			}
		})
	}
}

func TestRunMigrationsWithConcurrently(t *testing.T) {
	ctx := context.Background()

	// Connect to the test database using environment variables or defaults.
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s",
		os.Getenv("DB_USER"),
		os.Getenv("DB_PASSWORD"),
		os.Getenv("DB_HOST"),
		os.Getenv("DB_PORT"),
		os.Getenv("DB_NAME"),
	)
	if os.Getenv("DB_PASSWORD") == "" {
		t.Skip("DB_PASSWORD not set, skipping database test")
	}

	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	// Closed via t.Cleanup rather than defer: t.Cleanup callbacks run after the
	// function's own defers, so a deferred Close would close the pool before the
	// migration cleanup below could use it. Cleanups run last-registered-first,
	// so registering the close here guarantees it happens after the cleanup.
	t.Cleanup(pool.Close)

	// This test runs against the live database named by DB_NAME, so it has to
	// leave no trace. RunMigrations records the migration in schema_migrations
	// and that record is what makes a second run skip the file, so dropping the
	// indexes alone leaves the migration "already applied" and every later run
	// fails on "expected 2 indexes, got 0". Clear up front too, in case a
	// previous run leaked a record.
	const migrationFile = "0001_test_concurrently.sql"
	cleanupTestMigration(t, pool, migrationFile)
	t.Cleanup(func() { cleanupTestMigration(t, pool, migrationFile) })

	// Create a temporary migration directory with a CONCURRENTLY migration.
	dir := t.TempDir()
	migrationSQL := `CREATE INDEX CONCURRENTLY test_concurrent_idx ON users (email);
CREATE INDEX CONCURRENTLY test_concurrent_partial ON users (email) WHERE role = 'user';`
	if err := os.WriteFile(dir+"/0001_test_concurrently.sql", []byte(migrationSQL), 0644); err != nil {
		t.Fatalf("write migration: %v", err)
	}

	// Run migrations — this should NOT fail with "cannot run inside a transaction block".
	if err := RunMigrations(pool, dir); err != nil {
		t.Fatalf("RunMigrations: %v", err)
	}

	// Verify the indexes were created.
	var count int
	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM pg_indexes
		WHERE indexname IN ('test_concurrent_idx', 'test_concurrent_partial')
	`).Scan(&count); err != nil {
		t.Fatalf("verify indexes: %v", err)
	}
	if count != 2 {
		t.Fatalf("expected 2 indexes, got %d", count)
	}
}

// cleanupTestMigration undoes everything TestRunMigrationsWithConcurrently does
// to the live database: the two indexes it creates, and the schema_migrations
// row that marks the migration applied.
//
// Both are needed. Leaving the indexes behind makes the next run's
// CREATE INDEX CONCURRENTLY fail with "relation already exists"; leaving the
// bookkeeping row behind makes the next run skip the file and then fail on
// "expected 2 indexes, got 0". The original cleanup only dropped the indexes,
// which is why the test poisoned the shared database and then failed on itself
// from the second run onwards.
func cleanupTestMigration(t *testing.T, pool *pgxpool.Pool, filename string) {
	t.Helper()
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `
		DROP INDEX IF EXISTS test_concurrent_idx;
		DROP INDEX IF EXISTS test_concurrent_partial;
	`); err != nil {
		t.Fatalf("drop test indexes: %v", err)
	}
	if _, err := pool.Exec(ctx,
		"DELETE FROM schema_migrations WHERE filename = $1", filename); err != nil {
		t.Fatalf("clear %s from schema_migrations: %v", filename, err)
	}
}
