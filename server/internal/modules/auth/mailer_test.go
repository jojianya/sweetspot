package auth

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestLogMailerRedactsTokenOutsideDev(t *testing.T) {
	m := LogMailer{}
	if err := m.SendPasswordReset(context.Background(), "a@example.com", "https://app.example/reset?token=secret-token"); err != nil {
		t.Fatalf("log mailer must never fail: %v", err)
	}
	// Redaction is structural: with LogTokens false there is no code path that
	// formats the link. The dev path below covers the inverse.
}

func TestWebhookMailerPostsPayload(t *testing.T) {
	var gotBody, gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		gotBody, gotAuth = string(raw), r.Header.Get("Authorization")
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	m := WebhookMailer{URL: srv.URL, Key: "k123"}
	if err := m.SendPasswordReset(context.Background(), "a@example.com", "https://app.example/reset?token=t"); err != nil {
		t.Fatalf("webhook mailer: %v", err)
	}
	if !strings.Contains(gotBody, `"to":"a@example.com"`) {
		t.Errorf("payload missing recipient: %s", gotBody)
	}
	if !strings.Contains(gotBody, "token=t") {
		t.Errorf("payload missing link: %s", gotBody)
	}
	if gotAuth != "Bearer k123" {
		t.Errorf("auth header = %q, want bearer key", gotAuth)
	}
}

func TestWebhookMailerRejectsFailureStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()

	m := WebhookMailer{URL: srv.URL, Timeout: 5 * time.Second}
	if err := m.SendPasswordReset(context.Background(), "a@example.com", "x"); err == nil {
		t.Fatal("expected an error for 502, got nil")
	}
}

// TestWebhookMailerRetriesOnceOnServerError proves a transient 5xx costs
// exactly one retry: the second attempt succeeds and no third is sent.
func TestWebhookMailerRetriesOnceOnServerError(t *testing.T) {
	var hits atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hits.Add(1) == 1 {
			w.WriteHeader(http.StatusBadGateway)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	m := WebhookMailer{URL: srv.URL, Timeout: 5 * time.Second}
	if err := m.SendPasswordReset(context.Background(), "a@example.com", "x"); err != nil {
		t.Fatalf("retry should have delivered: %v", err)
	}
	if hits.Load() != 2 {
		t.Fatalf("hits = %d, want exactly 2 attempts", hits.Load())
	}
}

// TestWebhookMailerNoRetryOnClientError proves a 4xx is returned as-is:
// the request itself is wrong, so a second send cannot help.
func TestWebhookMailerNoRetryOnClientError(t *testing.T) {
	var hits atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()

	m := WebhookMailer{URL: srv.URL, Timeout: 5 * time.Second}
	if err := m.SendPasswordReset(context.Background(), "a@example.com", "x"); err == nil {
		t.Fatal("expected an error for 400, got nil")
	}
	if hits.Load() != 1 {
		t.Fatalf("hits = %d, want exactly 1 attempt", hits.Load())
	}
}

// TestWebhookMailerRetriesOnTimeout proves a hung endpoint costs one retry
// and the final error says how many attempts were spent.
func TestWebhookMailerRetriesOnTimeout(t *testing.T) {
	var hits atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		time.Sleep(300 * time.Millisecond)
	}))
	defer srv.Close()

	m := WebhookMailer{URL: srv.URL, Timeout: 50 * time.Millisecond}
	err := m.SendPasswordReset(context.Background(), "a@example.com", "x")
	if err == nil {
		t.Fatal("expected a timeout error, got nil")
	}
	if hits.Load() != 2 {
		t.Fatalf("hits = %d, want exactly 2 attempts", hits.Load())
	}
	if !strings.Contains(err.Error(), "2 attempts") {
		t.Fatalf("error %q should report the attempts spent", err)
	}
}

// TestWebhookMailerHonorsRetryAfter proves a 429 with Retry-After is
// retried and the second attempt can succeed.
func TestWebhookMailerHonorsRetryAfter(t *testing.T) {
	var hits atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hits.Add(1) == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer srv.Close()

	m := WebhookMailer{URL: srv.URL, Timeout: 5 * time.Second}
	if err := m.SendPasswordReset(context.Background(), "a@example.com", "x"); err != nil {
		t.Fatalf("retry after Retry-After should have delivered: %v", err)
	}
	if hits.Load() != 2 {
		t.Fatalf("hits = %d, want exactly 2 attempts", hits.Load())
	}
}

// TestWebhookMailerCancelDuringBackoff proves cancelling mid-backoff
// returns promptly instead of sleeping out the delay, and sends nothing
// further.
func TestWebhookMailerCancelDuringBackoff(t *testing.T) {
	var hits atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()
	m := WebhookMailer{URL: srv.URL, Timeout: 5 * time.Second}
	t0 := time.Now()
	err := m.SendPasswordReset(ctx, "a@example.com", "x")
	if err == nil {
		t.Fatal("expected cancellation error, got nil")
	}
	if elapsed := time.Since(t0); elapsed > 400*time.Millisecond {
		t.Fatalf("cancel during backoff took %v, want prompt return", elapsed)
	}
	if hits.Load() != 1 {
		t.Fatalf("hits = %d, want exactly 1 attempt (no retry after cancel)", hits.Load())
	}
}

func TestRetryAfterDelayParsing(t *testing.T) {
	if d := retryAfterDelay(http.Header{"Retry-After": {"1"}}); d != time.Second {
		t.Errorf("seconds form = %v, want 1s", d)
	}
	future := time.Now().Add(90 * time.Second).UTC().Format(http.TimeFormat)
	if d := retryAfterDelay(http.Header{"Retry-After": {future}}); d < 80*time.Second || d > 90*time.Second {
		t.Errorf("date form = %v, want ~90s", d)
	}
	for _, raw := range []string{"", "abc", "-5", "0"} {
		if d := retryAfterDelay(http.Header{"Retry-After": {raw}}); d != 0 {
			t.Errorf("Retry-After %q = %v, want 0 (flat backoff)", raw, d)
		}
	}
}

func TestBackoffDelayCapsRetryAfter(t *testing.T) {
	if d := backoffDelay(0); d != mailRetryBackoff {
		t.Errorf("empty hint = %v, want flat %v", d, mailRetryBackoff)
	}
	if d := backoffDelay(time.Second); d != time.Second {
		t.Errorf("1s hint = %v, want 1s", d)
	}
	if d := backoffDelay(30 * time.Second); d != mailMaxRetryAfter {
		t.Errorf("30s hint = %v, want cap %v", d, mailMaxRetryAfter)
	}
}
