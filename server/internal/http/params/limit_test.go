package params

// Boundary characterization of ParseOffset, matching the ParseLimit style:
// over-limit values are rejected with 400, like over-max limits.

import (
	"net/http"
	"strings"
	"testing"
)

func TestParseOffsetBoundaries(t *testing.T) {
	cases := []struct {
		name    string
		query   string
		want    int
		wantOK  bool
		message string
	}{
		{"missing defaults to 0", "", 0, true, ""},
		{"zero allowed", "offset=0", 0, true, ""},
		{"max allowed", "offset=10000", 10000, true, ""},
		{"over max rejected", "offset=10001", 0, false, "offset must be an integer between 0 and 10000"},
		{"negative rejected", "offset=-1", 0, false, "offset must be an integer between 0 and 10000"},
		{"non-numeric rejected", "offset=abc", 0, false, "offset must be an integer between 0 and 10000"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, w := bboxContext(tc.query)
			got, ok := ParseOffset(c)
			if ok != tc.wantOK {
				t.Fatalf("ParseOffset(%q) ok = %v, want %v", tc.query, ok, tc.wantOK)
			}
			if !tc.wantOK {
				if w.Code != http.StatusBadRequest {
					t.Errorf("status = %d, want 400", w.Code)
				}
				if body := w.Body.String(); !strings.Contains(body, tc.message) {
					t.Errorf("body %q does not contain %q", body, tc.message)
				}
			} else if got != tc.want {
				t.Errorf("ParseOffset(%q) = %d, want %d", tc.query, got, tc.want)
			}
		})
	}
}
