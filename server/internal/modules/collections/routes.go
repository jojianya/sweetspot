package collections

import (
	"github.com/gin-gonic/gin"
	"github.com/jojianya/sweetspot247-backend/internal/http/middleware"
	"github.com/jojianya/sweetspot247-backend/internal/http/validid"
	"github.com/jojianya/sweetspot247-backend/internal/platform/cache"
)

type RouteOptions struct {
	JWTSecret string
	Blacklist *cache.Blacklist
	Sessions  middleware.SessionChecker
}

func RegisterRoutes(rg *gin.RouterGroup, h *Handler, opts RouteOptions) {
	authRequired := middleware.AuthRequired(opts.JWTSecret, opts.Blacklist, opts.Sessions)
	optionalAuth := middleware.OptionalAuth(opts.JWTSecret, opts.Blacklist, opts.Sessions)

	rg.GET("/users/:id/collections", validid.Middleware(), optionalAuth, h.ListByUser)

	rg.GET("/collections", authRequired, h.ListMine)
	rg.POST("/collections", authRequired, h.Create)
	rg.GET("/collections/:id", validid.Middleware(), optionalAuth, h.Get)
	rg.PATCH("/collections/:id", validid.Middleware(), authRequired, h.Update)
	rg.DELETE("/collections/:id", validid.Middleware(), authRequired, h.Delete)

	rg.PUT("/collections/:id/pins/:pinId",
		validid.Middleware(), validid.MiddlewareParam("pinId"), authRequired, h.AddPin)
	rg.DELETE("/collections/:id/pins/:pinId",
		validid.Middleware(), validid.MiddlewareParam("pinId"), authRequired, h.RemovePin)
}
