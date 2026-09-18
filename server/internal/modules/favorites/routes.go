package favorites

import (
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
	auth := func(handlers ...gin.HandlerFunc) []gin.HandlerFunc {
		return append([]gin.HandlerFunc{validid.Middleware(), middleware.AuthRequired(opts.JWTSecret, opts.Blacklist)}, handlers...)
	}

	rg.GET("/favorites", middleware.AuthRequired(opts.JWTSecret, opts.Blacklist), h.GetSaved)
	rg.GET("/favorites/ids", middleware.AuthRequired(opts.JWTSecret, opts.Blacklist), h.GetSavedIDs)
	rg.PUT("/favorites/:id", auth(h.Save)...)
	rg.DELETE("/favorites/:id", auth(h.Unsave)...)
}