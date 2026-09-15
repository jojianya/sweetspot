package auth

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jojianya/sweetspot247-backend/internal/http/middleware"
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
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	u, token, err := h.service.Register(c.Request.Context(), req)
	if err != nil {
		if errors.Is(err, ErrConflict) {
			c.JSON(http.StatusConflict, gin.H{"error": "email or username already taken"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"user": u, "token": token})
}

func (h *Handler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if h.emailLim != nil && !h.emailLim.AllowKey(req.Email) {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "too many requests, try again later"})
		return
	}

	u, token, err := h.service.Login(c.Request.Context(), req)
	if err != nil {
		if errors.Is(err, ErrInvalidCredentials) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid email or password"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"user": u, "token": token})
}

func (h *Handler) Logout(c *gin.Context) {
	claims, ok := c.Get(middleware.CtxJWTClaims)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
		return
	}

	jwtClaims := claims.(*jwt.Claims)
	ttl := time.Until(jwtClaims.ExpiresAt.Time)
	if err := h.bl.Revoke(c.Request.Context(), jwtClaims.ID, ttl); err != nil {
		slog.Default().Error("logout revoke failed", "error", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "logged out"})
}

func (h *Handler) Me(c *gin.Context) {
	role := ""
	if r, err := h.service.Role(c.Request.Context(), middleware.GetUserID(c)); err == nil {
		role = r
	}
	c.JSON(http.StatusOK, gin.H{
		"user_id": middleware.GetUserID(c),
		"role":    role,
	})
}
