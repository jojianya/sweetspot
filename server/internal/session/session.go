package session

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

const keyPrefix = "jwt:blacklist:"

type Blacklist struct {
	client *redis.Client
}

func New(addr string) *Blacklist {
	return &Blacklist{client: redis.NewClient(&redis.Options{Addr: addr})}
}

func (b *Blacklist) Ping(ctx context.Context) error {
	return b.client.Ping(ctx).Err()
}

func (b *Blacklist) Revoke(ctx context.Context, jti string, ttl time.Duration) error {
	if ttl <= 0 {
		return nil
	}
	return b.client.Set(ctx, keyPrefix+jti, "1", ttl).Err()
}

func (b *Blacklist) IsRevoked(ctx context.Context, jti string) (bool, error) {
	n, err := b.client.Exists(ctx, keyPrefix+jti).Result()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}