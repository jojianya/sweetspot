# Codebase Map: sweetspot247-backend

Generated from codebase scan on 2026-10-08.

---

## 1. Structure

**Top-level layout:**
- `.git/` `.github/` `.pebble-run/` `certbot/` `client/` `docs/` `nginx/` `scripts/` `secrets/` `server/` `.dockerignore` `.env` `.env.example` `.gitignore` `docker-compose.prod.yml` `docker-compose.yml`

**Server (Go/Gin):** `server/cmd/api/main.go` entry point. Packages: `internal/{config,di,http,modules,observability,platform}`, `pkg/{geohash,jwt,password}`. Compiles to binary served via Docker.

**Client (Next.js 14):** `client/` with `src/` (app router + React), `public/`, `next.config.ts`, `package.json`. Built with `pnpm`.

**Nginx:** `nginx/default.conf.template` edge proxy with dev certbot sidecar. TLS termination in prod.

**Docker Compose:** Two stacks:
- `docker-compose.yml` — dev: postgres (postgis), redis, server (hot-reload), client (HMR), nginx (dev proxy).
- `docker-compose.prod.yml` — prod: proxy+certbot, server (no published ports), client, postgres (internal, SSL), redis (internal).

**Languages & frameworks:**
- Go 1.27 (server), Gin web framework, jackc/pgx v5 for Postgres.
- Node 22 + Next.js 14 (client), React, MapLibre-GL for mapping.
- PostgreSQL + PostGIS 16 for spatial data.
- Redis 7 for blacklist + SSE pub/sub.

**Main entry points:**
- Server: `cmd/api/main.go` — loads config, connects DB, runs migrations, starts Gin router.
- Client: `client/src/app/layout.tsx` (RootLayout), page routing via Next.js app router.
- Nginx: `nginx/Dockerfile` + `docker-entrypoint.d/30-bootstrap-certs.sh`.

---

## 2. Server (Go)

**Routes/Endpoints** (from `server/internal/modules/pins/routes.go`):

| Method | Path | Handler |
|---|---|---|
| GET | `/health` | `response.JSON ok` |
| GET | `/ready` | readiness check (DB+Redis) |
| GET | `/api/pins` | `h.GetPins` (bbox+category+limit) |
| GET | `/api/pins/search` | `h.SearchPins` (query+limit) |
| GET | `/api/pins/trending` | `h.GetTrending` (bbox+limit) |
| GET | `/api/users/:id/pins` | `h.ListByUser` |
| GET | `/api/pins/:id` | `h.GetPin` |
| POST | `/api/pins` | `h.CreatePin` (multipart, auth) |
| PATCH | `/api/pins/:id` | `h.UpdatePin` (multipart, auth) |
| DELETE | `/api/pins/:id` | `h.DeletePin` (auth) |
| POST | `/api/pins/:id/view` | `h.RegisterView` |
| GET | `/api/categories` | `h.ListCategories` |
| GET | `/api/events` | `realtimeHandler.Stream` (SSE) |
| POST | `/api/errors` | `ClientErrorIngest` (rate-limited) |
| POST/PATCH/DELETE | `/api/users/me` | user endpoints (multipart) |
| POST | `/api/register` / `/api/login` / `/api/logout` | auth |
| PATCH | `/api/users/me/role` | admin-only role change |

**Middleware** (from `server/internal/http/middleware/`):
- `Recover` — recover from panics
- `RequestLogger` — structured logger
- `ReportErrors` — report to Sentry/reporter
- `CORS` — configured from `CORS_ALLOWED_ORIGINS` env; defaults localhost + goodspot.test; credentials enabled
- `SecurityHeaders` — HSTS, X-Frame-Options, etc.
- `BodyLimit` — 1MB for JSON routes, 64MB for multipart (upload/pins, users/me)
- `AuthRequired` — validates JWT cookie or Bearer header + Redis blacklist + sessions_valid_after floor
- `OptionalAuth` — sets userID if cookie present, otherwise continues anonymously
- `rate_limit` — per-IP in-memory limiter (10/min create, 600/min view per-IP, 60/min per-user)
- `Report` — per-IP rate limit for error reports (30/min)

**Config & env vars** (from `server/internal/config/config.go`):
- Loaded via `godotenv.Load("../.env")` then `.env`
- 30+ env vars: `PORT`, `APP_ENV`, `DB_HOST/PORT/USER/PASS/NAME/SSLMODE`, `DB_POOL_MAX_CONNS/LIFETIME/IDLE/HEALTH_CHECK`, `LOG_LEVEL/FORMAT`, `JWT_SECRET`, `STORAGE_BACKEND/BASE_URL`, `QUARANTINE_DIR`, `QUARANTINE_SWEEP_DRY_RUN`, `RedisAddr/Password`, `CORS_ALLOWED_ORIGINS`, `SENTRY_DSN/ENV`, `MaxSSEConnections`, `CookieSameSite`, `TrustedProxies`, `PublicBaseURL`, `MailerWebhookURL/Key`
- Validation: JWT secret >= 32 chars, APP_ENV in {development,production}, STORAGE_BACKEND="only local", STORAGE_BASE_URL absolute http(s), quarantine dir outside ./uploads, cookie same-site strict/valid, public base URL not loopback in production, DB SSL mode require+ in production, pool bounds 1-100 positive durations.
- `Config.DSN()` builds `postgres://user:pass@host:port/dbname?sslmode=mode`

---

## 3. Database (Postgres + PostGIS)

**Extension:** `CREATE EXTENSION IF NOT EXISTS postgis;` (migration 0001)

**Tables (from migrations/):**

| Migration | Table | Key Columns |
|---|---|---|
| 0002 | `users` | `id UUID PK`, `email UNIQUE`, `password_hash NOT NULL`, `username UNIQUE`, `avatar_url`, `socials JSONB`, `role TEXT CHECK(user/admin/owner) DEFAULT 'user'`, `created_at`, `updated_at` |
| 0003 | `categories` | `id SERIAL PK`, `name TEXT UNIQUE NOT NULL`, `slug TEXT UNIQUE` |
| 0003 | `pins` | `id UUID PK DEFAULT gen_random_uuid()`, `user_id UUID FK`, `location GEOGRAPHY(POINT,4326) NOT NULL`, `geohash TEXT NOT NULL`, `caption TEXT`, `category_id INT FK`, `is_hidden BOOLEAN DEFAULT false`, `created_at` |
| 0003 | `pin_photos` | `id UUID PK`, `pin_id UUID FK CASCADE`, `photo_url TEXT NOT NULL`, `thumbnail_url TEXT`, `position SMALLINT`, `created_at` |
| 0004 | `reports` | `id UUID PK`, `pin_id UUID FK CASCADE`, `reporter_id UUID FK`, `reason TEXT`, `status CHECK(pending/reviewed/actioned) DEFAULT pending`, `resolved_by`, `resolved_at`, `created_at`; UNIQUE(pin_id, reporter_id) |
| 0005 | `streams` | vendor media table — `livekit_room_name` dropped by 0017 |
| 0006 | `pin_photos.thumbnail_url` | added column, backfilled, SET NOT NULL in 0015 |
| 0007 | `favorites` | PK(user_id, pin_id), index(user_id, created_at DESC) |
| 0008 | `pins.updated_at` | trigger `set_pins_updated_at` (guards against view increments) |
| 0009 | `comments` | PK(id), `pin_id FK CASCADE`, `user_id FK`, `body TEXT`, `is_hidden DEFAULT false`, `created_at`; index(pin_id, created_at ASC) |
| 0010 | `follows` | PK(follower_id, followee_id), indexes(follower_id, created_at DESC),(followee_id, created_at DESC) |
| 0011 | `collections` + `collection_pins` | collections.has `is_private DEFAULT false` (0020); collection_pins has position |
| 0012 | `pins.views` | BIGINT DEFAULT 0 |
| 0013 | `categories.slug` | added + unique constraint |
| 0014 | pin indexes | `CONCURRENTLY pins_created_at_idx`; `CONCURRENTLY pins_visible_created_idx (created_at DESC) WHERE is_hidden = false` (partial, used by ListPins/ListTrending) |
| 0015 | `pin_photos.thumbnail_url SET NOT NULL` | default = photo_url |
| 0016 | `pins_updated_at` trigger | BEFORE UPDATE, guards: IF NEW.views DISTINCT FROM OLD.views THEN RETURN NEW; END IF; NEW.updated_at = now() |
| 0017 | `streams DROP COLUMN livekit_room_name` + constraint | vendor-specific, removed |
| 0018 | `users.sessions_valid_after` | TIMESTAMPTZ NULLable; NULL = never revoked; password reset sets to now() |
| 0019 | `password_resets` | token_hash TEXT UNIQUE, expires_at NOT NULL, used_at, index(user_id, created_at DESC) |
| 0020 | `collections.is_private` | BOOLEAN DEFAULT false |

**Indexes relevant to pins:**
- `pins_location_idx` — GIST on location (spatial queries)
- `pins_geohash_idx` — geohash text index
- `pins_category_idx` — category_id
- `pins_created_at_idx` — overall created_at DESC
- `pins_visible_created_idx` — partial on is_hidden=false, created_at DESC (key for ListPins/ListTrending)
- `pin_photos_pin_id_idx`, `pin_photos_pin_id_position_idx`
- `password_resets_user_idx`

**PostGIS usage:** YES — pins.location stored as `GEOGRAPHY(POINT,4326)`. Queries use `ST_SetSRID(ST_MakePoint(lng, lat), 4326)::geography`, `ST_MakeEnvelope(minLng, minLat, maxLng, maxLat, 4326)`, `ST_Intersects`.

**Migration runner:** `database.RunMigrations(pool, database.MigrationsDir)` in `cmd/api/main.go:37`. Tests use `go test ./internal/platform/database/ -run TestMigrationUpDownUp`.

---

## 4. Pins and the Map

**Map queries pins for viewport:**
- API: `GET /api/pins?bbox=minLat,minLng,maxLat,maxLng&category=3`
- Handler parses `c.Query("bbox")` → `httpx.ParseBbox` → args `[minLat, minLng, maxLat, maxLng]`.
- SQL args reordered to `[minLng, minLat, maxLng, maxLat]` for `ST_MakeEnvelope`.
- SQL: `ST_Intersects(p.location, ST_MakeEnvelope($1, $2, $3, $4, 4326))`
- No zoom parameter sent to API; bbox derived from map viewport bounds.

**Pin creation** (`handler.go:321-330`):
- `location = ST_SetSRID(ST_MakePoint(lng, lat), 4326)::geography`
- `geohash = geohash.Encode(lat, lng)`
- Photos processed, saved, then pin + photo rows inserted in transaction.

**Map library:** MapLibre-GL v6.
**Tile source:** MapTiler hosted styles: `https://api.maptiler.com/maps/streets-v2/style.json?key=${KEY}` (light/dark).
**Pin icons:** SVG-based, custom icons per pin layer (`pins-base`, `pins-selected`, `pins-cluster`).
**Clustering:** Enabled, `clusterMaxZoom = 8` (from `pinLayers.ts:30`). Stops clustering past zoom 8.
**Min/max zoom:** map init `minZoom: 5, maxZoom: 21`; `PIN_FIT_MAX_ZOOM = 15`.

**SSE /events** (from `realtime/handler.go`):
- `GET /events?bbox=&category=` — streams `event: pin` messages.
- Heartbeat: `": heartbeat\n\n"` every 20 seconds (`sseHeartbeatInterval`).
- Connection cap: max 1000 concurrent (`MAX_SSE_CONNECTIONS` config).
- Filters: optional `bbox` and `category` query params; `matches()` checks `lat/lng` within bbox and category match.
- Also subscribes to `goodspot:pin-removed` Redis channel for `event: pin_removed`.
- `setWriteDeadline` per-write; no global WriteTimeout; peer that stops reading gets goroutine released.

**Pin visibility / hidden pins:**
- `is_hidden = false` partial index `pins_visible_created_idx` used by ListPins/ListTrending.
- Hidden pins 404 in `GetPin`/`DeletePin`/`UpdatePin` (checked via `PinVisible` / `VisiblePinExists`).
- Ownership/or-moderator checks enforced in repository UPDATE predicates (e.g. `WHERE id = $1 AND is_hidden = false AND (user_id = $2 OR $3)`).

---

## 5. Users and Auth

**User model** (`server/internal/modules/user/model.go`):
- `id`, `email`, `password_hash`, `username`, `avatar_url *string`, `socials map[string]any`, `role TEXT` (user/admin/owner), `sessions_valid_after *time.Time`, `created_at`, `updated_at`.

**Login/sessions:**
- Primary: httpOnly cookie `session_token`.
- Fallback: `Authorization: Bearer <jwt>` header.
- Redis blacklist stores JTI per token for revocation.
- `sessions_valid_after` (migration 0018): JWT issued before this timestamp → 401 "session was reset". Password reset sets this to `now()`, killing all existing sessions.
- `SessionChecker` interface: `CheckSession(ctx, userID) → SessionState` (validAfter + role).

**Roles/flags:**
- `RoleUser = "user"`, `RoleAdmin = "admin"`, `RoleOwner = "owner"`.
- Moderation: `users.IsModerator(h.roles, c)` checks live role from DB (not from JWT, so demotions take effect immediately).
- `applyRole(c, role)` caches role in `CtxRole` for helpers.

**Admin tooling:**
- `PATCH /api/users/me/role` — admin-only role change (handler in `user/routes.go`).
- No other admin endpoints visible in codebase scan.

---

## 6. Realtime and Media

**SSE /events:**
- Already covered in section 4. Heartbeat every 20s. Connection limiter 1000 max.
- No explicit "heartbeat" config beyond the 20s ticker; the `": heartbeat\n\n"` comment keeps the connection alive for proxies/Nginx.

**Uploads & quarantine sweep:**
- Pin photos: multipart form, max 10MB per file (`maxPhotoSize = 10 << 20`).
- Imaging: validate → process → webp encode → `storage.Save()` → returns `{full, thumb}` URLs like `{STORAGE_BASE_URL}/uploads/{id}.webp`.
- Hidden pins: `Quarantine()` moves files from `./uploads` to `./quarantine` (outside static root). Idempotent; cross-filesystem falls back to copy+rename.
- Boot: `VerifyQuarantineWritable()` probes quarantine dir writability; fails startup if broken.
- Sweep dry-run: `QUARANTINE_SWEEP_DRY_RUN` flag logs counts without moving files.
- `CleanStaleTemps()` removes `.qtmp-*` temp files from quarantine.

**No video/media streaming** currently beyond the abandoned `streams` table (livekit_room_name dropped in 0017). No video endpoints or handlers found.

---

## 7. Client

**Pages and routes** (Next.js app router):
- `layout.tsx`, `page.tsx`, `loading.tsx`, `error.tsx`, `not-found.tsx`.
- Feature routes: `login/`, `register/`, `users/`, `pin/`, `reports/`, `forgot-password/`, `reset-password/`, `roles/`, `legal/`.
- Map page: `src/app/map/page.tsx` (or via navigation).

**Main components:**
- `MapView.tsx` — MapLibre-GL map, pin layers, hover tooltip, cluster, bounds → bbox state, pin list sidebar.
- `MapApp.tsx` — top-level map container, pin selection, detail panel, trending, categories, create pin, saved pins, location search.
- `PinList.tsx` — sr-only list of pins for screen readers.
- `CreatePinButton` — opens create pin modal/form.
- `PinDetailPanel` — detail view for selected pin.
- `TrendingList` — trending pins panel.
- `LocateButton` — centers map on user geolocation.

**State & data fetching:**
- `usePins(bbox, category)` — fetches pins via `fetchPins API`, abortable, debounced on bbox change.
- `usePinStream(bbox, category, onPin, onPinRemoved, onReconnect)` — opens SSE EventSource to `/events?bbox&category`, enriches `PinEvent → PinListEntry`, handles reconnect catch-up.
- `useTrending(bbox)` — fetches trending pins.
- `usePinDetail(id)` — fetches single pin.
- `useCategories()` — fetches categories.
- `useAuth()` — auth state (user, token, login/logout).

**Client → Server reachability** (next.config.ts rewrites):
- `/api/:path*` → `${API_BASE_URL}/:path*` (same-origin, cookie passes through).
- `/events` → `${API_BASE_URL}/events` (SSE, cookie sent for auth).
- `/uploads/:path*` → `${API_BASE_URL}/uploads/:path*` (media serving).

---

## 8. Tests and CI

**Go tests:**
- `go test ./... -race -count=1` — full test suite.
- `go test ./internal/platform/database/ -run TestMigrationUpDownUp -count=1 -v` — migration up/down/up against PostGIS container.
- `go test ./internal/endpointtest/ -run 'TestDBDeletePin|TestDBPasswordReset|...' -race -count=1 -v` — endpoint tests against separate `endpoint_test` DB.
- `go vet ./...` — static analysis.

Client tests:
- `pnpm lint`, `pnpm test`, `pnpm test:typegen`, `pnpm build`.
- `next typegen` generates route types; `.next/types/` gitignored, so typegen must run on cold .next (CI).

**CI** (`.github/workflows/ci.yml`):
- **Client job:** checkout → pnpm setup → node 22 → `pnpm install --frozen-lockfile` → lint → `next typegen` → `tsc --noEmit` → `pnpm test` → `pnpm build`.
- **Server job:** checkout → Go 1.27 → cache → `go mod download` → `apt-get install libvips-dev` (cgo) → `go build ./...` → `go test ./... -race -count=1` → migration tests → endpoint tests → `go vet ./...`.
- DB containers: separate PostGIS container for migration tests; disposable `endpoint_test` DB for endpoint tests.

---

## 9. Infra

**docker-compose.yml (dev):**
| Service | Image | Ports | Volumes |
|---|---|---|---|
| postgres | postgis:16-3.4 | 127.0.0.1:5432:5432 | postgres_data |
| redis | redis:7-alpine | 127.0.0.1:6379:6379 | (requirepass from env) |
| server | sweetspot-server-dev | 0.0.0.0:8081:8081 | ./server:/app, server_uploads:/app/uploads |
| client | sweetspot-client-dev | 0.0.0.0:3000:3000 | ./client:/app, /app/node_modules, /app/.next |
| nginx | sweetspot-nginx-dev | 8080:80, 8443:443 | ./nginx/dev.conf.template, nginx_certs_dev |

**docker-compose.prod.yml:**
| Service | Ports | Volumes | Notes |
|---|---|---|---|
| proxy | 80/443 | nginx_certs, acme_webroot | TLS terminate, certbot |
| certbot | — | certbot_data, nginx_certs | ACME challenges |
| client | — | — | HOSTNAME=0.0.0.0, internal network |
| server | — (internal) | server_uploads, quarantine_data | port 8081, no published ports, TRUSTED_PROXIES=10.89.0.0/24 |
| postgres | — (internal) | postgres_data | SSL on, cert/key secrets |
| redis | — (internal) | — | requirepass, no volume |

**Network:** `app_net` bridge 10.89.0.0/24 (prod); compose default dev network.

**Nginx + certbot:** Dev: self-signed cert mints on first boot via `30-bootstrap-certs.sh`. Prod: certbot with acme_webroot, shares PID namespace with proxy for SIGHUP reload.

---

## 10. Risks

| Severity | Issue | File/Path |
|---|---|---|
| Medium | No index on `users.email` or `users.username` beyond UNIQUE constraint; UNIQUE enforces index but no additional performance indexes. | 0002_users.sql |
| Low | `TRUSTED_PROXIES` empty by default; per-IP rate limits shared per proxy behind Next rewrite (which does not forward X-Forwarded-For). | config.go:72-74 |
| Low | `absentAccountHash` hardcoded in auth/service.go:23 — if leaked, enables offline account enumeration attacks (but mitigated by constant-time compare). | auth/service.go:23 |
| Medium | JWT blacklist lives only in Redis process memory; across restarts or multi-instance deployments, revocation is not shared unless Redis is shared. | di/container.go:46 |
| Medium | No rate limit on `/api/auth/register` or `/api/auth/login` beyond per-IP/global middleware; lockout uses `loginLockKey(ip|identifier)` but no global user lockout. | auth/service.go:81-85 |
| Low | `streams` table vendor-specific (livekit) — column dropped in 0017 but table still exists with vendor-neutral columns. | migrations/0005_streams.sql, 0017_streams_drop_room_name.sql |
| Medium | Quarantine sweep runs as a service/container not code-reviewed in this scan; VerifyQuarantineWritable at boot is the primary safety net. | server/internal/platform/storage/quarantine.go |
| Low | Hardcoded `maxPhotoSize = 10 << 20` (10MB) in pins/handler.go:29; no config-driven override. | pins/handler.go:29 |
| UNSURE | No explicit premium/plan flag in DB or code; `premium_until` or `plan` column not found. | NOT FOUND |
| UNSURE | Zoom-based visibility rule (pins farther away not shown at high zoom) — not implemented. | NOT FOUND |
| Low | No `Content-Security-Policy` header beyond `SecurityHeaders` middleware; check if CSP is set elsewhere. | NOT FOUND |

---

## Premium Flag Placement

Where a premium flag (`plan`, `premium_until`) would plug in, and which files a zoom-based visibility rule would touch:

**Premium flag — likely additions:**
- Add `plan TEXT` and/or `premium_until TIMESTAMPTZ` to `users` table (migration similar to 0018 `sessions_valid_after`).
- Backfill in `users` repository or migration.
- Check `plan` or `premium_until` in auth/authorization: e.g., `PATCH /api/users/me/role`, pin creation restrictions, trending visibility.
- Files to touch: `server/internal/migrations/0022_users_plan.sql` (new migration), `server/internal/modules/user/repository.go`, `server/internal/modules/auth/service.go` (role check), `server/internal/config/config.go` (maybe not needed if stored in DB).

**Zoom-based visibility rule — would touch:**
- `server/internal/modules/pins/repository.go` — `ListPins` and `ListTrending` SQL currently uses `ST_Intersects` with no zoom awareness. Add condition like `ST_DWithin(p.location, ST_MakeEnvelope(...), zoom_dependent_distance())` or filter by geohash precision.
- `server/internal/http/params/bbox.go` — if zoom param added, parse and adjust bbox or add WHERE clause.
- `client/src/components/map/MapView.tsx` — already has `PIN_FIT_MAX_ZOOM = 15` and `fitPinNeighborhood`; zoom-aware source data filtering could be added in the `usePins` hook or API layer.
- `client/src/lib/api/pins.ts` — `fetchPins` currently sends bbox only; add `zoom` param if API supports it.

---

## Livestream / Media Plug-in Points

**Where livestream would plug in:**
- The abandoned `streams` table (0005) has `status`, `peak_viewer_count`, `started_at`, `ended_at` columns but no Go code reads/writes it. No stream routes registered in router.go.
- `internal/modules/streams/` exists only as an empty package (`package streams`).
- Likely plug-in points:
  - **DB**: Add livestream table or extend `streams` with new columns (endpoints, vendor-agnostic).
  - **Routes**: Register new endpoints under `jsonRoutes` or `uploadRoutes` in `router.go` (e.g., `POST /api/streams`, `GET /api/streams/:id`).
  - **Auth**: Protect stream endpoints with `AuthRequired` / `OptionalAuth` like other routes.
  - **Realtime**: SSE could broadcast `stream_started`/`stream_ended` events on a new Redis channel.
  - **Client**: Add stream/player component (similar to how `MapView` uses MapLibre); `usePinStream` pattern could be reused.

**What already exists for video/media:**
- `streams` table with `status CHECK(live/ended)`, `peak_viewer_count INT DEFAULT 0`, `started_at`, `ended_at`.
- Migration 0017 dropped `livekit_room_name` UNIQUE constraint/column (vendor-specific).
- No Go handlers, no API routes, no client components for video playback.
- The `realtime` module already has SSE infrastructure (`/events`, broker, heartbeat, connection limiter) that could carry stream events if needed.
- No upload/video processing pipeline beyond pin photos (imaging library used for photo re-encoding to webp).

---

## NOT FOUND

- `plan` or `premium_until` column in users table or any code reference — not present in current schema or code.
- `zoom` parameter sent to pins API — not currently supported; bbox is the only viewport query.
- `premium` or `visibility` filter in any SQL query or handler logic.
- `livekit_room_name` or any video/media endpoint handlers beyond the abandoned `streams` table.
- `Content-Security-Policy` header in middleware or nginx config (only security headers: HSTS, X-Frame-Options, etc.).
- Any `premium_until` or `plan` field in user DTOs, models, or API responses.
- Admin tooling beyond `PATCH /api/users/me/role` — no other admin endpoints found.

## UNSURE

- Whether the `absentAccountHash` constant in `auth/service.go` is still the correct bcrypt hash of a non-existent account; it was generated once and may need regeneration if the password hashing cost parameter changed.
- Exact bootstrap cert generation procedure in `nginx/docker-entrypoint.d/30-bootstrap-certs.sh` — not read in this scan.
- Whether `QUARANTINE_SWEEP_DRY_RUN` is ever set to `true` in production or only for dev/testing.
- If there are any TODO/FIXME comments in the codebase that were not captured during the scan; a thorough grep for `TODO` or `FIXME` was not performed.
- Exact interaction between `TRUSTED_PROXIES` and Next.js rewrite: whether X-Forwarded-For should be forwarded for per-IP rate limits to work correctly across proxies.
- Whether the `sessions_valid_after` floor of ~1 second window (comparison at second precision) is intentional or a potential edge-case bug.
- Exact backup/recovery procedure for postgres_data volume across dev/prod environments.
- Whether the `streams` table data needs to be cleaned up or if the dropped column left residuals in some databases.
