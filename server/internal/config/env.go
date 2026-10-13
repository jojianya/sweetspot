package config

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
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

// sweepIntervalBounds bound the periodic maintenance loops: "0" disables a
// loop while keeping any boot-time pass, and intervals never run more often
// than once a minute (a sweep is cheap but pointless to spin).
const minSweepInterval = time.Minute

// parseInterval parses a loop-interval env var. Empty means def; "0"
// disables the loop; anything else must be a valid duration at or above the
// minimum. Errors name the variable and the offending value.
func parseInterval(name, raw string, def time.Duration) (time.Duration, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return def, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("%s %q is not a valid duration (e.g. \"1h\", \"15m\")", name, raw)
	}
	if d < 0 {
		return 0, fmt.Errorf("%s %q must not be negative", name, raw)
	}
	if d == 0 {
		return 0, nil
	}
	if d < minSweepInterval {
		return 0, fmt.Errorf("%s %q is below the 1m minimum", name, raw)
	}
	return d, nil
}

// parseSweepInterval parses QUARANTINE_SWEEP_INTERVAL. Empty means hourly;
// "0" disables the periodic loop; anything else must be a valid duration
// at or above the minimum.
func parseSweepInterval(raw string) (time.Duration, error) {
	return parseInterval("QUARANTINE_SWEEP_INTERVAL", raw, time.Hour)
}

// mustParseSweepInterval fails fast at startup on an invalid interval.
func mustParseSweepInterval(raw string) time.Duration {
	d, err := parseSweepInterval(raw)
	if err != nil {
		log.Fatal(err)
	}
	return d
}

// parseResetCleanupInterval parses PASSWORD_RESET_CLEANUP_INTERVAL with the
// same empty/zero/minimum semantics as the sweep loop.
func parseResetCleanupInterval(raw string) (time.Duration, error) {
	return parseInterval("PASSWORD_RESET_CLEANUP_INTERVAL", raw, 24*time.Hour)
}

// mustParseResetCleanupInterval fails fast at startup on an invalid interval.
func mustParseResetCleanupInterval(raw string) time.Duration {
	d, err := parseResetCleanupInterval(raw)
	if err != nil {
		log.Fatal(err)
	}
	return d
}

// parseEnvInt reads a positive integer. An unset or empty value keeps the
// caller's fallback; anything else — unparseable, zero, negative — is an error.
//
// Splitting the parse from the exit mirrors parseInterval/mustParseSweepInterval
// and parseTrustedProxies/mustParseTrustedProxies: the pure half is what tests
// can assert on, and the must half is what call sites use.
func parseEnvInt(key string, fallback int) (int, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("%s %q must be a positive integer", key, v)
	}
	return n, nil
}

// mustEnvInt is parseEnvInt that fails fast, for use in the Load struct literal.
func mustEnvInt(key string, fallback int) int {
	n, err := parseEnvInt(key, fallback)
	if err != nil {
		log.Fatal(err)
	}
	return n
}

// parseEnvDuration reads a positive duration, with the same unset/empty and
// error split as parseEnvInt.
func parseEnvDuration(key string, fallback time.Duration) (time.Duration, error) {
	v := os.Getenv(key)
	if v == "" {
		return fallback, nil
	}
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		return 0, fmt.Errorf("%s %q must be a positive duration", key, v)
	}
	return d, nil
}

// mustEnvDuration is parseEnvDuration that fails fast.
func mustEnvDuration(key string, fallback time.Duration) time.Duration {
	d, err := parseEnvDuration(key, fallback)
	if err != nil {
		log.Fatal(err)
	}
	return d
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
