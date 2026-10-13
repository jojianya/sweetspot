package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
)

type recordingReporter struct {
	reported error
	attrs    map[string]any
}

func (r *recordingReporter) Report(_ context.Context, err error, attrs ...any) {
	r.reported = err
	r.attrs = make(map[string]any, len(attrs)/2)
	for i := 0; i+1 < len(attrs); i += 2 {
		if k, ok := attrs[i].(string); ok {
			r.attrs[k] = attrs[i+1]
		}
	}
}

func ingestJSON(t *testing.T, rep *recordingReporter, body string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	// Serve through a real engine (not a bare handler call): gin only
	// flushes the status to the recorder in its ServeHTTP flow.
	r := gin.New()
	r.POST("/errors", ClientErrorIngest(rep))
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/errors", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	return w
}

// TestClientErrorIngestTruncatesOversizedFields proves attacker-shaped
// reports cannot smuggle unbounded payloads to the reporter: stack and url
// are cut to their budgets, rune-safely (no broken UTF-8).
func TestClientErrorIngestTruncatesOversizedFields(t *testing.T) {
	rep := &recordingReporter{}
	// Multi-byte input: a byte cut would corrupt UTF-8.
	stack := strings.Repeat("é", clientErrorMaxStackRunes+100)
	url := strings.Repeat("ü", clientErrorMaxURLRunes+10)
	body := `{"message":"boom","stack":` + quote(stack) + `,"url":` + quote(url) + `}`
	if w := ingestJSON(t, rep, body); w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}
	gotStack, _ := rep.attrs["stack"].(string)
	if n := len([]rune(gotStack)); n != clientErrorMaxStackRunes {
		t.Fatalf("stack runes = %d, want %d", n, clientErrorMaxStackRunes)
	}
	if !utf8.ValidString(gotStack) {
		t.Fatal("truncated stack is not valid UTF-8")
	}
	gotURL, _ := rep.attrs["url"].(string)
	if n := len([]rune(gotURL)); n != clientErrorMaxURLRunes {
		t.Fatalf("url runes = %d, want %d", n, clientErrorMaxURLRunes)
	}
	if !utf8.ValidString(gotURL) {
		t.Fatal("truncated url is not valid UTF-8")
	}
}

func quote(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"`
}

// TestClientErrorIngestAllowlistsExtraKeys proves extra is filtered to the
// keys the real client sends. An allowlist, not a size cap: the cap kept the
// first N keys, which still forwarded N attacker-chosen keys with
// attacker-chosen names into logs and Sentry.
func TestClientErrorIngestAllowlistsExtraKeys(t *testing.T) {
	rep := &recordingReporter{}
	var sb strings.Builder
	sb.WriteString(`{"message":"boom","extra":{`)
	// Two allowlisted keys, then two that no real client sends.
	sb.WriteString(`"kind":"api","method":"GET",`)
	sb.WriteString(`"password":"hunter2","note":"arbitrary attacker text"`)
	sb.WriteString(`}}`)
	if w := ingestJSON(t, rep, sb.String()); w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}
	extra, ok := rep.attrs["extra"].(map[string]any)
	if !ok {
		t.Fatalf("extra has type %T, want map[string]any", rep.attrs["extra"])
	}
	if len(extra) != 2 {
		t.Fatalf("extra = %v, want only the allowlisted kind and method", extra)
	}
	if extra["kind"] != "api" || extra["method"] != "GET" {
		t.Fatalf("extra = %v, want kind=api and method=GET preserved", extra)
	}
	for _, dropped := range []string{"password", "note"} {
		if _, ok := extra[dropped]; ok {
			t.Errorf("extra[%q] survived the allowlist", dropped)
		}
	}
}

// TestClientErrorIngestTruncatesExtraValues proves a long value is still cut,
// so one surviving key cannot carry an unbounded payload.
func TestClientErrorIngestTruncatesExtraValues(t *testing.T) {
	rep := &recordingReporter{}
	long := strings.Repeat("y", clientErrorMaxExtraValueRunes+50)
	body := `{"message":"boom","extra":{"kind":"` + long + `"}}`
	if w := ingestJSON(t, rep, body); w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}
	extra, _ := rep.attrs["extra"].(map[string]any)
	got, _ := extra["kind"].(string)
	if n := len([]rune(got)); n != clientErrorMaxExtraValueRunes {
		t.Fatalf("extra[kind] runes = %d, want %d", n, clientErrorMaxExtraValueRunes)
	}
}

// TestClientErrorIngestScrubsPII proves a client-supplied string loses its
// emails and credentials before it reaches the reporter. The endpoint is
// public, so its payload is untrusted: a browser error message carries
// whatever the user typed, and logs and Sentry are read widely (9.2).
func TestClientErrorIngestScrubsPII(t *testing.T) {
	cases := []struct {
		name    string
		in      string
		mustNot string
	}{
		{"email in message", "TypeError for alice@example.com", "alice@example.com"},
		{"bearer header in stack", "authorization: Bearer eyJhbGciOiJIUzI1NiJ9.payload.sig", "eyJhbGciOiJIUzI1NiJ9"},
		{"token query param in url", "https://app.example/reset?token=abc123def456&x=1", "abc123def456"},
		{"api key in extra", "api_key=sk_live_abc123", "sk_live_abc123"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rep := &recordingReporter{}
			body := `{"message":` + quote(tc.in) + `,"url":` + quote(tc.in) + `,"stack":` + quote(tc.in) + `,"extra":{"kind":` + quote(tc.in) + `}}`
			if w := ingestJSON(t, rep, body); w.Code != http.StatusNoContent {
				t.Fatalf("expected 204, got %d", w.Code)
			}
			for _, field := range []string{"stack", "url"} {
				got, _ := rep.attrs[field].(string)
				if got == "" {
					continue
				}
				if tc.mustNot != "" && strings.Contains(got, tc.mustNot) {
					t.Errorf("%s still contains %q: %q", field, tc.mustNot, got)
				}
			}
			if tc.mustNot != "" && strings.Contains(rep.reported.Error(), tc.mustNot) {
				t.Errorf("message still contains %q: %q", tc.mustNot, rep.reported.Error())
			}
			extra, _ := rep.attrs["extra"].(map[string]any)
			if got, _ := extra["kind"].(string); tc.mustNot != "" && strings.Contains(got, tc.mustNot) {
				t.Errorf("extra[kind] still contains %q: %q", tc.mustNot, got)
			}
		})
	}
}

// TestScrubPIIKeepsTheMessageReadable is the other half: scrubbing must not
// destroy the diagnostic value of the report.
func TestScrubPIIKeepsTheMessageReadable(t *testing.T) {
	got := scrubPII("TypeError: cannot read 'id' of undefined at foo (bar.js:1:2)")
	if got != "TypeError: cannot read 'id' of undefined at foo (bar.js:1:2)" {
		t.Fatalf("scrubPII changed a clean message: %q", got)
	}
	got = scrubPII("failed for alice@example.com with Bearer abc.def")
	if !strings.Contains(got, "failed for") || !strings.Contains(got, "[EMAIL]") || !strings.Contains(got, "[TOKEN]") {
		t.Fatalf("scrubPII lost the surrounding text: %q", got)
	}
}

// TestClientErrorIngestPassesNormalPayload proves sizing a normal report
// below every budget leaves it byte-identical.
func TestClientErrorIngestPassesNormalPayload(t *testing.T) {
	rep := &recordingReporter{}
	body := `{"message":"the map broke","stack":"at foo (bar.js:1:2)","url":"https://app.example/map","extra":{"kind":"react"}}`
	if w := ingestJSON(t, rep, body); w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}
	if got := rep.reported.Error(); got != "the map broke" {
		t.Fatalf("message = %q, want unchanged", got)
	}
	if got, _ := rep.attrs["stack"].(string); got != "at foo (bar.js:1:2)" {
		t.Fatalf("stack = %q, want unchanged", got)
	}
	if got, _ := rep.attrs["url"].(string); got != "https://app.example/map" {
		t.Fatalf("url = %q, want unchanged", got)
	}
	extra, _ := rep.attrs["extra"].(map[string]any)
	if len(extra) != 1 || extra["kind"] != "react" {
		t.Fatalf("extra = %v, want unchanged", extra)
	}
}
