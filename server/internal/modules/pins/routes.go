package pins

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
	rg.GET("/categories", h.ListCategories)
	rg.GET("/pins", h.GetPins)
	rg.GET("/pins/search", h.SearchPins)
	rg.GET("/pins/:id", validid.Middleware(), middleware.OptionalAuth(opts.JWTSecret, opts.Blacklist), h.GetPin)

	createLimit := middleware.New(10, time.Minute)
	rg.POST("/pins", createLimit.Middleware(), middleware.AuthRequired(opts.JWTSecret, opts.Blacklist), h.CreatePin)
}
