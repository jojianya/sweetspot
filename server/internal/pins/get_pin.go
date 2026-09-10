package pins

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
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

		var categoryID *int
		if catStr := c.Query("category"); catStr != "" {
			id, err := strconv.Atoi(catStr)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "category must be an integer"})
				return
			}
			categoryID = &id
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

func GetPin(repo *Repository) gin.HandlerFunc {
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

		c.JSON(http.StatusOK, gin.H{"pin": pin})
	}
}
