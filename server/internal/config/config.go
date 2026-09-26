package config

import (
	"fmt"
	"log"
	"net"
	"net/url"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	Port               string
	AppEnv             string
	DBHost             string
	DBPort             string
	DBUser             string
	DBPass             string
	DBName             string
	LogLevel           string
	LogFormat          string
	JWTSecret          string
	StorageBackend     string
	StorageBase        string
	RedisAddr          string
	RedisPassword      string
	CORSAllowedOrigins []string
	SentryDSN          string
	SentryEnv          string
}

func Load() *Config {
	if err := godotenv.Load(); err != nil {
		log.Println("no .env file found, reading from environment")
	}

	cfg := &Config{
		Port:               getEnv("PORT", "8080"),
		AppEnv:             strings.ToLower(getEnv("APP_ENV", "development")),
		DBHost:             getEnv("DB_HOST", "localhost"),
		DBPort:             getEnv("DB_PORT", "5432"),
		DBUser:             getEnv("DB_USER", "postgres"),
		DBPass:             getEnv("DB_PASSWORD", ""),
		DBName:             getEnv("DB_NAME", "goodspotdb"),
		LogLevel:           getEnv("LOG_LEVEL", "info"),
		LogFormat:          getEnv("LOG_FORMAT", "text"),
		JWTSecret:          getEnv("JWT_SECRET", ""),
		StorageBackend:     getEnv("STORAGE_BACKEND", "local"),
		StorageBase:        getEnv("STORAGE_BASE_URL", "http://localhost:8081"),
		RedisAddr:          getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword:      getEnv("REDIS_PASSWORD", ""),
		CORSAllowedOrigins: getOrigins(getEnv("CORS_ALLOWED_ORIGINS", "http://localhost:3000,http://localhost:3001,http://localhost:3002,http://127.0.0.1:3000,http://127.0.0.1:3001,http://127.0.0.1:3002")),
		SentryDSN:          getEnv("SENTRY_DSN", ""),
		SentryEnv:          getEnv("SENTRY_ENV", "development"),
	}

	// A missing JWT_SECRET would silently boot with an empty HMAC key, letting
	// anyone mint tokens for any user/role. Fail fast instead of running
	// insecure. (docker-compose also fail-fasts via `:?` on this variable.)
	if cfg.JWTSecret == "" {
		log.Fatal("JWT_SECRET is required: set it in .env or the environment (generate with: openssl rand -hex 32)")
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

	return cfg
}

func validateAppEnv(value string) error {
	if value != "development" && value != "production" {
		return fmt.Errorf("APP_ENV must be development or production, got %q", value)
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
	if parsed.Scheme != "https" && !loopback {
		return fmt.Errorf("STORAGE_BASE_URL must use https unless it is a local address")
	}
	if strings.EqualFold(appEnv, "production") && loopback {
		return fmt.Errorf("STORAGE_BASE_URL must be publicly reachable in production")
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

func getOrigins(raw string) []string {
	var origins []string
	for _, o := range strings.Split(raw, ",") {
		if o = strings.TrimSpace(o); o != "" {
			origins = append(origins, o)
		}
	}
	return origins
}

func (c *Config) DSN() string {
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		c.DBHost, c.DBPort, c.DBUser, c.DBPass, c.DBName)
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
