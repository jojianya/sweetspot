package pins

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/jojianya/sweetspot247-backend/internal/http/middleware"
	httpx "github.com/jojianya/sweetspot247-backend/internal/http/params"
	"github.com/jojianya/sweetspot247-backend/internal/http/response"
	"github.com/jojianya/sweetspot247-backend/internal/modules/user"
	"github.com/jojianya/sweetspot247-backend/internal/platform/storage"
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

// photoErr carries an input-validation failure: the HTTP status to respond
// with and the client-facing message. Returned by the field validators in
// validate.go, which check request shape (handler-layer input validation).
type photoErr struct {
	status int
	msg    string
}

type Handler struct {
	repo   Repository
	store  *storage.Local
	events Events
	// roles resolves moderation rights from the database rather than the JWT,
	// so a demotion takes effect on the caller's next request.
	roles users.RoleReader
	svc   *Service
}

func NewHandler(repo Repository, store *storage.Local, events Events, roles users.RoleReader) *Handler {
	if events == nil {
		events = nopEvents{}
	}
	return &Handler{repo: repo, store: store, events: events, roles: roles, svc: NewService(repo, store, events, roles)}
}

// service returns the service, building it from the handler's dependencies
// when the handler was constructed as a struct literal (as some tests do)
// instead of via NewHandler.
func (h *Handler) service() *Service {
	if h.svc != nil {
		return h.svc
	}
	return NewService(h.repo, h.store, h.events, h.roles)
}

// isModerator resolves moderation rights for the caller. Without a wired
// RoleReader nobody is a moderator.
func (h *Handler) isModerator(c *gin.Context) bool {
	if h.roles == nil {
		return false
	}
	return users.IsModerator(h.roles, c)
}

// nopEvents is the zero-value event publisher used when realtime is disabled.
type nopEvents struct{}

func (nopEvents) PinCreated(context.Context, Event)      {}
func (nopEvents) PinRemoved(context.Context, PinRemoved) {}

func (h *Handler) ListCategories(c *gin.Context) {
	categories, err := h.service().ListCategories(c.Request.Context())
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

	limit, ok := httpx.ParseLimit(c, pinListDefaultLimit, pinListDefaultLimit)
	if !ok {
		return
	}

	pins, err := h.service().ListPins(c.Request.Context(), bbox, categoryID, limit)
	if err != nil {
		if errors.Is(err, ErrCategoryNotFound) {
			response.BadRequest(c, "category not found")
			return
		}
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

	pins, err := h.service().ListTrending(c.Request.Context(), bbox, limit)
	if err != nil {
		response.Internal(c, "pins: trending", err)
		return
	}

	response.OK(c, gin.H{"pins": pins})
}

func (h *Handler) GetPin(c *gin.Context) {
	pin, err := h.service().GetVisible(c.Request.Context(), c.Param("id"), middleware.GetUserID(c))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.NotFound(c, "pin not found")
			return
		}
		response.Internal(c, "pins: get", err, "pin_id", c.Param("id"))
		return
	}

	response.OK(c, gin.H{"pin": pin})
}

// RegisterView counts a unique per-account view. It is read-safe for
// anonymous visitors: without a session it returns the current count
// unchanged, so opening a pin logged out never errors and never counts.
func (h *Handler) RegisterView(c *gin.Context) {
	views, err := h.service().RegisterView(c.Request.Context(), c.Param("id"), middleware.GetUserID(c))
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

	if err := h.service().DeletePin(c.Request.Context(), c.Param("id"), userID, h.isModerator(c)); err != nil {
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

	files := form.File["photos"]
	input, verr := validateCreateFields(c.PostForm("lat"), c.PostForm("lng"), c.PostForm("category_id"), c.PostForm("caption"), len(files))
	if verr != nil {
		response.Error(c, verr.status, verr.msg)
		return
	}

	res, err := h.service().CreatePin(c.Request.Context(), CreatePinInput{
		UserID:     userID,
		Lat:        input.lat,
		Lng:        input.lng,
		Caption:    input.caption,
		CategoryID: input.categoryID,
		Files:      files,
	})
	if err != nil {
		switch {
		case errors.Is(err, ErrAccountMissing):
			response.Unauthorized(c, err.Error())
		case errors.Is(err, ErrCategoryNotFound):
			response.BadRequest(c, "category not found")
		case errors.Is(err, ErrPhotoTooLarge), errors.Is(err, ErrPhotoInvalid):
			response.BadRequest(c, err.Error())
		case errors.Is(err, ErrPhotoUnreadable):
			slog.Error("create pin: read uploaded file", "error", err.Error())
			response.Error(c, http.StatusInternalServerError, err.Error())
		case errors.Is(err, ErrPhotoSave):
			slog.Error("create pin: save photo", "error", err.Error())
			response.Error(c, http.StatusInternalServerError, "could not save uploaded file")
		default:
			response.Internal(c, "create pin: database insert", err,
				"user_id", userID,
				"lat", input.lat,
				"lng", input.lng,
				"category_id", input.categoryID,
				"photos", len(res.PhotoURLs))
		}
		return
	}

	photos := make([]gin.H, 0, len(res.PhotoURLs))
	for i := range res.PhotoURLs {
		photos = append(photos, gin.H{
			"photo_url":     res.PhotoURLs[i],
			"thumbnail_url": res.ThumbURLs[i],
			"position":      i,
		})
	}

	response.Created(c, gin.H{"pin": res.Pin, "photos": photos})
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

	pins, err := h.service().SearchPins(c.Request.Context(), q, limit)
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

	pins, err := h.service().ListByUser(c.Request.Context(), c.Param("id"), limit)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.NotFound(c, "user not found")
			return
		}
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
//
// The response carries the pin's photo set after the change, matching
// CreatePin's {"pin", "photos"} envelope, so the client can adopt the new photo
// URLs without refetching the pin.
func (h *Handler) UpdatePin(c *gin.Context) {
	userID := middleware.GetUserID(c)
	id := c.Param("id")

	form, err := c.MultipartForm()
	if err != nil {
		response.BadRequest(c, "expected multipart form data")
		return
	}

	fields, verr := validateUpdateFields(
		strings.TrimSpace(c.PostForm("caption")),
		strings.TrimSpace(c.PostForm("category_id")),
		len(form.File["photos"]),
	)
	if verr != nil {
		response.Error(c, verr.status, verr.msg)
		return
	}
	var captionPtr *string
	if _, ok := form.Value["caption"]; ok {
		captionPtr = &fields.caption
	}

	res, err := h.service().UpdatePin(c.Request.Context(), UpdatePinInput{
		ID:          id,
		UserID:      userID,
		IsModerator: h.isModerator(c),
		Caption:     captionPtr,
		CategoryID:  fields.categoryID,
		Files:       form.File["photos"],
	})
	if err != nil {
		switch {
		case errors.Is(err, ErrNotFound):
			response.NotFound(c, "pin not found")
		case errors.Is(err, ErrForbidden):
			response.Forbidden(c, "you can only edit your own pins")
		case errors.Is(err, ErrCategoryNotFound):
			response.BadRequest(c, "category not found")
		case errors.Is(err, ErrPhotoTooLarge), errors.Is(err, ErrPhotoInvalid):
			response.BadRequest(c, err.Error())
		case errors.Is(err, ErrPhotoUnreadable):
			slog.Error("update pin: read uploaded file", "error", err.Error())
			response.Error(c, http.StatusInternalServerError, err.Error())
		case errors.Is(err, ErrPhotoSave):
			slog.Error("update pin: save photo", "error", err.Error())
			response.Error(c, http.StatusInternalServerError, "could not save uploaded file")
		default:
			response.Internal(c, "update pin: database update", err, "pin_id", id)
		}
		return
	}

	response.OK(c, gin.H{"pin": res.Pin, "photos": res.Photos})
}
