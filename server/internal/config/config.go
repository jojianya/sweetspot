package config

import (
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	Port               string
	DBHost             string
	DBPort             string
	DBUser             string
	DBPass             string
	DBName             string
	LogLevel           string
	LogFormat          string
	JWTSecret          string
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
		DBHost:             getEnv("DB_HOST", "localhost"),
		DBPort:             getEnv("DB_PORT", "5432"),
		DBUser:             getEnv("DB_USER", "postgres"),
		DBPass:             getEnv("DB_PASSWORD", ""),
		DBName:             getEnv("DB_NAME", "goodspotdb"),
		LogLevel:           getEnv("LOG_LEVEL", "info"),
		LogFormat:          getEnv("LOG_FORMAT", "text"),
		JWTSecret:          getEnv("JWT_SECRET", ""),
		StorageBase:        getEnv("STORAGE_BASE_URL", ""),
		RedisAddr:          getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword:      getEnv("REDIS_PASSWORD", ""),
		CORSAllowedOrigins: getOrigins(getEnv("CORS_ALLOWED_ORIGINS", "http://localhost:3000,http://localhost:3001,http://127.0.0.1:3000,http://127.0.0.1:3001")),
		SentryDSN:          getEnv("SENTRY_DSN", ""),
		SentryEnv:          getEnv("SENTRY_ENV", "development"),
	}

	// A missing JWT_SECRET would silently boot with an empty HMAC key, letting
	// anyone mint tokens for any user/role. Fail fast instead of running
	// insecure. (docker-compose also fail-fasts via `:?` on this variable.)
	if cfg.JWTSecret == "" {
		log.Fatal("JWT_SECRET is required: set it in .env or the environment (generate with: openssl rand -hex 32)")
	}

	return cfg
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
