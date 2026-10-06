package database

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

const MigrationsDir = "internal/platform/database/migrations"

// downSuffix marks a rollback script in DownMigrationsDir. A rollback is named
// after the migration it reverts with .sql swapped for .down.sql, so
// 0016_pins_updated_at_trigger.down.sql rolls back 0016_pins_updated_at_trigger.sql.
const downSuffix = ".down.sql"

// downFileFor maps an applied migration filename to its rollback filename.
func downFileFor(migrationFile string) string {
	return strings.TrimSuffix(migrationFile, ".sql") + downSuffix
}

// downDirName is the sibling directory holding rollback scripts. Keeping them
// out of the main directory matters: RunMigrations applies every .sql file it
// finds there, so a rollback script sitting alongside its migration would be
// executed immediately after it.
const downDirName = "down"

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
	defer rows.Close()
	for rows.Next() {
		var filename string
		if err := rows.Scan(&filename); err != nil {
			return err
		}
		applied[filename] = true
	}
	if err := rows.Err(); err != nil {
		return err
	}

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
// comments, string literals and dollar-quoted bodies. It is sufficient for
// migration files that do not nest dollar quotes inside dollar-quoted bodies,
// which plpgsql does not allow anyway.
func splitStatements(sql string) []string {
	var statements []string
	var current strings.Builder
	inString := false
	inLineComment := false
	inBlockComment := false
	var dollarTag string // non-empty while inside a $tag$ ... $tag$ body

	for i := 0; i < len(sql); i++ {
		c := sql[i]

		// Dollar-quoted bodies swallow everything verbatim until the closing
		// tag, semicolons and comment markers included. This has to be checked
		// before the single-quote and comment cases, otherwise a plpgsql body
		// like BEGIN ... ; ... END is chopped into fragments that each fail to
		// parse.
		if dollarTag != "" {
			current.WriteByte(c)
			if c == '$' && strings.HasPrefix(sql[i:], dollarTag) {
				current.WriteString(dollarTag[1:])
				i += len(dollarTag) - 1
				dollarTag = ""
			}
			continue
		}

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
		case '$':
			// $1-style placeholders are not quotes; only $...$ is. The tag is
			// an optional identifier, so the empty $$ form is the common case.
			if tag, ok := dollarQuoteTag(sql[i:]); ok {
				dollarTag = tag
				current.WriteString(tag)
				i += len(tag) - 1
			} else {
				current.WriteByte(c)
			}
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

// dollarQuoteTag reports the complete $tag$ delimiter starting at s[0] == '$',
// including both dollar signs. It returns false when the dollar sign does not
// open a dollar-quoted string, which is the case for positional placeholders
// such as $1 and for a bare `$`.
func dollarQuoteTag(s string) (string, bool) {
	for i := 1; i < len(s); i++ {
		c := s[i]
		if c == '$' {
			return s[:i+1], true
		}
		isWord := c == '_' ||
			(c >= 'a' && c <= 'z') ||
			(c >= 'A' && c <= 'Z') ||
			(i > 1 && c >= '0' && c <= '9')
		if !isWord {
			return "", false
		}
	}
	return "", false
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

// RollbackLastMigration reverts the most recently applied migration that ships
// a rollback script, then removes its schema_migrations row so a subsequent
// RunMigrations re-applies it.
//
// It refuses to roll back a migration with no .down.sql script rather than
// silently marking it unapplied. Forgetting the bookkeeping row while leaving
// the schema change in place is the failure mode this guards against: the
// migration looks pending forever and re-running up on a database that already
// has the change fails on "already exists".
func RollbackLastMigration(pool *pgxpool.Pool, migrationsDir string) error {
	ctx := context.Background()

	var applied []string
	rows, err := pool.Query(ctx,
		"SELECT filename FROM schema_migrations WHERE filename NOT LIKE '%"+"."+downSuffix+"' ORDER BY filename DESC")
	if err != nil {
		return fmt.Errorf("querying applied migrations: %w", err)
	}
	for rows.Next() {
		var filename string
		if err := rows.Scan(&filename); err != nil {
			rows.Close()
			return err
		}
		applied = append(applied, filename)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	if len(applied) == 0 {
		return fmt.Errorf("no applied migrations to roll back")
	}

	for _, file := range applied {
		downPath := filepath.Join(migrationsDir, downDirName, downFileFor(file))
		sql, err := os.ReadFile(downPath)
		if errors.Is(err, os.ErrNotExist) {
			// No rollback authored for this one. Keep walking down the list so a
			// recent data migration does not permanently block reverting anything.
			continue
		}
		if err != nil {
			return fmt.Errorf("reading rollback for %s: %w", file, err)
		}

		if err := executeMigration(pool, file+" (down)", string(sql)); err != nil {
			return err
		}
		if _, err := pool.Exec(ctx,
			"DELETE FROM schema_migrations WHERE filename = $1", file); err != nil {
			return fmt.Errorf("clearing %s from schema_migrations: %w", file, err)
		}

		fmt.Printf("rolled back: %s\n", file)
		return nil
	}

	return fmt.Errorf("no applied migration has a rollback script in %s", filepath.Join(downDirName, "*"+downSuffix))
}
