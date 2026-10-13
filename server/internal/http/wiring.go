package http

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jojianya/sweetspot247-backend/internal/config"
	"github.com/jojianya/sweetspot247-backend/internal/di"
	"github.com/jojianya/sweetspot247-backend/internal/http/middleware"
	"github.com/jojianya/sweetspot247-backend/internal/modules/auth"
	"github.com/jojianya/sweetspot247-backend/internal/modules/collections"
	"github.com/jojianya/sweetspot247-backend/internal/modules/comments"
	"github.com/jojianya/sweetspot247-backend/internal/modules/favorites"
	"github.com/jojianya/sweetspot247-backend/internal/modules/pins"
	"github.com/jojianya/sweetspot247-backend/internal/modules/reactions"
	"github.com/jojianya/sweetspot247-backend/internal/modules/realtime"
	"github.com/jojianya/sweetspot247-backend/internal/modules/reports"
	"github.com/jojianya/sweetspot247-backend/internal/modules/social"
	"github.com/jojianya/sweetspot247-backend/internal/modules/user"
)

// selectMailer chooses HTTP email delivery when a webhook is configured and
// the log-only adapter otherwise. Log tokens are enabled outside production
// only, so full reset tokens can never reach production logs through it.
func selectMailer(cfg *config.Config) auth.Mailer {
	if cfg.MailerWebhookURL != "" {
		return auth.WebhookMailer{URL: cfg.MailerWebhookURL, Key: cfg.MailerWebhookKey}
	}
	return auth.LogMailer{LogTokens: cfg.AppEnv != "production"}
}

// registerModuleRoutes wires every module's handlers onto the JSON and upload
// route groups. Kept out of NewRouter so the engine bootstrap (mode, trusted
// proxies, middleware, static files, health) reads separately from the
// per-module handler construction.
func registerModuleRoutes(jsonRoutes, uploadRoutes *gin.RouterGroup, cfg *config.Config, c *di.Container, pool *pgxpool.Pool) {
	authHandler := auth.NewHandler(
		auth.NewService(c.UserService, cfg.JWTSecret),
		c.Blacklist,
		middleware.New(5, time.Minute),
		// Per-account backstop: 20 failures per 15 minutes per identifier
		// across all IPs. High enough that a stranger cannot lock the real
		// owner out, low enough to stop a distributed grind.
		middleware.New(20, 15*time.Minute),
		auth.SameSiteMode(cfg.CookieSameSite),
		cfg.TrustedProxies,
	)
	mailer := selectMailer(cfg)
	auth.RegisterRoutes(jsonRoutes, authHandler, auth.RouteOptions{
		JWTSecret:      cfg.JWTSecret,
		Blacklist:      c.Blacklist,
		CookieSameSite: auth.SameSiteMode(cfg.CookieSameSite),
		Sessions:       c.UserService,
		UserService:    c.UserService,
		ResetStore:     auth.NewResetStore(pool),
		Mailer:         mailer,
		BaseURL:        cfg.PublicBaseURL,
		TrustedProxies: cfg.TrustedProxies,
	})

	userHandler := users.NewHandler(c.UserService, c.Store)
	// Registered on uploadRoutes: PATCH /users/me is multipart (avatar upload).
	users.RegisterRoutes(uploadRoutes, userHandler, users.RouteOptions{JWTSecret: cfg.JWTSecret, Blacklist: c.Blacklist, Sessions: c.UserService})

	// Real-time stream of newly created pins (SSE). Registered before the
	// pin routes so /events never collides with a parameter route.
	realtimeHandler := realtime.NewHandler(c.Events, cfg.MaxSSEConnections)
	jsonRoutes.GET("/events", realtimeHandler.Stream)

	pinHandler := pins.NewHandler(c.PinRepo, c.Store, c.Events, c.UserService)
	pins.RegisterRoutes(uploadRoutes, pinHandler, pins.RouteOptions{
		JWTSecret: cfg.JWTSecret,
		Blacklist: c.Blacklist,
		Sessions:  c.UserService,
		// Only GET /pins/:id answers reacted_by_me. The reaction repository is
		// passed directly: it already exposes ReactedByMe, and handing the pins
		// handler the repository (not the service) keeps the dependency narrow.
		Reactions: c.ReactionRepo,
	})

	reportHandler := reports.NewHandler(reports.NewService(c.ReportRepo), c.Events).
		WithQuarantine(c.Store, c.PinRepo)
	reports.RegisterRoutes(jsonRoutes, reportHandler, reports.RouteOptions{
		JWTSecret:   cfg.JWTSecret,
		Blacklist:   c.Blacklist,
		UserService: c.UserService,
		Sessions:    c.UserService,
	})

	favoriteHandler := favorites.NewHandler(c.FavoriteRepo)
	favorites.RegisterRoutes(jsonRoutes, favoriteHandler, favorites.RouteOptions{
		JWTSecret: cfg.JWTSecret,
		Blacklist: c.Blacklist,
		Sessions:  c.UserService,
	})

	commentHandler := comments.NewHandler(c.CommentRepo, c.UserService)
	comments.RegisterRoutes(jsonRoutes, commentHandler, comments.RouteOptions{JWTSecret: cfg.JWTSecret, Blacklist: c.Blacklist, Sessions: c.UserService})

	reactionHandler := reactions.NewHandler(c.ReactionRepo)
	reactions.RegisterRoutes(jsonRoutes, reactionHandler, reactions.RouteOptions{JWTSecret: cfg.JWTSecret, Blacklist: c.Blacklist, Sessions: c.UserService})

	socialHandler := social.NewHandler(c.SocialRepo)
	social.RegisterRoutes(jsonRoutes, socialHandler, social.RouteOptions{JWTSecret: cfg.JWTSecret, Blacklist: c.Blacklist, Sessions: c.UserService})

	collectionHandler := collections.NewHandler(c.CollectionRepo, c.UserService)
	collections.RegisterRoutes(jsonRoutes, collectionHandler, collections.RouteOptions{JWTSecret: cfg.JWTSecret, Blacklist: c.Blacklist, Sessions: c.UserService})
}
