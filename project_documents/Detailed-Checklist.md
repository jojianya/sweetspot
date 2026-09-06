# GoodSpot247 — Detailed Development Checklist

Granular, task-level checklist version of the Roadmap. Each phase is broken into concrete steps you can check off one at a time. Use this as your actual working checklist; use the Roadmap doc for the higher-level view.

---

## Phase 0 — Project Setup

### 0.1 Environment
- [ ] Install Go (`go version` confirms it works)
- [ ] Install Docker Desktop
- [ ] Install Postman (or equivalent)
- [ ] Install a Postgres client (TablePlus, DBeaver, or `psql`)

### 0.2 Project scaffolding
- [ ] `mkdir goodspot247-backend && cd goodspot247-backend`
- [ ] `go mod init github.com/yourusername/goodspot247-backend`
- [ ] Create folder structure: `config/`, `db/`, `pkg/jwt/`, `pkg/password/`, `pkg/geohash/`, `pkg/ratelimit/`, `storage/`, `internal/users/`, `internal/auth/`, `internal/pins/`, `internal/reports/`, `internal/realtime/`, `internal/streams/`
- [ ] `go get github.com/gin-gonic/gin`
- [ ] `go get github.com/joho/godotenv`
- [ ] Create `.env` file (add to `.gitignore` immediately)
- [ ] Create `.env.example` (committed, no real values) listing required vars

### 0.3 Docker Compose for local services
- [ ] Write `docker-compose.yml` with:
  - [ ] `postgres` service (use `postgis/postgis` image, not plain postgres)
  - [ ] `redis` service
- [ ] `docker-compose up -d`
- [ ] Confirm Postgres is reachable (`psql` or DB client connects)
- [ ] Confirm Redis is reachable (`redis-cli ping` → `PONG`)

### 0.4 Base server
- [ ] Write minimal `main.go` with Gin, one `GET /health` route
- [ ] `go run main.go` → confirm it starts
- [ ] Add DB connection (`db/postgres.go`) using `pgx` or `sqlx`
- [ ] Update `/health` to also ping the DB, return DB status in response
- [ ] `go get github.com/jackc/pgx/v5` (or `sqlx` equivalent)

### 0.5 Git & repo hygiene
- [ ] `git init`, first commit
- [ ] `.gitignore`: `.env`, `/uploads`, compiled binaries
- [ ] Push to GitHub/GitLab (private repo)

**Phase 0 done when:** `go run main.go`, hit `GET /health` in Postman, get back `{"status":"ok","db":"connected"}`.

---

## Phase 1 — Auth

### 1.1 Database
- [ ] Enable `postgis` extension (`CREATE EXTENSION IF NOT EXISTS postgis;`) — migration `0001_init_extensions.sql`
- [ ] Write migration `0002_users.sql` for `users` table (id, email, password_hash, display_name, created_at)
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
- [ ] Request struct (`internal/auth/dto.go`) with `binding` validation tags (email format, password min length)
- [ ] Check email uniqueness before insert
- [ ] Hash password before storing
- [ ] Return user (without password hash) + JWT on success
- [ ] Route: `POST /auth/register`
- [ ] Test in Postman: valid registration -> 201 + token
- [ ] Test in Postman: duplicate email -> proper error, not a crash
- [ ] Test in Postman: invalid email format -> 400 with validation message

### 1.5 Login endpoint
- [ ] `internal/auth/login.go` -> `Login` handler
- [ ] Look up user by email, compare password hash
- [ ] Return JWT on success, generic error on failure (don't leak "email not found" vs "wrong password" — same message for both, security best practice)
- [ ] Route: `POST /auth/login`
- [ ] Test in Postman: correct credentials -> 200 + token
- [ ] Test in Postman: wrong password -> 401, generic message

### 1.6 Auth middleware
- [ ] `internal/auth/middleware.go` — reads `Authorization: Bearer <token>` header, validates JWT, sets user ID in context
- [ ] Write a temporary `GET /me` protected test route that returns the authenticated user's ID from context
- [ ] Test in Postman: `/me` with valid token -> 200
- [ ] Test in Postman: `/me` with no token -> 401
- [ ] Test in Postman: `/me` with garbage/expired token -> 401

### 1.7 Public user profile
- [ ] `internal/users/get_user.go` -> `GetUser` handler, `internal/users/dto.go` -> `PublicUser` (strips password hash)
- [ ] Route: `GET /users/:id` (public, no auth middleware)
- [ ] Test in Postman: returns public profile fields only, never password_hash

**Phase 1 done when:** Full register -> login -> access protected route flow works end-to-end in Postman, and invalid/missing token cases are properly rejected.

---

## Phase 2 — Pins & Categories

### 2.1 Categories + Pins migration
- [ ] Write migration `0003_pins.sql` — `categories` table created first, then `pins` table referencing it (single migration file, since categories only exists to support pins)
- [ ] Seed script inserting fixed category list (Food, Nature, Event, Nightlife, etc. — finalize the actual list)
- [ ] `GIST` index on `pins.location`
- [ ] Index on `pins.geohash`
- [ ] Index on `pins.category_id`
- [ ] `internal/pins/model.go` — Pin + Category structs
- [ ] Pick + install a geohash library (e.g. `github.com/mmcloughlin/geohash`)

### 2.2 Categories endpoint
- [ ] `internal/pins/get_pin.go` -> `GetCategories` handler (categories are read alongside pins in this module, no separate feature folder)
- [ ] Route: `GET /categories` (public, no auth middleware)
- [ ] Test in Postman: returns seeded list

### 2.3 Create pin endpoint
- [ ] `internal/pins/upload_pin.go` -> `CreatePin` handler
- [ ] Request struct (`internal/pins/dto.go`): lat, lng, caption (optional), category_id, photo_url (placeholder string for now — real upload comes in Phase 3)
- [ ] Validate lat/lng ranges, category_id exists
- [ ] Compute geohash from lat/lng, store alongside `GEOGRAPHY(POINT)`
- [ ] Apply auth middleware to this route
- [ ] Route: `POST /pins`
- [ ] Test in Postman: valid pin with token -> 201
- [ ] Test in Postman: no token -> 401
- [ ] Test in Postman: invalid category_id -> 400
- [ ] Test in Postman: lat/lng out of range -> 400

### 2.4 Get pins (viewport query)
- [ ] `internal/pins/get_pin.go` -> `GetPins` handler
- [ ] Parse `bbox` query param (4 floats)
- [ ] PostGIS bounding-box query (`ST_MakeEnvelope` + `ST_Within`, or `&&` operator)
- [ ] Optional `category` query param -> add `WHERE category_id = ...`
- [ ] No auth middleware (public)
- [ ] Route: `GET /pins?bbox=...&category=...`
- [ ] Test in Postman: no filters -> returns all pins in bbox
- [ ] Test in Postman: with category filter -> returns filtered subset
- [ ] Test in Postman: bbox with no pins -> returns empty array, not an error

### 2.5 Get single pin
- [ ] `internal/pins/get_pin.go` -> `GetPin` handler
- [ ] Route: `GET /pins/:id` (public)
- [ ] Test in Postman: valid ID -> pin details
- [ ] Test in Postman: invalid/nonexistent ID -> 404

**Phase 2 done when:** Can create pins (authenticated) and query them by viewport + category (unauthenticated), fully tested in Postman, no photo upload yet.

---

## Phase 2.5 — Reports (Content Moderation Baseline)

### 2.5.1 Reports table
- [ ] Write migration `0004_reports.sql` (id, pin_id, reporter_id, reason, status, created_at, resolved_at)
- [ ] `internal/reports/model.go`

### 2.5.2 Create report endpoint
- [ ] `internal/reports/create_report.go` -> `CreateReport` handler
- [ ] Request struct (`internal/reports/dto.go`): reason (required)
- [ ] Apply auth middleware — reporting requires login
- [ ] Route: `POST /pins/:id/report`
- [ ] Test in Postman: valid report with token -> 201
- [ ] Test in Postman: no token -> 401
- [ ] Test in Postman: report on nonexistent pin -> 404

### 2.5.3 Admin review endpoints
- [ ] `internal/reports/list_reports.go` -> `ListReports` handler (admin-only — simplest MVP approach: a hardcoded admin user ID/email check, or an `is_admin` flag on `users`)
- [ ] `internal/reports/resolve_report.go` -> `ResolveReport` handler, updates status to `reviewed`/`actioned`
- [ ] Routes: `GET /reports`, `POST /reports/:id/resolve` (both admin-only)
- [ ] Test in Postman: non-admin user gets 403
- [ ] Test in Postman: admin can list + resolve reports

**Phase 2.5 done when:** A logged-in user can report a pin, and you (as admin) can list and resolve reports via Postman. This is the MVP's baseline content moderation mechanism — no automated scanning yet, just user reporting + manual review.

---

## Phase 3 — Photo Upload

### 3.1 Local storage handler
- [ ] `storage/local.go` — save uploaded file to `./uploads/`, return local path/URL
- [ ] Add static file serving: `r.Static("/uploads", "./uploads")`
- [ ] Add `./uploads` to `.gitignore`

### 3.2 Update create-pin flow
- [ ] Change `POST /pins` to accept `multipart/form-data` instead of pure JSON
- [ ] Parse photo file from form data
- [ ] Validate file type (jpg/png only) and size limit (e.g. max 10MB)
- [ ] Save via storage handler, get back URL
- [ ] Store URL in `photo_url` column
- [ ] Test in Postman: form-data with real image + fields -> pin created, photo saved locally
- [ ] Test in Postman: oversized file -> rejected with clear error
- [ ] Test in Postman: wrong file type -> rejected

### 3.3 (Optional but recommended) Image processing
- [ ] `go get github.com/disintegration/imaging` (or similar)
- [ ] Resize/compress uploaded images before saving (reduces storage + bandwidth later)

**Phase 3 done when:** Real photo files can be uploaded via Postman form-data, saved locally, and retrieved via their stored URL.

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
- [ ] Display photo, caption, category, user, timestamp

### 4.5 Auth screens
- [ ] Login page/form -> calls `POST /auth/login`, stores JWT (e.g. in memory + httpOnly cookie or secure storage)
- [ ] Register page/form -> calls `POST /auth/register`
- [ ] Global auth state (Zustand store): current user, token, `isLoggedIn`

### 4.6 Create-pin flow
- [ ] "+" button on map
- [ ] If not logged in -> show login/register prompt instead of the form
- [ ] If logged in -> open create-pin form (photo picker, category select, caption)
- [ ] Submit -> `POST /pins` with auth header, multipart form data
- [ ] On success -> new pin appears on map immediately (optimistic update)

**Phase 4 done when:** A real person (not just you testing Postman) can open the app, browse the map as a guest, and — after logging in — successfully post a pin with a photo that shows up. This is your first genuinely demoable version.

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
- [ ] Test upload against R2 in a staging environment
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

### 7.2 Frontend deploy
- [ ] Deploy Next.js app (Vercel is the natural choice)
- [ ] Point frontend's API base URL to production backend

### 7.3 Pre-launch checklist
- [ ] Privacy Policy + Terms of Service published and linked in-app
- [ ] Content moderation: `reports` feature (Phase 2.5) is live and you have a routine for checking `GET /reports` regularly
- [ ] Rate limiting on `POST /auth/register` and `POST /pins` (prevent spam/abuse)
- [ ] Basic uptime monitoring (even a free tool like UptimeRobot)
- [ ] Error tracking (Sentry free tier is enough to start)
- [ ] Test the full flow end-to-end in production: register -> browse -> post -> see it live

**Phase 7 done when:** GoodSpot247 is live at a real public URL and you (and a few trusted testers) can use it fully.

---

## Phase 8 — Livestreaming (post-MVP, only after Phase 7 is validated)

### 8.1 LiveKit setup
- [ ] Create LiveKit Cloud account
- [ ] `go get github.com/livekit/server-sdk-go`
- [ ] Add LiveKit API key/secret to env config

### 8.2 Streams table + endpoints
- [ ] Write migration `0005_streams.sql`
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
- [ ] Handle participant join/leave webhooks -> update count -> push via WebSocket

**Phase 8 done when:** A user can go live from a pin, another user can watch + chat in real time, and the stream ends cleanly.

---

## Quick Reference — Checklist Count by Phase

| Phase | Approx. checklist items |
|---|---|
| 0. Setup | 15 |
| 1. Auth (+ public user profile) | 23 |
| 2. Pins & Categories | 20 |
| 2.5 Reports | 11 |
| 3. Photo Upload | 10 |
| 4. Frontend Map View | 20 |
| 5. Real-Time Layer | 15 |
| 6. Storage Swap | 7 |
| 7. Deploy | 12 |
| 8. Livestreaming | 15 |

Work top to bottom, phase by phase. Don't skip ahead to Phase 5+ until Phase 4's exit criteria genuinely passes — that's the point where you have something real to show and test.
