package config

import (
	"log"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// defaultCORSAllowedOrigins is the fallback for CORS_ALLOWED_ORIGINS.
//
// The Next.js client proxies /api/* to this server (client/next.config.ts), so
// the browser reaches the API on its own origin and never triggers a CORS
// check. These defaults are therefore NOT load-bearing for the app. They exist
// so a developer can still hit the API directly from a browser on a loopback
// origin (devtools console, a scratch page) while debugging — non-browser
// clients such as curl and the Go tests are not subject to CORS at all. Set
// CORS_ALLOWED_ORIGINS to override the list, or to "" to allow nothing.
const defaultCORSAllowedOrigins = "http://localhost:3000,http://localhost:3001,http://localhost:3002," +
	"http://127.0.0.1:3000,http://127.0.0.1:3001,http://127.0.0.1:3002"

// defaultCookieSameSite is the fallback for COOKIE_SAMESITE.
//
// Strict is the CSRF-safe mode and this service issues no anti-CSRF token, so
// SameSite=Strict plus the same-origin /api rewrite is the whole defence. Lax is
// reachable only by setting COOKIE_SAMESITE=lax, which exists for plain-HTTP
// LAN development where a strict cookie is never sent. Keep this strict;
// see docs/SECURITY.md.
const defaultCookieSameSite = "strict"

type Config struct {
	Port              string
	AppEnv            string
	DBHost            string
	DBPort            string
	DBUser            string
	DBPass            string
	DBName            string
	DBSSLMode         string
	DBPoolMaxConns    int
	DBPoolMaxLifetime time.Duration
	DBPoolMaxIdle     time.Duration
	DBPoolHealthCheck time.Duration
	LogLevel          string
	LogFormat         string
	JWTSecret         string
	StorageBackend    string
	StorageBase       string
	// QuarantineDir holds files moved out of ./uploads when their pin is
	// hidden. It must sit outside the static root so /uploads/<file> 404s
	// after the move; restores are a plain move back.
	QuarantineDir string
	// QuarantineDryRun makes the startup sweep report hidden-pin files that
	// would move without moving anything.
	QuarantineDryRun bool
	// QuarantineSweepInterval spaces the periodic quarantine resweep that
	// heals moves the immediate path missed. Zero disables the loop (the
	// boot sweep still runs).
	QuarantineSweepInterval time.Duration
	// ResetCleanupInterval spaces the periodic deletion of dead password
	// reset rows (used, or expired for over a day). Zero disables it.
	ResetCleanupInterval time.Duration
	RedisAddr            string
	RedisPassword        string
	CORSAllowedOrigins   []string
	SentryDSN            string
	SentryEnv            string
	MaxSSEConnections    int
	CookieSameSite       string // "strict" or "lax"
	// TrustedProxies is the gin trusted-proxy list. nginx proxies /api/*,
	// /events and /uploads/* straight to Go and overwrites X-Forwarded-For
	// with the real peer, so ClientIP() sees the real client for proxied
	// traffic. Empty (default outside compose) keeps the spoof-safe
	// behavior: ClientIP() returns the direct TCP peer and X-Forwarded-For
	// is ignored.
	TrustedProxies []string
	// PublicBaseURL is the public site origin. Password reset links are built
	// only from this value, never from request headers (which an attacker
	// controls and could point at their own host).
	PublicBaseURL string
	// MailerWebhookURL/Key configure HTTP email delivery for password reset.
	// Empty URL selects the log-only mailer (dev and tests).
	MailerWebhookURL string
	MailerWebhookKey string
}

// defaultStorageBaseURL derives the local-dev STORAGE_BASE_URL default from
// the API port so minted URLs land where router.go serves /uploads.
// `go run` listens on PORT=8080 by default while compose sets PORT=8081, so a
// hardcoded port orphans one of them. Production never uses this: compose
// requires STORAGE_BASE_URL explicitly (public https origin).
func defaultStorageBaseURL(port string) string {
	port = strings.TrimSpace(port)
	if port == "" {
		port = "8080"
	}
	return "http://localhost:" + port
}

func Load() *Config {
	// Single source of truth: the repo-root .env (the same file docker compose
	// interpolates from). A server-local .env is only consulted when the root
	// file is absent (legacy fallback). "../" resolves to the repo root when
	// running from server/ (`make run`); in containers compose injects env and
	// neither file exists.
	if err := godotenv.Load("../.env"); err != nil {
		if err := godotenv.Load(".env"); err != nil {
			log.Println("no .env file found, reading from environment")
		}
	}

	port := getEnv("PORT", "8080")
	cfg := &Config{
		Port:              port,
		AppEnv:            strings.ToLower(getEnv("APP_ENV", "development")),
		DBHost:            getEnv("DB_HOST", "localhost"),
		DBPort:            getEnv("DB_PORT", "5432"),
		DBUser:            getEnv("DB_USER", "postgres"),
		DBPass:            getEnv("DB_PASSWORD", ""),
		DBName:            getEnv("DB_NAME", "goodspotdb"),
		DBSSLMode:         strings.ToLower(strings.TrimSpace(getEnv("DATABASE_SSLMODE", "disable"))),
		DBPoolMaxConns:    mustEnvInt("DB_POOL_MAX_CONNS", 10),
		DBPoolMaxLifetime: mustEnvDuration("DB_POOL_MAX_LIFETIME", 30*time.Minute),
		DBPoolMaxIdle:     mustEnvDuration("DB_POOL_MAX_IDLE", 5*time.Minute),
		DBPoolHealthCheck: mustEnvDuration("DB_POOL_HEALTH_CHECK", time.Minute),
		LogLevel:          getEnv("LOG_LEVEL", "info"),
		LogFormat:         getEnv("LOG_FORMAT", "text"),
		JWTSecret:         getEnv("JWT_SECRET", ""),
		StorageBackend:    getEnv("STORAGE_BACKEND", "local"),
		StorageBase:       getEnv("STORAGE_BASE_URL", defaultStorageBaseURL(port)),
		QuarantineDir:     getEnv("QUARANTINE_DIR", "./quarantine"),
		QuarantineDryRun:  getEnvBool("QUARANTINE_SWEEP_DRY_RUN", true),
		// The resweep loop starts in app.Run; an invalid interval is fatal
		// like every other malformed setting, so a typo cannot silently
		// disable healing.
		QuarantineSweepInterval: mustParseSweepInterval(getEnv("QUARANTINE_SWEEP_INTERVAL", "1h")),
		// Dead reset rows are inert, so the janitor runs daily; like the
		// sweep loop, "0" disables it.
		ResetCleanupInterval: mustParseResetCleanupInterval(getEnv("PASSWORD_RESET_CLEANUP_INTERVAL", "24h")),
		RedisAddr:            getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword:        getEnv("REDIS_PASSWORD", ""),
		CORSAllowedOrigins:   getOrigins(getEnv("CORS_ALLOWED_ORIGINS", defaultCORSAllowedOrigins)),
		SentryDSN:            getEnv("SENTRY_DSN", ""),
		SentryEnv:            getEnv("SENTRY_ENV", "development"),
		MaxSSEConnections:    mustEnvInt("MAX_SSE_CONNECTIONS", 1000),
		CookieSameSite:       strings.ToLower(strings.TrimSpace(getEnv("COOKIE_SAMESITE", defaultCookieSameSite))),
		TrustedProxies:       mustParseTrustedProxies(getEnv("TRUSTED_PROXIES", "")),
		PublicBaseURL:        strings.TrimRight(strings.TrimSpace(getEnv("PUBLIC_BASE_URL", "http://localhost:3000")), "/"),
		MailerWebhookURL:     strings.TrimSpace(getEnv("MAILER_WEBHOOK_URL", "")),
		MailerWebhookKey:     os.Getenv("MAILER_WEBHOOK_KEY"),
	}

	if err := validateJWTSecret(cfg.JWTSecret); err != nil {
		log.Fatal(err)
	}
	if err := validateAppEnv(cfg.AppEnv); err != nil {
		log.Fatal(err)
	}
	if err := validateStorageBackend(cfg.StorageBackend); err != nil {
		log.Fatal(err)
	}
	if err := validateStorageBase(cfg.StorageBase, cfg.AppEnv); err != nil {
		log.Fatal(err)
	}
	if err := validateQuarantineDir(cfg.QuarantineDir); err != nil {
		log.Fatal(err)
	}
	if err := validateCookieSameSite(cfg.CookieSameSite); err != nil {
		log.Fatal(err)
	}
	if err := validatePublicBaseURL(cfg.PublicBaseURL, cfg.AppEnv); err != nil {
		log.Fatal(err)
	}
	if err := validateMailer(cfg.MailerWebhookURL, cfg.AppEnv); err != nil {
		log.Fatal(err)
	}
	if err := validateDBSSLMode(cfg.DBSSLMode, cfg.AppEnv); err != nil {
		log.Fatal(err)
	}
	if err := validatePoolOptions(cfg.DBPoolMaxConns, cfg.DBPoolMaxLifetime, cfg.DBPoolMaxIdle, cfg.DBPoolHealthCheck); err != nil {
		log.Fatal(err)
	}

	return cfg
}
