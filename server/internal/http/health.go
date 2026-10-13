package http

import (
	"context"
	stdhttp "net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jojianya/sweetspot247-backend/internal/http/response"
	"github.com/jojianya/sweetspot247-backend/internal/platform/cache"
)

// livenessHandler answers /health. Liveness means the process is alive:
// deliberately dependency-free so a Redis blip cannot mark a healthy process
// for restart.
func livenessHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		response.JSON(c, stdhttp.StatusOK, gin.H{"status": "ok"})
	}
}

// readinessHandler answers /ready: can this instance serve users right now?
// It pings both backing services. Redis is not optional: the auth middleware
// consults the session blacklist on every protected request, so a Redis
// outage is an authentication outage. Reporting "ok" while Redis is
// unreachable would keep routing users to a server that cannot verify
// sessions. Uptime monitors and load balancers should watch /ready; container
// restarts key off /health.
//
// quarantineStatus is "ok" or "unwritable" as probed once at boot (see
// app.Run): whether hidden pins' files can actually leave /uploads. It rides
// along in /ready without affecting its HTTP status — unlike DB/Redis, a
// broken quarantine must be loud but must not take all traffic down.
func readinessHandler(pool *pgxpool.Pool, blacklist *cache.Blacklist, quarantineStatus string) gin.HandlerFunc {
	return func(c *gin.Context) {
		ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
		defer cancel()

		dbStatus := "connected"
		if err := pool.Ping(ctx); err != nil {
			dbStatus = "unreachable"
		}

		redisStatus := "connected"
		if blacklist == nil {
			redisStatus = "not_configured"
		} else if err := blacklist.Ping(ctx); err != nil {
			redisStatus = "unreachable"
		}

		if dbStatus != "connected" || redisStatus == "unreachable" {
			response.JSON(c, stdhttp.StatusServiceUnavailable, gin.H{
				"status":     "degraded",
				"db":         dbStatus,
				"redis":      redisStatus,
				"quarantine": quarantineStatus,
			})
			return
		}
		response.JSON(c, stdhttp.StatusOK, gin.H{"status": "ok", "db": dbStatus, "redis": redisStatus, "quarantine": quarantineStatus})
	}
}
