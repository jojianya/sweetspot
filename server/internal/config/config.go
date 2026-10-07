package config

import (
	"fmt"
	"log"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
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
	QuarantineDryRun   bool
	RedisAddr          string
	RedisPassword      string
	CORSAllowedOrigins []string
	SentryDSN          string
	SentryEnv          string
	MaxSSEConnections  int
	CookieSameSite     string // "strict" or "lax"
	// TrustedProxies is the gin trusted-proxy list. Empty (default) keeps
	// today's behavior: ClientIP() returns the direct TCP peer and
	// X-Forwarded-For is ignored, so per-IP rate limits are shared per proxy
	// behind the Next rewrite (which does not forward X-Forwarded-For).
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
		Port:               port,
		AppEnv:             strings.ToLower(getEnv("APP_ENV", "development")),
		DBHost:             getEnv("DB_HOST", "localhost"),
		DBPort:             getEnv("DB_PORT", "5432"),
		DBUser:             getEnv("DB_USER", "postgres"),
		DBPass:             getEnv("DB_PASSWORD", ""),
		DBName:             getEnv("DB_NAME", "goodspotdb"),
		DBSSLMode:          strings.ToLower(strings.TrimSpace(getEnv("DATABASE_SSLMODE", "disable"))),
		DBPoolMaxConns:     getEnvInt("DB_POOL_MAX_CONNS", 10),
		DBPoolMaxLifetime:  getEnvDuration("DB_POOL_MAX_LIFETIME", 30*time.Minute),
		DBPoolMaxIdle:      getEnvDuration("DB_POOL_MAX_IDLE", 5*time.Minute),
		DBPoolHealthCheck:  getEnvDuration("DB_POOL_HEALTH_CHECK", time.Minute),
		LogLevel:           getEnv("LOG_LEVEL", "info"),
		LogFormat:          getEnv("LOG_FORMAT", "text"),
		JWTSecret:          getEnv("JWT_SECRET", ""),
		StorageBackend:     getEnv("STORAGE_BACKEND", "local"),
		StorageBase:        getEnv("STORAGE_BASE_URL", defaultStorageBaseURL(port)),
		QuarantineDir:      getEnv("QUARANTINE_DIR", "./quarantine"),
		QuarantineDryRun:   getEnvBool("QUARANTINE_SWEEP_DRY_RUN", false),
		RedisAddr:          getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword:      getEnv("REDIS_PASSWORD", ""),
		CORSAllowedOrigins: getOrigins(getEnv("CORS_ALLOWED_ORIGINS", defaultCORSAllowedOrigins)),
		SentryDSN:          getEnv("SENTRY_DSN", ""),
		SentryEnv:          getEnv("SENTRY_ENV", "development"),
		MaxSSEConnections:  getEnvInt("MAX_SSE_CONNECTIONS", 1000),
		CookieSameSite:     strings.ToLower(strings.TrimSpace(getEnv("COOKIE_SAMESITE", defaultCookieSameSite))),
		TrustedProxies:     mustParseTrustedProxies(getEnv("TRUSTED_PROXIES", "")),
		PublicBaseURL:      strings.TrimRight(strings.TrimSpace(getEnv("PUBLIC_BASE_URL", "http://localhost:3000")), "/"),
		MailerWebhookURL:   strings.TrimSpace(getEnv("MAILER_WEBHOOK_URL", "")),
		MailerWebhookKey:   os.Getenv("MAILER_WEBHOOK_KEY"),
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

func validateAppEnv(value string) error {
	if value != "development" && value != "production" {
		return fmt.Errorf("APP_ENV must be development or production, got %q", value)
	}
	return nil
}

func validateCookieSameSite(value string) error {
	if value != "strict" && value != "lax" {
		return fmt.Errorf("COOKIE_SAMESITE must be 'strict' or 'lax', got %q", value)
	}
	return nil
}

// validatePublicBaseURL checks the origin reset links are built from. Like
// storage origins it must be absolute http(s) without credentials, and
// publicly reachable in production so emailed links work.
func validatePublicBaseURL(raw, appEnv string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fmt.Errorf("PUBLIC_BASE_URL is required")
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return fmt.Errorf("PUBLIC_BASE_URL must be an absolute http or https URL")
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" {
		return fmt.Errorf("PUBLIC_BASE_URL must not include credentials, a query, or a fragment")
	}
	if strings.EqualFold(appEnv, "production") && (isLoopbackHost(parsed.Hostname()) || isPrivateIP(parsed.Hostname())) {
		return fmt.Errorf("PUBLIC_BASE_URL must be publicly reachable in production")
	}
	return nil
}

// validateMailer requires real delivery in production: a password reset the
// user never receives is a locked account with no recourse. Development and
// tests use the log-only mailer.
func validateMailer(webhookURL, appEnv string) error {
	if strings.EqualFold(appEnv, "production") && webhookURL == "" {
		return fmt.Errorf("MAILER_WEBHOOK_URL is required in production so password reset emails are actually delivered (email provider: [FILL IN])")
	}
	return nil
}

func validateStorageBackend(value string) error {
	if value != "local" {
		return fmt.Errorf("STORAGE_BACKEND %q is not supported; only local storage is implemented", value)
	}
	return nil
}

func validateStorageBase(raw, appEnv string) error {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fmt.Errorf("STORAGE_BASE_URL is required")
	}

	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" || (parsed.Scheme != "http" && parsed.Scheme != "https") {
		return fmt.Errorf("STORAGE_BASE_URL must be an absolute http or https URL")
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.ForceQuery || parsed.Fragment != "" {
		return fmt.Errorf("STORAGE_BASE_URL must not include credentials, a query, or a fragment")
	}

	loopback := isLoopbackHost(parsed.Hostname())
	private := isPrivateIP(parsed.Hostname())
	if parsed.Scheme != "https" && !loopback && !private {
		return fmt.Errorf("STORAGE_BASE_URL must use https unless it is a local or private address")
	}
	if strings.EqualFold(appEnv, "production") && (loopback || private) {
		return fmt.Errorf("STORAGE_BASE_URL must be publicly reachable in production")
	}
	return nil
}

// validateDBSSLMode checks the Postgres sslmode. Unknown values always fail;
// in production anything weaker than require fails too, so the DB password
// and data never travel in cleartext off-host.
// validateQuarantineDir keeps the hidden-pin quarantine outside the static
// root: quarantined bytes must not be reachable via /uploads, and an empty
// or uploads-equal value would either disable the quarantine or move files
// onto themselves.
func validateQuarantineDir(raw string) error {
	v := strings.TrimSpace(raw)
	if v == "" {
		return fmt.Errorf("QUARANTINE_DIR is required")
	}
	clean := filepath.Clean(v)
	if clean == "." || clean == "./uploads" || clean == "uploads" {
		return fmt.Errorf("QUARANTINE_DIR must sit outside the static uploads root, got %q", raw)
	}
	return nil
}

func validateDBSSLMode(mode, appEnv string) error {
	switch mode {
	case "disable", "allow", "prefer", "require", "verify-ca", "verify-full":
	default:
		return fmt.Errorf("DATABASE_SSLMODE must be one of disable, allow, prefer, require, verify-ca, verify-full; got %q", mode)
	}
	if strings.EqualFold(appEnv, "production") {
		switch mode {
		case "require", "verify-ca", "verify-full":
			return nil
		default:
			return fmt.Errorf("DATABASE_SSLMODE must be require or stronger in production, got %q", mode)
		}
	}
	return nil
}

// validatePoolOptions bounds the connection pool so one deployment cannot
// starve Postgres (too many) or serialize every request (too few), and so a
// zero duration cannot disable lifetime/idle/health sweeps by accident.
func validatePoolOptions(maxConns int, maxLifetime, maxIdle, healthCheck time.Duration) error {
	if maxConns < 1 || maxConns > 100 {
		return fmt.Errorf("DB_POOL_MAX_CONNS must be between 1 and 100, got %d", maxConns)
	}
	if maxLifetime <= 0 || maxIdle <= 0 || healthCheck <= 0 {
		return fmt.Errorf("DB pool lifetimes must be positive durations")
	}
	return nil
}

// validateJWTSecret checks that the JWT secret is non-empty and at least 32
// characters (256 bits) for sufficient entropy against brute force.
func validateJWTSecret(secret string) error {
	if secret == "" {
		return fmt.Errorf("JWT_SECRET is required: set it in .env or the environment (generate with: openssl rand -hex 32)")
	}
	if len(secret) < 32 {
		return fmt.Errorf("JWT_SECRET must be at least 32 characters (256 bits)")
	}
	return nil
}

func isLoopbackHost(host string) bool {
	host = strings.ToLower(strings.Trim(host, "[]"))
	if host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && (ip.IsLoopback() || ip.IsUnspecified())
}

// isPrivateIP reports whether the host is a private (RFC 1918) IP address.
// Allows HTTP for LAN development without requiring TLS certificates.
func isPrivateIP(host string) bool {
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	// Convert to 4-byte IPv4 representation if it's an IPv4-mapped IPv6 address.
	ip4 := ip.To4()
	if ip4 == nil {
		return false // not an IPv4 address
	}
	// 10.0.0.0/8
	if ip4[0] == 10 {
		return true
	}
	// 172.16.0.0/12
	if ip4[0] == 172 && ip4[1] >= 16 && ip4[1] <= 31 {
		return true
	}
	// 192.168.0.0/16
	if ip4[0] == 192 && ip4[1] == 168 {
		return true
	}
	return false
}

func getOrigins(raw string) []string {
	var origins []string
	for _, o := range strings.Split(raw, ",") {
		if o = strings.TrimSpace(o); o != "" {
			origins = append(origins, o)
		}
	}
	return origins
}

// parseTrustedProxies parses TRUSTED_PROXIES as comma-separated IPs or CIDRs
// for gin's SetTrustedProxies. Empty input returns nil, which preserves
// today's behavior (ClientIP returns the direct peer, X-Forwarded-For
// ignored). Open ranges 0.0.0.0/0 and ::/0 are rejected because trusting every
// address would let any client spoof X-Forwarded-For and evade per-IP rate
// limits.
func parseTrustedProxies(raw string) ([]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	var out []string
	for _, entry := range strings.Split(raw, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		if entry == "0.0.0.0/0" || entry == "::/0" {
			return nil, fmt.Errorf("TRUSTED_PROXIES %q trusts every address and would allow IP spoofing; list only your proxy addresses or CIDRs", entry)
		}
		if strings.Contains(entry, "/") {
			if _, _, err := net.ParseCIDR(entry); err != nil {
				return nil, fmt.Errorf("TRUSTED_PROXIES %q is not a valid IP or CIDR", entry)
			}
		} else if ip := net.ParseIP(entry); ip == nil {
			return nil, fmt.Errorf("TRUSTED_PROXIES %q is not a valid IP or CIDR", entry)
		}
		out = append(out, entry)
	}
	return out, nil
}

// mustParseTrustedProxies fails fast at startup on an invalid entry.
func mustParseTrustedProxies(raw string) []string {
	parsed, err := parseTrustedProxies(raw)
	if err != nil {
		log.Fatal(err)
	}
	return parsed
}

func (c *Config) DSN() string {
	u := &url.URL{
		Scheme:  "postgres",
		User:    url.UserPassword(c.DBUser, c.DBPass),
		Host:    net.JoinHostPort(c.DBHost, c.DBPort),
		Path:    "/" + c.DBName,
		RawPath: "/" + url.PathEscape(c.DBName),
	}
	q := url.Values{}
	q.Set("sslmode", c.DBSSLMode)
	u.RawQuery = q.Encode()
	return u.String()
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return fallback
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d > 0 {
			return d
		}
	}
	return fallback
}

// getEnvBool reads a boolean flag: 1, true, yes (any case) enable it,
// everything else (including unset) yields the fallback.
func getEnvBool(key string, fallback bool) bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	switch v {
	case "1", "true", "yes":
		return true
	case "0", "false", "no":
		return false
	default:
		return fallback
	}
}
