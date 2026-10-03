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
- **Defense in depth**: bcrypt cost 12 for passwords; passwords never serialized to JSON; SQL is fully parameterized; upload filenames are server-generated; JWT validation pins the exact HS256 method (rejecting HS384/512, `none` and RSA); login errors are generic; CORS is origin-listed, not wildcard; `gin.ReleaseMode` is enforced (`server/internal/http/router.go`).
- Credentials and secrets should be rotated if this repository history has ever been shared outside the repository owner (see the git history note per H2).

## Residual low-risk notes (M3/M4)

These are accepted/contextual and require no code change today:

- **Horizontal rate limiting**: the limiter is in-process and in-memory. It is correct for a single instance; a horizontally scaled deployment should back limits with a shared store (e.g. Redis counters) and must not reintroduce forwarded headers without configuring `SetTrustedProxies` for the real proxy chain.
- **Client IP behind the Next proxy**: the browser reaches Go through the Next.js `/api/*` rewrite, which does not forward `X-Forwarded-For`, and Gin is configured with no trusted proxies by default (`TRUSTED_PROXIES` empty). `ClientIP()` therefore returns the Next container's address for every user, so per-IP limits (register/login/pins/comments/`/errors`) are shared per proxy rather than per user. What still holds: the global register/login caps and the per-identifier login lockout do not depend on IP. A full fix needs both halves: the proxy must append `X-Forwarded-For` and `TRUSTED_PROXIES` must list only that proxy's addresses/CIDRs (never `0.0.0.0/0` or `::/0`). Until then, do not treat per-IP counters as per-user attribution in logs or abuse decisions.
- **CSRF**: the session is cookie-primary. `session_token` is httpOnly and `SameSite=Strict` by default (`server/internal/modules/auth/cookie.go`), tried before the `Bearer` fallback (`server/internal/http/middleware/auth.go`). CSRF protection today is `SameSite` + `Secure` (set iff TLS/`X-Forwarded-Proto:https`) + the same-origin `/api/*` proxy (`client/next.config.ts`) + origin-listed CORS (`server/internal/http/middleware/cors.go`). There is no anti-CSRF token and no `Origin`/`Referer` check, so this is weaker than token-based CSRF defense: `COOKIE_SAMESITE=lax` (needed for plain-HTTP LAN per root `.env.example`) still blocks cross-site POSTs but allows top-level GETs to carry the cookie, plain HTTP drops `Secure`, and a broad `CORS_ALLOWED_ORIGINS` widens the trusted set. Keep `strict` + HTTPS + narrow CORS in production.
- **User enumeration**: public user profiles are retrievable by ID and rate-limited on POST endpoints; GET profile reads are intentionally public and cheap.
- **Static uploads**: `/uploads` is served by the API without authentication, which is consistent with displaying pin photos publicly; upload ingestion remains authenticated and size/dimension limited.
- **Unique pin views**: first opens are recorded per account in `pin_views` (pin, viewer, timestamp) and counted once toward trending; repeats, owner opens and signed-out opens are not recorded. No viewer lists are ever returned by the API.
- **One-second session-revocation window**: a password reset stamps `sessions_valid_after` with microsecond precision, but JWT issued-at is whole seconds, so the middleware compares at second precision (`server/internal/http/middleware/auth.go`). A token minted *before* the reset but inside the same wall second still passes. The window is bounded by one second and only affects tokens already in hand at reset time; exact comparison would need sub-second token timestamps (non-standard, churn across issuance and every consumer), so the window was accepted. Locked by `TestValidateTokenFloorSecondPrecision` and `TestDBPasswordResetImmediateLoginStaysValid`.

## Git history

`server/.env` (containing `DB_PASSWORD=admin123` and `JWT_SECRET=change_me_to_a_long_random_secret`) previously existed in history (`93b2570`..`5253ce6`). New credentials are already rotated and live only in the gitignored `.env`; scrubbing the old history with `git filter-repo`/BFG is recommended hygiene but not required for safety.

## Verification

- `go build ./...`, `go vet ./...`, `go test ./...` from `server/` pass after every change.
- Regression tests live in `server/internal/endpointtest/endpoints_test.go` (spoofed-IP bypass, login lockout/reset, H1 email visibility, oversize body/field lengths, security headers).