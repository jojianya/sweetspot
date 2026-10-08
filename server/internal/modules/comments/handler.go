package comments

import (
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jojianya/sweetspot247-backend/internal/http/middleware"
	httpx "github.com/jojianya/sweetspot247-backend/internal/http/params"
	"github.com/jojianya/sweetspot247-backend/internal/http/response"
	"github.com/jojianya/sweetspot247-backend/internal/modules/user"
)

const (
	maxCommentLength = 500

	commentListDefaultLimit = 50
	commentListMaxLimit     = 200
)

type Handler struct {
	repo Repository
	// roles resolves moderation rights from the database rather than the JWT,
	// so a demotion takes effect on the caller's next request.
	roles users.RoleReader
}

func NewHandler(repo Repository, roles users.RoleReader) *Handler {
	return &Handler{repo: repo, roles: roles}
}

// isModerator reports whether the caller holds a moderation role.
func (h *Handler) isModerator(c *gin.Context) bool {
	return users.IsModerator(h.roles, c)
}

func (h *Handler) List(c *gin.Context) {
	exists, err := h.repo.PinExistsVisible(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.Internal(c, "comments: pin exists", err, "pin_id", c.Param("id"))
		return
	}
	if !exists {
		response.NotFound(c, "pin not found")
		return
	}

	limit, ok := httpx.ParseLimit(c, commentListDefaultLimit, commentListMaxLimit)
	if !ok {
		return
	}
	offset, ok := httpx.ParseOffset(c)
	if !ok {
		return
	}

	comments, total, err := h.repo.ListByPin(c.Request.Context(), c.Param("id"), limit, offset)
	if err != nil {
		response.Internal(c, "comments: list", err, "pin_id", c.Param("id"))
		return
	}
	response.OK(c, gin.H{"comments": comments, "total": total})
}

func (h *Handler) Create(c *gin.Context) {
	var req CreateCommentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "body is required")
		return
	}

	body := strings.TrimSpace(req.Body)
	if body == "" {
		response.BadRequest(c, "body is required")
		return
	}
	if utf8.RuneCountInString(body) > maxCommentLength {
		response.BadRequest(c, "body must be at most 500 characters")
		return
	}

	exists, err := h.repo.PinExistsVisible(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.Internal(c, "comments: pin exists", err, "pin_id", c.Param("id"))
		return
	}
	if !exists {
		response.NotFound(c, "pin not found")
		return
	}

	comment, err := h.repo.Create(c.Request.Context(), c.Param("id"), middleware.GetUserID(c), body)
	if err != nil {
		// Safety net: the pin vanished between the visibility check and the
		// insert (or raced a hide) — report it as a missing pin, not a 500.
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23503" {
			response.NotFound(c, "pin not found")
			return
		}
		response.Internal(c, "comments: create", err, "pin_id", c.Param("id"))
		return
	}

	response.Created(c, gin.H{"comment": comment})
}

func (h *Handler) Delete(c *gin.Context) {
	userID := middleware.GetUserID(c)
	comment, err := h.repo.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			response.NotFound(c, "comment not found")
			return
		}
		response.Internal(c, "comments: get", err, "comment_id", c.Param("id"))
		return
	}

	// Moderators soft-delete (keeps an audit trail); the author removes it
	// outright.
	if h.isModerator(c) {
		if err := h.repo.Hide(c.Request.Context(), comment.ID); err != nil {
			response.Internal(c, "comments: hide", err, "comment_id", comment.ID)
			return
		}
	} else if comment.UserID == userID {
		if err := h.repo.Delete(c.Request.Context(), comment.ID); err != nil {
			response.Internal(c, "comments: delete", err, "comment_id", comment.ID)
			return
		}
	} else {
		response.Forbidden(c, "you can only delete your own comments")
		return
	}

	response.NoContent(c)
}
