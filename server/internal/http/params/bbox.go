package params

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jojianya/sweetspot247-backend/internal/http/response"
)

// ParseBbox validates and parses the bbox query param
// (minLat,minLng,maxLat,maxLng). On failure it writes the error response and
// returns ok=false.
func ParseBbox(c *gin.Context) (bbox [4]float64, ok bool) {
	bboxStr := c.Query("bbox")
	if bboxStr == "" {
		response.BadRequest(c, "bbox query param required (minLat,minLng,maxLat,maxLng)")
		return bbox, false
	}

	parts := strings.Split(bboxStr, ",")
	if len(parts) != 4 {
		response.BadRequest(c, "bbox must be 4 comma-separated floats (minLat,minLng,maxLat,maxLng)")
		return bbox, false
	}
	for i, p := range parts {
		v, err := strconv.ParseFloat(strings.TrimSpace(p), 64)
		if err != nil {
			response.BadRequest(c, "bbox must be 4 comma-separated floats")
			return bbox, false
		}
		bbox[i] = v
	}
	if bbox[0] < -90 || bbox[0] > 90 || bbox[2] < -90 || bbox[2] > 90 {
		response.BadRequest(c, "latitudes must be between -90 and 90")
		return bbox, false
	}
	if bbox[1] < -180 || bbox[1] > 180 || bbox[3] < -180 || bbox[3] > 180 {
		response.BadRequest(c, "longitudes must be between -180 and 180")
		return bbox, false
	}
	if bbox[0] > bbox[2] || bbox[1] > bbox[3] {
		response.BadRequest(c, "bbox min must not exceed max (minLat,minLng,maxLat,maxLng)")
		return bbox, false
	}
	if bbox[0] == bbox[2] || bbox[1] == bbox[3] {
		response.BadRequest(c, "bbox must have non-zero area (minLat < maxLat and minLng < maxLng)")
		return bbox, false
	}
	return bbox, true
}
