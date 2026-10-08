// Package password wraps bcrypt, which has a hard 72-byte input limit.
//
// The limit is in bytes, while the length validation on the request DTO counts
// runes. Those disagree for any non-ASCII password, so a password can pass
// validation and then be rejected by bcrypt. This package owns the byte limit
// and the validation that has to match it, so the two cannot drift.
package password

import (
	"crypto/rand"
	"fmt"
	"sync"

	"golang.org/x/crypto/bcrypt"
)

const bcryptCost = 12

// AbsentAccountHash is a bcrypt hash of a value no one can submit.
// It is generated lazily once, on first use, using the same cost as real passwords,
// so a missing account costs the same as a wrong password and the two
// cannot be told apart by response time. It must have cost >= 12
// to match the timing of real password comparisons.
var AbsentAccountHash string

// AbsentAccountHash is generated lazily once, on first use, using the same
// bcrypt cost as real passwords. This avoids generating it if the package
// is imported but never used for login (e.g. in test environments without
// a database).
var absentAccountHashOnce sync.Once
var absentAccountHashErr error

func initAbsentAccountHash() {
	hash, err := GenerateAbsentAccountHash()
	if err != nil {
		absentAccountHashErr = err
		return
	}
	AbsentAccountHash = hash
}

func init() {
	absentAccountHashOnce.Do(initAbsentAccountHash)
}

// GetAbsentAccountHash returns the absent account hash, ensuring it is
// initialized via sync.Once if not already done.
func GetAbsentAccountHash() string {
	absentAccountHashOnce.Do(initAbsentAccountHash)
	return AbsentAccountHash
}

// EnsureAbsentAccountHashInitialized forces initialization of the absent
// account hash. Returns an error if initialization failed.
func EnsureAbsentAccountHashInitialized() error {
	absentAccountHashOnce.Do(initAbsentAccountHash)
	if absentAccountHashErr != nil {
		return absentAccountHashErr
	}
	return nil
}

// GenerateAbsentAccountHash generates a bcrypt hash of a random value that no one can submit.
// The caller (Login) compares passwords against this hash when the account does not exist,
// so a missing account costs the same as a wrong password and the two cannot be told apart
// by response time. The cost matches real password hashing so the timing channel is preserved.
func GenerateAbsentAccountHash() (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	// The random byte string is chosen so it will never match a real password,
	// ensuring the timing-safe login guard works correctly.
	randomStr := string(buf)
	if len(randomStr) > MaxLen {
		return "", ErrTooLong
	}
	return Hash(randomStr)
}

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
