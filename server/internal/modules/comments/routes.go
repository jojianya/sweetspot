package comments

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jojianya/sweetspot247-backend/internal/http/middleware"
	"github.com/jojianya/sweetspot247-backend/internal/platform/cache"
	"github.com/jojianya/sweetspot247-backend/pkg/validid"
)

type RouteOptions struct {
	JWTSecret string
	Blacklist *cache.Blacklist
}

func RegisterRoutes(rg *gin.RouterGroup, h *Handler, opts RouteOptions) {
	rg.GET("/pins/:id/comments", validid.Middleware(), h.List)

	commentLimit := middleware.New(30, time.Minute)
	rg.POST("/pins/:id/comments", validid.Middleware(), commentLimit.Middleware(), middleware.AuthRequired(opts.JWTSecret, opts.Blacklist), h.Create)

	rg.DELETE("/comments/:id", validid.Middleware(), middleware.AuthRequired(opts.JWTSecret, opts.Blacklist), h.Delete)
}
