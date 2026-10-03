package realtime

// Pins the realtime Stream endpoint's bbox errors to params.ParseBbox's
// messages. Stream validates through ParseBbox first, so every invalid bbox
// answers with ParseBbox's exact message and status — including the cases
// the local validateBbox would word differently (it never gets the chance).

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestStreamBboxErrorsMatchParseBbox(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cases := []struct {
		bbox    string
		message string
	}{
		{"abc", "bbox must be 4 comma-separated floats"},
		{"-91,0,10,10", "latitudes must be between -90 and 90"},
		{"20,0,10,10", "bbox min must not exceed max (minLat,minLng,maxLat,maxLng)"},
		{"10,10,10,20", "bbox must have non-zero area"},
	}
	for _, tc := range cases {
		t.Run(tc.bbox, func(t *testing.T) {
			h := NewHandler(NewBroker("127.0.0.1:1", ""), 10)
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodGet, "/events?bbox="+tc.bbox, nil)
			h.Stream(c)
			if w.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400", w.Code)
			}
			if !strings.Contains(w.Body.String(), tc.message) {
				t.Errorf("body %q does not contain %q", w.Body.String(), tc.message)
			}
		})
	}
}
