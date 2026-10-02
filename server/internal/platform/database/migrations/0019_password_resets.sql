-- 0019_password_resets.sql
--
-- Single-use password reset tokens. Only the SHA-256 hash is stored; the raw
-- token is emailed once and never persisted. Expiry is enforced in the reset
-- transaction (used_at IS NULL AND expires_at > now()), and issuing a new
-- token marks earlier unused ones for the same user used, so only the latest
-- emailed link can work.

CREATE TABLE password_resets (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash TEXT NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at    TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX password_resets_user_idx ON password_resets (user_id, created_at DESC);
