package params

// Characterization of ParseBbox: every rejection, its exact message and the
// 400 status mapping. The realtime Stream endpoint delegates to ParseBbox,
// so these messages are also the realtime endpoint's current messages.

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func bboxContext(query string) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/pins?"+query, nil)
	return c, w
}

func TestParseBboxErrors(t *testing.T) {
	cases := []struct {
		name    string
		query   string
		message string
	}{
		{"missing", "", "bbox query param required (minLat,minLng,maxLat,maxLng)"},
		{"not four parts", "bbox=1,2,3", "bbox must be 4 comma-separated floats (minLat,minLng,maxLat,maxLng)"},
		{"non-numeric", "bbox=1,2,three,4", "bbox must be 4 comma-separated floats"},
		{"NaN latitude", "bbox=NaN,0,10,10", "bbox components must be finite numbers"},
		{"NaN longitude", "bbox=0,0,10,NaN", "bbox components must be finite numbers"},
		{"+Inf", "bbox=0,0,10,+Inf", "bbox components must be finite numbers"},
		{"-Inf", "bbox=-Inf,0,10,10", "bbox components must be finite numbers"},
		{"latitude out of range", "bbox=-91,0,10,10", "latitudes must be between -90 and 90"},
		{"longitude out of range", "bbox=0,-181,10,10", "longitudes must be between -180 and 180"},
		{"min exceeds max", "bbox=20,0,10,10", "bbox min must not exceed max (minLat,minLng,maxLat,maxLng)"},
		{"zero area", "bbox=10,10,10,20", "bbox must have non-zero area"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, w := bboxContext(tc.query)
			if _, ok := ParseBbox(c); ok {
				t.Fatalf("ParseBbox(%q) accepted, want rejection", tc.query)
			}
			if w.Code != http.StatusBadRequest {
				t.Errorf("status = %d, want 400", w.Code)
			}
			if body := w.Body.String(); !strings.Contains(body, tc.message) {
				t.Errorf("body %q does not contain %q", body, tc.message)
			}
		})
	}
}

func TestParseBboxAccepts(t *testing.T) {
	c, _ := bboxContext("bbox=10.5,20.5,11.5,21.5")
	bbox, ok := ParseBbox(c)
	if !ok {
		t.Fatal("ParseBbox rejected a valid bbox")
	}
	want := [4]float64{10.5, 20.5, 11.5, 21.5}
	if bbox != want {
		t.Errorf("bbox = %v, want %v", bbox, want)
	}
}
