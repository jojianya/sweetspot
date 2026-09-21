package collections

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
	authRequired := middleware.AuthRequired(opts.JWTSecret, opts.Blacklist)

	rg.GET("/users/:id/collections", validid.Middleware(), h.ListByUser)

	rg.GET("/collections", authRequired, h.ListMine)
	rg.POST("/collections", authRequired, h.Create)
	rg.GET("/collections/:id", validid.Middleware(), h.Get)
	rg.PATCH("/collections/:id", validid.Middleware(), authRequired, h.Update)
	rg.DELETE("/collections/:id", validid.Middleware(), authRequired, h.Delete)

	rg.PUT("/collections/:id/pins/:pinId",
		validid.Middleware(), validid.MiddlewareParam("pinId"), authRequired, h.AddPin)
	rg.DELETE("/collections/:id/pins/:pinId",
		validid.Middleware(), validid.MiddlewareParam("pinId"), authRequired, h.RemovePin)
}
