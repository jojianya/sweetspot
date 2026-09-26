// Package password wraps bcrypt, which has a hard 72-byte input limit.
//
// The limit is in bytes, while the length validation on the request DTO counts
// runes. Those disagree for any non-ASCII password, so a password can pass
// validation and then be rejected by bcrypt. This package owns the byte limit
// and the validation that has to match it, so the two cannot drift.
package password

import (
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

const bcryptCost = 12

// MaxLen is the longest password bcrypt will accept, in bytes. Anything longer
// is rejected by bcrypt itself, so callers must not submit it.
const MaxLen = 72

// ErrTooLong is returned by Hash and Verify for input past MaxLen bytes.
//
// bcrypt reports this natively, but a sentinel lets callers distinguish an
// over-long password from a genuine internal failure and answer 400 instead of
// 500. Its message is written for the person registering, since the auth handler
// returns it verbatim — it must not leak bcrypt's internal wording. It still
// satisfies errors.Is(err, bcrypt.ErrPasswordTooLong).
var ErrTooLong error = &tooLongError{}

type tooLongError struct{}

func (*tooLongError) Error() string {
	return fmt.Sprintf("password must be %d bytes or fewer", MaxLen)
}

func (*tooLongError) Is(target error) bool {
	return target == bcrypt.ErrPasswordTooLong
}

// MinRunes is the shortest password accepted at registration, counted in runes
// so that "8 characters" means 8 characters to the person typing it, whatever
// alphabet they are using.
const MinRunes = 8

// Validate reports whether p is acceptable for registration: at least MinRunes
// runes, and within MaxLen bytes.
//
// Both bounds are checked in this one place because they are measured in
// different units, which is exactly the mismatch that caused the original bug.
func Validate(p string) error {
	if n := len([]rune(p)); n < MinRunes {
		return fmt.Errorf("password must be at least %d characters", MinRunes)
	}
	if len(p) > MaxLen {
		return ErrTooLong
	}
	return nil
}

func Hash(plain string) (string, error) {
	if len(plain) > MaxLen {
		return "", ErrTooLong
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(plain), bcryptCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

func Verify(plain, hash string) bool {
	if len(plain) > MaxLen {
		// bcrypt cannot hash input this long, so it cannot match any stored
		// hash. Returning false here keeps the answer consistent instead of
		// depending on how bcrypt treats over-long input internally.
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}
