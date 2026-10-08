package users

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/jojianya/sweetspot247-backend/internal/http/middleware"
	httpx "github.com/jojianya/sweetspot247-backend/internal/http/params"
	"github.com/jojianya/sweetspot247-backend/internal/http/response"
	"github.com/jojianya/sweetspot247-backend/internal/platform/imaging"
	"github.com/jojianya/sweetspot247-backend/internal/platform/storage"
)

const (
	userSearchDefaultLimit = 20
	userSearchMaxLimit     = 50
	userListDefaultLimit   = 50

	maxAvatarSize    = 5 << 20
	maxSocialKeys    = 20
	maxSocialKeyLen  = 64
	maxSocialValLen  = 500
	maxMultipartMem  = 32 << 20
	usernameMinRunes = 3
	usernameMaxRunes = 30
)

type Handler struct {
	service Service
	store   *storage.Local
}

func NewHandler(service Service, store *storage.Local) *Handler {
	return &Handler{service: service, store: store}
}

func (h *Handler) Get(c *gin.Context) {
	user, err := h.service.GetByID(c.Request.Context(), c.Param("id"))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.NotFound(c, "user not found")
			return
		}
		response.Internal(c, "user: get", err)
		return
	}

	if viewerID := middleware.GetUserID(c); viewerID != "" && viewerID == user.ID {
		response.OK(c, user.ToPrivate())
		return
	}
	response.OK(c, user.ToPublic())
}

func (h *Handler) UpdateRole(c *gin.Context) {
	var req UpdateRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	user, err := h.service.UpdateRole(c.Request.Context(), middleware.GetUserID(c), c.Param("id"), req.Role)
	if err != nil {
		switch {
		case errors.Is(err, ErrNotFound):
			response.NotFound(c, "user not found")
			return
		case errors.Is(err, ErrCannotChangeOwnRole), errors.Is(err, ErrCannotDemoteLastOwner):
			response.BadRequest(c, err.Error())
			return
		default:
			response.Internal(c, "user: update role", err)
			return
		}
	}

	response.OK(c, user.ToPublic())
}

// UpdateMe updates the caller's own profile (multipart/form-data): optional
// `username`, `socials` (JSON object), and `avatar` (jpg/png) fields. Any
// combination may be sent; sending none is a 400.
func (h *Handler) UpdateMe(c *gin.Context) {
	userID := middleware.GetUserID(c)

	current, err := h.service.GetByID(c.Request.Context(), userID)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.NotFound(c, "user not found")
			return
		}
		response.Internal(c, "user: get", err)
		return
	}

	if err := c.Request.ParseMultipartForm(maxMultipartMem); err != nil {
		response.BadRequest(c, "could not parse form")
		return
	}
	fields := c.Request.MultipartForm.Value

	input, msg := validateProfileFields(fields["username"], fields["socials"], len(c.Request.MultipartForm.File["avatar"]) > 0)
	if msg != "" {
		response.BadRequest(c, msg)
		return
	}

	var patch UpdateProfilePatch
	patch.Username = input.username
	patch.Socials = input.socials

	var newAvatar *string
	if fhs := c.Request.MultipartForm.File["avatar"]; len(fhs) > 0 {
		fh := fhs[0]
		if fh.Size > maxAvatarSize {
			response.BadRequest(c, "avatar exceeds 5MB")
			return
		}
		src, err := fh.Open()
		if err != nil {
			response.Internal(c, "user: open avatar", err)
			return
		}
		// Cap at max+1 so the length check below enforces actual bytes, not
		// the client-claimed FileHeader.Size.
		data, readErr := io.ReadAll(io.LimitReader(src, maxAvatarSize+1))
		src.Close()
		if readErr != nil {
			response.Internal(c, "user: read avatar", readErr)
			return
		}
		if len(data) > maxAvatarSize {
			response.BadRequest(c, "avatar exceeds 5MB")
			return
		}
		if err := imaging.Validate(data); err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		processed, err := imaging.Avatar(data)
		if err != nil {
			response.Internal(c, "user: process avatar", err)
			return
		}
		url, err := h.store.Save(processed, "webp")
		if err != nil {
			response.Internal(c, "user: save avatar", err)
			return
		}
		newAvatar = &url
		patch.AvatarURL = &url
	}

	updated, err := h.service.UpdateProfile(c.Request.Context(), userID, patch)
	if err != nil {
		if newAvatar != nil {
			// Best-effort: the update already failed, so a delete failure only
			// leaves an orphan file for the operator sweep.
			if derr := h.store.Delete(*newAvatar); derr != nil {
				slog.Warn("update profile cleanup: remove new avatar", "error", derr.Error(), "url", *newAvatar, "user_id", userID)
			}
		}
		switch {
		case errors.Is(err, ErrNotFound):
			response.NotFound(c, "user not found")
			return
		case errors.Is(err, ErrUsernameTaken):
			response.Conflict(c, "username already taken")
			return
		default:
			response.Internal(c, "user: update profile", err)
			return
		}
	}

	// The previous avatar file is superseded; drop it (best-effort: a delete
	// failure leaves an orphan file, logged for the operator sweep).
	if newAvatar != nil && current.AvatarURL != nil && *current.AvatarURL != *newAvatar {
		if err := h.store.Delete(*current.AvatarURL); err != nil {
			slog.Warn("update profile: remove old avatar", "error", err.Error(), "url", *current.AvatarURL, "user_id", userID)
		}
	}

	response.OK(c, updated.ToPrivate())
}

// parseSocials decodes the `socials` form field into a flat JSON object of
// string, boolean, or number values, with bounded sizes.
func parseSocials(raw string) (map[string]any, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, errors.New("socials must be a JSON object")
	}
	var socials map[string]any
	if err := json.Unmarshal([]byte(raw), &socials); err != nil {
		return nil, errors.New("socials must be a JSON object")
	}
	if len(socials) > maxSocialKeys {
		return nil, errors.New("too many socials (max 20)")
	}
	for k, v := range socials {
		if utf8.RuneCountInString(k) > maxSocialKeyLen {
			return nil, errors.New("social name too long")
		}
		switch val := v.(type) {
		case string:
			if utf8.RuneCountInString(val) > maxSocialValLen {
				return nil, errors.New("social value too long")
			}
		case bool, float64:
		default:
			return nil, errors.New("socials values must be strings, booleans, or numbers")
		}
	}
	return socials, nil
}

// List returns users for the owner's role-management console. With `q` it
// filters to usernames containing the query (case-insensitive); without, it
// returns every user (newest first). Both modes are paginated via limit/offset,
// and the response always includes `total` — the number of registered users
// when listing, or the number of matches when searching. Owner-gated at the
// route level.
func (h *Handler) List(c *gin.Context) {
	query := strings.TrimSpace(c.Query("q"))
	if len(query) > 64 {
		response.BadRequest(c, "q must be at most 64 characters")
		return
	}

	defLimit := userSearchDefaultLimit
	if query == "" {
		defLimit = userListDefaultLimit
	}
	limit, ok := httpx.ParseLimit(c, defLimit, userSearchMaxLimit)
	if !ok {
		return
	}
	offset, ok := httpx.ParseOffset(c)
	if !ok {
		return
	}

	var (
		found []User
		total int
		err   error
	)
	if query == "" {
		found, total, err = h.service.ListUsers(c.Request.Context(), limit, offset)
	} else {
		found, total, err = h.service.SearchUsers(c.Request.Context(), query, limit, offset)
	}
	if err != nil {
		response.Internal(c, "user: list", err)
		return
	}

	items := make([]PublicUser, 0, len(found))
	for i := range found {
		items = append(items, found[i].ToPublic())
	}
	response.OK(c, gin.H{"users": items, "total": total})
}

// RoleReader is the subset of Service needed to resolve a caller's role.
//
// Narrower than Service on purpose: every module that needs a role check
// depends on this single-method interface instead of the full user service,
// which keeps the dependency obvious and lets tests supply a one-method stub
// rather than implementing all ten.
type RoleReader interface {
	GetByID(ctx context.Context, id string) (User, error)
}

// CurrentRole resolves the caller role from the request context using the user
// service.
//
// When the auth middleware already resolved the session it caches the live
// role on the context, and that cached value wins: it came from the same
// database read that enforced session revocation, so reusing it adds no query
// and cannot disagree with the middleware. Otherwise this falls back to a
// live lookup, preserving the stale-JWT-role fix for callers outside
// middleware (tests, optional-auth paths without a checker).
func CurrentRole(svc RoleReader, c *gin.Context) string {
	if role := c.GetString(middleware.CtxRole); role != "" {
		return role
	}
	userID := middleware.GetUserID(c)
	if userID == "" {
		return ""
	}
	user, err := svc.GetByID(c.Request.Context(), userID)
	if err != nil {
		return ""
	}
	return user.Role
}

// IsModerator reports whether the caller currently holds a moderation role.
//
// Like CurrentRole this reads through to the database, so a demotion takes
// effect on the caller's next request.
func IsModerator(svc RoleReader, c *gin.Context) bool {
	role := CurrentRole(svc, c)
	return role == RoleAdmin || role == RoleOwner
}

func RequireAdmin(svc RoleReader) gin.HandlerFunc {
	return func(c *gin.Context) {
		if role := CurrentRole(svc, c); role != RoleAdmin && role != RoleOwner {
			response.AbortError(c, http.StatusForbidden, "admin access required")
			return
		}
		c.Next()
	}
}

func RequireOwner(svc RoleReader) gin.HandlerFunc {
	return func(c *gin.Context) {
		if role := CurrentRole(svc, c); role != RoleOwner {
			response.AbortError(c, http.StatusForbidden, "owner access required")
			return
		}
		c.Next()
	}
}
