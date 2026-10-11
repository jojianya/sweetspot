package http

import (
	"context"
	"fmt"
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

// TestClientErrorIngestCapsExtraKeys proves extra is bounded to the first
// 20 keys by sorted order (deterministic), with long string values cut.
func TestClientErrorIngestCapsExtraKeys(t *testing.T) {
	rep := &recordingReporter{}
	var sb strings.Builder
	sb.WriteString(`{"message":"boom","extra":{`)
	for i := 0; i < 25; i++ {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(`"k` + fmt.Sprintf("%02d", i) + `":"v` + strings.Repeat("x", clientErrorMaxExtraValueRunes+50) + `"`)
	}
	sb.WriteString(`}}`)
	if w := ingestJSON(t, rep, sb.String()); w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}
	extra, ok := rep.attrs["extra"].(map[string]any)
	if !ok {
		t.Fatalf("extra has type %T, want map[string]any", rep.attrs["extra"])
	}
	if len(extra) != clientErrorMaxExtraKeys {
		t.Fatalf("extra keys = %d, want %d", len(extra), clientErrorMaxExtraKeys)
	}
	for k, v := range extra {
		s, ok := v.(string)
		if !ok {
			t.Fatalf("extra[%q] has type %T, want string", k, v)
		}
		if n := len([]rune(s)); n != clientErrorMaxExtraValueRunes {
			t.Fatalf("extra[%q] runes = %d, want %d", k, n, clientErrorMaxExtraValueRunes)
		}
		if !strings.HasPrefix(s, "v") {
			t.Fatalf("extra[%q] lost its value prefix", k)
		}
	}
	if _, ok := extra["k00"]; !ok {
		t.Fatal("expected lowest sorted key k00 to survive the cap")
	}
	if _, ok := extra["k24"]; ok {
		t.Fatal("key k24 should have been cut by the cap")
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
