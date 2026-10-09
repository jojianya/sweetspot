package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
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

// Delivery bounds: at most two attempts with a single short backoff. The
// caller (password reset) sends from a detached goroutine, so even the worst
// case (timeout + backoff + timeout) stays inside its 30s budget and never
// touches response latency.
const (
	mailMaxAttempts   = 2
	mailRetryBackoff  = 500 * time.Millisecond
	mailMaxRetryAfter = 2 * time.Second
)

func (m WebhookMailer) SendPasswordReset(ctx context.Context, to, link string) error {
	timeout := m.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	body, err := json.Marshal(map[string]string{
		"to":      to,
		"subject": "Reset your Goodspot password",
		"text":    fmt.Sprintf("Someone requested a password reset for this address. Use this link within 30 minutes (it works once):\n\n%s\n\nIf that was not you, ignore this message.", link),
	})
	if err != nil {
		return err
	}

	var lastErr error
	for attempt := 1; attempt <= mailMaxAttempts; attempt++ {
		// Each attempt gets the full timeout: a slow first try must not
		// steal the second try's budget.
		attemptCtx, cancel := context.WithTimeout(ctx, timeout)
		status, retryAfter, err := m.postOnce(attemptCtx, body)
		cancel()
		if err == nil {
			return nil
		}
		lastErr = err
		if attempt == mailMaxAttempts || !retryable(status, err) {
			// Surface how many sends were spent so the final-failure log
			// (which carries no email, URL, or key) still shows whether a
			// retry happened. Single-attempt failures keep their exact error.
			if attempt > 1 {
				return fmt.Errorf("email delivery failed after %d attempts: %w", attempt, lastErr)
			}
			return lastErr
		}
		if wait := backoffDelay(retryAfter); !sleepOrDone(ctx, wait) {
			return ctx.Err()
		}
	}
	return lastErr
}

// retryable reports whether a failed attempt is worth repeating. A failure
// with no status is a transport error or timeout (no response to judge), so
// it is retried; a status is judged on its own: 429 and 5xx may be transient,
// while 4xx means the request itself is wrong and retrying cannot help.
func retryable(status int, err error) bool {
	if status == 0 {
		return err != nil
	}
	return status == http.StatusTooManyRequests || status >= 500
}

// backoffDelay returns how long to wait before the retry: the server's
// Retry-After hint capped at mailMaxRetryAfter, else the flat backoff.
func backoffDelay(retryAfter time.Duration) time.Duration {
	if retryAfter <= 0 {
		return mailRetryBackoff
	}
	if retryAfter > mailMaxRetryAfter {
		return mailMaxRetryAfter
	}
	return retryAfter
}

// sleepOrDone waits out d but returns false at once if ctx is cancelled, so
// shutdown never sleeps through a backoff.
func sleepOrDone(ctx context.Context, d time.Duration) bool {
	select {
	case <-ctx.Done():
		return false
	case <-time.After(d):
		return true
	}
}

// postOnce sends one delivery attempt. It returns the HTTP status with any
// Retry-After hint, or a transport error with no status. The body is drained
// and closed per attempt so the connection can be reused.
func (m WebhookMailer) postOnce(ctx context.Context, body []byte) (int, time.Duration, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.URL, bytes.NewReader(body))
	if err != nil {
		return 0, 0, err
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
		return 0, 0, err
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4<<10))
	resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return resp.StatusCode, retryAfterDelay(resp.Header), fmt.Errorf("email webhook answered %d", resp.StatusCode)
	}
	return resp.StatusCode, 0, nil
}

// retryAfterDelay parses a Retry-After response header: delay seconds, or an
// HTTP date. Anything unparseable, in the past, or non-positive yields zero
// (caller falls back to the flat backoff).
func retryAfterDelay(h http.Header) time.Duration {
	raw := strings.TrimSpace(h.Get("Retry-After"))
	if raw == "" {
		return 0
	}
	if secs, err := strconv.Atoi(raw); err == nil {
		if secs <= 0 {
			return 0
		}
		return time.Duration(secs) * time.Second
	}
	if at, err := time.Parse(http.TimeFormat, raw); err == nil {
		if d := time.Until(at); d > 0 {
			return d
		}
	}
	return 0
}
