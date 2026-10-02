-- 0018_users_sessions_valid_after.sql
--
-- Per-user session revocation floor. Any JWT issued before this timestamp is
-- rejected by the auth middleware, on top of the per-token jti blacklist in
-- Redis (which still handles single-session logout). Password reset sets this
-- to now(), killing every existing session without touching Redis.
--
-- Nullable with no backfill: NULL means "never revoked", which is exactly
-- right for every pre-existing row.

ALTER TABLE users ADD COLUMN sessions_valid_after TIMESTAMPTZ NULL;
