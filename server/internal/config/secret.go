package config

import (
	"fmt"
	"strings"
)

// validateJWTSecret checks that the JWT secret is non-empty, at least 32
// characters (256 bits), and not a known placeholder. Placeholders like the
// leaked example value pass a length check while providing zero entropy, so
// length alone is not sufficient.
func validateJWTSecret(secret string) error {
	if secret == "" {
		return fmt.Errorf("JWT_SECRET is required: set it in .env or the environment (generate with: openssl rand -hex 32)")
	}
	if len(secret) < 32 {
		return fmt.Errorf("JWT_SECRET must be at least 32 characters (256 bits)")
	}
	if isPlaceholderSecret(secret) {
		return fmt.Errorf("JWT_SECRET must not be a placeholder value (generate with: openssl rand -hex 32)")
	}
	return nil
}

// isPlaceholderSecret reports well-known non-secrets, case-insensitively:
// the leaked example value, anything starting with change_me/changeme, and
// the bare values "secret" and "password".
func isPlaceholderSecret(secret string) bool {
	lowered := strings.ToLower(secret)
	if lowered == "change_me_to_a_long_random_secret" {
		return true
	}
	if strings.HasPrefix(lowered, "change_me") || strings.HasPrefix(lowered, "changeme") {
		return true
	}
	return lowered == "secret" || lowered == "password"
}
