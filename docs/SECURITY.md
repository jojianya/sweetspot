# Security

Status of the security review findings against this codebase. All fixes are applied per finding in dedicated commits.

## Implemented controls

- **Secrets in code/compose removed (H2)**: `admin123`, the placeholder `JWT_SECRET`, and default Redis setups are gone. Live values live in a gitignored root `.env` (`POSTGRES_PASSWORD`, `REDIS_PASSWORD`, `JWT_SECRET`), generated with `openssl rand`. Root `.env.example` documents the required variables. Postgres and Redis publish ports to `127.0.0.1` only, and Redis requires authentication (`requirepass`, healthcheck authenticates).
- **Proxy trust disabled (C3)**: the Gin engine calls `SetTrustedProxies(nil)`, so `ClientIP()` is the raw connection peer and `X-Forwarded-For`/`X-Real-IP` cannot be spoofed to evade per-IP rate limits.
- **Login lockout (M2, fixed with C3)**: an account is locked after 5 failed login attempts within a minute; the counter resets on successful login. Global register (20/min) and login (60/min) caps sit outside the per-IP caps. The counter is keyed on **client IP + identifier**, so failures from one address cannot lock the real owner out from another; a successful login clears only its own pair.
- **Email disclosure (H1)**: `GET /users/:id` returns a public profile without `email`; only the account owner sees their own email (private profile).
- **Request limits (H3)**: 1 MB body cap on JSON endpoints and 64 MB on upload endpoints (`http.MaxBytesReader`); report `reason` is capped at 1000 chars and pin `caption` at 500 chars.
- **Image handling (H4)**: uploads are rejected above 8000x8000 px (checked via metadata before decode) and concurrent libvips operations are capped at 2 to bound memory use.
- **Security headers (M1)**: `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`, a restrictive `Content-Security-Policy`, `Referrer-Policy: no-referrer`, and HSTS.
- **Defense in depth**: bcrypt cost 12 for passwords; passwords never serialized to JSON; SQL is fully parameterized; upload filenames are server-generated; JWT validation pins the exact HS256 method (rejecting HS384/512, `none` and RSA); login errors are generic; CORS is origin-listed, not wildcard; `gin.ReleaseMode` is enforced (`server/internal/http/router.go`).
- Credentials and secrets should be rotated if this repository history has ever been shared outside the repository owner (see the git history note per H2).

## Cookie and CSRF assumptions

This service issues **no anti-CSRF token**. There is no synchronizer token, no
double-submit cookie, and no `Origin`/`Referer` check on any endpoint. CSRF
protection rests entirely on four things working together:

1. **`SameSite=Strict` on `session_token`** (`server/internal/modules/auth/cookie.go`).
   The default is `strict` (`config.defaultCookieSameSite`, pinned by
   `TestCookieSameSiteDefaultsToStrict`); `COOKIE_SAMESITE=lax` is an explicit
   opt-in that exists only for plain-HTTP LAN development, where a strict cookie
   would never be sent and the session would appear broken.
2. **Same-origin API access.** The browser only ever calls `/api/*` on its own
   origin; `client/next.config.ts` rewrites that to the Go server, so the cookie
   is same-origin for every request the app makes.
3. **`HttpOnly`**, so script cannot read the session, and **`Secure`**, which is
   set whenever the request arrived over TLS (directly, or via a trusted proxy's
   `X-Forwarded-Proto` — only from peers in `TRUSTED_PROXIES`).
4. **Origin-listed CORS**, not a wildcard (`server/internal/http/middleware/cors.go`).

**Changing any one of these requires adding real CSRF protection first.** In
particular:

- Setting `COOKIE_SAMESITE=lax` (or `none`) in a deployed environment means
  cross-site top-level GETs will carry the session cookie. Any state-changing
  route reachable by GET would then be CSRF-able. `lax` still blocks cross-site
  POSTs, but that is the only thing holding.
- Serving the client and API from **different origins** removes the same-origin
  rewrite, which is half of what makes `SameSite` sufficient.
- Terminating TLS in front of the Go server **without** listing that proxy in
  `TRUSTED_PROXIES` means `Secure` is never set, so the cookie travels over
  plaintext. Conversely, listing `0.0.0.0/0` lets any client assert
  `X-Forwarded-Proto` and control the flag.

The same-origin rewrite and the cookie attributes are therefore one design, not
two independent hardening steps. The cookie's production attributes are asserted
by `TestSessionCookieIsStrictAndHttpOnlyUnderProductionConfig`.

## Residual low-risk notes (M3/M4)

These are accepted/contextual and require no code change today:

- **Horizontal rate limiting**: the limiter is in-process and in-memory. It is correct for a single instance; a horizontally scaled deployment should back limits with a shared store (e.g. Redis counters) and must not reintroduce forwarded headers without configuring `SetTrustedProxies` for the real proxy chain.
- **Client IP behind the Next proxy**: the browser reaches Go through the Next.js `/api/*` rewrite, which does not forward `X-Forwarded-For`, and Gin is configured with no trusted proxies by default (`TRUSTED_PROXIES` empty). `ClientIP()` therefore returns the Next container's address for every user, so per-IP limits (register/login/pins/comments/`/errors`) are shared per proxy rather than per user. What still holds: the global register/login caps and the per-identifier login lockout do not depend on IP. A full fix needs both halves: the proxy must append `X-Forwarded-For` and `TRUSTED_PROXIES` must list only that proxy's addresses/CIDRs (never `0.0.0.0/0` or `::/0`). Until then, do not treat per-IP counters as per-user attribution in logs or abuse decisions.
- **CSRF**: see "Cookie and CSRF assumptions" above.
- **User enumeration**: public user profiles are retrievable by ID and rate-limited on POST endpoints; GET profile reads are intentionally public and cheap.
- **Static uploads**: `/uploads` is served by the API without authentication, which is consistent with displaying pin photos publicly; upload ingestion remains authenticated and size/dimension limited.
- **Unique pin views**: first opens are recorded per account in `pin_views` (pin, viewer, timestamp) and counted once toward trending; repeats, owner opens and signed-out opens are not recorded. No viewer lists are ever returned by the API.
- **One-second session-revocation window**: a password reset stamps `sessions_valid_after` with microsecond precision, but JWT issued-at is whole seconds, so the middleware compares at second precision (`server/internal/http/middleware/auth.go`). A token minted *before* the reset but inside the same wall second still passes. The window is bounded by one second and only affects tokens already in hand at reset time; exact comparison would need sub-second token timestamps (non-standard, churn across issuance and every consumer), so the window was accepted. Locked by `TestValidateTokenFloorSecondPrecision` and `TestDBPasswordResetImmediateLoginStaysValid`.

## Git history

`server/.env` (containing `DB_PASSWORD=admin123` and `JWT_SECRET=change_me_to_a_long_random_secret`) previously existed in history (`93b2570`..`5253ce6`). New credentials are already rotated and live only in the gitignored `.env`; scrubbing the old history with `git filter-repo`/BFG is recommended hygiene but not required for safety.

## Verification

- `go build ./...`, `go vet ./...`, `go test ./...` from `server/` pass after every change.
- Regression tests live in `server/internal/endpointtest/endpoints_test.go` (spoofed-IP bypass, login lockout/reset, H1 email visibility, oversize body/field lengths, security headers).