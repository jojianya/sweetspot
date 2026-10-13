package collections

import (
	"errors"

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
	svc   *Service
}

func NewHandler(repo Repository, roles users.RoleReader) *Handler {
	return &Handler{repo: repo, roles: roles, svc: NewService(repo)}
}

// service returns the service, building it from the handler's repository when
// the handler was constructed as a struct literal (as some tests do) instead
// of via NewHandler.
func (h *Handler) service() *Service {
	if h.svc != nil {
		return h.svc
	}
	return NewService(h.repo)
}

func (h *Handler) ListMine(c *gin.Context) {
	collections, err := h.service().ListMine(c.Request.Context(), middleware.GetUserID(c))
	if err != nil {
		response.Internal(c, "collections: list mine", err, "user_id", middleware.GetUserID(c))
		return
	}
	response.OK(c, gin.H{"collections": collections})
}

func (h *Handler) ListByUser(c *gin.Context) {
	userID := c.Param("id")

	collections, err := h.service().ListByUser(c.Request.Context(), userID, middleware.GetUserID(c))
	if err != nil {
		if errors.Is(err, ErrUserNotFound) {
			response.NotFound(c, "user not found")
			return
		}
		response.Internal(c, "collections: list by user", err, "user_id", userID)
		return
	}
	response.OK(c, gin.H{"collections": collections})
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
	collection, err := h.service().Create(c.Request.Context(), middleware.GetUserID(c), name, description, isPrivate)
	if err != nil {
		response.Internal(c, "collections: create", err, "user_id", middleware.GetUserID(c))
		return
	}

	response.Created(c, gin.H{"collection": collection})
}

func (h *Handler) Get(c *gin.Context) {
	id := c.Param("id")

	limit, ok := httpx.ParseLimit(c, collectionPinListDefaultLimit, collectionPinListMaxLimit)
	if !ok {
		return
	}
	offset, ok := httpx.ParseOffset(c)
	if !ok {
		return
	}

	detail, err := h.service().GetDetail(c.Request.Context(), id, middleware.GetUserID(c), limit, offset)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.NotFound(c, "collection not found")
			return
		}
		response.Internal(c, "collections: get", err, "collection_id", id)
		return
	}

	response.OK(c, gin.H{"collection": detail})
}

func (h *Handler) Update(c *gin.Context) {
	var req CollectionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "name is required")
		return
	}

	name, description, ok := normalize(c, req.Name, req.Description)
	if !ok {
		return
	}

	if err := h.service().Update(c.Request.Context(), c.Param("id"), middleware.GetUserID(c), users.IsModeratorHandler(h.roles, c), name, description, req.IsPrivate); err != nil {
		if errors.Is(err, ErrNotFound) {
			response.NotFound(c, "collection not found")
			return
		}
		if errors.Is(err, ErrForbidden) {
			response.Forbidden(c, "you can only modify your own collections")
			return
		}
		response.Internal(c, "collections: update", err, "collection_id", c.Param("id"))
		return
	}

	response.NoContent(c)
}

func (h *Handler) Delete(c *gin.Context) {
	if err := h.service().Delete(c.Request.Context(), c.Param("id"), middleware.GetUserID(c), users.IsModeratorHandler(h.roles, c)); err != nil {
		if errors.Is(err, ErrNotFound) {
			response.NotFound(c, "collection not found")
			return
		}
		if errors.Is(err, ErrForbidden) {
			response.Forbidden(c, "you can only modify your own collections")
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
	if err := h.service().AddPin(c.Request.Context(), c.Param("id"), pinID); err != nil {
		if errors.Is(err, ErrPinNotFound) {
			response.NotFound(c, "pin not found")
			return
		}
		response.Internal(c, "collections: add pin", err, "collection_id", c.Param("id"), "pin_id", pinID)
		return
	}

	response.NoContent(c)
}

func (h *Handler) RemovePin(c *gin.Context) {
	if _, ok := h.requireOwner(c); !ok {
		return
	}

	if err := h.service().RemovePin(c.Request.Context(), c.Param("id"), c.Param("pinId")); err != nil {
		response.Internal(c, "collections: remove pin", err, "collection_id", c.Param("id"), "pin_id", c.Param("pinId"))
		return
	}

	response.NoContent(c)
}
