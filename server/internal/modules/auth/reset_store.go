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
	// BatchSize overrides resetCleanupBatch when positive. Production never
	// sets it; only tests use it to shrink one pass so a pack of rows spans
	// several batches.
	BatchSize int
	// AfterBatch, when non-nil, runs right after each DELETE pass commits and
	// before the loop re-checks the context. Production never sets it; tests
	// use it to cancel at a deterministic point instead of racing a timer.
	AfterBatch func()
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

// resetCleanupBatch bounds rows per DELETE so a big first backfill cannot
// hold locks or run long; CleanupPasswordResets repeats until a pass deletes
// less than a full batch.
const resetCleanupBatch = 1000

// CleanupPasswordResets deletes dead reset rows: used ones, and unused ones
// expired for over a day. Neither can ever validate (ConsumeToken requires
// unused and unexpired, with a day of grace past the 30-minute life), and
// IssueToken's UPDATE ... WHERE used_at IS NULL only serializes against the
// janitor on rows already unusable. A cancelled context stops the loop
// between batches, reporting what was deleted with no error: shutdown is
// not a failure.
func (s *ResetStore) CleanupPasswordResets(ctx context.Context) (deleted int, err error) {
	batch := resetCleanupBatch
	if s.BatchSize > 0 {
		batch = s.BatchSize
	}
	for {
		select {
		case <-ctx.Done():
			return deleted, nil
		default:
		}
		tag, err := s.pool.Exec(ctx, `
			DELETE FROM password_resets
			WHERE id IN (
				SELECT id FROM password_resets
				WHERE used_at IS NOT NULL OR expires_at < now() - interval '1 day'
				LIMIT $1
			)
		`, batch)
		if err != nil {
			return deleted, err
		}
		n := int(tag.RowsAffected())
		deleted += n
		if s.AfterBatch != nil {
			s.AfterBatch()
		}
		if n < batch {
			return deleted, nil
		}
	}
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
