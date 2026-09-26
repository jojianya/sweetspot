package password

import (
	"errors"
	"strings"
	"testing"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"
)

// The bug: go-playground/validator counts runes, bcrypt counts bytes. A
// password of 72 multi-byte runes passes a `max=72` tag and is then rejected by
// bcrypt, which surfaced to the user as a 500. These tests pin the byte limit so
// the two cannot drift apart again.

func TestValidate(t *testing.T) {
	tests := []struct {
		name     string
		password string
		wantErr  bool
	}{
		{"ascii at the byte limit", strings.Repeat("a", 72), false},
		{"ascii over the byte limit", strings.Repeat("a", 73), true},
		{"ascii at the rune minimum", strings.Repeat("a", 8), false},
		{"ascii under the rune minimum", strings.Repeat("a", 7), true},
		{"empty", "", true},

		// The exact cases that used to slip through: within 72 runes, over 72
		// bytes. These must be rejected.
		{"24 three-byte runes is exactly the limit", strings.Repeat("漢", 24), false},
		{"25 three-byte runes is over", strings.Repeat("漢", 25), true},
		{"72 three-byte runes is far over", strings.Repeat("漢", 72), true},
		{"72 two-byte runes is over", strings.Repeat("é", 72), true},
		{"36 two-byte runes is exactly the limit", strings.Repeat("é", 36), false},
		// 8 emoji: 4 bytes each, 32 total — fine on both counts.
		{"eight emoji", strings.Repeat("\U0001F600", 8), false},
		{"emoji over the byte limit", strings.Repeat("\U0001F600", 20), true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := Validate(tc.password)
			if tc.wantErr != (err != nil) {
				t.Fatalf("Validate(%d runes, %d bytes) error = %v, wantErr %v",
					utf8.RuneCountInString(tc.password), len(tc.password), err, tc.wantErr)
			}
		})
	}
}

// TestValidateAgreesWithBcrypt is the assertion that matters: whatever Validate
// accepts, bcrypt must also accept. If bcrypt's limit ever changes, this fails
// instead of the mismatch resurfacing as a 500.
func TestValidateAgreesWithBcrypt(t *testing.T) {
	for _, p := range []string{
		strings.Repeat("a", 71), strings.Repeat("a", 72), strings.Repeat("a", 73),
		strings.Repeat("漢", 23), strings.Repeat("漢", 24), strings.Repeat("漢", 25),
		strings.Repeat("é", 35), strings.Repeat("é", 36), strings.Repeat("é", 37),
		strings.Repeat("\U0001F600", 8), strings.Repeat("\U0001F600", 19),
	} {
		runes, bytes := utf8.RuneCountInString(p), len(p)
		_, bcryptErr := bcrypt.GenerateFromPassword([]byte(p), bcrypt.MinCost)
		bcryptOK := bcryptErr == nil

		if bcryptOK != (Validate(p) == nil) {
			t.Fatalf("Validate disagrees with bcrypt for %d runes / %d bytes: Validate ok=%v, bcrypt ok=%v",
				runes, bytes, Validate(p) == nil, bcryptOK)
		}
	}
}

func TestHashRejectsOverLongPassword(t *testing.T) {
	for _, p := range []string{strings.Repeat("漢", 25), strings.Repeat("a", 73)} {
		if _, err := Hash(p); !errors.Is(err, ErrTooLong) {
			t.Fatalf("Hash(%d bytes) error = %v, want ErrTooLong", len(p), err)
		}
	}
}

// The sentinel has to match bcrypt's own error, so callers can check either.
func TestErrTooLongWrapsBcryptError(t *testing.T) {
	if !errors.Is(ErrTooLong, bcrypt.ErrPasswordTooLong) {
		t.Fatal("ErrTooLong must wrap bcrypt.ErrPasswordTooLong")
	}
	if errors.Is(ErrTooLong, bcrypt.ErrMismatchedHashAndPassword) {
		t.Fatal("ErrTooLong must not match an unrelated bcrypt error")
	}
}

// The auth handler returns this message to the person registering, so it must
// read as guidance rather than as an internal error.
func TestErrTooLongMessageIsUserFacing(t *testing.T) {
	msg := ErrTooLong.Error()
	if strings.Contains(msg, "bcrypt:") {
		t.Fatalf("message leaks bcrypt internals: %q", msg)
	}
	if !strings.Contains(msg, "72") {
		t.Fatalf("message should state the limit, got %q", msg)
	}
}

func TestHashAndVerifyRoundTrip(t *testing.T) {
	p := "correct horse battery staple"
	hash, err := Hash(p)
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	if !Verify(p, hash) {
		t.Fatal("Verify should accept the original password")
	}
	if Verify("wrong", hash) {
		t.Fatal("Verify should reject a wrong password")
	}
}

// A password at the byte limit must still round-trip, so the limit is not
// enforced by mangling the input.
func TestVerifyAtByteLimit(t *testing.T) {
	p := strings.Repeat("漢", 24) // exactly 72 bytes
	hash, err := Hash(p)
	if err != nil {
		t.Fatalf("Hash at the byte limit: %v", err)
	}
	if !Verify(p, hash) {
		t.Fatal("a 72-byte password should verify against its own hash")
	}
	if Verify(p+"漢", hash) {
		t.Fatal("a password one rune past the limit must not verify")
	}
}

func TestVerifyRejectsOverLongPassword(t *testing.T) {
	// Build the hash from a password inside the limit, then submit one past it.
	// Verify must refuse rather than treat the shared prefix as a match.
	hash, err := Hash(strings.Repeat("a", 72))
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	if Verify(strings.Repeat("a", 73), hash) {
		t.Fatal("an over-long password must not verify")
	}
	// A different password that shares the first 72 bytes with the stored one but
	// differs after — no prefix matching may leak in.
	if Verify(strings.Repeat("a", 72)+"extra", hash) {
		t.Fatal("an over-long password sharing a prefix must not verify")
	}
}
