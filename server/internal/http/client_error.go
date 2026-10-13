package http

import (
	"errors"
	stdhttp "net/http"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jojianya/sweetspot247-backend/internal/http/middleware"
)

// Bounds for the POST /errors payload before it reaches the reporter. The
// endpoint is public, so attacker-shaped reports must not produce unbounded
// Sentry events: the message was already capped, and stack, url and extra
// get the same treatment here. The 1 MB body cap and the 30/min limiter are
// the outer bounds; these keep any single report small inside them.
const (
	clientErrorMaxStackRunes      = 8 * 1024
	clientErrorMaxURLRunes        = 2 * 1024
	clientErrorMaxMessageRunes    = 2048
	clientErrorMaxExtraValueRunes = 1024
)

// truncateRunes cuts s to at most max runes, so multi-byte characters are
// never split mid-encoding.
func truncateRunes(s string, max int) string {
	if r := []rune(s); len(r) > max {
		return string(r[:max])
	}
	return s
}

// clientErrorAllowedExtraKeys is the allowlist of `extra` keys that survive
// into the report. The endpoint is public and the payload is attacker-shaped,
// so anything the real client does not send is dropped rather than forwarded
// into logs and Sentry. These four are exactly what client/src/lib/monitoring.ts
// attaches (reportError's context, plus kind for the global handlers).
//
// An allowlist rather than a size cap: capping kept the first N keys, which
// still forwarded N attacker-chosen keys with attacker-chosen names.
var clientErrorAllowedExtraKeys = map[string]bool{
	"kind":   true,
	"method": true,
	"url":    true,
	"status": true,
}

// allowlistExtra keeps only the allowlisted keys, with values truncated and
// scrubbed. An unknown or oversized key is dropped, so a hand-crafted report
// can neither smuggle arbitrary context nor use a key name to smuggle content.
func allowlistExtra(extra map[string]any) map[string]any {
	out := make(map[string]any, len(clientErrorAllowedExtraKeys))
	for k, v := range extra {
		if !clientErrorAllowedExtraKeys[k] {
			continue
		}
		out[k] = capExtraValue(v)
	}
	return out
}

func capExtraValue(v any) any {
	if s, ok := v.(string); ok {
		return scrubPII(truncateRunes(s, clientErrorMaxExtraValueRunes))
	}
	return v
}

// Patterns for the two things a browser error message can carry that must not
// reach a log store: a typed email address, and a credential in a header or a
// query string. Logs are copied widely and read by many people, so anything in
// them is effectively published (practice 9.2).
var (
	clientErrorEmailRe   = regexp.MustCompile(`[\w.+-]+@[\w-]+\.[\w.-]+`)
	clientErrorBearerRe  = regexp.MustCompile(`(?i)\bbearer\s+[\w\-.=+/]+`)
	clientErrorTokenKVRe = regexp.MustCompile(`(?i)\b(token|access_token|api_?key|secret|password|authorization)=([^\s&"']+)`)
)

// scrubPII replaces emails and credentials in s with a marker that keeps the
// shape readable without the value. Applied to every client-supplied string
// before it leaves this handler.
func scrubPII(s string) string {
	s = clientErrorEmailRe.ReplaceAllString(s, "[EMAIL]")
	s = clientErrorBearerRe.ReplaceAllString(s, "Bearer [TOKEN]")
	return clientErrorTokenKVRe.ReplaceAllString(s, `$1=[TOKEN]`)
}

// ClientErrorIngest handles POST /errors: it forwards a client-side error
// report to the reporter under the "client" source so crashes in the browser
// land in the same monitoring pipeline as server errors. It always answers 204
// and never fails the request, even for a malformed body.
func ClientErrorIngest(rep middleware.ErrorReporter) gin.HandlerFunc {
	return func(c *gin.Context) {
		var payload struct {
			Message string         `json:"message"`
			Stack   string         `json:"stack"`
			URL     string         `json:"url"`
			Extra   map[string]any `json:"extra"`
		}
		if err := c.ShouldBindJSON(&payload); err != nil {
			c.Status(stdhttp.StatusNoContent)
			return
		}
		msg := strings.TrimSpace(payload.Message)
		if msg == "" {
			msg = "client error"
		}
		if r := []rune(msg); len(r) > clientErrorMaxMessageRunes {
			msg = string(r[:clientErrorMaxMessageRunes])
		}
		if rep != nil {
			rep.Report(c.Request.Context(), errors.New(scrubPII(msg)),
				"source", "client",
				"url", scrubPII(truncateRunes(payload.URL, clientErrorMaxURLRunes)),
				"stack", scrubPII(truncateRunes(payload.Stack, clientErrorMaxStackRunes)),
				"extra", allowlistExtra(payload.Extra),
			)
		}
		c.Status(stdhttp.StatusNoContent)
	}
}
