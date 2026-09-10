package pins

import (
	"errors"
	"io"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/jojianya/sweetspot247-backend/internal/auth"
	"github.com/jojianya/sweetspot247-backend/internal/storage"
	"github.com/jojianya/sweetspot247-backend/pkg/geohash"
)

const maxPhotoSize = 10 << 20

func CreatePin(repo *Repository, store *storage.Local) gin.HandlerFunc {
	return func(c *gin.Context) {
		form, err := c.MultipartForm()
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "expected multipart form data"})
			return
		}

		lat, err := strconv.ParseFloat(c.PostForm("lat"), 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "lat must be a number"})
			return
		}
		lng, err := strconv.ParseFloat(c.PostForm("lng"), 64)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "lng must be a number"})
			return
		}
		if lat < -90 || lat > 90 || lng < -180 || lng > 180 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "latitude or longitude out of range"})
			return
		}

		categoryID, err := strconv.Atoi(c.PostForm("category_id"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "category_id must be an integer"})
			return
		}

		var caption *string
		if c := c.PostForm("caption"); c != "" {
			caption = &c
		}

		files := form.File["photos"]
		if len(files) < 1 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "at least one photo is required"})
			return
		}
		if len(files) > maxPhotosPerPin {
			c.JSON(http.StatusBadRequest, gin.H{"error": "photo count exceeds maximum"})
			return
		}

		photoURLs := make([]string, 0, len(files))
		for _, fh := range files {
			if fh.Size > maxPhotoSize {
				c.JSON(http.StatusBadRequest, gin.H{"error": "one or more photos exceed 10MB"})
				return
			}

			src, err := fh.Open()
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "could not read uploaded file"})
				return
			}
			data, err := io.ReadAll(src)
			src.Close()
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "could not read uploaded file"})
				return
			}

			ext, err := validateImage(data)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}

			url, err := store.Save(data, ext)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "could not save uploaded file"})
				return
			}
			photoURLs = append(photoURLs, url)
		}

		exists, err := repo.CategoryExists(c.Request.Context(), categoryID)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}
		if !exists {
			c.JSON(http.StatusBadRequest, gin.H{"error": "category not found"})
			return
		}

		pin, err := repo.CreatePin(c.Request.Context(), NewPin{
			UserID:     auth.GetUserID(c),
			Lat:        lat,
			Lng:        lng,
			Caption:    caption,
			CategoryID: categoryID,
			PhotoURLs:  photoURLs,
			Geohash:    geohash.Encode(lat, lng),
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}

		c.JSON(http.StatusCreated, gin.H{"pin": pin})
	}
}

func validateImage(data []byte) (string, error) {
	switch http.DetectContentType(data) {
	case "image/jpeg":
		return "jpg", nil
	case "image/png":
		return "png", nil
	default:
		return "", errors.New("only jpg and png images are allowed")
	}
}
