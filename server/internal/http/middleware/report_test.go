package middleware

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
)

type captureReporter struct {
	mu    sync.Mutex
	calls []string
}

func (c *captureReporter) Report(_ context.Context, err error, _ ...any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls = append(c.calls, err.Error())
}

func (c *captureReporter) messages() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.calls...)
}

func TestReportErrorsReportsOnly5xx(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rep := &captureReporter{}
	r := gin.New()
	r.Use(Recover(rep), ReportErrors(rep))
	r.GET("/ok", func(c *gin.Context) { c.Status(http.StatusOK) })
	r.GET("/boom", func(c *gin.Context) { c.Status(http.StatusInternalServerError) })

	for _, tc := range []struct {
		path string
		want int
	}{
		{"/ok", http.StatusOK},
		{"/boom", http.StatusInternalServerError},
	} {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, tc.path, nil))
		if w.Code != tc.want {
			t.Fatalf("%s: expected %d, got %d", tc.path, tc.want, w.Code)
		}
	}

	if calls := rep.messages(); len(calls) != 1 || calls[0] != "request failed with status 500" {
		t.Fatalf("expected exactly the 500 report, got %v", calls)
	}
}

func TestReportErrorsUsesAttachedError(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rep := &captureReporter{}
	r := gin.New()
	r.Use(ReportErrors(rep))
	r.GET("/boom", func(c *gin.Context) {
		_ = c.Error(errors.New("database exploded"))
		c.Status(http.StatusInternalServerError)
	})

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/boom", nil))
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}

	if calls := rep.messages(); len(calls) != 1 || calls[0] != "database exploded" {
		t.Fatalf("expected the attached error, got %v", calls)
	}
}

// attrReporter captures report attributes so tests can assert on what is
// (and is not) shipped to Sentry.
type attrReporter struct {
	mu    sync.Mutex
	attrs []any
}

func (r *attrReporter) Report(_ context.Context, err error, attrs ...any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.attrs = append([]any(nil), attrs...)
}

func (r *attrReporter) keys() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []string
	for i := 0; i+1 < len(r.attrs); i += 2 {
		if k, ok := r.attrs[i].(string); ok {
			out = append(out, k)
		}
	}
	return out
}

// TestReportErrorsOmitsClientIP proves error reports carry no client IP:
// the request logger already records it for operators, and Sentry must not
// receive PII.
func TestReportErrorsOmitsClientIP(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rep := &attrReporter{}
	r := gin.New()
	r.Use(ReportErrors(rep))
	r.GET("/boom", func(c *gin.Context) {
		_ = c.Error(errors.New("database exploded"))
		c.Status(http.StatusInternalServerError)
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/boom", nil)
	req.RemoteAddr = "192.0.2.9:1234"
	r.ServeHTTP(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}

	for _, k := range rep.keys() {
		if k == "ip" {
			t.Fatalf("report shipped a client-ip attribute: %v", rep.keys())
		}
	}
}

func TestRecoverReportsPanics(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rep := &captureReporter{}
	r := gin.New()
	r.Use(Recover(rep))
	r.GET("/panic", func(c *gin.Context) { panic("kaboom") })

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/panic", nil))
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}

	if calls := rep.messages(); len(calls) != 1 || calls[0] != "panic: kaboom" {
		t.Fatalf("expected the panic report, got %v", calls)
	}
}

func TestRecoverNilReporterStillRecovers(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(Recover(nil))
	r.GET("/panic", func(c *gin.Context) { panic("kaboom") })

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/panic", nil))
	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", w.Code)
	}
}
