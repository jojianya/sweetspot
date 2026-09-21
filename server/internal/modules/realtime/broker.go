package realtime

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/jojianya/sweetspot247-backend/internal/modules/pins"
	"github.com/redis/go-redis/v9"
)

const pinsChannel = "goodspot:pins"

// Broker publishes pin events to a Redis pub/sub channel and hands out
// subscriptions for the SSE stream. It implements pins.Events so the create
// flow can broadcast without depending on Redis directly.
type Broker struct {
	client *redis.Client
}

func NewBroker(addr, password string) *Broker {
	return &Broker{client: redis.NewClient(&redis.Options{Addr: addr, Password: password})}
}

// Ping verifies the Redis connection (used at startup for an informative log).
func (b *Broker) Ping(ctx context.Context) error {
	return b.client.Ping(ctx).Err()
}

// PinCreated implements pins.Events; failures are logged and dropped so a
// Redis hiccup can never fail a pin creation.
func (b *Broker) PinCreated(ctx context.Context, ev pins.Event) {
	data, err := json.Marshal(ev)
	if err != nil {
		slog.Error("realtime: marshal pin event", "error", err.Error())
		return
	}
	if err := b.client.Publish(ctx, pinsChannel, data).Err(); err != nil {
		slog.Error("realtime: publish pin event", "error", err.Error())
	}
}

// Subscribe opens a subscription to the pin channel. Callers must Close it.
func (b *Broker) Subscribe(ctx context.Context) *redis.PubSub {
	return b.client.Subscribe(ctx, pinsChannel)
}
