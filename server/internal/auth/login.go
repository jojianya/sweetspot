package auth

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jojianya/sweetspot247-backend/internal/users"
	"github.com/jojianya/sweetspot247-backend/pkg/jwt"
	"github.com/jojianya/sweetspot247-backend/pkg/password"
	"github.com/jojianya/sweetspot247-backend/pkg/ratelimit"
)

var errInvalidCredentials = errors.New("invalid credentials")

type LoginHandler struct {
	users     *users.Repository
	jwtSecret string
	emailLim  *ratelimit.Limiter
}

func NewLoginHandler(users *users.Repository, jwtSecret string, emailLim *ratelimit.Limiter) *LoginHandler {
	return &LoginHandler{users: users, jwtSecret: jwtSecret, emailLim: emailLim}
}

func (h *LoginHandler) Handle(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if h.emailLim != nil && !h.emailLim.AllowKey(req.Email) {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": "too many requests, try again later"})
		return
	}

	user, err := h.users.GetByEmail(c.Request.Context(), req.Email)
	if err != nil {
		if errors.Is(err, users.ErrNotFound) {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid email or password"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	if !password.Verify(req.Password, user.PasswordHash) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid email or password"})
		return
	}

	token, err := jwt.Generate(h.jwtSecret, user.ID, user.Role, tokenExpiry)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"user":  user,
		"token": token,
	})
}
