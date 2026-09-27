package auth

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jojianya/sweetspot247-backend/pkg/password"
)

// Service-level coverage for the password length check.
//
// The bug was a rune/byte mismatch: the DTO's binding tag counted runes while
// bcrypt counts bytes, so a non-ASCII password passed validation and was then
// refused inside bcrypt, surfacing as HTTP 500 on user input. The handler-level
// status codes are covered in internal/endpointtest; these assert the error
// classification the handler depends on.

func TestRegisterRejectsInvalidPassword(t *testing.T) {
	tests := []struct {
		name      string
		password  string
		wantLong  bool
		wantError bool
	}{
		{"too short ascii", "short", false, true},
		{"too short non-ascii", "漢漢", false, true},
		{"at the byte limit", strings.Repeat("a", password.MaxLen), false, false},
		{"at the rune minimum", strings.Repeat("a", password.MinRunes), false, false},
		{"one byte over", strings.Repeat("a", password.MaxLen+1), true, true},
		{"25 three-byte runes", strings.Repeat("漢", 25), true, true},
		{"72 two-byte runes", strings.Repeat("é", 72), true, true},
		{"emoji over the limit", strings.Repeat("\U0001F600", 20), true, true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := newTestService(&stubUserService{})
			_, _, err := svc.Register(context.Background(), RegisterRequest{
				Email:    "new@example.com",
				Password: tc.password,
				Username: "newuser",
			})

			// The boundary cases must still be accepted: a check that rejects
			// 72 bytes would lock out a legitimate password.
			if !tc.wantError {
				if err != nil {
					t.Fatalf("a %d-rune / %d-byte password was rejected: %v",
						len([]rune(tc.password)), len(tc.password), err)
				}
				return
			}

			if err == nil {
				t.Fatalf("a %d-rune / %d-byte password was accepted", len([]rune(tc.password)), len(tc.password))
			}
			// The handler branches on this: 400 rather than 500.
			if !errors.Is(err, ErrInvalidPassword) {
				t.Fatalf("error = %v, want ErrInvalidPassword", err)
			}
			// A rejected password must not be mistaken for a taken email.
			if errors.Is(err, ErrConflict) {
				t.Fatal("a length failure must not be reported as a conflict")
			}
			if got := errors.Is(err, password.ErrTooLong); got != tc.wantLong {
				t.Fatalf("errors.Is(err, ErrTooLong) = %v, want %v (err: %v)", got, tc.wantLong, err)
			}
			// The message reaches the client verbatim, so it must not leak
			// bcrypt's internal wording.
			if strings.Contains(err.Error(), "bcrypt:") {
				t.Fatalf("message leaks bcrypt internals: %q", err.Error())
			}
		})
	}
}

// bcrypt is unreachable for these, so a length rejection must happen before any
// hashing work — the stub would otherwise record a create.
func TestRegisterRejectsBeforeCreatingUser(t *testing.T) {
	svc := newTestService(&stubUserService{})
	if _, _, err := svc.Register(context.Background(), RegisterRequest{
		Email:    "new@example.com",
		Password: strings.Repeat("漢", 30),
		Username: "newuser",
	}); !errors.Is(err, ErrInvalidPassword) {
		t.Fatalf("expected ErrInvalidPassword, got %v", err)
	}
}
