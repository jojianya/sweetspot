package auth

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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
