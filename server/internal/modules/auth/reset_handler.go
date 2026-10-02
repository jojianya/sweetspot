package auth

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jojianya/sweetspot247-backend/internal/http/middleware"
	"github.com/jojianya/sweetspot247-backend/internal/http/response"
	"github.com/jojianya/sweetspot247-backend/internal/modules/user"
	"github.com/jojianya/sweetspot247-backend/pkg/password"
)

// resetAck is the only success body for the request endpoint, returned
// identically whether or not the email belongs to an account.
const resetAck = "if an account exists for that email, a reset link is on its way"

// ResetHandler serves the password reset flow: public request + confirm
// endpoints over the ResetStore, with email delivery behind the Mailer
// interface and links built only from the configured public base URL.
type ResetHandler struct {
	users    users.Service
	store    *ResetStore
	mailer   Mailer
	baseURL  string
	idLim    *middleware.Limiter
	sameSite SameSiteMode
	trusted  []*net.IPNet
}

func NewResetHandler(usersSvc users.Service, store *ResetStore, mailer Mailer, baseURL string, idLim *middleware.Limiter, sameSite SameSiteMode, trustedProxies []string) *ResetHandler {
	return &ResetHandler{users: usersSvc, store: store, mailer: mailer, baseURL: strings.TrimRight(baseURL, "/"), idLim: idLim, sameSite: sameSite, trusted: parseTrustedProxies(trustedProxies)}
}

// Request issues a reset token for the account (if any) behind an email
// address and always answers the same way. The lookup runs in both cases so
// response shape and database work do not reveal existence; delivery goes out
// in a detached goroutine so mail latency cannot leak it either.
func (h *ResetHandler) Request(c *gin.Context) {
	var req ResetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "a valid email is required")
		return
	}
	email := strings.ToLower(strings.TrimSpace(req.Email))

	if h.idLim != nil && !h.idLim.AllowKey("reset:"+email) {
		response.TooManyRequests(c, "too many requests, try again later")
		return
	}

	u, err := h.users.GetByEmail(c.Request.Context(), email)
	if err != nil {
		if !errors.Is(err, users.ErrNotFound) {
			response.Internal(c, "auth: reset lookup", err)
			return
		}
		response.OK(c, gin.H{"message": resetAck})
		return
	}

	raw, _, err := h.store.IssueToken(c.Request.Context(), u.ID)
	if err != nil {
		response.Internal(c, "auth: reset issue", err, "user_id", u.ID)
		return
	}

	link := h.baseURL + "/reset-password?token=" + raw
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := h.mailer.SendPasswordReset(ctx, u.Email, link); err != nil {
			slog.Warn("password reset delivery failed", "error", err.Error())
		}
	}()

	response.OK(c, gin.H{"message": resetAck})
}

// Reset consumes a reset token with a new password. Unknown, expired,
// already-used and superseded tokens all answer the same generic 400. On
// success every existing session is revoked (sessions_valid_after) and the
// session cookie is cleared, so the caller signs in fresh.
func (h *ResetHandler) Reset(c *gin.Context) {
	var req ResetConfirm
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "a reset token and new password are required")
		return
	}
	if err := password.Validate(req.Password); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	hash, err := password.Hash(req.Password)
	if err != nil {
		if errors.Is(err, password.ErrTooLong) {
			response.BadRequest(c, err.Error())
			return
		}
		response.Internal(c, "auth: reset hash", err)
		return
	}

	if _, err := h.store.ConsumeToken(c.Request.Context(), strings.TrimSpace(req.Token), hash); err != nil {
		if errors.Is(err, ErrInvalidResetToken) {
			response.BadRequest(c, ErrInvalidResetToken.Error())
			return
		}
		response.Internal(c, "auth: reset consume", err)
		return
	}

	ClearSessionCookie(c.Writer, c.Request, h.sameSite, h.trusted)
	response.OK(c, gin.H{"message": "password updated, please sign in"})
}
