# GoodSpot247 — Detailed Development Checklist

Granular, task-level checklist version of the Roadmap. Each phase is broken into concrete steps you can check off one at a time. Use this as your actual working checklist; use the Roadmap doc for the higher-level view.

---

## Phase 0 — Project Setup

### 0.1 Environment

- [x] Install Go (`go version` confirms it works)
- [x] Install Docker Desktop
- [x] Install Postman (or equivalent)
- [x] Install a Postgres client (TablePlus, DBeaver, or `psql`)

### 0.2 Project scaffolding

- [x] `mkdir goodspot247-backend && cd goodspot247-backend`
- [x] `go mod init github.com/jojianya/sweetspot247-backend`
- [x] Create folder structure — **actual layout used:** `cmd/api/`, `internal/app/`, `internal/config/`, `internal/di/`, `internal/http/` (+ `middleware/`, `response/`), `internal/modules/{auth,pins,reports,user,realtime,streams}/`, `internal/observability/`, `internal/platform/{database,cache,storage}/`, `pkg/{jwt,password,geohash,validid}/` (the originally planned `config/`, `db/`, `storage/`, `pkg/ratelimit/` top-level folders were consolidated into this structure during the modular refactor)
- [x] `go get github.com/gin-gonic/gin`
- [x] `go get github.com/joho/godotenv`
- [x] Create `.env` file (add to `.gitignore` immediately)
- [x] Create `.env.example` (committed, no real values) listing required vars

### 0.3 Docker Compose for local services

- [x] Write `docker-compose.yml` with:
  - [x] `postgres` service (use `postgis/postgis` image, not plain postgres)
  - [x] `redis` service
- [x] `docker-compose up -d`
- [x] Confirm Postgres is reachable (`psql` or DB client connects)
- [x] Confirm Redis is reachable (`redis-cli ping` → `PONG`)

### 0.4 Base server

- [x] Write minimal `main.go` with Gin, one `GET /health` route
- [x] `go run main.go` → confirm it starts
- [x] Add DB connection (`db/postgres.go`) using `pgx` or `sqlx`
- [x] Update `/health` to also ping the DB, return DB status in response
- [x] `go get github.com/jackc/pgx/v5` (or `sqlx` equivalent)
- [x] Add CORS middleware now (even permissive for local dev) — configured for localhost:3000/3001; lockdown for production origin is a Phase 7.3 item
- [x] House rule for the whole backend: always use parameterized queries (`$1`, `$2`...) via `pgx`/`sqlx` — never build SQL with string concatenation or `fmt.Sprintf`, even for "safe" internal values

### 0.5 Git & repo hygiene

- [x] `git init`, first commit
- [x] `.gitignore`: `.env`, `/uploads`, compiled binaries
- [x] Push to GitHub/GitLab (private repo)

**Phase 0 done when:** `go run main.go`, hit `GET /health` in Postman, get back `{"status":"ok","db":"connected"}`.

---

## Phase 1 — Auth & Roles

### 1.1 Database

- [x] Enable `postgis` extension (`CREATE EXTENSION IF NOT EXISTS postgis;`) — migration `0001_init_extensions.sql`
- [x] Write migration `0002_users.sql` for `users` table: `id`, `email`, `password_hash`, `username`, `avatar_url`, `socials` (`jsonb`), `role` (`text`, `CHECK IN ('user','admin','owner')`, default `'user'`), `created_at`, `updated_at`
- [x] Add `set_updated_at()` trigger function + `BEFORE UPDATE` trigger on `users` so `updated_at` actually refreshes on edits
- [x] Run migration against local DB
- [x] Write `internal/users/model.go` struct matching the table

### 1.2 Password handling

- [x] `go get golang.org/x/crypto/bcrypt`
- [x] Write hash + compare helper functions (`pkg/password/password.go`)

### 1.3 JWT

- [x] `go get github.com/golang-jwt/jwt/v5`
- [x] Add `JWT_SECRET` to `.env`
- [x] Write JWT generation function (`pkg/jwt/jwt.go`) — include user ID + expiry claim
- [x] Write JWT validation function
- [x] Decide now (not after the frontend is built): short-lived access token + refresh token, or a single longer-lived token for MVP. Retrofitting this later means reworking the frontend auth store too, so pick deliberately even if the answer is "long-lived token for now, revisit post-MVP" — **Decision: single 30-day JWT access token for MVP (`tokenExpiry = 30 * 24h`); tenably upgraded to access + refresh token post-MVP**

### 1.4 Register endpoint

- [x] `internal/modules/auth/handler.go` -> `Register` handler (module files consolidated into `handler.go`/`service.go`/`routes.go` during the modular refactor — `register.go` was the pre-refactor file name)
- [x] Request struct (`internal/auth/dto.go`) with `binding` validation tags (email format, password min length, username min/max length)
- [x] Check email uniqueness before insert (lowercase the email first — Postgres `UNIQUE` is case-sensitive, so `Dave@x.com`/`dave@x.com` won't collide otherwise)
- [x] Check username uniqueness before insert
- [x] Hash password before storing
- [x] New users default to `role = 'user'` — never accept `role` from the request body
- [x] Return user (without password hash) + JWT on success
- [x] Route: `POST /auth/register`
- [x] Wire up basic rate limiting on this route now using the in-memory rate limiter (`internal/http/middleware/rate_limit.go`) — per-IP + global caps, cheaper to add alongside the handler than to retrofit once traffic exists; Phase 7.3 just re-verifies it's in place before launch
- [x] Test in Postman: valid registration -> 201 + token
- [x] Test in Postman: duplicate email (including different casing) -> proper error, not a crash
- [x] Test in Postman: invalid email format -> 400 with validation message
- [x] Test in Postman: rate limit trips after N rapid attempts -> 429

### 1.5 Login endpoint

- [x] `internal/modules/auth/handler.go` -> `Login` handler
- [x] Look up user by (lowercased) email, compare password hash
- [x] Return JWT on success, generic error on failure (don't leak "email not found" vs "wrong password" — same message for both, security best practice)
- [x] Route: `POST /auth/login`
- [x] Wire up rate limiting on this route (same in-memory limiter, but stricter than register — per-IP _and_ per-email lockout, since this is the actual brute-force target)
- [x] Test in Postman: correct credentials -> 200 + token
- [x] Test in Postman: wrong password -> 401, generic message
- [x] Test in Postman: rate limit trips after N rapid failed attempts -> 429

### 1.6 Auth middleware

- [x] `internal/http/middleware/auth.go` — reads `Authorization: Bearer <token>` header, validates JWT, sets user ID + role in context; **role-gated `RequireAdmin`/`RequireOwner` live in `internal/modules/user/routes.go`** and re-validate the role against the DB on every request
- [x] Write a temporary `GET /me` protected test route that returns the authenticated user's ID + role from context
- [x] Test in Postman: `/me` with valid token -> 200
- [x] Test in Postman: `/me` with no token -> 401
- [x] Test in Postman: `/me` with garbage/expired token -> 401

### 1.7 Public user profile

- [x] `internal/modules/user/handler.go` -> `GetUser` handler, `internal/modules/user/dto.go` -> `PublicUser`/`PrivateUser` (strips password hash; email only visible to the account owner)
- [x] Route: `GET /users/:id` (public, no auth middleware)
- [x] Test in Postman: returns public profile fields only, never password_hash

### 1.8 Roles & permission middleware

- [x] `internal/modules/user/routes.go` -> `RequireAdmin` (passes for `role = 'admin'` or `role = 'owner'`)
- [x] `internal/modules/user/routes.go` -> `RequireOwner` (passes only for `role = 'owner'`)
- [x] Bootstrap the very first owner directly in the database. **Policy (revised):** `owner` is now also assignable via the API by an existing owner (self-demotion is blocked with `cannot change your own role`), so a second owner can be created in-app instead of only via SQL: `UPDATE users SET role = 'owner' WHERE email = '<you>';`
- [x] `internal/modules/user/handler.go` -> `UpdateRole` handler — body: `{ "role": "user" | "admin" | "owner" }` (owner only assignable by an owner; an owner may not change their own role)
- [x] Route: `PATCH /users/:id/role` — protected by `RequireOwner`
- [x] Decide the last-owner policy before shipping — don't leave this open: either (a) `UpdateRole`/demotion logic rejects any change that would leave zero owners, or (b) explicitly accept the risk and document a manual DB-recovery path (who has prod DB access, how a new owner gets bootstrapped if the only owner account is lost/compromised). A single point of failure with no recovery plan is a launch risk, not a nice-to-have — **Chosen: option (a).** `UpdateRole` rejects demotions of the last owner (`cannot demote the last owner`) **and** blocks owners from changing their own role (`cannot change your own role`), so the sole-owner account can never be demoted or self-demoted. A second owner can be created via the API (owner → owner) or by direct DB bootstrap.
- [x] Test in Postman: owner promotes a user to admin -> 200, role updated
- [x] Test in Postman: non-owner attempts the same -> 403
- [x] Test in Postman: invalid role value -> 400

**Phase 1 done when:** Full register -> login -> access protected route flow works end-to-end in Postman, invalid/missing token cases are properly rejected, and the owner can promote a user to admin via the API.

---

## Phase 2 — Pins, Categories & Multi-Photo

### 2.1 Categories + Pins + Pin Photos migration

- [x] Write migration `0003_pins.sql` — `categories` table created first, then `pins` (referencing `categories`, `category_id` **NOT NULL**, `ON DELETE RESTRICT`), then `pin_photos` (single migration file, since categories and pin_photos only exist to support pins)
- [x] `pins` gets an `is_hidden BOOLEAN NOT NULL DEFAULT false` column (used by moderation in Phase 2.5)
- [x] `pin_photos`: `id`, `pin_id` (`ON DELETE CASCADE`), `photo_url`, `position` (`SMALLINT`, default 0), `created_at`
- [x] Seed script inserting fixed category list (Food, Nature, Event, Nightlife, etc. — finalize the actual list)
- [x] `GIST` index on `pins.location`
- [x] Index on `pins.geohash`
- [x] Index on `pins.category_id`
- [x] Index on `pin_photos.pin_id`
- [x] Unique index on `pin_photos (pin_id, position)`
- [x] `internal/modules/pins/model.go` — Pin + Category + PinPhoto structs
- [x] Pick + install a geohash library (`github.com/mmcloughlin/geohash`, wrapped in `pkg/geohash/`)
- [x] Migration `0006_pin_photo_thumbnails.sql` adds `pin_photos.thumbnail_url` (400px square thumbnail) and backfills from `photo_url`

### 2.2 Categories endpoint

- [x] `internal/modules/pins/handler.go` -> `ListCategories` handler (categories are read alongside pins in this module, no separate feature folder)
- [x] Route: `GET /categories` (public, no auth middleware)
- [x] Test in Postman: returns seeded list

### 2.3 Create pin endpoint

- [x] `internal/modules/pins/handler.go` -> `CreatePin` handler
- [x] Request struct (`internal/modules/pins/dto.go`): lat, lng, caption (optional), category_id (required) — `CreatePinRequest` JSON DTO is now dead code since the flow moved to multipart in Phase 3
- [x] Decide + validate a max photo count per pin (e.g. 5)
- [x] Validate lat/lng ranges, category_id exists
- [x] Compute geohash from lat/lng, store alongside `GEOGRAPHY(POINT)`
- [x] Insert the `pins` row and all `pin_photos` rows (with `position` = array index) in **one DB transaction**, so a pin can never end up with zero photos or orphaned photo rows
- [x] Apply auth middleware to this route
- [x] Route: `POST /pins`
- [x] Wire up basic rate limiting on this route now (same in-memory limiter as register) — prevents pin-spam abuse before it's a problem, not after
- [x] Test in Postman: valid pin with token, multiple photo URLs -> 201, all photos attached
- [x] Test in Postman: no token -> 401
- [x] Test in Postman: invalid category_id -> 400
- [x] Test in Postman: lat/lng out of range -> 400
- [x] Test in Postman: photo count over the max -> 400
- [x] Test in Postman: rate limit trips after N rapid submissions -> 429

### 2.4 Get pins (viewport query)

- [x] `internal/modules/pins/handler.go` -> `GetPins` handler
- [x] Parse `bbox` query param (4 floats)
- [x] PostGIS bounding-box query (`ST_MakeEnvelope` + `ST_Within`, or `&&` operator), filtered to `is_hidden = false`
- [x] Optional `category` query param -> add `WHERE category_id = ...`
- [x] Cap the result set (e.g. `LIMIT 200`, most recent first) — an unbounded bbox query will eventually return thousands of rows in a dense area; decide now whether to just cap+limit for MVP or add real clustering, but don't ship with no ceiling at all
- [x] No auth middleware (public)
- [x] Route: `GET /pins?bbox=...&category=...`
- [x] Test in Postman: no filters -> returns all visible pins in bbox
- [x] Test in Postman: with category filter -> returns filtered subset
- [x] Test in Postman: bbox with no pins -> returns empty array, not an error
- [x] Test in Postman: a hidden pin never appears in results
- [x] Test in Postman: bbox with more pins than the cap -> returns capped count, not everything

### 2.5 Get single pin

- [x] `internal/modules/pins/handler.go` -> `GetPin` handler — joins `pin_photos` (ordered by `position`; includes `thumbnail_url`) and `users` (for `username`/`avatar_url`)
- [x] Route: `GET /pins/:id` (public)
- [x] Test in Postman: valid ID -> pin details with an ordered photo array
- [x] Test in Postman: invalid/nonexistent ID -> 404

**Phase 2 done when:** Can create pins with multiple photos (authenticated) and query them by viewport + category (unauthenticated), fully tested in Postman, no real photo upload yet.

---

## Phase 2.5 — Reports & Moderation

### 2.5.1 Reports table

- [x] Write migration `0004_reports.sql`: `id`, `pin_id` (`ON DELETE CASCADE`), `reporter_id` (`ON DELETE SET NULL`), `reason`, `status` (`NOT NULL`, `CHECK IN ('pending','reviewed','actioned')`, default `'pending'`), `resolved_by` (`ON DELETE SET NULL` — which admin/owner actioned it), `resolved_at`, `created_at`
- [x] Unique constraint `UNIQUE (pin_id, reporter_id)` — one report per user per pin
- [x] `internal/modules/reports/model.go`

### 2.5.2 Create report endpoint

- [x] `internal/modules/reports/handler.go` -> `CreateReport` handler
- [x] Request struct (`internal/modules/reports/dto.go`): reason (required, ≥3 chars)
- [x] Apply auth middleware — reporting requires login
- [x] Route: `POST /pins/:id/report`
- [x] Test in Postman: valid report with token -> 201
- [x] Test in Postman: no token -> 401
- [x] Test in Postman: report on nonexistent pin -> 404
- [x] Test in Postman: same user reports the same pin twice -> rejected (unique constraint)

### 2.5.3 Report review endpoint (admin role)

- [x] `internal/modules/reports/handler.go` -> `ReviewReport` handler — body: `{ "action": "approve" | "dismiss" }`, uses `SELECT ... FOR UPDATE` row locking in the repository to prevent double-resolution races
- [x] **Approve** -> `pins.is_hidden = true`, `reports.status = 'actioned'`, set `resolved_by` (from JWT) + `resolved_at`
- [x] **Dismiss** -> `reports.status = 'reviewed'`, set `resolved_by` + `resolved_at`, pin untouched
- [x] Route: `PATCH /reports/:id` — protected by `RequireAdmin` (Phase 1.8)
- [x] Test in Postman: admin approves a report -> pin becomes hidden, disappears from `GET /pins`
- [x] Test in Postman: admin dismisses a report -> pin unaffected, report marked `reviewed`
- [x] Test in Postman: regular user attempts either action -> 403
- [x] (Fallback, keep for emergencies) Direct DB query still works if the API is ever down: `SELECT * FROM reports WHERE status = 'pending' ORDER BY created_at;`

**Phase 2.5 done when:** A logged-in user can report a pin via `POST /pins/:id/report`, and an admin can review + action it in-app via `PATCH /reports/:id`, hiding the pin on approval. Direct database review remains available as a fallback, but is no longer the primary path now that roles exist (see Phase 1.8 for how the first admin/owner gets set up).

---

## Phase 3 — Photo Upload (Multi-File)

### 3.1 Local storage handler

- [x] `storage/local.go` — save an uploaded file to `./uploads/`, return local path/URL — **actual:** `internal/platform/storage/local.go` (+ `fileid.go` for server-generated random hex IDs)
- [x] Add static file serving: `r.Static("/uploads", "./uploads")`
- [x] Add `./uploads` to `.gitignore`

### 3.2 Update create-pin flow for multiple files

- [x] Change `POST /pins` to accept `multipart/form-data` instead of pure JSON, with photos sent under a repeated field name (e.g. `photos`)
- [x] In Gin, read the file slice via `form.File["photos"]` instead of a single `c.FormFile(...)`
- [x] Validate the whole batch up front: each file's type (jpg/png only) and size (e.g. max 10MB), and total count against the max from Phase 2.3 — reject the entire request if anything in the batch fails
- [x] Check actual file content, not just the extension or declared MIME type — read the file's magic bytes (e.g. `http.DetectContentType` in Go) before trusting it's really an image; a renamed malicious file shouldn't pass just because it's called `photo.jpg`
- [x] Upload each file via the storage handler, collecting the resulting URLs in order
- [x] Insert the `pins` row and all `pin_photos` rows (URL + `position`) in one DB transaction (same transaction requirement as Phase 2.3, now with real files instead of placeholder URLs)
- [x] Test in Postman: form-data with 3 real images + fields -> pin created, all 3 photos saved locally with correct order
- [x] Test in Postman: one oversized file in the batch -> whole request rejected with clear error
- [x] Test in Postman: one wrong file type in the batch -> whole request rejected

### 3.3 Image processing (required, not optional)

This app is photo-heavy and map-based — unprocessed multi-photo uploads at up to 10MB each will hurt storage costs and load times fast once there's real traffic. Do this now, before Phase 4 puts real images in front of real users, not as a later optimization pass.

- [x] `go get github.com/h2non/bimg` — **chosen:** libvips via `bimg` (CGO binding; requires libvips installed), implemented in `internal/platform/imaging/imaging.go` (moved from `internal/modules/pins/imaging/` in B6)
- [x] Resize to a max dimension (1600px) and compress uploaded images to WebP q80 before saving
- [x] Generate a 400px square thumbnail alongside the full size for map/list views (stored in `pin_photos.thumbnail_url`, migration `0006`)
- [x] Reject images above 8000×8000 px (checked via metadata before decode) and cap concurrent libvips processes at 2
- [x] Test in Postman: uploaded image is resized/compressed on disk, not stored at original size

**Phase 3 done when:** Real photo files (one or more per pin) can be uploaded via Postman form-data, resized/compressed, saved locally, and retrieved via their stored URLs in the correct order.

---

## Phase 4 — Frontend Map View

### 4.1 Frontend scaffolding

- [x] `npx create-next-app@latest goodspot247-web` (TypeScript, App Router) — created as `client/` in this repo (Next.js 16, TypeScript, App Router)
- [x] Install: `maplibre-gl`, `axios`, `zustand`, `zod`
- [x] Set up `.env.local` with API base URL
- [x] House rule: never use `dangerouslySetInnerHTML` (or any raw-HTML render) on user-generated content — captions, usernames, `socials` JSON. React escapes by default; only breaks if you explicitly opt out of it, so just don't

### 4.2 Map screen

- [x] Basic MapLibre map component, centered on user's location (or default city) — **MapTiler "toner-lite" style (requires `NEXT_PUBLIC_MAPTILER_API_KEY`)**, centered via geolocation (default city Hyderabad 17.385, 78.4867 was left commented out)
- [x] Fetch pins on load via `GET /pins?bbox=...` (compute bbox from current map view) — bbox `minLat,minLng,maxLat,maxLng`
- [x] Render pins as markers — GeoJSON circle layer (clickable)
- [x] Re-fetch on `moveend`/`zoomend` map events — `moveend` only

### 4.3 Category filter UI

- [x] Fetch categories via `GET /categories` on load
- [x] Render filter chips — horizontal chip bar
- [x] Selecting a chip re-fetches pins with `&category=...`

### 4.4 Pin detail view

- [x] Tap/click a marker -> open detail view (modal or side panel) — right-side panel
- [x] Display photo carousel (multiple photos, ordered), caption, category, user, timestamp — prev/next buttons + thumbnail strip + `n/N` counter

### 4.5 Auth screens

- [x] Login page/form -> calls `POST /auth/login`, stores JWT (e.g. in memory + httpOnly cookie or secure storage) — JWT persisted via Zustand `persist` to localStorage (deliberate dev choice over httpOnly cookies: separate API origin, token sent via `Authorization: Bearer` header)
- [x] Register page/form -> calls `POST /auth/register`
- [x] Global auth state (Zustand store): current user, token, role, `isLoggedIn` — `role` lives on the stored `user`

### 4.6 Create-pin flow

- [x] "+" button on map — floating action button
- [x] If not logged in -> show login/register prompt instead of the form
- [x] If logged in -> open create-pin form (multi-photo picker, category select, caption) — JPG/PNG only, max 5 photos, 10MB each
- [x] Submit -> `POST /pins` with auth header, multipart form data (multiple files)
- [x] On success -> new pin appears on map immediately (optimistic update) — inserted locally, detail panel opens, map flies to it

### 4.7 (If applicable) Basic admin UI

- [ ] Simple "Reports" screen, visible only when `role` is `admin`/`owner` — lists pending reports, approve/dismiss buttons calling `PATCH /reports/:id`
- [ ] Simple "Manage roles" screen, visible only when `role` is `owner` — promote/demote a user via `PATCH /users/:id/role`

> **Skipped by owner decision** — backend endpoints (`GET /reports`, `PATCH /users/:id/role`) exist and are tested; the admin UI is intentionally deferred. Revisit if an admin console becomes a priority.

**Phase 4 done when:** A real person (not just you testing Postman) can open the app, browse the map as a guest, and — after logging in — successfully post a pin with multiple photos that shows up. This is your first genuinely demoable version.

---

## Phase 5 — Real-Time Layer

> **Status:** the plan was revised from WebSocket to Server-Sent Events. The live design is `GET /events` (one global stream per process, no rooms): `internal/modules/realtime/broker.go` publishes `pins.Event` JSON to Redis channel `goodspot:pins`; each client connection subscribes and `runStreamLoop` (in `realtime/handler.go`) filters server-side by bbox (`matches`) and category, writes `event: pin` messages, and emits a `: heartbeat` comment every 20 s. `MAX_SSE_CONNECTIONS` caps total connections per process; nginx disables buffering for `/events` so frames are not buffered.

### 5.1 WebSocket server setup — superseded by SSE

- [x] `go get github.com/gorilla/websocket` — superseded: no WebSocket library; SSE instead.
- [x] `internal/realtime/hub.go` — connection registry (`map[string][]*Connection` keyed by geohash cell) — superseded: single global broker per process, per-subscriber Redis subscription, no geohash cell rooms.
- [x] `internal/realtime/handler.go` — WebSocket upgrade handler, route: `GET /ws` — superseded: `Handler.Stream` at `GET /events`.
- [x] Handle client `subscribe`/`unsubscribe` messages (cell list) — superseded: client passes `bbox`/`category` as query params; `runStreamLoop`/`matches` decide which events reach that subscriber.

### 5.2 Redis pub/sub — done (SSE)

- [x] `go get github.com/redis/go-redis/v9`
- [x] `REDIS_URL` in `.env`
- [x] `internal/realtime/broker.go` — on pin creation, `Broker.PinCreated` publishes the event to Redis channel `goodspot:pins`.
- [x] Each `GET /events` connection subscribes and receives cross-instance events.
- [x] (Known gap, deferred) Pin write and Redis publish are two separate steps — a crash between them means a pin is saved but never broadcast. Documented, not fixed; a transactional outbox table + poller can close it if it becomes a real problem.

### 5.3 Batching — deferred

- [ ] Batching of bursts (one event per create today). Deferred: no burst requirement demonstrated; revisit only if a real storm appears.

### 5.4 Frontend WebSocket client — superseded by SSE

- [x] Native `WebSocket` connect on map load — superseded: `openPinStream` uses `EventSource` (`client/src/lib/api/realtime.ts`).
- [x] Compute visible geohash cells, send `subscribe` — superseded: client sends `bbox` + `category` query params (`realtime.ts`, `usePinStream`).
- [x] Update subscription on viewport change — `usePinStream` re-opens on `[bbox, category, onPin, onPinRemoved, onReconnect]` (`usePinStream`).
- [x] On `pin_batch` message, merge new pins without a full refetch — live: one `event: pin` per create; `usePinStream` enriches it and calls `onPin` (current behavior verified; payload schema is client-validated with zod).

### 5.5 Testing

- [ ] Open app in two browser tabs/windows
- [ ] Post a pin in tab A
- [ ] Confirm it appears in tab B (no refresh) — with matching category and live bbox overlap.
- [ ] Delete a pin in tab A; confirm it disappears from tab B and closes its open detail panel.

### 5.6 Removal events and reconnect catch-up

- [x] Separate Redis channel for removals: `goodspot:pin-removed` (never mixed into `goodspot:pins`).
- [x] Test proving a removal cannot produce `event: pin` (it must emit `event: pin_removed` only).
- [x] Publish `pin_removed` from the pin delete handler (`Handler.DeletePin`).
- [x] Publish `pin_removed` from report approve, with `reports` free of a `pins` import (publisher interface defined inside `reports`, broker implements it, wired in router/routes).
- [x] Handler tests for `pin_removed`.
- [x] DB-backed publish-after-commit tests: repository returns the pin location only after the commit (`TestDBReviewApproveReturnsLocationAfterCommit`, `TestDBReviewDismissReturnsNilLocation`, `TestDBReviewUpdateErrorSurfaces`: UPDATE-error fix); handler publishes nothing when the service errors (`TestReviewErrorPublishesNothing`). No test asserts the Redis publish itself fires after commit.
- [x] Client: map list removes the pin on `pin_removed`.
- [x] Client: detail panel closes when its open pin is removed.
- [x] Client: permalink page behavior (`/pin/[id]` does not use `usePinStream`; document/leave as-is).
- [x] Reconnect catch-up: `onerror` only marks the episode — no refetch while the stream is broken (SSE has no replay, so that refetch could be immediately stale). The first `onopen` after any error always refetches, even if time has passed; an episode yields exactly one refetch if a reconnect follows, zero otherwise.
- [x] The seven fake-`EventSource` test cases: (1) normal `pin_removed` delivered calls `onPinRemoved` with the pin id, (2) `pin_removed` never calls `onPin` (removals cannot produce pin events), (3) error alone never refetches (no timer; waits for open), (4) clean intentional re-subscription does not refetch (bbox change without error), (5) error then open refetches exactly once, (6) error, time passes, then open refetches on open, (7) repeated errors before one open refetch exactly once.
- [x] Arch test passes without an allowlist change (`reports` must not import `pins`).

> **Note:** Removals share the same known gap as pin creation — a crash between the DB commit and the Redis publish means the removal is persisted but never broadcast. Reconnect catch-up (refetch on the first `onopen` after any error — the only refetch; nothing fires while the stream is broken) is the partial mitigation, not a full outbox.

**Phase 5 done when:** Two-tab live-update test passes reliably.

---

## Phase 6 — Production Storage Swap

- [ ] Confirm libvips is installed in the production container/host before first deploy (bimg is a CGO binding, won't build/run without it)
- [ ] Create Cloudflare account + R2 bucket
- [ ] Generate R2 API credentials
- [ ] `go get github.com/aws/aws-sdk-go-v2/service/s3` (+ config packages)
- [ ] `storage/r2.go` implementing the same interface as `storage/local.go`
- [ ] Config flag/env var to switch storage backend (`STORAGE_DRIVER=local|r2`)
- [ ] Test upload against R2 in a staging environment (single and multi-photo pins)
- [ ] Set up custom domain or Cloudflare CDN in front of the bucket
- [ ] Confirm uploaded photos are publicly viewable via CDN URL

**Phase 6 done when:** Switching `STORAGE_DRIVER=r2` in env config results in uploads landing in R2 and being servable via CDN, with zero code changes needed elsewhere.

---

## Phase 7 — Deploy MVP

### 7.1 Infrastructure

- [ ] Choose + set up backend hosting (Railway/Render/Fly.io/VPS)
- [ ] Set up managed Postgres (Supabase/Neon/RDS) — or migrate local DB schema to it
- [ ] Set up managed Redis (Upstash/Redis Cloud)
- [ ] Configure all production env vars/secrets on the host
- [ ] Bootstrap the production owner directly in the managed DB (same manual step as Phase 1.8, done again in prod)

### 7.2 Frontend deploy

- [ ] Deploy Next.js app (Vercel is the natural choice)
- [ ] Point frontend's API base URL to production backend

### 7.3 Pre-launch checklist

- [ ] Privacy Policy + Terms of Service published and linked in-app
- [ ] Content moderation: `reports` feature (Phase 2.5) is live, at least one admin exists (promoted by the owner), and pending reports are being reviewed in-app via `PATCH /reports/:id` on a regular cadence — direct DB query kept only as a fallback
- [ ] Verify rate limiting on `POST /auth/register`, `POST /auth/login`, and `POST /pins` (built in Phases 1.4/1.5/2.3) is actually active in the production config, not just local
- [ ] HTTPS enforced end-to-end in production — no plain HTTP fallback for the API or frontend; most hosts (Railway/Render/Vercel) do this by default, but confirm rather than assume
- [ ] Confirm CORS is locked down to the actual production frontend origin, not the permissive local-dev setting from Phase 0
- [ ] Run a dependency vulnerability scan (`govulncheck ./...` for the backend, `npm audit` for the frontend) and address anything critical before launch; set up Dependabot (or equivalent) for ongoing scans
- [ ] Confirm `JWT_SECRET` is a strong, unique production value (not the dev one) and document the plan for what happens if it ever leaks (rotate secret -> all existing tokens invalidate -> users re-login; acceptable for MVP, just know it in advance)
- [ ] Basic uptime monitoring (even a free tool like UptimeRobot)
- [ ] Error tracking (Sentry free tier is enough to start)
- [ ] Test the full flow end-to-end in production: register -> browse -> post (multi-photo) -> see it live -> report -> admin actions it

**Phase 7 done when:** GoodSpot247 is live at a real public URL and you (and a few trusted testers) can use it fully.


## Quick Reference — Checklist Count by Phase

| Phase                             | Approx. checklist items |
| --------------------------------- | ----------------------- |
| 0. Setup                          | 15                      |
| 1. Auth & Roles                   | 30                      |
| 2. Pins, Categories & Multi-Photo | 26                      |
| 2.5 Reports & Moderation          | 14                      |
| 3. Photo Upload (Multi-File)      | 10                      |
| 4. Frontend Map View              | 23                      |
| 5. Real-Time Layer                | 16                      |
| 6. Storage Swap                   | 7                       |
| 7. Deploy                         | 13                      |

Work top to bottom, phase by phase. Don't skip ahead to Phase 5+ until Phase 4's exit criteria genuinely passes — that's the point where you have something real to show and test.
