package database

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

const MigrationsDir = "internal/platform/database/migrations"

func RunMigrations(pool *pgxpool.Pool, migrationsDir string) error {
	ctx := context.Background()

	_, err := pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			filename TEXT PRIMARY KEY,
			applied_at TIMESTAMPTZ DEFAULT now()
		)
	`)
	if err != nil {
		return fmt.Errorf("creating schema_migrations table: %w", err)
	}

	_, err = pool.Exec(ctx, "SELECT 1 FROM schema_migrations LIMIT 0")
	if err != nil {
		return fmt.Errorf("verifying schema_migrations table: %w", err)
	}

	applied := make(map[string]bool)
	rows, err := pool.Query(ctx, "SELECT filename FROM schema_migrations")
	if err != nil {
		return fmt.Errorf("querying applied migrations: %w", err)
	}
	for rows.Next() {
		var filename string
		if err := rows.Scan(&filename); err != nil {
			return err
		}
		applied[filename] = true
	}
	rows.Close()

	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		return fmt.Errorf("reading migrations dir: %w", err)
	}

	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)

	for _, file := range files {
		if applied[file] {
			continue
		}

		path := filepath.Join(migrationsDir, file)
		sql, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("reading %s: %w", file, err)
		}

		if err := executeMigration(pool, file, string(sql)); err != nil {
			return err
		}

		_, err = pool.Exec(context.Background(), "INSERT INTO schema_migrations (filename) VALUES ($1)", file)
		if err != nil {
			return fmt.Errorf("recording %s: %w", file, err)
		}

		fmt.Printf("migrated: %s\n", file)
	}

	if err := verifySchema(ctx, pool); err != nil {
		return err
	}

	return nil
}

// executeMigration runs a migration file. When the file contains
// CREATE INDEX CONCURRENTLY, the statements are executed individually
// because CONCURRENTLY cannot run inside a transaction block.
func executeMigration(pool *pgxpool.Pool, file, sql string) error {
	// Fast path: no CONCURRENTLY, run as before.
	if !strings.Contains(strings.ToUpper(sql), "CONCURRENTLY") {
		_, err := pool.Exec(context.Background(), sql)
		if err != nil {
			return fmt.Errorf("executing %s: %w", file, err)
		}
		return nil
	}

	// CONCURRENTLY detected: split and run each statement separately.
	statements := splitStatements(sql)
	for _, stmt := range statements {
		if strings.TrimSpace(stmt) == "" {
			continue
		}
		if _, err := pool.Exec(context.Background(), stmt); err != nil {
			return fmt.Errorf("executing %s: %w", file, err)
		}
	}
	return nil
}

// splitStatements splits a SQL string into individual statements, respecting
// comments and string literals. It is sufficient for migration files that do
// not use dollar-quoted function bodies.
func splitStatements(sql string) []string {
	var statements []string
	var current strings.Builder
	inString := false
	inLineComment := false
	inBlockComment := false

	for i := 0; i < len(sql); i++ {
		c := sql[i]

		if inLineComment {
			current.WriteByte(c)
			if c == '\n' {
				inLineComment = false
			}
			continue
		}
		if inBlockComment {
			current.WriteByte(c)
			if c == '*' && i+1 < len(sql) && sql[i+1] == '/' {
				current.WriteByte('/')
				inBlockComment = false
				i++
			}
			continue
		}
		if inString {
			current.WriteByte(c)
			if c == '\'' {
				if i+1 < len(sql) && sql[i+1] == '\'' {
					current.WriteByte('\'')
					i++
				} else {
					inString = false
				}
			}
			continue
		}

		switch c {
		case '-':
			if i+1 < len(sql) && sql[i+1] == '-' {
				inLineComment = true
				current.WriteByte(c)
				current.WriteByte('-')
				i++
			} else {
				current.WriteByte(c)
			}
		case '/':
			if i+1 < len(sql) && sql[i+1] == '*' {
				inBlockComment = true
				current.WriteByte(c)
				current.WriteByte('*')
				i++
			} else {
				current.WriteByte(c)
			}
		case '\'':
			inString = true
			current.WriteByte(c)
		case ';':
			current.WriteByte(c)
			statements = append(statements, current.String())
			current.Reset()
		default:
			current.WriteByte(c)
		}
	}

	if current.Len() > 0 {
		statements = append(statements, current.String())
	}

	// Trim whitespace and filter out empty statements.
	var result []string
	for _, s := range statements {
		trimmed := strings.TrimSpace(s)
		if trimmed != "" {
			result = append(result, s)
		}
	}

	return result
}

var requiredTables = []string{
	"users",
	"categories",
	"pins",
	"pin_photos",
	"reports",
}

// verifySchema guards against migrations silently no-op'ing (e.g. an empty
// migrations dir). If a required table is missing after all migrations ran,
// startup fails loudly instead of booting into a broken, empty database.
func verifySchema(ctx context.Context, pool *pgxpool.Pool) error {
	for _, table := range requiredTables {
		var exists bool
		if err := pool.QueryRow(ctx,
			"SELECT to_regclass($1) IS NOT NULL", "public."+table,
		).Scan(&exists); err != nil {
			return fmt.Errorf("verifying table %s: %w", table, err)
		}
		if !exists {
			return fmt.Errorf("schema verification failed: table %q missing after migrations — "+
				"the database is empty and was likely recreated or wiped", table)
		}
	}
	return nil
}
