package auth

import (
	"errors"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jojianya/sweetspot247-backend/internal/http/middleware"
	"github.com/jojianya/sweetspot247-backend/internal/http/response"
	"github.com/jojianya/sweetspot247-backend/internal/platform/cache"
	"github.com/jojianya/sweetspot247-backend/pkg/jwt"
)

type Handler struct {
	service  Service
	bl       *cache.Blacklist
	emailLim *middleware.Limiter
}

func NewHandler(service Service, bl *cache.Blacklist, emailLim *middleware.Limiter) *Handler {
	return &Handler{service: service, bl: bl, emailLim: emailLim}
}

func (h *Handler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	u, token, err := h.service.Register(c.Request.Context(), req)
	if err != nil {
		if errors.Is(err, ErrConflict) {
			response.Conflict(c, "email or username already taken")
			return
		}
		if errors.Is(err, ErrInvalidPassword) {
			// Client input, not a server fault. Return the specific reason so the
			// user can tell a short password from an over-long one.
			response.BadRequest(c, err.Error())
			return
		}
		response.Internal(c, "auth: register", err)
		return
	}

	response.Created(c, gin.H{"user": u, "token": token})
}

func (h *Handler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	if h.emailLim != nil && h.emailLim.Locked(req.Identifier) {
		response.TooManyRequests(c, "account locked, try again later")
		return
	}

	u, token, err := h.service.Login(c.Request.Context(), req)
	if err != nil {
		if errors.Is(err, ErrInvalidCredentials) {
			if h.emailLim != nil {
				h.emailLim.AllowKey(req.Identifier)
			}
			response.Unauthorized(c, "invalid email, username, or password")
			return
		}
		response.Internal(c, "auth: login", err)
		return
	}

	if h.emailLim != nil {
		h.emailLim.Reset(req.Identifier)
	}

	response.OK(c, gin.H{"user": u, "token": token})
}

func (h *Handler) Logout(c *gin.Context) {
	claims, ok := c.Get(middleware.CtxJWTClaims)
	if !ok {
		response.Unauthorized(c, "invalid or expired token")
		return
	}

	jwtClaims := claims.(*jwt.Claims)
	ttl := time.Until(jwtClaims.ExpiresAt.Time)
	if err := h.bl.Revoke(c.Request.Context(), jwtClaims.ID, ttl); err != nil {
		response.Internal(c, "logout revoke failed", err)
		return
	}

	response.OK(c, gin.H{"message": "logged out"})
}

func (h *Handler) Me(c *gin.Context) {
	role := ""
	if r, err := h.service.Role(c.Request.Context(), middleware.GetUserID(c)); err == nil {
		role = r
	}
	response.OK(c, gin.H{
		"user_id": middleware.GetUserID(c),
		"role":    role,
	})
}
