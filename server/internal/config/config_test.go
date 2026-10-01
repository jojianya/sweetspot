package config

import (
	"os"
	"strings"
	"testing"
)

func TestValidateAppEnv(t *testing.T) {
	for _, value := range []string{"development", "production"} {
		if err := validateAppEnv(value); err != nil {
			t.Fatalf("expected %q to be valid: %v", value, err)
		}
	}
	if err := validateAppEnv("prod"); err == nil {
		t.Fatal("expected an unknown environment to be rejected")
	}
}

func TestValidateJWTSecret(t *testing.T) {
	tests := []struct {
		name    string
		secret  string
		wantErr bool
	}{
		{"valid 32 chars", strings.Repeat("a", 32), false},
		{"valid 64 chars", strings.Repeat("b", 64), false},
		{"empty", "", true},
		{"too short 31", strings.Repeat("c", 31), true},
		{"too short 16", strings.Repeat("d", 16), true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateJWTSecret(tc.secret)
			if tc.wantErr {
				if err == nil {
					t.Errorf("validateJWTSecret(%q): expected error, got nil", tc.secret)
				}
			} else {
				if err != nil {
					t.Errorf("validateJWTSecret(%q): unexpected error: %v", tc.secret, err)
				}
			}
		})
	}
}

func TestValidateStorageBackend(t *testing.T) {
	if err := validateStorageBackend("local"); err != nil {
		t.Fatalf("expected local storage to be supported: %v", err)
	}
	if err := validateStorageBackend("r2"); err == nil {
		t.Fatal("expected unimplemented r2 backend to be rejected")
	}
}

func TestValidateCookieSameSite(t *testing.T) {
	// Strict is the default and must stay valid without any opt-in, because
	// it is the CSRF-safe mode. Lax is only ever reachable via the explicit
	// COOKIE_SAMESITE=lax flag (needed for plain-HTTP LAN development).
	for _, ok := range []string{"strict", "lax", "STRICT", " lax "} {
		if err := validateCookieSameSite(strings.ToLower(strings.TrimSpace(ok))); err != nil {
			t.Errorf("validateCookieSameSite(%q): unexpected error: %v", ok, err)
		}
	}

	for _, bad := range []string{"", "none", "StrictMode", "laxx", "disabled"} {
		if err := validateCookieSameSite(bad); err == nil {
			t.Errorf("validateCookieSameSite(%q): expected rejection, got nil", bad)
		}
	}
}

func TestValidateStorageBase(t *testing.T) {
	t.Run("LocalDevelopment", func(t *testing.T) {
		if err := validateStorageBase("http://localhost:8081", "development"); err != nil {
			t.Fatalf("expected local media URL to be valid: %v", err)
		}
	})

	t.Run("PublicProduction", func(t *testing.T) {
		if err := validateStorageBase("https://media.example.com", "production"); err != nil {
			t.Fatalf("expected public media URL to be valid: %v", err)
		}
	})

	tests := []struct {
		name   string
		value  string
		appEnv string
	}{
		{name: "Missing", value: "", appEnv: "development"},
		{name: "Relative", value: "/uploads", appEnv: "development"},
		{name: "UnsupportedScheme", value: "ftp://media.example.com", appEnv: "development"},
		{name: "PublicHTTP", value: "http://media.example.com", appEnv: "development"},
		{name: "ProductionLoopback", value: "https://localhost:8081", appEnv: "production"},
		{name: "Credentials", value: "https://user:pass@media.example.com", appEnv: "production"},
		{name: "Query", value: "https://media.example.com?token=secret", appEnv: "production"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := validateStorageBase(tt.value, tt.appEnv); err == nil {
				t.Fatalf("expected %q to be rejected", tt.value)
			}
		})
	}
}

// TestGetEnvInt tests the getEnvInt helper.
func TestGetEnvInt(t *testing.T) {
	oldVal := os.Getenv("TEST_INT")
	defer func() {
		if oldVal == "" {
			os.Unsetenv("TEST_INT")
		} else {
			os.Setenv("TEST_INT", oldVal)
		}
	}()

	os.Setenv("TEST_INT", "42")
	if got := getEnvInt("TEST_INT", 0); got != 42 {
		t.Errorf("getEnvInt(TEST_INT, 0) = %d, want 42", got)
	}

	os.Setenv("TEST_INT", "invalid")
	if got := getEnvInt("TEST_INT", 99); got != 99 {
		t.Errorf("getEnvInt(TEST_INT, 99) = %d, want 99 (fallback)", got)
	}

	os.Unsetenv("TEST_INT")
	if got := getEnvInt("TEST_INT", 7); got != 7 {
		t.Errorf("getEnvInt(unset, 7) = %d, want 7 (fallback)", got)
	}

	// Zero and negative should fall back
	os.Setenv("TEST_INT", "0")
	if got := getEnvInt("TEST_INT", 5); got != 5 {
		t.Errorf("getEnvInt(TEST_INT=0, 5) = %d, want 5 (fallback)", got)
	}
	os.Setenv("TEST_INT", "-1")
	if got := getEnvInt("TEST_INT", 5); got != 5 {
		t.Errorf("getEnvInt(TEST_INT=-1, 5) = %d, want 5 (fallback)", got)
	}
}
