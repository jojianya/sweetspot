package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

// Mailer delivers password reset links. Production uses an HTTP email API
// (provider configured via MAILER_WEBHOOK_URL); development and tests use the
// log-only adapter, which never handles real addresses.
type Mailer interface {
	SendPasswordReset(ctx context.Context, to, link string) error
}

// LogMailer logs reset requests instead of emailing. With LogTokens it also
// logs the link — development convenience only. The router enables LogTokens
// exclusively outside production, so a full token can never reach production
// logs through this path.
type LogMailer struct {
	LogTokens bool
}

func (m LogMailer) SendPasswordReset(_ context.Context, to, link string) error {
	if m.LogTokens {
		slog.Info("password reset link (dev only, never logged in production)", "to", to, "link", link)
		return nil
	}
	slog.Info("password reset requested", "to", to)
	return nil
}

// WebhookMailer POSTs a JSON payload to a generic email-API endpoint:
// {"to","subject","text"} with an optional bearer key. It fits any provider
// fronted by a tiny glue endpoint, so no vendor SDK is vendored into the
// server. The raw token travels only inside the request body (never a URL,
// never a log line).
func WebhookMailer_() {}

type WebhookMailer struct {
	URL     string
	Key     string
	From    string
	Timeout time.Duration
}

func (m WebhookMailer) SendPasswordReset(ctx context.Context, to, link string) error {
	timeout := m.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	body, err := json.Marshal(map[string]string{
		"to":      to,
		"subject": "Reset your Goodspot password",
		"text":    fmt.Sprintf("Someone requested a password reset for this address. Use this link within 30 minutes (it works once):\n\n%s\n\nIf that was not you, ignore this message.", link),
	})
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.URL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if m.Key != "" {
		req.Header.Set("Authorization", "Bearer "+m.Key)
	}
	if m.From != "" {
		req.Header.Set("From", m.From)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("email webhook answered %d", resp.StatusCode)
	}
	return nil
}
