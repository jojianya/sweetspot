package reports

// Pins the List offset behavior before unifying with httpx.ParseOffset:
// missing, valid, negative, non-numeric and huge inputs must behave
// identically after the swap.

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() { gin.SetMode(gin.TestMode) }

type stubReportService struct {
	Service

	lists   int
	offsets []int
}

func (s *stubReportService) ListReports(_ context.Context, _ *string, _, offset int) ([]ReportListEntry, error) {
	s.lists++
	s.offsets = append(s.offsets, offset)
	return []ReportListEntry{}, nil
}

func listReports(path string) *httptest.ResponseRecorder {
	svc := &stubReportService{}
	h := NewHandler(svc)
	r := gin.New()
	r.GET("/reports", h.List)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
	return w
}

func TestListOffsetBehavior(t *testing.T) {
	cases := []struct {
		name   string
		path   string
		status int
	}{
		{"missing defaults to zero", "/reports", http.StatusOK},
		{"valid passes through", "/reports?offset=7", http.StatusOK},
		{"zero passes through", "/reports?offset=0", http.StatusOK},
		{"negative is rejected", "/reports?offset=-1", http.StatusBadRequest},
		{"non-numeric is rejected", "/reports?offset=many", http.StatusBadRequest},
		{"huge is rejected", "/reports?offset=99999999999999999999", http.StatusBadRequest},
		{"empty counts as missing", "/reports?offset=", http.StatusOK},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := listReports(tc.path)
			if w.Code != tc.status {
				t.Fatalf("status = %d, want %d (body: %s)", w.Code, tc.status, w.Body.String())
			}
		})
	}
}
