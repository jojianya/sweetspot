package config

import "testing"

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

func TestValidateStorageBackend(t *testing.T) {
	if err := validateStorageBackend("local"); err != nil {
		t.Fatalf("expected local storage to be supported: %v", err)
	}
	if err := validateStorageBackend("r2"); err == nil {
		t.Fatal("expected unimplemented r2 backend to be rejected")
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
