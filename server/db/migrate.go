package db

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

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

		_, err = pool.Exec(context.Background(), string(sql))
		if err != nil {
			return fmt.Errorf("executing %s: %w", file, err)
		}

		_, err = pool.Exec(context.Background(), "INSERT INTO schema_migrations (filename) VALUES ($1)", file)
		if err != nil {
			return fmt.Errorf("recording %s: %w", file, err)
		}

		fmt.Printf("migrated: %s\n", file)
	}

	return nil
}
