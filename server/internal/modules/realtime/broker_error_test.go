package realtime

// Coverage for the SSE subscription path against a real Redis, plus the
// failure mode that used to pass silently.
//
// Two things are pinned here:
//
//  1. Broker.Subscribe and BrokerRemoved must report a failed subscription.
//     go-redis's Client.Subscribe discards the error from PubSub.Subscribe
//     (redis.go: `_ = pubsub.Subscribe(...)`), so a subscription that could not
//     be established comes back as a *redis.PubSub that looks healthy while its
//     channel will never fire. Handler.Stream would then write 200 and
//     heartbeat forever, giving the client a stream that stays open and
//     delivers nothing while every retry does the same and holds a limiter slot.
//
//  2. The two-step subscribe used to keep that error must not break the happy
//     path: published events still arrive on the subscription.
//
// An earlier version of this file chased a different theory: that the go-redis
// pub/sub pool caps concurrent subscriptions at Options.PoolSize. It does not.
// internal/pool.PubSubPool.NewConn dials a fresh connection per subscription and
// never reads PoolSize or PoolTimeout, so there is no ceiling to hit and sizing
// that pool in di/container.go would change nothing. What is real is the cost of
// two connections per stream (Handler.Stream subscribes to two channels) and the
// swallowed error covered below.

import (
	"context"
	"net"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jojianya/sweetspot247-backend/internal/modules/pins"
	"github.com/redis/go-redis/v9"
)

// deadRedisAddr is a port nothing listens on: the dial fails fast.
const deadRedisAddr = "127.0.0.1:1"

// liveRedisAddr is the reachable Redis used by the DB-gated endpoint tests.
func liveRedisAddr() string {
	if a := os.Getenv("REDIS_ADDR"); a != "" {
		return a
	}
	return "127.0.0.1:6379"
}

// liveBroker returns a broker against a reachable Redis, or nil (with a skip
// already recorded) when there is none. Same bar as the DB-backed tests.
func liveBroker(t *testing.T) *Broker {
	t.Helper()
	broker := NewBroker(liveRedisAddr(), os.Getenv("REDIS_PASSWORD"))
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := broker.Ping(ctx); err != nil {
		t.Skipf("redis unreachable at %s: %v", liveRedisAddr(), err)
	}
	return broker
}

// TestSubscribeChannelReportsFailure is the repro for the silent dead stream:
// an unreachable Redis must produce an error, not a healthy-looking PubSub.
func TestSubscribeChannelReportsFailure(t *testing.T) {
	client := redis.NewClient(&redis.Options{Addr: deadRedisAddr, DialTimeout: time.Second})
	defer client.Close()

	sub, err := subscribeChannel(context.Background(), client, pinsChannel)
	if err == nil {
		sub.Close()
		t.Fatal("subscribeChannel returned no error for an unreachable Redis; the caller would get a subscription that never fires")
	}
}

// TestStreamReportsUnreachableBroker proves the client-visible half: the stream
// answers 503 with Retry-After instead of 200 plus a dead stream.
func TestStreamReportsUnreachableBroker(t *testing.T) {
	gin.SetMode(gin.TestMode)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	r := gin.New()
	r.GET("/events", NewHandler(NewBroker(deadRedisAddr, ""), 10).Stream)
	srv := &http.Server{Handler: r}
	go func() { _ = srv.Serve(ln) }()
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+ln.Addr().String()+"/events", nil)
	if err != nil {
		t.Fatalf("build request: %v", err)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("GET /events: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", resp.StatusCode)
	}
	if got := resp.Header.Get("Retry-After"); got == "" {
		t.Error("missing Retry-After; the browser's EventSource would reconnect immediately")
	}
}

// TestBrokerSubscribeStillDelivers proves the two-step subscribe used to keep
// the error did not break delivery: a published pin still reaches the
// subscription. Subscribing only writes the SUBSCRIBE command and does not wait
// for the server's confirmation, so this publishes on a short interval rather
// than assuming the subscription is live the instant Subscribe returns.
func TestBrokerSubscribeStillDelivers(t *testing.T) {
	broker := liveBroker(t)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pinSub, err := broker.Subscribe(ctx)
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	defer pinSub.Close()

	removed, err := broker.SubscribeRemoved(ctx)
	if err != nil {
		t.Fatalf("SubscribeRemoved: %v", err)
	}
	defer removed.Close()

	gotPin := make(chan string, 1)
	go func() {
		for msg := range pinSub.Channel() {
			gotPin <- msg.Payload
			return
		}
	}()
	gotRemoved := make(chan string, 1)
	go func() {
		for msg := range removed.Channel() {
			gotRemoved <- msg.Payload
			return
		}
	}()

	wantID := "delivery-check"
	want := `{"id":"` + wantID + `"`
	deadline := time.Now().Add(5 * time.Second)
	for {
		broker.PinCreated(ctx, pins.Event{ID: wantID, Location: "POINT(0 0)", CategoryID: 1})
		broker.PinRemoved(ctx, pins.PinRemoved{ID: wantID, Location: "POINT(0 0)"})

		pinOK, removedOK := false, false
		for !pinOK || !removedOK {
			if time.Now().After(deadline) {
				t.Fatalf("no delivery within 5s: pin=%v removed=%v", pinOK, removedOK)
			}
			select {
			case payload := <-gotPin:
				if containsSubstr(payload, want) {
					pinOK = true
				}
			case payload := <-gotRemoved:
				if containsSubstr(payload, want) {
					removedOK = true
				}
			case <-time.After(200 * time.Millisecond):
				// Re-publish on the next loop turn.
			}
		}
		if pinOK && removedOK {
			return
		}
	}
}

func containsSubstr(s, sub string) bool {
	return len(s) >= len(sub) && (s[:len(sub)] == sub || len(s) > len(sub) && containsSubstr(s[1:], sub))
}
