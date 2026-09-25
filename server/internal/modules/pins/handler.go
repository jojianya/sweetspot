package pins

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/jojianya/sweetspot247-backend/internal/http/middleware"
	httpx "github.com/jojianya/sweetspot247-backend/internal/http/params"
	"github.com/jojianya/sweetspot247-backend/internal/http/response"
	"github.com/jojianya/sweetspot247-backend/internal/modules/pins/imaging"
	"github.com/jojianya/sweetspot247-backend/internal/platform/storage"
	"github.com/jojianya/sweetspot247-backend/pkg/geohash"
)

const (
	pinListDefaultLimit  = 200
	trendingDefaultLimit = 10
	trendingMaxLimit     = 25
	maxPhotoSize         = 10 << 20

	searchDefaultLimit = 10
	searchMaxLimit     = 25
	searchMaxQueryLen  = 100

	// Public profile pages show a bounded set of the user's pins.
	profileDefaultLimit = 50
	profileMaxLimit     = 100
)

type validatedFile struct {
	data  []byte
	thumb []byte
	ext   string
}

// photoErr carries a photo-upload failure: the HTTP status to respond with and
// the client-facing message.
type photoErr struct {
	status int
	msg    string
}

type Handler struct {
	service Service
	store   *storage.Local
	events  Events
}

func NewHandler(service Service, store *storage.Local, events Events) *Handler {
	if events == nil {
		events = nopEvents{}
	}
	return &Handler{service: service, store: store, events: events}
}

// nopEvents is the zero-value event publisher used when realtime is disabled.
type nopEvents struct{}

func (nopEvents) PinCreated(context.Context, Event) {}

func (h *Handler) ListCategories(c *gin.Context) {
	categories, err := h.service.ListCategories(c.Request.Context())
	if err != nil {
		response.Internal(c, "pins: list categories", err)
		return
	}

	response.OK(c, categories)
}

func (h *Handler) GetPins(c *gin.Context) {
	bbox, ok := httpx.ParseBbox(c)
	if !ok {
		return
	}

	var categoryID *int
	if catStr := c.Query("category"); catStr != "" {
		id, err := strconv.Atoi(catStr)
		if err != nil {
			response.BadRequest(c, "category must be an integer")
			return
		}
		categoryID = &id
	}

	if categoryID != nil {
		exists, err := h.service.CategoryExists(c.Request.Context(), *categoryID)
		if err != nil {
			response.Internal(c, "pins: category exists", err, "category_id", *categoryID)
			return
		}
		if !exists {
			response.BadRequest(c, "category not found")
			return
		}
	}

	limit, ok := httpx.ParseLimit(c, pinListDefaultLimit, pinListDefaultLimit)
	if !ok {
		return
	}

	pins, err := h.service.ListPins(c.Request.Context(), bbox, categoryID, limit)
	if err != nil {
		response.Internal(c, "pins: list", err)
		return
	}

	response.OK(c, gin.H{"pins": pins})
}

// GetTrending lists the most engaged recent pins in the viewport. The server
// ranks them by a hotness score (views and comments, decayed by age) so fresh
// pins with the same activity outrank older ones.
func (h *Handler) GetTrending(c *gin.Context) {
	bbox, ok := httpx.ParseBbox(c)
	if !ok {
		return
	}

	limit, ok := httpx.ParseLimit(c, trendingDefaultLimit, trendingMaxLimit)
	if !ok {
		return
	}

	pins, err := h.service.ListTrending(c.Request.Context(), bbox, limit)
	if err != nil {
		response.Internal(c, "pins: trending", err)
		return
	}

	response.OK(c, gin.H{"pins": pins})
}

func (h *Handler) GetPin(c *gin.Context) {
	pin, err := h.service.GetPin(c.Request.Context(), c.Param("id"))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.NotFound(c, "pin not found")
			return
		}
		response.Internal(c, "pins: get", err, "pin_id", c.Param("id"))
		return
	}

	if pin.IsHidden && !canViewHidden(c, pin) {
		response.NotFound(c, "pin not found")
		return
	}

	response.OK(c, gin.H{"pin": pin})
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

// RegisterView counts a view of a pin. It is public: anyone who opens a pin
// counts, so SSR fetches and crawlers calling GET /pins/:id do not inflate the
// number — the client registers views explicitly when a detail is opened.
func (h *Handler) RegisterView(c *gin.Context) {
	views, err := h.service.RegisterView(c.Request.Context(), c.Param("id"))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.NotFound(c, "pin not found")
			return
		}
		response.Internal(c, "pins: register view", err, "pin_id", c.Param("id"))
		return
	}

	response.OK(c, gin.H{"views": views})
}

func (h *Handler) DeletePin(c *gin.Context) {
	userID := middleware.GetUserID(c)
	if userID == "" {
		response.Unauthorized(c, "authentication required")
		return
	}

	if err := h.service.DeletePin(c.Request.Context(), c.Param("id"), userID); err != nil {
		if errors.Is(err, ErrNotFound) {
			response.NotFound(c, "pin not found")
			return
		}
		if errors.Is(err, ErrForbidden) {
			response.Forbidden(c, "you can only delete your own pins")
			return
		}
		response.Internal(c, "delete pin", err, "pin_id", c.Param("id"), "user_id", userID)
		return
	}

	response.NoContent(c)
}

func (h *Handler) CreatePin(c *gin.Context) {
	form, err := c.MultipartForm()
	if err != nil {
		response.BadRequest(c, "expected multipart form data")
		return
	}

	userID := middleware.GetUserID(c)
	userExists, err := h.service.UserExists(c.Request.Context(), userID)
	if err != nil {
		response.Internal(c, "create pin: user exists", err, "user_id", userID)
		return
	}
	if !userExists {
		response.Unauthorized(c, "account no longer exists, please sign in again")
		return
	}

	lat, err := strconv.ParseFloat(c.PostForm("lat"), 64)
	if err != nil {
		response.BadRequest(c, "lat must be a number")
		return
	}
	lng, err := strconv.ParseFloat(c.PostForm("lng"), 64)
	if err != nil {
		response.BadRequest(c, "lng must be a number")
		return
	}
	if lat < -90 || lat > 90 || lng < -180 || lng > 180 {
		response.BadRequest(c, "latitude or longitude out of range")
		return
	}

	categoryID, err := strconv.Atoi(c.PostForm("category_id"))
	if err != nil {
		response.BadRequest(c, "category_id must be an integer")
		return
	}

	var caption *string
	if v := c.PostForm("caption"); v != "" {
		if len([]rune(v)) > 500 {
			response.BadRequest(c, "caption must be at most 500 characters")
			return
		}
		caption = &v
	}

	files := form.File["photos"]
	if len(files) < 1 {
		response.BadRequest(c, "at least one photo is required")
		return
	}
	if len(files) > maxPhotosPerPin {
		response.BadRequest(c, "photo count exceeds maximum")
		return
	}

	validated, perr := processPhotos(files)
	if perr != nil {
		if perr.status == http.StatusInternalServerError {
			slog.Error("create pin: read uploaded file", "error", perr.msg)
		}
		response.Error(c, perr.status, perr.msg)
		return
	}

	exists, err := h.service.CategoryExists(c.Request.Context(), categoryID)
	if err != nil {
		response.Internal(c, "create pin: category exists", err, "category_id", categoryID)
		return
	}
	if !exists {
		response.BadRequest(c, "category not found")
		return
	}

	photoURLs, thumbURLs, err := h.savePhotos(validated)
	if err != nil {
		slog.Error("create pin: save photo", "error", err.Error())
		response.Error(c, http.StatusInternalServerError, "could not save uploaded file")
		return
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
		// The photos are already on disk; remove them so a failed insert
		// cannot orphan files.
		for i := range photoURLs {
			_ = h.store.Delete(photoURLs[i])
			_ = h.store.Delete(thumbURLs[i])
		}
		response.Internal(c, "create pin: database insert", err,
			"user_id", middleware.GetUserID(c),
			"lat", lat,
			"lng", lng,
			"category_id", categoryID,
			"photos", len(photoURLs))
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

	// Broadcast to connected maps (best-effort; never fails the create).
	cover := ""
	if len(thumbURLs) > 0 {
		cover = thumbURLs[0]
	} else if len(photoURLs) > 0 {
		cover = photoURLs[0]
	}
	h.events.PinCreated(c.Request.Context(), Event{
		ID:         pin.ID.String(),
		UserID:     pin.UserID.String(),
		Location:   fmt.Sprintf("POINT(%v %v)", lng, lat),
		Caption:    pin.Caption,
		CategoryID: pin.CategoryID,
		CoverURL:   cover,
		CreatedAt:  pin.CreatedAt,
	})

	response.Created(c, gin.H{"pin": pin, "photos": photos})
}

// processPhotos opens, validates, and re-encodes every uploaded photo. The
// returned *photoErr is non-nil on failure and carries the exact status and
// message to respond with.
func processPhotos(files []*multipart.FileHeader) ([]validatedFile, *photoErr) {
	validated := make([]validatedFile, 0, len(files))
	for i, fh := range files {
		if fh.Size > maxPhotoSize {
			return nil, &photoErr{http.StatusBadRequest, "one or more photos exceed 10MB"}
		}

		src, err := fh.Open()
		if err != nil {
			return nil, &photoErr{http.StatusInternalServerError, "could not read uploaded file"}
		}
		data, err := io.ReadAll(src)
		src.Close()
		if err != nil {
			return nil, &photoErr{http.StatusInternalServerError, "could not read uploaded file"}
		}

		if err := imaging.Validate(data); err != nil {
			return nil, &photoErr{http.StatusBadRequest, fmt.Sprintf("photo %d: %s", i+1, err)}
		}

		proc, err := imaging.Process(data)
		if err != nil {
			return nil, &photoErr{http.StatusBadRequest, fmt.Sprintf("photo %d: %s", i+1, err)}
		}

		validated = append(validated, validatedFile{data: proc.Full, thumb: proc.Thumb, ext: "webp"})
	}
	return validated, nil
}

// savePhotos writes the processed photos to storage. On any failure it removes
// every file it already wrote, so a failed create cannot orphan files on disk.
func (h *Handler) savePhotos(validated []validatedFile) (photoURLs, thumbURLs []string, err error) {
	type stored struct{ full, thumb string }
	written := make([]stored, 0, len(validated))
	cleanup := func() {
		for _, s := range written {
			_ = h.store.Delete(s.full)
			_ = h.store.Delete(s.thumb)
		}
	}

	photoURLs = make([]string, 0, len(validated))
	thumbURLs = make([]string, 0, len(validated))
	for _, vf := range validated {
		full, err := h.store.Save(vf.data, vf.ext)
		if err != nil {
			cleanup()
			return nil, nil, err
		}
		thumb, err := h.store.Save(vf.thumb, vf.ext)
		if err != nil {
			cleanup()
			return nil, nil, err
		}
		written = append(written, stored{full: full, thumb: thumb})
		photoURLs = append(photoURLs, full)
		thumbURLs = append(thumbURLs, thumb)
	}
	return photoURLs, thumbURLs, nil
}

func (h *Handler) SearchPins(c *gin.Context) {
	q := strings.TrimSpace(c.Query("q"))
	if q == "" {
		response.BadRequest(c, "q query param is required")
		return
	}
	if utf8.RuneCountInString(q) > searchMaxQueryLen {
		response.BadRequest(c, "q must be at most 100 characters")
		return
	}

	limit, ok := httpx.ParseLimit(c, searchDefaultLimit, searchMaxLimit)
	if !ok {
		return
	}

	pins, err := h.service.SearchPins(c.Request.Context(), q, limit)
	if err != nil {
		response.Internal(c, "search pins", err, "query", q)
		return
	}

	response.OK(c, gin.H{"pins": pins})
}

// ListByUser returns the public pins of a user for their profile page.
func (h *Handler) ListByUser(c *gin.Context) {
	limit, ok := httpx.ParseLimit(c, profileDefaultLimit, profileMaxLimit)
	if !ok {
		return
	}

	pins, err := h.service.ListByUser(c.Request.Context(), c.Param("id"), limit)
	if err != nil {
		response.Internal(c, "pins: list by user", err, "user_id", c.Param("id"))
		return
	}

	response.OK(c, gin.H{"pins": pins})
}

// UpdatePin edits a pin owned by the caller (or any pin for moderators).
// Multipart contract (the client always sends the whole current state):
//   - caption:     present string; "" clears the caption
//   - category_id: optional; when present it replaces the category
//   - photos[]:    optional; when present it replaces the whole photo set
func (h *Handler) UpdatePin(c *gin.Context) {
	userID := middleware.GetUserID(c)
	id := c.Param("id")

	existing, err := h.service.GetPin(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.NotFound(c, "pin not found")
			return
		}
		response.Internal(c, "update pin: get", err, "pin_id", id)
		return
	}

	role := middleware.GetRole(c)
	if existing.UserID.String() != userID && role != "admin" && role != "owner" {
		response.Forbidden(c, "you can only edit your own pins")
		return
	}

	form, err := c.MultipartForm()
	if err != nil {
		response.BadRequest(c, "expected multipart form data")
		return
	}

	caption := strings.TrimSpace(c.PostForm("caption"))
	if utf8.RuneCountInString(caption) > 500 {
		response.BadRequest(c, "caption must be at most 500 characters")
		return
	}

	var categoryID *int
	if catStr := strings.TrimSpace(c.PostForm("category_id")); catStr != "" {
		idv, err := strconv.Atoi(catStr)
		if err != nil {
			response.BadRequest(c, "category_id must be an integer")
			return
		}
		exists, err := h.service.CategoryExists(c.Request.Context(), idv)
		if err != nil {
			response.Internal(c, "update pin: category exists", err, "category_id", idv)
			return
		}
		if !exists {
			response.BadRequest(c, "category not found")
			return
		}
		categoryID = &idv
	}

	patch := UpdatePinPatch{
		Caption:    &caption,
		CategoryID: categoryID,
	}

	files := form.File["photos"]
	if len(files) > maxPhotosPerPin {
		response.BadRequest(c, "photo count exceeds maximum")
		return
	}
	if len(files) > 0 {
		validated, perr := processPhotos(files)
		if perr != nil {
			if perr.status == http.StatusInternalServerError {
				slog.Error("update pin: read uploaded file", "error", perr.msg)
			}
			response.Error(c, perr.status, perr.msg)
			return
		}

		photoURLs, thumbURLs, err := h.savePhotos(validated)
		if err != nil {
			slog.Error("update pin: save photo", "error", err.Error())
			response.Error(c, http.StatusInternalServerError, "could not save uploaded file")
			return
		}

		patch.Photos = make([]NewPhoto, 0, len(photoURLs))
		for i := range photoURLs {
			patch.Photos = append(patch.Photos, NewPhoto{PhotoURL: photoURLs[i], ThumbnailURL: thumbURLs[i]})
		}
	}

	updated, err := h.service.UpdatePin(c.Request.Context(), id, patch)
	if err != nil {
		// The new photos are already on disk; remove them so a failed update
		// cannot orphan files.
		for _, ph := range patch.Photos {
			_ = h.store.Delete(ph.PhotoURL)
			_ = h.store.Delete(ph.ThumbnailURL)
		}
		if errors.Is(err, ErrNotFound) {
			response.NotFound(c, "pin not found")
			return
		}
		response.Internal(c, "update pin: database update", err, "pin_id", id)
		return
	}

	// Best-effort cleanup of the replaced photos now that the swap succeeded.
	if patch.Photos != nil {
		for _, ph := range existing.Photos {
			_ = h.store.Delete(ph.PhotoURL)
			_ = h.store.Delete(ph.ThumbnailURL)
		}
	}

	response.OK(c, gin.H{"pin": updated})
}
