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
	"github.com/jojianya/sweetspot247-backend/internal/modules/user"
	"github.com/jojianya/sweetspot247-backend/internal/platform/imaging"
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
	repo   Repository
	store  *storage.Local
	events Events
	// roles resolves moderation rights from the database rather than the JWT,
	// so a demotion takes effect on the caller's next request.
	roles users.RoleReader
}

func NewHandler(repo Repository, store *storage.Local, events Events, roles users.RoleReader) *Handler {
	if events == nil {
		events = nopEvents{}
	}
	return &Handler{repo: repo, store: store, events: events, roles: roles}
}

// nopEvents is the zero-value event publisher used when realtime is disabled.
type nopEvents struct{}

func (nopEvents) PinCreated(context.Context, Event)      {}
func (nopEvents) PinRemoved(context.Context, PinRemoved) {}

func (h *Handler) ListCategories(c *gin.Context) {
	categories, err := h.repo.ListCategories(c.Request.Context())
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
		exists, err := h.repo.CategoryExists(c.Request.Context(), *categoryID)
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

	pins, err := h.repo.ListPins(c.Request.Context(), bbox, categoryID, limit)
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

	pins, err := h.repo.ListTrending(c.Request.Context(), bbox, limit)
	if err != nil {
		response.Internal(c, "pins: trending", err)
		return
	}

	response.OK(c, gin.H{"pins": pins})
}

func (h *Handler) GetPin(c *gin.Context) {
	pin, err := h.repo.GetPin(c.Request.Context(), c.Param("id"))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.NotFound(c, "pin not found")
			return
		}
		response.Internal(c, "pins: get", err, "pin_id", c.Param("id"))
		return
	}

	if pin.IsHidden && !h.canViewHidden(c, pin) {
		response.NotFound(c, "pin not found")
		return
	}

	response.OK(c, gin.H{"pin": pin})
}

func (h *Handler) canViewHidden(c *gin.Context, pin PinDetail) bool {
	viewerID := middleware.GetUserID(c)
	if viewerID == "" {
		return false
	}
	if viewerID == pin.UserID {
		return true
	}

	// Only reached for a hidden pin owned by someone else, so the lookup below
	// is off the common path.
	return users.IsModerator(h.roles, c)
}

// RegisterView counts a unique per-account view. It is read-safe for
// anonymous visitors: without a session it returns the current count
// unchanged, so opening a pin logged out never errors and never counts.
func (h *Handler) RegisterView(c *gin.Context) {
	views, err := h.repo.RegisterView(c.Request.Context(), c.Param("id"), middleware.GetUserID(c))
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

	id := c.Param("id")

	// Moderation check reuses the same live-DB IsModerator as UpdatePin, so a
	// demotion takes effect immediately. It is computed unconditionally (not
	// from a pre-loaded owner) because authorization lives in the repository
	// UPDATE predicate below — the pre-delete read is only for photo cleanup.
	isModerator := false
	if h.roles != nil {
		isModerator = users.IsModerator(h.roles, c)
	}

	if err := h.repo.DeletePin(c.Request.Context(), id, userID, isModerator); err != nil {
		if errors.Is(err, ErrNotFound) {
			response.NotFound(c, "pin not found")
			return
		}
		if errors.Is(err, ErrForbidden) {
			response.Forbidden(c, "you can only delete your own pins")
			return
		}
		response.Internal(c, "delete pin", err, "pin_id", id, "user_id", userID)
		return
	}

	// Load the (now hidden) pin for photo cleanup and moderator audit logging.
	// pin_photos rows survive the soft-hide, so this read sees the
	// authoritative set. A read failure here must not fail the delete; files
	// are unreferenced either way and the miss is logged for a sweep.
	existing, err := h.repo.GetPin(c.Request.Context(), id)
	if err != nil {
		slog.Warn("delete pin: load for cleanup", "error", err.Error(), "pin_id", id, "user_id", userID)
		response.NoContent(c)
		return
	}

	if existing.UserID != userID {
		slog.Info("moderator deleted pin", "moderator_id", userID, "pin_id", id, "owner_id", existing.UserID)
	}

	// Best-effort cleanup, matching what UpdatePin does for replaced photos. The
	// pin is already gone, so a storage failure must not fail the request; the
	// files are unreferenced either way. Logged so an operator can sweep them.
	for _, ph := range existing.Photos {
		if err := h.store.Delete(ph.PhotoURL); err != nil {
			slog.Warn("delete pin: remove photo", "error", err.Error(), "url", ph.PhotoURL, "pin_id", id)
		}
		if err := h.store.Delete(ph.ThumbnailURL); err != nil {
			slog.Warn("delete pin: remove thumbnail", "error", err.Error(), "url", ph.ThumbnailURL, "pin_id", id)
		}
	}

	// Publish pin_removed event for realtime updates (best-effort; failure logged but not fatal).
	if h.events != nil {
		h.events.PinRemoved(c.Request.Context(), PinRemoved{
			ID:       id,
			Location: existing.Location,
		})
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
	userExists, err := h.repo.UserExists(c.Request.Context(), userID)
	if err != nil {
		response.Internal(c, "create pin: user exists", err, "user_id", userID)
		return
	}
	if !userExists {
		response.Unauthorized(c, "account no longer exists, please sign in again")
		return
	}

	files := form.File["photos"]
	input, verr := validateCreateFields(c.PostForm("lat"), c.PostForm("lng"), c.PostForm("category_id"), c.PostForm("caption"), len(files))
	if verr != nil {
		response.Error(c, verr.status, verr.msg)
		return
	}
	lat, lng, categoryID, caption := input.lat, input.lng, input.categoryID, input.caption

	validated, perr := processPhotos(files)
	if perr != nil {
		if perr.status == http.StatusInternalServerError {
			slog.Error("create pin: read uploaded file", "error", perr.msg)
		}
		response.Error(c, perr.status, perr.msg)
		return
	}

	exists, err := h.repo.CategoryExists(c.Request.Context(), categoryID)
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

	pin, err := h.repo.CreatePin(c.Request.Context(), NewPin{
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
		// cannot orphan files. Use min length to guard against mismatched slices.
		// Log deletion failures instead of discarding them.
		n := len(photoURLs)
		if len(thumbURLs) < n {
			n = len(thumbURLs)
		}
		for i := 0; i < n; i++ {
			if err := h.store.Delete(photoURLs[i]); err != nil {
				slog.Warn("cleanup failed: photo", "url", photoURLs[i], "error", err)
			}
			if err := h.store.Delete(thumbURLs[i]); err != nil {
				slog.Warn("cleanup failed: thumbnail", "url", thumbURLs[i], "error", err)
			}
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
		ID:         pin.ID,
		UserID:     pin.UserID,
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
		// Cap the read at max+1 so a lied-about FileHeader.Size cannot push
		// an unbounded body into memory; the length check below is authoritative.
		data, err := io.ReadAll(io.LimitReader(src, maxPhotoSize+1))
		src.Close()
		if err != nil {
			return nil, &photoErr{http.StatusInternalServerError, "could not read uploaded file"}
		}
		if len(data) > maxPhotoSize {
			return nil, &photoErr{http.StatusBadRequest, "one or more photos exceed 10MB"}
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
		// Best-effort: the create already failed, so a delete failure only
		// leaves an orphan file; log it for the operator sweep.
		for _, s := range written {
			if err := h.store.Delete(s.full); err != nil {
				slog.Warn("create pin cleanup: remove photo", "error", err.Error(), "url", s.full)
			}
			if err := h.store.Delete(s.thumb); err != nil {
				slog.Warn("create pin cleanup: remove thumbnail", "error", err.Error(), "url", s.thumb)
			}
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

	pins, err := h.repo.SearchPins(c.Request.Context(), q, limit)
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

	userID := c.Param("id")
	exists, err := h.repo.UserExists(c.Request.Context(), userID)
	if err != nil {
		response.Internal(c, "pins: check profile user", err, "user_id", userID)
		return
	}
	if !exists {
		response.NotFound(c, "user not found")
		return
	}

	pins, err := h.repo.ListByUser(c.Request.Context(), userID, limit)
	if err != nil {
		response.Internal(c, "pins: list by user", err, "user_id", userID)
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

	existing, err := h.repo.GetPin(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.NotFound(c, "pin not found")
			return
		}
		response.Internal(c, "update pin: get", err, "pin_id", id)
		return
	}

	// Hidden pins do not exist for edits. Checked before any work is done so a
	// hidden pin answers 404 rather than revealing that it exists, the same rule
	// DeletePin, comments, favorites, reports and collections follow. The UPDATE
	// predicate repeats the check; this one just avoids staging uploads for a pin
	// that cannot be edited.
	visible, err := h.repo.PinVisible(c.Request.Context(), id)
	if err != nil {
		response.Internal(c, "update pin: visibility check", err, "pin_id", id)
		return
	}
	if !visible {
		response.NotFound(c, "pin not found")
		return
	}

	// Without a wired RoleReader nobody is a moderator (same as DeletePin).
	// Ownership itself is enforced by the UPDATE predicate, so there is no
	// load-then-decide window between this read and the write.
	isModerator := false
	if h.roles != nil {
		isModerator = users.IsModerator(h.roles, c)
	}

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
	caption, categoryID := fields.caption, fields.categoryID
	var captionPtr *string
	if _, ok := form.Value["caption"]; ok {
		captionPtr = &caption
	}

	if categoryID != nil {
		exists, err := h.repo.CategoryExists(c.Request.Context(), *categoryID)
		if err != nil {
			response.Internal(c, "update pin: category exists", err, "category_id", *categoryID)
			return
		}
		if !exists {
			response.BadRequest(c, "category not found")
			return
		}
	}

	patch := UpdatePinPatch{
		Caption:    captionPtr,
		CategoryID: categoryID,
	}

	files := form.File["photos"]
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

	updated, photos, err := h.repo.UpdatePin(c.Request.Context(), id, userID, isModerator, patch)
	if err != nil {
		// The new photos are already on disk; remove them so a failed update
		// cannot orphan files. Best-effort: the update already failed, so a
		// delete failure only leaves an orphan file for the operator sweep.
		for _, ph := range patch.Photos {
			if derr := h.store.Delete(ph.PhotoURL); derr != nil {
				slog.Warn("update pin cleanup: remove photo", "error", derr.Error(), "url", ph.PhotoURL, "pin_id", id)
			}
			if derr := h.store.Delete(ph.ThumbnailURL); derr != nil {
				slog.Warn("update pin cleanup: remove thumbnail", "error", derr.Error(), "url", ph.ThumbnailURL, "pin_id", id)
			}
		}
		if errors.Is(err, ErrNotFound) {
			response.NotFound(c, "pin not found")
			return
		}
		if errors.Is(err, ErrForbidden) {
			response.Forbidden(c, "you can only edit your own pins")
			return
		}
		response.Internal(c, "update pin: database update", err, "pin_id", id)
		return
	}

	// Best-effort cleanup of the replaced photos now that the swap succeeded.
	if patch.Photos != nil {
		for _, ph := range existing.Photos {
			if err := h.store.Delete(ph.PhotoURL); err != nil {
				slog.Warn("update pin: remove replaced photo", "error", err.Error(), "url", ph.PhotoURL, "pin_id", id)
			}
			if err := h.store.Delete(ph.ThumbnailURL); err != nil {
				slog.Warn("update pin: remove replaced thumbnail", "error", err.Error(), "url", ph.ThumbnailURL, "pin_id", id)
			}
		}
	}

	response.OK(c, gin.H{"pin": updated, "photos": photos})
}
