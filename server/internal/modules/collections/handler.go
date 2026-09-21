package collections

import (
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/jojianya/sweetspot247-backend/internal/http/middleware"
	"github.com/jojianya/sweetspot247-backend/internal/http/response"
)

// CreateCollectionRequest / UpdateCollectionRequest share the same shape:
// name is required, description is optional.
type CollectionRequest struct {
	Name        string  `json:"name" binding:"required"`
	Description *string `json:"description"`
}

type Handler struct {
	service Service
}

func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) ListMine(c *gin.Context) {
	collections, err := h.service.ListByUser(c.Request.Context(), middleware.GetUserID(c))
	if err != nil {
		response.Internal(c, "collections: list mine", err, "user_id", middleware.GetUserID(c))
		return
	}
	response.OK(c, gin.H{"collections": collections})
}

func (h *Handler) ListByUser(c *gin.Context) {
	userID := c.Param("id")
	exists, err := h.service.UserExists(c.Request.Context(), userID)
	if err != nil {
		response.Internal(c, "collections: user exists", err, "user_id", userID)
		return
	}
	if !exists {
		response.NotFound(c, "user not found")
		return
	}

	collections, err := h.service.ListByUser(c.Request.Context(), userID)
	if err != nil {
		response.Internal(c, "collections: list by user", err, "user_id", userID)
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

	collection, err := h.service.Create(c.Request.Context(), middleware.GetUserID(c), name, description)
	if err != nil {
		response.Internal(c, "collections: create", err, "user_id", middleware.GetUserID(c))
		return
	}

	response.Created(c, gin.H{"collection": collection})
}

func (h *Handler) Get(c *gin.Context) {
	detail, err := h.service.CollectionDetail(c.Request.Context(), c.Param("id"))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.NotFound(c, "collection not found")
			return
		}
		response.Internal(c, "collections: get", err, "collection_id", c.Param("id"))
		return
	}

	response.OK(c, gin.H{"collection": detail})
}

// requireOwner aborts unless the caller owns the collection (or moderates).
func (h *Handler) requireOwner(c *gin.Context) bool {
	collection, err := h.service.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.NotFound(c, "collection not found")
			return false
		}
		response.Internal(c, "collections: get", err, "collection_id", c.Param("id"))
		return false
	}

	userID := middleware.GetUserID(c)
	if collection.UserID.String() != userID {
		role := middleware.GetRole(c)
		if role != "admin" && role != "owner" {
			response.Forbidden(c, "you can only modify your own collections")
			return false
		}
	}
	return true
}

func (h *Handler) Update(c *gin.Context) {
	if !h.requireOwner(c) {
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

	if err := h.service.Update(c.Request.Context(), c.Param("id"), name, description); err != nil {
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
	if !h.requireOwner(c) {
		return
	}

	if err := h.service.Delete(c.Request.Context(), c.Param("id")); err != nil {
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
	if !h.requireOwner(c) {
		return
	}

	pinID := c.Param("pinId")
	exists, err := h.service.PinExists(c.Request.Context(), pinID)
	if err != nil {
		response.Internal(c, "collections: pin exists", err, "pin_id", pinID)
		return
	}
	if !exists {
		response.NotFound(c, "pin not found")
		return
	}

	if err := h.service.AddPin(c.Request.Context(), c.Param("id"), pinID); err != nil {
		response.Internal(c, "collections: add pin", err, "collection_id", c.Param("id"), "pin_id", pinID)
		return
	}

	response.NoContent(c)
}

func (h *Handler) RemovePin(c *gin.Context) {
	if !h.requireOwner(c) {
		return
	}

	if err := h.service.RemovePin(c.Request.Context(), c.Param("id"), c.Param("pinId")); err != nil {
		response.Internal(c, "collections: remove pin", err, "collection_id", c.Param("id"), "pin_id", c.Param("pinId"))
		return
	}

	response.NoContent(c)
}
