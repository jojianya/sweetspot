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

func TestValidateAcceptsAnyHMACAlgorithm(t *testing.T) {
	// Characterization, not endorsement: the keyfunc accepts every
	// SigningMethodHMAC (HS256/384/512) with the shared secret and only
	// rejects non-HMAC algorithms. Reported separately; do not "fix" here.
	hs384 := jwt.NewWithClaims(jwt.SigningMethodHS384, Claims{UserID: "user-1"})
	tokenString, err := hs384.SignedString([]byte("secret"))
	if err != nil {
		t.Fatalf("sign HS384: %v", err)
	}
	if _, err := Validate("secret", tokenString); err != nil {
		t.Errorf("Validate rejected an HS384 token: %v", err)
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
