# Security

Status of the security review findings against this codebase. All fixes are applied per finding in dedicated commits.

## Implemented controls

- **Secrets in code/compose removed (H2)**: `admin123`, the placeholder `JWT_SECRET`, and default Redis setups are gone. Live values live in a gitignored root `.env` (`POSTGRES_PASSWORD`, `REDIS_PASSWORD`, `JWT_SECRET`), generated with `openssl rand`. Root `.env.example` documents the required variables. Postgres and Redis publish ports to `127.0.0.1` only, and Redis requires authentication (`requirepass`, healthcheck authenticates).
- **Proxy trust disabled (C3)**: the Gin engine calls `SetTrustedProxies(nil)`, so `ClientIP()` is the raw connection peer and `X-Forwarded-For`/`X-Real-IP` cannot be spoofed to evade per-IP rate limits.
- **Login lockout (M2, fixed with C3)**: an account is locked after 5 failed login attempts within a minute; the counter resets on successful login. Global register (20/min) and login (60/min) caps sit outside the per-IP caps.
- **Email disclosure (H1)**: `GET /users/:id` returns a public profile without `email`; only the account owner sees their own email (private profile).
- **Request limits (H3)**: 1 MB body cap on JSON endpoints and 64 MB on upload endpoints (`http.MaxBytesReader`); report `reason` is capped at 1000 chars and pin `caption` at 500 chars.
- **Image handling (H4)**: uploads are rejected above 8000x8000 px (checked via metadata before decode) and concurrent libvips operations are capped at 2 to bound memory use.
- **Security headers (M1)**: `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`, a restrictive `Content-Security-Policy`, `Referrer-Policy: no-referrer`, and HSTS.
- **Defense in depth**: bcrypt cost 12 for passwords; passwords never serialized to JSON; SQL is fully parameterized; upload filenames are server-generated; JWT validation rejects non-HMAC algorithms; login errors are generic; CORS is origin-listed, not wildcard; `gin.ReleaseMode` is enforced (`server/internal/http/router.go`).
- Credentials and secrets should be rotated if this repository history has ever been shared outside the repository owner (see the git history note per H2).

## Residual low-risk notes (M3/M4)

These are accepted/contextual and require no code change today:

- **Horizontal rate limiting**: the limiter is in-process and in-memory. It is correct for a single instance; a horizontally scaled deployment should back limits with a shared store (e.g. Redis counters) and must not reintroduce forwarded headers without configuring `SetTrustedProxies` for the real proxy chain.
- **CSRF**: all state-changing routes require a `Bearer` JWT in the `Authorization` header, which browsers do not attach automatically; there is no cookie-based session, so cross-site request forgery does not apply to the API. Revisit if cookie authentication is ever introduced.
- **User enumeration**: public user profiles are retrievable by ID and rate-limited on POST endpoints; GET profile reads are intentionally public and cheap.
- **Static uploads**: `/uploads` is served by the API without authentication, which is consistent with displaying pin photos publicly; upload ingestion remains authenticated and size/dimension limited.

## Git history

`server/.env` (containing `DB_PASSWORD=admin123` and `JWT_SECRET=change_me_to_a_long_random_secret`) previously existed in history (`93b2570`..`5253ce6`). New credentials are already rotated and live only in the gitignored `.env`; scrubbing the old history with `git filter-repo`/BFG is recommended hygiene but not required for safety.

## Verification

- `go build ./...`, `go vet ./...`, `go test ./...` from `server/` pass after every change.
- Regression tests live in `server/internal/endpointtest/endpoints_test.go` (spoofed-IP bypass, login lockout/reset, H1 email visibility, oversize body/field lengths, security headers).