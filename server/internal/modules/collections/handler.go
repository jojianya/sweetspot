package collections

import (
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/jojianya/sweetspot247-backend/internal/http/middleware"
	httpx "github.com/jojianya/sweetspot247-backend/internal/http/params"
	"github.com/jojianya/sweetspot247-backend/internal/http/response"
	"github.com/jojianya/sweetspot247-backend/internal/modules/user"
)

const (
	collectionPinListDefaultLimit = 50
	collectionPinListMaxLimit     = 200
)

// CreateCollectionRequest / UpdateCollectionRequest share the same shape:
// name is required, description is optional, and is_private is optional
// (absent means public on create and unchanged on update).
type CollectionRequest struct {
	Name        string  `json:"name" binding:"required"`
	Description *string `json:"description"`
	IsPrivate   *bool   `json:"is_private"`
}

type Handler struct {
	repo Repository
	// roles resolves moderation rights from the database rather than the JWT,
	// so a demotion takes effect on the caller's next request.
	roles users.RoleReader
}

func NewHandler(repo Repository, roles users.RoleReader) *Handler {
	return &Handler{repo: repo, roles: roles}
}

func (h *Handler) ListMine(c *gin.Context) {
	collections, err := h.repo.ListByUser(c.Request.Context(), middleware.GetUserID(c))
	if err != nil {
		response.Internal(c, "collections: list mine", err, "user_id", middleware.GetUserID(c))
		return
	}
	response.OK(c, gin.H{"collections": collections})
}

func (h *Handler) ListByUser(c *gin.Context) {
	userID := c.Param("id")
	exists, err := h.repo.UserExists(c.Request.Context(), userID)
	if err != nil {
		response.Internal(c, "collections: user exists", err, "user_id", userID)
		return
	}
	if !exists {
		response.NotFound(c, "user not found")
		return
	}

	// Private collections are visible only to their owner; everyone else,
	// logged in or not, sees the public subset.
	callerID := middleware.GetUserID(c)
	var collections []Collection
	var listErr error
	if callerID != "" && callerID == userID {
		collections, listErr = h.repo.ListByUser(c.Request.Context(), userID)
	} else {
		collections, listErr = h.repo.ListPublicByUser(c.Request.Context(), userID)
	}
	if listErr != nil {
		response.Internal(c, "collections: list by user", listErr, "user_id", userID)
		return
	}
	response.OK(c, gin.H{"collections": collections})
}

// normalize validates and trims the name/description fields.
func normalize(c *gin.Context, name string, description *string) (string, *string, bool) {
	name = strings.TrimSpace(name)
	if name == "" {
		response.BadRequest(c, "name is required")
		return "", nil, false
	}
	if utf8.RuneCountInString(name) > maxCollectionNameLength {
		response.BadRequest(c, "name must be at most 60 characters")
		return "", nil, false
	}
	if description != nil {
		trimmed := strings.TrimSpace(*description)
		if utf8.RuneCountInString(trimmed) > maxCollectionDescLength {
			response.BadRequest(c, "description must be at most 200 characters")
			return "", nil, false
		}
		description = &trimmed
	}
	return name, description, true
}

func (h *Handler) Create(c *gin.Context) {
	var req CollectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "name is required")
		return
	}

	name, description, ok := normalize(c, req.Name, req.Description)
	if !ok {
		return
	}

	isPrivate := req.IsPrivate != nil && *req.IsPrivate
	collection, err := h.repo.Create(c.Request.Context(), middleware.GetUserID(c), name, description, isPrivate)
	if err != nil {
		response.Internal(c, "collections: create", err, "user_id", middleware.GetUserID(c))
		return
	}

	response.Created(c, gin.H{"collection": collection})
}

func (h *Handler) Get(c *gin.Context) {
	id := c.Param("id")
	collection, err := h.repo.Get(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.NotFound(c, "collection not found")
			return
		}
		response.Internal(c, "collections: get", err, "collection_id", id)
		return
	}

	// Private collections exist only for their owner: anyone else gets 404,
	// the same convention as hidden pins, so privacy is not enumerable.
	if collection.IsPrivate && collection.UserID.String() != middleware.GetUserID(c) {
		response.NotFound(c, "collection not found")
		return
	}

	limit, ok := httpx.ParseLimit(c, collectionPinListDefaultLimit, collectionPinListMaxLimit)
	if !ok {
		return
	}
	offset, ok := httpx.ParseOffset(c)
	if !ok {
		return
	}

	pins, total, err := h.repo.ListPins(c.Request.Context(), id, limit, offset)
	if err != nil {
		response.Internal(c, "collections: get pins", err, "collection_id", id)
		return
	}

	response.OK(c, gin.H{"collection": CollectionDetail{Collection: collection, Pins: pins, PinTotal: total}})
}

// requireOwner aborts unless the caller owns the collection (or moderates),
// returning the collection so callers can reuse the read.
func (h *Handler) requireOwner(c *gin.Context) (Collection, bool) {
	collection, err := h.repo.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.NotFound(c, "collection not found")
			return Collection{}, false
		}
		response.Internal(c, "collections: get", err, "collection_id", c.Param("id"))
		return Collection{}, false
	}

	userID := middleware.GetUserID(c)
	if collection.UserID.String() != userID {
		// Only non-owners reach the role lookup, keeping it off the common path.
		if !users.IsModerator(h.roles, c) {
			response.Forbidden(c, "you can only modify your own collections")
			return Collection{}, false
		}
	}
	return collection, true
}

func (h *Handler) Update(c *gin.Context) {
	existing, ok := h.requireOwner(c)
	if !ok {
		return
	}

	var req CollectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "name is required")
		return
	}

	name, description, ok := normalize(c, req.Name, req.Description)
	if !ok {
		return
	}

	// Absent means unchanged, so partial updates cannot flip visibility by
	// accident; only the owner can change it (requireOwner already enforced
	// ownership or moderation, and moderators editing a private collection
	// they can already see keep its flag unless they set it).
	isPrivate := existing.IsPrivate
	if req.IsPrivate != nil {
		isPrivate = *req.IsPrivate
	}

	if err := h.repo.Update(c.Request.Context(), c.Param("id"), name, description, isPrivate); err != nil {
		if errors.Is(err, ErrNotFound) {
			response.NotFound(c, "collection not found")
			return
		}
		response.Internal(c, "collections: update", err, "collection_id", c.Param("id"))
		return
	}

	response.NoContent(c)
}

func (h *Handler) Delete(c *gin.Context) {
	if _, ok := h.requireOwner(c); !ok {
		return
	}

	if err := h.repo.Delete(c.Request.Context(), c.Param("id")); err != nil {
		if errors.Is(err, ErrNotFound) {
			response.NotFound(c, "collection not found")
			return
		}
		response.Internal(c, "collections: delete", err, "collection_id", c.Param("id"))
		return
	}

	response.NoContent(c)
}

func (h *Handler) AddPin(c *gin.Context) {
	if _, ok := h.requireOwner(c); !ok {
		return
	}

	pinID := c.Param("pinId")
	exists, err := h.repo.PinExists(c.Request.Context(), pinID)
	if err != nil {
		response.Internal(c, "collections: pin exists", err, "pin_id", pinID)
		return
	}
	if !exists {
		response.NotFound(c, "pin not found")
		return
	}

	if err := h.repo.AddPin(c.Request.Context(), c.Param("id"), pinID); err != nil {
		response.Internal(c, "collections: add pin", err, "collection_id", c.Param("id"), "pin_id", pinID)
		return
	}

	response.NoContent(c)
}

func (h *Handler) RemovePin(c *gin.Context) {
	if _, ok := h.requireOwner(c); !ok {
		return
	}

	if err := h.repo.RemovePin(c.Request.Context(), c.Param("id"), c.Param("pinId")); err != nil {
		response.Internal(c, "collections: remove pin", err, "collection_id", c.Param("id"), "pin_id", c.Param("pinId"))
		return
	}

	response.NoContent(c)
}
