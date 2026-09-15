package reports

import (
	"github.com/gin-gonic/gin"
	"github.com/jojianya/sweetspot247-backend/internal/http/middleware"
	"github.com/jojianya/sweetspot247-backend/internal/modules/user"
	"github.com/jojianya/sweetspot247-backend/internal/platform/cache"
	"github.com/jojianya/sweetspot247-backend/pkg/validid"
)

type RouteOptions struct {
	JWTSecret   string
	Blacklist   *cache.Blacklist
	UserService users.Service
}

func RegisterRoutes(rg *gin.RouterGroup, h *Handler, opts RouteOptions) {
	rg.POST("/pins/:id/report", middleware.AuthRequired(opts.JWTSecret, opts.Blacklist), validid.Middleware(), h.Create)
	rg.GET("/reports", middleware.AuthRequired(opts.JWTSecret, opts.Blacklist), users.RequireAdmin(opts.UserService), h.List)
	rg.PATCH("/reports/:id", middleware.AuthRequired(opts.JWTSecret, opts.Blacklist), users.RequireAdmin(opts.UserService), validid.Middleware(), h.Review)
}
