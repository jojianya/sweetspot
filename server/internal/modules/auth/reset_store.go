package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// ErrInvalidResetToken is the single generic failure for unknown, expired,
// already-used, or superseded reset tokens. Callers must not distinguish:
// telling "expired" from "unknown" would confirm which emails have accounts.
var ErrInvalidResetToken = errors.New("invalid or expired reset link")

// resetTokenBytes and resetTokenExpiry bound the credential: 256 random bits,
// 30 minutes to live.
const (
	resetTokenBytes  = 32
	resetTokenExpiry = 30 * time.Minute
)

// ResetStore persists single-use password reset tokens. Only hashes reach the
// database; raw tokens exist in memory and in the one email that carries them.
type ResetStore struct {
	pool *pgxpool.Pool
}

func NewResetStore(pool *pgxpool.Pool) *ResetStore {
	return &ResetStore{pool: pool}
}

func hashResetToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func newRawToken() (string, error) {
	b := make([]byte, resetTokenBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// IssueToken invalidates earlier unused tokens for the user and stores a new
// one, returning the raw token to email. Only the latest emailed link works.
func (s *ResetStore) IssueToken(ctx context.Context, userID string) (raw string, expiresAt time.Time, err error) {
	raw, err = newRawToken()
	if err != nil {
		return "", time.Time{}, err
	}
	expiresAt = time.Now().Add(resetTokenExpiry)

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", time.Time{}, err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `
		UPDATE password_resets SET used_at = now()
		WHERE user_id = $1 AND used_at IS NULL
	`, userID); err != nil {
		return "", time.Time{}, err
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO password_resets (user_id, token_hash, expires_at)
		VALUES ($1, $2, $3)
	`, userID, hashResetToken(raw), expiresAt); err != nil {
		return "", time.Time{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", time.Time{}, err
	}
	return raw, expiresAt, nil
}

// ConsumeToken validates the token and, in the same transaction, sets the new
// password hash, revokes every existing session (sessions_valid_after), and
// marks the token used. Any validation failure returns ErrInvalidResetToken.
func (s *ResetStore) ConsumeToken(ctx context.Context, raw, newHash string) (string, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer tx.Rollback(ctx)

	var id, userID string
	var expiresAt time.Time
	var usedAt *time.Time
	err = tx.QueryRow(ctx, `
		SELECT id, user_id, expires_at, used_at FROM password_resets
		WHERE token_hash = $1 FOR UPDATE
	`, hashResetToken(raw)).Scan(&id, &userID, &expiresAt, &usedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrInvalidResetToken
	}
	if err != nil {
		return "", err
	}
	if usedAt != nil || time.Now().After(expiresAt) {
		return "", ErrInvalidResetToken
	}

	if _, err := tx.Exec(ctx, `
		UPDATE users SET password_hash = $2, sessions_valid_after = now()
		WHERE id = $1
	`, userID, newHash); err != nil {
		return "", err
	}
	if _, err := tx.Exec(ctx, `
		UPDATE password_resets SET used_at = now() WHERE id = $1
	`, id); err != nil {
		return "", err
	}
	if err := tx.Commit(ctx); err != nil {
		return "", err
	}
	return userID, nil
}
