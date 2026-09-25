package users

import (
	"encoding/json"
	"errors"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/jojianya/sweetspot247-backend/internal/http/middleware"
	httpx "github.com/jojianya/sweetspot247-backend/internal/http/params"
	"github.com/jojianya/sweetspot247-backend/internal/http/response"
	"github.com/jojianya/sweetspot247-backend/internal/modules/pins/imaging"
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

	var patch UpdateProfilePatch
	if vals := fields["username"]; len(vals) > 0 {
		username := strings.TrimSpace(vals[0])
		if username == "" {
			response.BadRequest(c, "username cannot be empty")
			return
		}
		if n := utf8.RuneCountInString(username); n < usernameMinRunes || n > usernameMaxRunes {
			response.BadRequest(c, "username must be between 3 and 30 characters")
			return
		}
		patch.Username = &username
	}

	if vals := fields["socials"]; len(vals) > 0 {
		socials, parseErr := parseSocials(vals[0])
		if parseErr != nil {
			response.BadRequest(c, parseErr.Error())
			return
		}
		patch.Socials = &socials
	}

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
		data, readErr := io.ReadAll(src)
		src.Close()
		if readErr != nil {
			response.Internal(c, "user: read avatar", readErr)
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

	if patch.Username == nil && patch.Socials == nil && patch.AvatarURL == nil {
		response.BadRequest(c, "nothing to update")
		return
	}

	updated, err := h.service.UpdateProfile(c.Request.Context(), userID, patch)
	if err != nil {
		if newAvatar != nil {
			_ = h.store.Delete(*newAvatar)
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

	// The previous avatar file is superseded; drop it (best-effort).
	if newAvatar != nil && current.AvatarURL != nil && *current.AvatarURL != *newAvatar {
		_ = h.store.Delete(*current.AvatarURL)
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
// and the response always includes `total` — the number of registered users.
// Owner-gated at the route level.
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
		err   error
	)
	if query == "" {
		found, err = h.service.ListUsers(c.Request.Context(), limit, offset)
	} else {
		found, err = h.service.SearchUsers(c.Request.Context(), query, limit)
	}
	if err != nil {
		response.Internal(c, "user: list", err)
		return
	}

	total, err := h.service.CountUsers(c.Request.Context())
	if err != nil {
		response.Internal(c, "user: count", err)
		return
	}

	items := make([]PublicUser, 0, len(found))
	for i := range found {
		items = append(items, found[i].ToPublic())
	}
	response.OK(c, gin.H{"users": items, "total": total})
}
