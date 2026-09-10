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
- [x] `go mod init github.com/yourusername/goodspot247-backend`
- [x] Create folder structure: `config/`, `db/`, `pkg/jwt/`, `pkg/password/`, `pkg/geohash/`, `pkg/ratelimit/`, `storage/`, `internal/users/`, `internal/auth/`, `internal/pins/`, `internal/reports/`, `internal/realtime/`, `internal/streams/`
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

### 0.5 Git & repo hygiene

- [x] `git init`, first commit
- [x] `.gitignore`: `.env`, `/uploads`, compiled binaries
- [x] Push to GitHub/GitLab (private repo)

**Phase 0 done when:** `go run main.go`, hit `GET /health` in Postman, get back `{"status":"ok","db":"connected"}`.

---

## Phase 1 — Auth & Roles

### 1.1 Database

- [ ] Enable `postgis` extension (`CREATE EXTENSION IF NOT EXISTS postgis;`) — migration `0001_init_extensions.sql`
- [ ] Write migration `0002_users.sql` for `users` table: `id`, `email`, `password_hash`, `username`, `avatar_url`, `socials` (`jsonb`), `role` (`text`, `CHECK IN ('user','admin','owner')`, default `'user'`), `created_at`, `updated_at`
- [ ] Add `set_updated_at()` trigger function + `BEFORE UPDATE` trigger on `users` so `updated_at` actually refreshes on edits
- [ ] Run migration against local DB
- [ ] Write `internal/users/model.go` struct matching the table

### 1.2 Password handling

- [ ] `go get golang.org/x/crypto/bcrypt`
- [ ] Write hash + compare helper functions (`pkg/password/password.go`)

### 1.3 JWT

- [ ] `go get github.com/golang-jwt/jwt/v5`
- [ ] Add `JWT_SECRET` to `.env`
- [ ] Write JWT generation function (`pkg/jwt/jwt.go`) — include user ID + expiry claim
- [ ] Write JWT validation function

### 1.4 Register endpoint

- [ ] `internal/auth/register.go` -> `Register` handler
- [ ] Request struct (`internal/auth/dto.go`) with `binding` validation tags (email format, password min length, username min/max length)
- [ ] Check email uniqueness before insert (lowercase the email first — Postgres `UNIQUE` is case-sensitive, so `Dave@x.com`/`dave@x.com` won't collide otherwise)
- [ ] Check username uniqueness before insert
- [ ] Hash password before storing
- [ ] New users default to `role = 'user'` — never accept `role` from the request body
- [ ] Return user (without password hash) + JWT on success
- [ ] Route: `POST /auth/register`
- [ ] Test in Postman: valid registration -> 201 + token
- [ ] Test in Postman: duplicate email (including different casing) -> proper error, not a crash
- [ ] Test in Postman: invalid email format -> 400 with validation message

### 1.5 Login endpoint

- [ ] `internal/auth/login.go` -> `Login` handler
- [ ] Look up user by (lowercased) email, compare password hash
- [ ] Return JWT on success, generic error on failure (don't leak "email not found" vs "wrong password" — same message for both, security best practice)
- [ ] Route: `POST /auth/login`
- [ ] Test in Postman: correct credentials -> 200 + token
- [ ] Test in Postman: wrong password -> 401, generic message

### 1.6 Auth middleware

- [ ] `internal/auth/middleware.go` — reads `Authorization: Bearer <token>` header, validates JWT, sets user ID + role in context
- [ ] Write a temporary `GET /me` protected test route that returns the authenticated user's ID + role from context
- [ ] Test in Postman: `/me` with valid token -> 200
- [ ] Test in Postman: `/me` with no token -> 401
- [ ] Test in Postman: `/me` with garbage/expired token -> 401

### 1.7 Public user profile

- [ ] `internal/users/get_user.go` -> `GetUser` handler, `internal/users/dto.go` -> `PublicUser` (strips password hash)
- [ ] Route: `GET /users/:id` (public, no auth middleware)
- [ ] Test in Postman: returns public profile fields only, never password_hash

### 1.8 Roles & permission middleware

- [ ] `internal/auth/middleware.go` -> `RequireAdmin` (passes for `role = 'admin'` or `role = 'owner'`)
- [ ] `internal/auth/middleware.go` -> `RequireOwner` (passes only for `role = 'owner'`)
- [ ] Bootstrap the very first owner directly in the database — no endpoint should ever be able to grant `role = 'owner'`: `UPDATE users SET role = 'owner' WHERE email = '<you>';`
- [ ] `internal/users/update_role.go` -> `UpdateRole` handler — body: `{ "role": "admin" | "user" }`
- [ ] Route: `PATCH /users/:id/role` — protected by `RequireOwner`
- [ ] Guard against removing the last owner (or decide this is out of scope for MVP and note it as a known risk)
- [ ] Test in Postman: owner promotes a user to admin -> 200, role updated
- [ ] Test in Postman: non-owner attempts the same -> 403
- [ ] Test in Postman: invalid role value -> 400

**Phase 1 done when:** Full register -> login -> access protected route flow works end-to-end in Postman, invalid/missing token cases are properly rejected, and the owner can promote a user to admin via the API.

---

## Phase 2 — Pins, Categories & Multi-Photo

### 2.1 Categories + Pins + Pin Photos migration

- [ ] Write migration `0003_pins.sql` — `categories` table created first, then `pins` (referencing `categories`, `category_id` **NOT NULL**, `ON DELETE RESTRICT`), then `pin_photos` (single migration file, since categories and pin_photos only exist to support pins)
- [ ] `pins` gets an `is_hidden BOOLEAN NOT NULL DEFAULT false` column (used by moderation in Phase 2.5)
- [ ] `pin_photos`: `id`, `pin_id` (`ON DELETE CASCADE`), `photo_url`, `position` (`SMALLINT`, default 0), `created_at`
- [ ] Seed script inserting fixed category list (Food, Nature, Event, Nightlife, etc. — finalize the actual list)
- [ ] `GIST` index on `pins.location`
- [ ] Index on `pins.geohash`
- [ ] Index on `pins.category_id`
- [ ] Index on `pin_photos.pin_id`
- [ ] Unique index on `pin_photos (pin_id, position)`
- [ ] `internal/pins/model.go` — Pin + Category + PinPhoto structs
- [ ] Pick + install a geohash library (e.g. `github.com/mmcloughlin/geohash`)

### 2.2 Categories endpoint

- [ ] `internal/pins/get_pin.go` -> `GetCategories` handler (categories are read alongside pins in this module, no separate feature folder)
- [ ] Route: `GET /categories` (public, no auth middleware)
- [ ] Test in Postman: returns seeded list

### 2.3 Create pin endpoint

- [ ] `internal/pins/upload_pin.go` -> `CreatePin` handler
- [ ] Request struct (`internal/pins/dto.go`): lat, lng, caption (optional), category_id (required), photo_urls (placeholder list of strings for now — real multi-file upload comes in Phase 3)
- [ ] Decide + validate a max photo count per pin (e.g. 5)
- [ ] Validate lat/lng ranges, category_id exists
- [ ] Compute geohash from lat/lng, store alongside `GEOGRAPHY(POINT)`
- [ ] Insert the `pins` row and all `pin_photos` rows (with `position` = array index) in **one DB transaction**, so a pin can never end up with zero photos or orphaned photo rows
- [ ] Apply auth middleware to this route
- [ ] Route: `POST /pins`
- [ ] Test in Postman: valid pin with token, multiple photo URLs -> 201, all photos attached
- [ ] Test in Postman: no token -> 401
- [ ] Test in Postman: invalid category_id -> 400
- [ ] Test in Postman: lat/lng out of range -> 400
- [ ] Test in Postman: photo count over the max -> 400

### 2.4 Get pins (viewport query)

- [ ] `internal/pins/get_pin.go` -> `GetPins` handler
- [ ] Parse `bbox` query param (4 floats)
- [ ] PostGIS bounding-box query (`ST_MakeEnvelope` + `ST_Within`, or `&&` operator), filtered to `is_hidden = false`
- [ ] Optional `category` query param -> add `WHERE category_id = ...`
- [ ] No auth middleware (public)
- [ ] Route: `GET /pins?bbox=...&category=...`
- [ ] Test in Postman: no filters -> returns all visible pins in bbox
- [ ] Test in Postman: with category filter -> returns filtered subset
- [ ] Test in Postman: bbox with no pins -> returns empty array, not an error
- [ ] Test in Postman: a hidden pin never appears in results

### 2.5 Get single pin

- [ ] `internal/pins/get_pin.go` -> `GetPin` handler — joins `pin_photos` (ordered by `position`) and `users` (for `username`/`avatar_url`)
- [ ] Route: `GET /pins/:id` (public)
- [ ] Test in Postman: valid ID -> pin details with an ordered photo array
- [ ] Test in Postman: invalid/nonexistent ID -> 404

**Phase 2 done when:** Can create pins with multiple photos (authenticated) and query them by viewport + category (unauthenticated), fully tested in Postman, no real photo upload yet.

---

## Phase 2.5 — Reports & Moderation

### 2.5.1 Reports table

- [ ] Write migration `0004_reports.sql`: `id`, `pin_id` (`ON DELETE CASCADE`), `reporter_id` (`ON DELETE SET NULL`), `reason`, `status` (`NOT NULL`, `CHECK IN ('pending','reviewed','actioned')`, default `'pending'`), `resolved_by` (`ON DELETE SET NULL` — which admin/owner actioned it), `resolved_at`, `created_at`
- [ ] Unique constraint `UNIQUE (pin_id, reporter_id)` — one report per user per pin
- [ ] `internal/reports/model.go`

### 2.5.2 Create report endpoint

- [ ] `internal/reports/create_report.go` -> `CreateReport` handler
- [ ] Request struct (`internal/reports/dto.go`): reason (required)
- [ ] Apply auth middleware — reporting requires login
- [ ] Route: `POST /pins/:id/report`
- [ ] Test in Postman: valid report with token -> 201
- [ ] Test in Postman: no token -> 401
- [ ] Test in Postman: report on nonexistent pin -> 404
- [ ] Test in Postman: same user reports the same pin twice -> rejected (unique constraint)

### 2.5.3 Report review endpoint (admin role)

- [ ] `internal/reports/review_report.go` -> `ReviewReport` handler — body: `{ "action": "approve" | "dismiss" }`
- [ ] **Approve** -> `pins.is_hidden = true`, `reports.status = 'actioned'`, set `resolved_by` (from JWT) + `resolved_at`
- [ ] **Dismiss** -> `reports.status = 'reviewed'`, set `resolved_by` + `resolved_at`, pin untouched
- [ ] Route: `PATCH /reports/:id` — protected by `RequireAdmin` (Phase 1.8)
- [ ] Test in Postman: admin approves a report -> pin becomes hidden, disappears from `GET /pins`
- [ ] Test in Postman: admin dismisses a report -> pin unaffected, report marked `reviewed`
- [ ] Test in Postman: regular user attempts either action -> 403
- [ ] (Fallback, keep for emergencies) Direct DB query still works if the API is ever down: `SELECT * FROM reports WHERE status = 'pending' ORDER BY created_at;`

**Phase 2.5 done when:** A logged-in user can report a pin via `POST /pins/:id/report`, and an admin can review + action it in-app via `PATCH /reports/:id`, hiding the pin on approval. Direct database review remains available as a fallback, but is no longer the primary path now that roles exist (see Phase 1.8 for how the first admin/owner gets set up).

---

## Phase 3 — Photo Upload (Multi-File)

### 3.1 Local storage handler

- [ ] `storage/local.go` — save an uploaded file to `./uploads/`, return local path/URL
- [ ] Add static file serving: `r.Static("/uploads", "./uploads")`
- [ ] Add `./uploads` to `.gitignore`

### 3.2 Update create-pin flow for multiple files

- [ ] Change `POST /pins` to accept `multipart/form-data` instead of pure JSON, with photos sent under a repeated field name (e.g. `photos`)
- [ ] In Gin, read the file slice via `form.File["photos"]` instead of a single `c.FormFile(...)`
- [ ] Validate the whole batch up front: each file's type (jpg/png only) and size (e.g. max 10MB), and total count against the max from Phase 2.3 — reject the entire request if anything in the batch fails
- [ ] Upload each file via the storage handler, collecting the resulting URLs in order
- [ ] Insert the `pins` row and all `pin_photos` rows (URL + `position`) in one DB transaction (same transaction requirement as Phase 2.3, now with real files instead of placeholder URLs)
- [ ] Test in Postman: form-data with 3 real images + fields -> pin created, all 3 photos saved locally with correct order
- [ ] Test in Postman: one oversized file in the batch -> whole request rejected with clear error
- [ ] Test in Postman: one wrong file type in the batch -> whole request rejected

### 3.3 (Optional but recommended) Image processing

- [ ] `go get github.com/disintegration/imaging` (or similar)
- [ ] Resize/compress uploaded images before saving (reduces storage + bandwidth later)

**Phase 3 done when:** Real photo files (one or more per pin) can be uploaded via Postman form-data, saved locally, and retrieved via their stored URLs in the correct order.

---

## Phase 4 — Frontend Map View

### 4.1 Frontend scaffolding

- [ ] `npx create-next-app@latest goodspot247-web` (TypeScript, App Router)
- [ ] Install: `maplibre-gl`, `axios`, `zustand`, `zod`
- [ ] Set up `.env.local` with API base URL

### 4.2 Map screen

- [ ] Basic MapLibre map component, centered on user's location (or default city)
- [ ] Fetch pins on load via `GET /pins?bbox=...` (compute bbox from current map view)
- [ ] Render pins as markers
- [ ] Re-fetch on `moveend`/`zoomend` map events

### 4.3 Category filter UI

- [ ] Fetch categories via `GET /categories` on load
- [ ] Render filter chips
- [ ] Selecting a chip re-fetches pins with `&category=...`

### 4.4 Pin detail view

- [ ] Tap/click a marker -> open detail view (modal or side panel)
- [ ] Display photo carousel (multiple photos, ordered), caption, category, user, timestamp

### 4.5 Auth screens

- [ ] Login page/form -> calls `POST /auth/login`, stores JWT (e.g. in memory + httpOnly cookie or secure storage)
- [ ] Register page/form -> calls `POST /auth/register`
- [ ] Global auth state (Zustand store): current user, token, role, `isLoggedIn`

### 4.6 Create-pin flow

- [ ] "+" button on map
- [ ] If not logged in -> show login/register prompt instead of the form
- [ ] If logged in -> open create-pin form (multi-photo picker, category select, caption)
- [ ] Submit -> `POST /pins` with auth header, multipart form data (multiple files)
- [ ] On success -> new pin appears on map immediately (optimistic update)

### 4.7 (If applicable) Basic admin UI

- [ ] Simple "Reports" screen, visible only when `role` is `admin`/`owner` — lists pending reports, approve/dismiss buttons calling `PATCH /reports/:id`
- [ ] Simple "Manage roles" screen, visible only when `role` is `owner` — promote/demote a user via `PATCH /users/:id/role`

**Phase 4 done when:** A real person (not just you testing Postman) can open the app, browse the map as a guest, and — after logging in — successfully post a pin with multiple photos that shows up. This is your first genuinely demoable version.

---

## Phase 5 — Real-Time Layer

### 5.1 WebSocket server setup

- [ ] `go get github.com/gorilla/websocket`
- [ ] `internal/realtime/hub.go` — connection registry (`map[string][]*Connection` keyed by geohash cell)
- [ ] `internal/realtime/handler.go` — WebSocket upgrade handler, route: `GET /ws`
- [ ] Handle client `subscribe`/`unsubscribe` messages (cell list) — shapes defined in `internal/realtime/dto.go`

### 5.2 Redis pub/sub

- [ ] `go get github.com/redis/go-redis/v9`
- [ ] `REDIS_URL` in `.env`
- [ ] `internal/realtime/pubsub.go` — on pin creation, publish event to Redis channel keyed by geohash cell
- [ ] Realtime hub subscribes to relevant Redis channels, receives cross-instance events
- [ ] (Known gap, not required for MVP) Pin write and Redis publish are two separate steps today — a crash between them means a pin is saved but never broadcast. Revisit with a transactional outbox table + poller before relying on real-time delivery being 100% reliable.

### 5.3 Batching

- [ ] `internal/realtime/broadcast.go` — per-cell buffer + ticker (500ms–1s)
- [ ] On tick, flush buffered pins as one `pin_batch` message to all connections in that room

### 5.4 Frontend WebSocket client

- [ ] Native `WebSocket` (or `reconnecting-websocket`) connects on map load
- [ ] Compute visible geohash cells from current viewport, send `subscribe`
- [ ] On viewport change, update subscription (unsubscribe old cells, subscribe new ones)
- [ ] On `pin_batch` message, merge new pins into map state without a full refetch

### 5.5 Testing

- [ ] Open app in two browser tabs/windows
- [ ] Post a pin in tab A
- [ ] Confirm it appears in tab B within the batch window, no refresh

**Phase 5 done when:** Two-tab live-update test passes reliably.

---

## Phase 6 — Production Storage Swap

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
- [ ] Rate limiting on `POST /auth/register` and `POST /pins` (prevent spam/abuse)
- [ ] Basic uptime monitoring (even a free tool like UptimeRobot)
- [ ] Error tracking (Sentry free tier is enough to start)
- [ ] Test the full flow end-to-end in production: register -> browse -> post (multi-photo) -> see it live -> report -> admin actions it

**Phase 7 done when:** GoodSpot247 is live at a real public URL and you (and a few trusted testers) can use it fully.

---

## Phase 8 — Livestreaming (post-MVP, only after Phase 7 is validated)

### 8.1 LiveKit setup

- [ ] Create LiveKit Cloud account
- [ ] `go get github.com/livekit/server-sdk-go`
- [ ] Add LiveKit API key/secret to env config

### 8.2 Streams table + endpoints

- [ ] Write migration `0005_streams.sql`: `id`, `pin_id`, `broadcaster_id`, `livekit_room_name` (`NOT NULL UNIQUE`), `status` (`NOT NULL`, `CHECK IN ('live','ended')`, default `'live'`), `peak_viewer_count` (`INT NOT NULL DEFAULT 0`), `started_at`, `ended_at`
- [ ] Partial index: `CREATE INDEX streams_status_idx ON streams (status) WHERE status = 'live';`
- [ ] `internal/streams/handler.go` -> `POST /streams` — creates LiveKit room, returns broadcaster token
- [ ] `internal/streams/handler.go` -> `GET /streams/:id` — returns stream info + viewer token
- [ ] `internal/streams/handler.go` -> `POST /streams/:id/end` — closes room, updates status

### 8.3 Frontend streaming

- [ ] `npm install livekit-client`
- [ ] "Go Live" button (auth required) -> connects as broadcaster
- [ ] Live pins render distinctly on the map
- [ ] Viewer screen connects to LiveKit room, renders video
- [ ] Chat UI wired to existing WebSocket hub, keyed by `streamId`

### 8.4 Viewer count

- [ ] Configure LiveKit webhook endpoint on your backend
- [ ] Handle participant join/leave webhooks -> update live count -> push via WebSocket -> update `streams.peak_viewer_count` whenever the live count exceeds the stored peak

**Phase 8 done when:** A user can go live from a pin, another user can watch + chat in real time, viewer counts (including the peak) are tracked, and the stream ends cleanly.

---

## Quick Reference — Checklist Count by Phase

| Phase                            | Approx. checklist items |
| --------------------------------- | ------------------------ |
| 0. Setup                         | 15                       |
| 1. Auth & Roles                  | 30                       |
| 2. Pins, Categories & Multi-Photo | 26                       |
| 2.5 Reports & Moderation         | 14                       |
| 3. Photo Upload (Multi-File)     | 10                       |
| 4. Frontend Map View             | 23                       |
| 5. Real-Time Layer               | 16                       |
| 6. Storage Swap                  | 7                        |
| 7. Deploy                        | 13                       |
| 8. Livestreaming                 | 16                       |

Work top to bottom, phase by phase. Don't skip ahead to Phase 5+ until Phase 4's exit criteria genuinely passes — that's the point where you have something real to show and test.
