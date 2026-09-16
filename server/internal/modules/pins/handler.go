package pins

import (
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/jojianya/sweetspot247-backend/internal/http/middleware"
	"github.com/jojianya/sweetspot247-backend/internal/modules/pins/imaging"
	"github.com/jojianya/sweetspot247-backend/internal/platform/storage"
	"github.com/jojianya/sweetspot247-backend/pkg/geohash"
)

const (
	pinListDefaultLimit = 200
	maxPhotoSize        = 10 << 20

	searchDefaultLimit = 10
	searchMaxLimit     = 25
	searchMaxQueryLen  = 100
)

type validatedFile struct {
	data  []byte
	thumb []byte
	ext   string
}

type Handler struct {
	service Service
	store   *storage.Local
}

func NewHandler(service Service, store *storage.Local) *Handler {
	return &Handler{service: service, store: store}
}

func (h *Handler) ListCategories(c *gin.Context) {
	categories, err := h.service.ListCategories(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	c.JSON(http.StatusOK, categories)
}

func (h *Handler) GetPins(c *gin.Context) {
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
		exists, err := h.service.CategoryExists(c.Request.Context(), *categoryID)
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

	pins, err := h.service.ListPins(c.Request.Context(), bbox, categoryID, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"pins": pins})
}

func (h *Handler) GetPin(c *gin.Context) {
	pin, err := h.service.GetPin(c.Request.Context(), c.Param("id"))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "pin not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	if pin.IsHidden && !canViewHidden(c, pin) {
		c.JSON(http.StatusNotFound, gin.H{"error": "pin not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"pin": pin})
}

func canViewHidden(c *gin.Context, pin PinDetail) bool {
	viewerID := middleware.GetUserID(c)
	if viewerID == "" {
		return false
	}
	if viewerID == pin.UserID.String() {
		return true
	}

	role := middleware.GetRole(c)
	return role == "admin" || role == "owner"
}

func (h *Handler) CreatePin(c *gin.Context) {
	form, err := c.MultipartForm()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "expected multipart form data"})
		return
	}

	userID := middleware.GetUserID(c)
	userExists, err := h.service.UserExists(c.Request.Context(), userID)
	if err != nil {
		slog.Error("create pin: user exists", "user_id", userID, "error", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	if !userExists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "account no longer exists, please sign in again"})
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
	if v := c.PostForm("caption"); v != "" {
		if len([]rune(v)) > 500 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "caption must be at most 500 characters"})
			return
		}
		caption = &v
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

	validated := make([]validatedFile, 0, len(files))
	for i, fh := range files {
		if fh.Size > maxPhotoSize {
			c.JSON(http.StatusBadRequest, gin.H{"error": "one or more photos exceed 10MB"})
			return
		}

		src, err := fh.Open()
		if err != nil {
			slog.Error("create pin: open uploaded file", "error", err.Error())
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not read uploaded file"})
			return
		}
		data, err := io.ReadAll(src)
		src.Close()
		if err != nil {
			slog.Error("create pin: read uploaded file", "error", err.Error())
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not read uploaded file"})
			return
		}

		if err := imaging.Validate(data); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "photo " + strconv.Itoa(i+1) + ": " + err.Error()})
			return
		}

		proc, err := imaging.Process(data)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "photo " + strconv.Itoa(i+1) + ": " + err.Error()})
			return
		}

		validated = append(validated, validatedFile{data: proc.Full, thumb: proc.Thumb, ext: "webp"})
	}

	exists, err := h.service.CategoryExists(c.Request.Context(), categoryID)
	if err != nil {
		slog.Error("create pin: category exists", "category_id", categoryID, "error", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}
	if !exists {
		c.JSON(http.StatusBadRequest, gin.H{"error": "category not found"})
		return
	}

	photoURLs := make([]string, 0, len(validated))
	thumbURLs := make([]string, 0, len(validated))
	for _, vf := range validated {
		url, err := h.store.Save(vf.data, vf.ext)
		if err != nil {
			slog.Error("create pin: save photo", "error", err.Error())
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not save uploaded file"})
			return
		}
		thumbURL, err := h.store.Save(vf.thumb, vf.ext)
		if err != nil {
			slog.Error("create pin: save thumbnail", "error", err.Error())
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not save uploaded file"})
			return
		}
		photoURLs = append(photoURLs, url)
		thumbURLs = append(thumbURLs, thumbURL)
	}

	pin, err := h.service.CreatePin(c.Request.Context(), NewPin{
		UserID:        middleware.GetUserID(c),
		Lat:           lat,
		Lng:           lng,
		Caption:       caption,
		CategoryID:    categoryID,
		PhotoURLs:     photoURLs,
		ThumbnailURLs: thumbURLs,
		Geohash:       geohash.Encode(lat, lng),
	})
	if err != nil {
		slog.Error("create pin: database insert",
			"user_id", middleware.GetUserID(c),
			"lat", lat,
			"lng", lng,
			"category_id", categoryID,
			"photos", len(photoURLs),
			"error", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	photos := make([]gin.H, 0, len(photoURLs))
	for i := range photoURLs {
		photos = append(photos, gin.H{
			"photo_url":     photoURLs[i],
			"thumbnail_url": thumbURLs[i],
			"position":      i,
		})
	}

	c.JSON(http.StatusCreated, gin.H{"pin": pin, "photos": photos})
}

func (h *Handler) SearchPins(c *gin.Context) {
	q := strings.TrimSpace(c.Query("q"))
	if q == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "q query param is required"})
		return
	}
	if utf8.RuneCountInString(q) > searchMaxQueryLen {
		c.JSON(http.StatusBadRequest, gin.H{"error": "q must be at most 100 characters"})
		return
	}

	limit := searchDefaultLimit
	if l := c.Query("limit"); l != "" {
		n, err := strconv.Atoi(l)
		if err != nil || n < 1 || n > searchMaxLimit {
			c.JSON(http.StatusBadRequest, gin.H{"error": "limit must be an integer between 1 and 25"})
			return
		}
		limit = n
	}

	pins, err := h.service.SearchPins(c.Request.Context(), q, limit)
	if err != nil {
		slog.Error("search pins", "query", q, "error", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"pins": pins})
}
