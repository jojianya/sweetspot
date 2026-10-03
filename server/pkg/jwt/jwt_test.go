package jwt

// Characterization of the session token contract: round trip, expiry,
// wrong secret, wrong algorithm and tampering. The middleware and the
// no-role-claim invariant (live_role_test.go) build on exactly this.

import (
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestGenerateValidateRoundTrip(t *testing.T) {
	tokenString, err := Generate("secret", "user-1", time.Hour)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	claims, err := Validate("secret", tokenString)
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if claims.UserID != "user-1" {
		t.Errorf("UserID = %q, want %q", claims.UserID, "user-1")
	}
	if claims.ID == "" {
		t.Error("expected a JTI claim, got empty")
	}
	if claims.ExpiresAt == nil || time.Until(claims.ExpiresAt.Time) > time.Hour {
		t.Errorf("expiry not ~1h out: %v", claims.ExpiresAt)
	}
	if claims.IssuedAt == nil {
		t.Error("expected an IssuedAt claim, got nil")
	}
}

func TestValidateRejectsExpired(t *testing.T) {
	tokenString, err := Generate("secret", "user-1", -time.Hour)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if _, err := Validate("secret", tokenString); err == nil {
		t.Error("Validate accepted an expired token")
	}
}

func TestValidateRejectsWrongSecret(t *testing.T) {
	tokenString, err := Generate("secret", "user-1", time.Hour)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if _, err := Validate("other-secret", tokenString); err == nil {
		t.Error("Validate accepted a token signed with a different secret")
	}
}

func TestValidateAcceptsOnlyHS256(t *testing.T) {
	// HS384 and HS512 share the HMAC family but must not verify: only the
	// HS256 tokens Generate issues are valid.
	for _, method := range []any{jwt.SigningMethodHS384, jwt.SigningMethodHS512} {
		signed, err := jwt.NewWithClaims(method.(jwt.SigningMethod), Claims{UserID: "user-1"}).
			SignedString([]byte("secret"))
		if err != nil {
			t.Fatalf("sign: %v", err)
		}
		if _, err := Validate("secret", signed); err == nil {
			t.Errorf("Validate accepted a %v token", method)
		}
	}

	none := jwt.NewWithClaims(jwt.SigningMethodNone, Claims{UserID: "user-1"})
	unsigned, err := none.SignedString(jwt.UnsafeAllowNoneSignatureType)
	if err != nil {
		t.Fatalf("sign none: %v", err)
	}
	if _, err := Validate("secret", unsigned); err == nil {
		t.Error("Validate accepted an unsigned (none-algorithm) token")
	}
}

func TestValidateRejectsTampered(t *testing.T) {
	tokenString, err := Generate("secret", "user-1", time.Hour)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	parts := strings.Split(tokenString, ".")
	if len(parts) != 3 {
		t.Fatalf("expected 3 JWT parts, got %d", len(parts))
	}
	// Flip the payload segment: the signature must stop matching.
	tampered := parts[0] + "." + parts[1] + "A" + "." + parts[2]
	if _, err := Validate("secret", tampered); err == nil {
		t.Error("Validate accepted a tampered token")
	}
	if _, err := Validate("secret", "not-a-token"); err == nil {
		t.Error("Validate accepted a malformed token")
	}
}
