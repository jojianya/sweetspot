package pins

import (
	"net/http"
	"testing"
)

// TestValidateCreateFieldsRejectsNonFinite pins down the NaN/Inf guard:
// strconv.ParseFloat accepts "NaN"/"Inf", and NaN would pass the range
// checks and then fail inside PostGIS (500 instead of 400).
func TestValidateCreateFieldsRejectsNonFinite(t *testing.T) {
	cases := []struct{ lat, lng string }{
		{"NaN", "10"},
		{"10", "NaN"},
		{"+Inf", "10"},
		{"10", "-Inf"},
	}
	for _, tc := range cases {
		in, err := validateCreateFields(tc.lat, tc.lng, "1", "", 1)
		if err == nil {
			t.Fatalf("validateCreateFields(%q, %q) accepted, want 400", tc.lat, tc.lng)
		}
		if in != nil {
			t.Fatalf("expected nil input on rejection, got %+v", in)
		}
		if err.status != http.StatusBadRequest {
			t.Errorf("status = %d, want 400", err.status)
		}
	}
}

// TestValidateCreateFieldsAcceptsFinite sanity-checks that normal values and
// the boundary values still pass.
func TestValidateCreateFieldsAcceptsFinite(t *testing.T) {
	for _, ll := range [][2]string{{"17.385", "78.4867"}, {"90", "180"}, {"-90", "-180"}, {"0", "0"}} {
		in, err := validateCreateFields(ll[0], ll[1], "1", "", 1)
		if err != nil {
			t.Fatalf("validateCreateFields(%q, %q) rejected: %+v", ll[0], ll[1], err)
		}
		if in == nil {
			t.Fatalf("expected parsed input for %v", ll)
		}
	}
}
