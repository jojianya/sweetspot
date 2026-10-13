package params

// Characterization of ParseCategoryID: absent or blank yields (nil, true),
// an integer yields a pointer to it, and anything else answers 400 with the
// same message both former call sites (pins list, realtime stream) wrote.

import (
	"net/http"
	"strings"
	"testing"
)

func TestParseCategoryID(t *testing.T) {
	cases := []struct {
		name    string
		query   string
		want    *int
		wantOK  bool
		message string
	}{
		{"missing yields nil", "", nil, true, ""},
		{"blank yields nil", "category=%20", nil, true, ""},
		{"integer yields pointer", "category=3", intPtr(3), true, ""},
		{"zero yields pointer", "category=0", intPtr(0), true, ""},
		{"non-numeric rejected", "category=abc", nil, false, "category must be an integer"},
		{"float rejected", "category=1.5", nil, false, "category must be an integer"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, w := bboxContext(tc.query)
			got, ok := ParseCategoryID(c)
			if ok != tc.wantOK {
				t.Fatalf("ParseCategoryID(%q) ok = %v, want %v", tc.query, ok, tc.wantOK)
			}
			if !tc.wantOK {
				if w.Code != http.StatusBadRequest {
					t.Errorf("status = %d, want 400", w.Code)
				}
				if body := w.Body.String(); !strings.Contains(body, tc.message) {
					t.Errorf("body %q does not contain %q", body, tc.message)
				}
				return
			}
			if tc.want == nil {
				if got != nil {
					t.Errorf("ParseCategoryID(%q) = %v, want nil", tc.query, *got)
				}
				return
			}
			if got == nil || *got != *tc.want {
				t.Errorf("ParseCategoryID(%q) = %v, want %d", tc.query, got, *tc.want)
			}
		})
	}
}

func intPtr(v int) *int { return &v }
