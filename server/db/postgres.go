package db

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func Connect(dsn string) (*pgxpool.Pool, error) {
	const maxAttempts = 10
	const retryDelay = 3 * time.Second

	var pool *pgxpool.Pool
	var err error

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		pool, err = pgxpool.New(context.Background(), dsn)
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
