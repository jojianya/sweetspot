package comments

import (
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/jojianya/sweetspot247-backend/internal/http/middleware"
	"github.com/jojianya/sweetspot247-backend/internal/http/response"
)

const maxCommentLength = 500

type Handler struct {
	repo Repository
}

func NewHandler(repo Repository) *Handler {
	return &Handler{repo: repo}
}

// isModerator reports whether the caller holds a moderation role.
func isModerator(c *gin.Context) bool {
	role := middleware.GetRole(c)
	return role == "admin" || role == "owner"
}

func (h *Handler) List(c *gin.Context) {
	comments, err := h.repo.ListByPin(c.Request.Context(), c.Param("id"))
	if err != nil {
		response.Internal(c, "comments: list", err, "pin_id", c.Param("id"))
		return
	}
	response.OK(c, gin.H{"comments": comments})
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

	comment, err := h.repo.Create(c.Request.Context(), c.Param("id"), middleware.GetUserID(c), body)
	if err != nil {
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
	if isModerator(c) {
		if err := h.repo.Hide(c.Request.Context(), comment.ID.String()); err != nil {
			response.Internal(c, "comments: hide", err, "comment_id", comment.ID.String())
			return
		}
	} else if comment.UserID.String() == userID {
		if err := h.repo.Delete(c.Request.Context(), comment.ID.String()); err != nil {
			response.Internal(c, "comments: delete", err, "comment_id", comment.ID.String())
			return
		}
	} else {
		response.Forbidden(c, "you can only delete your own comments")
		return
	}

	response.NoContent(c)
}
