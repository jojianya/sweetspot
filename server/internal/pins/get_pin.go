package pins

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jojianya/sweetspot247-backend/internal/auth"
	"github.com/jojianya/sweetspot247-backend/internal/users"
)

const pinListDefaultLimit = 200

func ListCategories(repo *Repository) gin.HandlerFunc {
	return func(c *gin.Context) {
		categories, err := repo.ListCategories(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}

		c.JSON(http.StatusOK, categories)
	}
}

func GetPins(repo *Repository) gin.HandlerFunc {
	return func(c *gin.Context) {
		bboxStr := c.Query("bbox")
		if bboxStr == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "bbox query param required (minLat,minLng,maxLat,maxLng)"})
			return
		}

		var bbox [4]float64
		parts := strings.Split(bboxStr, ",")
		if len(parts) != 4 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "bbox must be 4 comma-separated floats (minLat,minLng,maxLat,maxLng)"})
			return
		}
		for i, p := range parts {
			v, err := strconv.ParseFloat(strings.TrimSpace(p), 64)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "bbox must be 4 comma-separated floats"})
				return
			}
			bbox[i] = v
		}
		if bbox[0] < -90 || bbox[0] > 90 || bbox[2] < -90 || bbox[2] > 90 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "latitudes must be between -90 and 90"})
			return
		}
		if bbox[1] < -180 || bbox[1] > 180 || bbox[3] < -180 || bbox[3] > 180 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "longitudes must be between -180 and 180"})
			return
		}
		if bbox[0] > bbox[2] || bbox[1] > bbox[3] {
			c.JSON(http.StatusBadRequest, gin.H{"error": "bbox min must not exceed max (minLat,minLng,maxLat,maxLng)"})
			return
		}

		var categoryID *int
		if catStr := c.Query("category"); catStr != "" {
			id, err := strconv.Atoi(catStr)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "category must be an integer"})
				return
			}
			categoryID = &id
		}

		if categoryID != nil {
			exists, err := repo.CategoryExists(c.Request.Context(), *categoryID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
				return
			}
			if !exists {
				c.JSON(http.StatusBadRequest, gin.H{"error": "category not found"})
				return
			}
		}

		limit := pinListDefaultLimit
		if lStr := c.Query("limit"); lStr != "" {
			l, err := strconv.Atoi(lStr)
			if err != nil || l < 1 || l > pinListDefaultLimit {
				c.JSON(http.StatusBadRequest, gin.H{"error": "limit must be an integer between 1 and 200"})
				return
			}
			limit = l
		}

		pins, err := repo.ListPins(c.Request.Context(), bbox, categoryID, limit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"pins": pins})
	}
}

func GetPin(repo *Repository, users *users.Repository) gin.HandlerFunc {
	return func(c *gin.Context) {
		pin, err := repo.GetPin(c.Request.Context(), c.Param("id"))
		if err != nil {
			if errors.Is(err, ErrNotFound) {
				c.JSON(http.StatusNotFound, gin.H{"error": "pin not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}

		if pin.IsHidden && !canViewHidden(c, users, pin) {
			c.JSON(http.StatusNotFound, gin.H{"error": "pin not found"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"pin": pin})
	}
}

func canViewHidden(c *gin.Context, repo *users.Repository, pin PinDetail) bool {
	viewerID := auth.GetUserID(c)
	if viewerID == "" {
		return false
	}
	if viewerID == pin.UserID.String() {
		return true
	}

	viewer, err := repo.GetByID(c.Request.Context(), viewerID)
	if err != nil {
		return false
	}
	return viewer.Role == "admin" || viewer.Role == "owner"
}
