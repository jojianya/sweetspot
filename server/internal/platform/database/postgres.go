package database

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func Connect(dsn string) (*pgxpool.Pool, error) {
	return ConnectWithOptions(dsn, DefaultPoolOptions())
}

// PoolOptions bounds the pgx connection pool. Zero values select the
// defaults, so callers that do not care (tests, one-off tools) get sane
// behavior without repeating the numbers.
type PoolOptions struct {
	MaxConns          int32
	MaxConnLifetime   time.Duration
	MaxConnIdleTime   time.Duration
	HealthCheckPeriod time.Duration
}

// DefaultPoolOptions fits a small single-host deployment: enough connections
// for API + SSE + migrations without starving Postgres, with lifetime/idle
// sweeps so a managed proxy (PgBouncer-style) cannot silently kill pooled
// connections underneath us.
func DefaultPoolOptions() PoolOptions {
	return PoolOptions{
		MaxConns:          10,
		MaxConnLifetime:   30 * time.Minute,
		MaxConnIdleTime:   5 * time.Minute,
		HealthCheckPeriod: time.Minute,
	}
}

func ConnectWithOptions(dsn string, opts PoolOptions) (*pgxpool.Pool, error) {
	const maxAttempts = 10
	const retryDelay = 3 * time.Second

	def := DefaultPoolOptions()
	if opts.MaxConns <= 0 {
		opts.MaxConns = def.MaxConns
	}
	if opts.MaxConnLifetime <= 0 {
		opts.MaxConnLifetime = def.MaxConnLifetime
	}
	if opts.MaxConnIdleTime <= 0 {
		opts.MaxConnIdleTime = def.MaxConnIdleTime
	}
	if opts.HealthCheckPeriod <= 0 {
		opts.HealthCheckPeriod = def.HealthCheckPeriod
	}

	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse database DSN: %w", err)
	}
	cfg.MaxConns = opts.MaxConns
	cfg.MaxConnLifetime = opts.MaxConnLifetime
	cfg.MaxConnIdleTime = opts.MaxConnIdleTime
	cfg.HealthCheckPeriod = opts.HealthCheckPeriod

	var pool *pgxpool.Pool

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		pool, err = pgxpool.NewWithConfig(context.Background(), cfg)
		if err == nil {
			pingErr := pool.Ping(context.Background())
			if pingErr == nil {
				return pool, nil
			}
			err = pingErr
			pool.Close()
		}

		log.Printf("db connect attempt %d/%d failed: %v — retrying in %s", attempt, maxAttempts, err, retryDelay)
		time.Sleep(retryDelay)
	}

	return nil, fmt.Errorf("could not connect to database after %d attempts: %w", maxAttempts, err)
}
