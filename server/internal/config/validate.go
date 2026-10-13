package config

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"time"
)

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

// validateDBSSLMode checks the Postgres sslmode. Unknown values always fail;
// in production anything weaker than require fails too, so the DB password
// and data never travel in cleartext off-host.
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
