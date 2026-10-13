package realtime

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/jojianya/sweetspot247-backend/internal/modules/pins"
	"github.com/redis/go-redis/v9"
)

const pinsChannel = "goodspot:pins"
const pinRemovedChannel = "goodspot:pin-removed"

// Broker publishes pin events to a Redis pub/sub channel and hands out
// subscriptions for the SSE stream. It implements pins.Events so the create
// flow can broadcast without depending on Redis directly.
type Broker struct {
	client *redis.Client
}

// NewBroker creates a Broker with its own client. Prefer NewBrokerWithClient
// so the caller can share a single Redis connection across the app.
func NewBroker(addr, password string) *Broker {
	return &Broker{client: redis.NewClient(&redis.Options{Addr: addr, Password: password})}
}

// NewBrokerWithClient creates a Broker backed by an existing client, so the
// connection is shared rather than duplicated.
func NewBrokerWithClient(client *redis.Client) *Broker {
	return &Broker{client: client}
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

// PinRemoved publishes a pin removal event to a separate channel
// (goodspot:pin-removed, never mixed into goodspot:pins). Failures are
// logged and dropped so a Redis hiccup can never fail a removal.
func (b *Broker) PinRemoved(ctx context.Context, ev pins.PinRemoved) {
	data, err := json.Marshal(ev)
	if err != nil {
		slog.Error("realtime: marshal pin removed event", "error", err.Error())
		return
	}
	if err := b.client.Publish(ctx, pinRemovedChannel, data).Err(); err != nil {
		slog.Error("realtime: publish pin removed event", "error", err.Error())
	}
}

// PublishRemoval implements reports.Publisher structurally (same channel and
// JSON shape as PinRemoved, but with plain data so reports never imports
// pins). Kept as a separate method so reports and pins stay decoupled.
func (b *Broker) PublishRemoval(ctx context.Context, id string, location string) {
	b.PinRemoved(ctx, pins.PinRemoved{ID: id, Location: location})
}

// Subscribe opens a subscription to the pin channel. Callers must Close it.
//
// The error must not be dropped. go-redis's Client.Subscribe discards the
// error from PubSub.Subscribe (redis.go: `_ = pubsub.Subscribe(...)`), so a
// subscription that could not be established comes back as a *redis.PubSub
// that looks healthy while its channel will never fire. Subscribing in two
// steps — an empty Subscribe that cannot fail, then the explicit one that
// returns its error — keeps it, so Stream can answer 503 instead of handing
// the client a stream that stays open and delivers nothing.
func (b *Broker) Subscribe(ctx context.Context) (*redis.PubSub, error) {
	return subscribeChannel(ctx, b.client, pinsChannel)
}

// SubscribeRemoved opens a subscription to the pin removed channel.
func (b *Broker) SubscribeRemoved(ctx context.Context) (*redis.PubSub, error) {
	return subscribeChannel(ctx, b.client, pinRemovedChannel)
}

// subscribeChannel subscribes to one channel and reports any failure.
func subscribeChannel(ctx context.Context, client *redis.Client, channel string) (*redis.PubSub, error) {
	ps := client.Subscribe(ctx) // no channels: nothing to fail here
	if err := ps.Subscribe(ctx, channel); err != nil {
		ps.Close()
		return nil, err
	}
	return ps, nil
}
