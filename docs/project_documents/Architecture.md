# GoodSpot247 — System Architecture

## 1. High-Level Overview

```
                                ┌─────────────────────┐
                                │      CLIENTS         │
                                │  Next.js (Web)        │
                                └──────────┬───────────┘
                                           │
                        ┌───────────────────┼───────────────────┐
                        │                   │                   │
                  REST API (HTTPS)   WebSocket (real-time)  Media (WebRTC)
                        │              [Phase 5, planned]  [Phase 8, planned]
                        ▼                   ▼                   ▼
             ┌─────────────────┐  ┌──────────────────┐  ┌──────────────────┐
             │   Go API Server  │  │  Go Realtime Hub  │  │  LiveKit SFU      │
             │  (Gin)           │  │  [scaffolded,     │  │ [Phase 8,        │
             │                  │  │   not yet built]  │  │  planned]        │
             └────────┬─────────┘  └─────────┬─────────┘  └─────────┬────────┘
                      │                      │                      │
                      │              ┌───────┴────────┐             │
                      │              │  Redis Pub/Sub  │◄────────────┘
                      │              │ [Phase 5,       │  (stream events,
                      │              │  planned]       │   viewer presence)
                      │              └────────────────┘
                      │
         ┌────────────┼─────────────────┐
         ▼            ▼                 ▼
  ┌────────────┐ ┌───────────┐   ┌──────────────┐
  │ PostgreSQL  │ │ Cloudflare │   │  Auth Service │
  │ + PostGIS   │ │    R2      │   │ (JWT-based,   │
  │             │ │ [Phase 6,  │   │  gated actions │
  │             │ │  planned]  │   │  + role checks │
  │             │ │            │   │  for admin/    │
  │             │ │            │   │  owner actions)│
  └────────────┘ └───────────┘   └──────────────┘
         │
         ▼
  ┌────────────┐
  │  Geo index  │
  │ (GIST)      │
  └────────────┘
```

---

## 2. Core Components

### 2.1 Client Layer

- **Web:** Next.js 16 (App Router) + MapLibre GL JS + React 19 + TypeScript
- **Mobile:** React Native + MapLibre Native SDK — **future consideration, not yet started**
- Client talks to the Go backend via REST + Zod schema validation on API responses
- WebSocket (real-time) and WebRTC (LiveKit) planned for Phases 5 and 8

### 2.2 Go API Server (REST)

Handles standard CRUD + business logic:

- Auth (register/login/logout, JWT issuance, JWT blacklist via Redis) — required only for write/interactive actions (creating a pin, later: going live, liking, commenting). Browsing pins and viewing pin details is open to unauthenticated/guest requests.
- Roles & moderation — `role` (`user`/`admin`/`owner`) gates report review and role management; enforced via `RequireAdmin`/`RequireOwner` middleware, which re-validate the role against the DB on every request. The base `AuthRequired` middleware extracts the role from JWT claims.
- Pins (create with one or more photos, viewport query via PostGIS, get by id, filter by category) — photos are processed to WebP with thumbnails via libvips (bimg)
- Categories (fixed list — Food, Nature, Event, Nightlife, Art, Sports, Travel, Other)
- Photos (upload handling via multipart form-data, multiple files per pin → local filesystem storage in dev, Cloudflare R2 planned for production)
- Reports (create, admin review/action with row-level locking)
- Streams — **scaffolded (DB migration exists), not yet implemented** (Phase 8)
- Future: likes/comments, sponsored pins, analytics endpoints

Framework: **Gin** (not Chi — the original plan was revised)

### 2.3 Go Realtime Hub (WebSocket layer) — **Phase 5, scaffolded, not yet implemented**

Separate logical service (can run as its own goroutine pool within the same binary initially, split out later if needed):

- Manages WebSocket connections
- Groups clients into **geohash-based rooms** (only broadcast to viewers actually watching that map cell)
- **Batches** broadcasts (e.g., every 500ms–1s) instead of pushing every event instantly
- Publishes/subscribes via **Redis Pub/Sub** so multiple Go instances stay in sync

Used for:

- New pin notifications in a viewport
- Live event pin updates
- Chat/reactions during livestreams
- Viewer count updates

**Current status:** `internal/modules/realtime/` contains empty placeholder files (`package realtime` only). No WebSocket library is in `go.mod`. No `/ws` route is registered. Redis is currently used only as a JWT blacklist. The geohash encoding utility (`pkg/geohash/`) is implemented and ready for room assignment.

### 2.4 Livestreaming (WebRTC + SFU) — **Phase 8, scaffolded, not yet implemented**

- **LiveKit** (Go-based, open source) — managed via LiveKit Cloud to start, self-hostable later
- Go API server creates "rooms" via LiveKit's API, issues join tokens
- Actual video routing handled entirely by LiveKit — your backend never touches raw video

**Current status:** `internal/modules/streams/` contains empty placeholder files (`package streams` only). DB migration `0005_streams.sql` exists with the schema. No LiveKit SDK is in `go.mod`. No stream routes are registered.

### 2.5 Database — PostgreSQL + PostGIS

- Single source of truth: users, pins, pin photos, categories, streams, reports, (later) likes/comments
- Pins support multiple photos via a separate `pin_photos` table (one row per photo, ordered, with `thumbnail_url` for map/list views) rather than a single `photo_url` column
- `GIST` index on `location GEOGRAPHY(POINT, 4326)` for fast viewport/radius queries
- Scales via read replicas later if needed
- Migrations: `0001` PostGIS extension, `0002` users, `0003` categories+pins+pin_photos, `0004` reports, `0005` streams (scaffold), `0006` pin_photos.thumbnail_url

### 2.6 Storage — Local Filesystem (dev) / Cloudflare R2 (planned for production)

- **Currently:** Local filesystem storage (`internal/platform/storage/local.go`) — saves processed photos to `./uploads/`, served via `GET /uploads/*`
- **Planned (Phase 6):** Cloudflare R2 — chosen over AWS S3 specifically for zero egress fees — GoodSpot247's usage pattern (many views per photo upload) is egress-heavy, so this avoids costs scaling with popularity
- Photos uploaded from client → Go backend → validated (JPG/PNG, ≤8000×8000px) → resized (≤1600px WebP q80) + 400px square thumbnail → stored locally
- Cloudflare CDN delivery planned for production
- **R2 config env vars are documented but not yet wired** (`STORAGE_BACKEND`, `R2_*` in `.env.example` are aspirational)

### 2.7 Redis

**Currently:** JWT blacklist only — logout revokes the token's `jti` in Redis for the token's remaining TTL

**Planned:**

- **Pub/Sub** — coordinates real-time broadcasts across multiple Go instances (Phase 5)
- Optional: cache hot queries (e.g., trending spots) later

---

## 3. Data Flow Examples

### A) User posts a new pin (requires login)

```
Client → (must have valid JWT) → POST /pins (Go API) → validate → save pin + pin_photos rows
                                       to Postgres/PostGIS in one transaction
                                       → process photos (validate → WebP + thumbnail via libvips)
                                       → save processed files to local filesystem
                                       → return pin + photos to client

[Phase 5, planned] → publish event to Redis
                         → Realtime Hub picks up
                         → broadcasts to geohash room
                         → connected viewers see it live
```

### B) User goes live at a location — **Phase 8, not yet implemented**

```
Client → POST /streams (Go API) → create LiveKit room → return join token
Broadcaster → connects to LiveKit via WebRTC (video/audio)
Viewers → GET /streams/:id → get token → connect to LiveKit → watch
Chat/reactions → WebSocket → Realtime Hub → geohash/room-based fan-out
Viewer join/leave webhooks → update viewer_count and streams.peak_viewer_count
```

### C) User browses the map (no login required)

```
Client → GET /pins?bbox=...&category=... (Go API, public endpoint) → PostGIS bounding-box query (excludes is_hidden pins) → return pins

[Phase 5, planned]
Client → opens WebSocket → subscribes to visible geohash cells
       → receives live updates as new pins appear in view
```

### D) Admin reviews a report

```
Admin/owner client → (valid JWT, role = admin or owner) → PATCH /reports/:id { action: approve|dismiss }
   → RequireAdmin middleware checks role
   → if approve: reports.status = 'actioned', reports.resolved_by/resolved_at set, pins.is_hidden = true
   → if dismiss: reports.status = 'reviewed', reports.resolved_by/resolved_at set
```

### E) Owner promotes a user to admin

```
Owner client → (valid JWT, role = owner) → PATCH /users/:id/role { role: "admin" }
   → RequireOwner middleware checks role
   → users.role updated
```

---

## 4. Scaling Path (as traffic grows)

1. **Single Go instance** — current state, fine for MVP and early growth
2. **Horizontal scaling** — run multiple Go API/Realtime instances behind a load balancer, coordinated via Redis Pub/Sub (requires Phase 5)
3. **Read replicas** for Postgres if read load grows
4. **Self-host LiveKit** if managed pricing becomes a bottleneck at scale
5. Only if a genuine, measured bottleneck appears in the WebSocket layer specifically (rare) — consider splitting Realtime Hub into its own dedicated service/cluster

---

## 5. Repo Structure (actual)

```
sweetspot/
├── .env / .env.example              # shared secrets for docker-compose
├── docker-compose.yml               # 4 services: postgres, redis, server, client
├── SECURITY.md                      # security audit findings and controls
│
├── project_documents/               # product & engineering docs (this folder)
│
├── server/                          # Go backend
│   ├── cmd/api/
│   │   └── main.go                  # application entrypoint
│   ├── internal/
│   │   ├── app/
│   │   │   ├── app.go               # bootstrap: DI container, Redis ping, router, HTTP server
│   │   │   └── graceful_shutdown.go # signal handling + graceful shutdown
│   │   ├── config/
│   │   │   └── config.go            # env-based configuration loader
│   │   ├── di/
│   │   │   └── container.go         # manual dependency injection wiring
│   │   ├── http/
│   │   │   ├── router.go            # Gin engine, route registration, middleware wiring
│   │   │   ├── middleware/
│   │   │   │   ├── auth.go          # JWT auth, RequireAdmin, RequireOwner middleware
│   │   │   │   ├── body_limit.go    # request body size limits (1MB JSON / 64MB uploads)
│   │   │   │   ├── cors.go          # CORS middleware
│   │   │   │   ├── logger.go        # request logging
│   │   │   │   ├── rate_limit.go    # in-memory per-IP sliding-window rate limiter
│   │   │   │   ├── recover.go       # panic recovery
│   │   │   │   └── security_headers.go  # security response headers
│   │   │   └── response/
│   │   │       ├── response.go      # JSON response helpers
│   │   │       └── errors.go        # error response helpers
│   │   ├── modules/
│   │   │   ├── auth/
│   │   │   │   ├── handler.go       # Register, Login, Logout, Me handlers
│   │   │   │   ├── service.go       # business logic (register, login, role lookup)
│   │   │   │   ├── routes.go        # route registration + rate limiters
│   │   │   │   ├── dto.go           # RegisterRequest, LoginRequest
│   │   │   │   ├── errors.go        # ErrConflict, ErrInvalidCredentials
│   │   │   │   └── auth_test.go
│   │   │   ├── pins/
│   │   │   │   ├── handler.go       # CRUD handlers: ListCategories, GetPins, GetPin, CreatePin
│   │   │   │   ├── service.go       # business logic
│   │   │   │   ├── routes.go        # route registration + rate limiters
│   │   │   │   ├── dto.go           # NewPin, CreatePinRequest (unused DTO)
│   │   │   │   ├── model.go         # Pin, PinPhoto, PinDetail, PinListEntry, Category
│   │   │   │   ├── repository.go    # PostGIS queries
│   │   │   │   ├── errors.go        # ErrNotFound
│   │   │   │   ├── pins_test.go
│   │   │   │   └── imaging/
│   │   │   │       └── imaging.go   # image validation + processing (libvips via bimg)
│   │   │   ├── reports/
│   │   │   │   ├── handler.go       # Create, List, Review handlers
│   │   │   │   ├── service.go       # business logic
│   │   │   │   ├── routes.go        # route registration + RequireAdmin guard
│   │   │   │   ├── dto.go           # CreateReportRequest, ReviewReportRequest
│   │   │   │   ├── model.go         # Report, ReportListEntry, status constants, errors
│   │   │   │   ├── repository.go    # Postgres queries with FOR UPDATE locking
│   │   │   │   └── reports_test.go
│   │   │   ├── user/
│   │   │   │   ├── handler.go       # Get (public/private profile), UpdateRole handlers
│   │   │   │   ├── service.go       # role-change invariants (can't demote last owner, etc.)
│   │   │   │   ├── routes.go        # route registration + RequireAdmin/RequireOwner guards
│   │   │   │   ├── dto.go           # PublicUser, PrivateUser, UpdateRoleRequest
│   │   │   │   ├── model.go         # User struct, Role constants
│   │   │   │   ├── repository.go
│   │   │   │   ├── errors.go        # ErrNotFound, ErrCannotChangeOwnRole, ErrCannotDemoteLastOwner
│   │   │   │   └── user_test.go
│   │   │   ├── realtime/            # Phase 5 — scaffolded, not yet built
│   │   │   │   ├── handler.go       # (empty)
│   │   │   │   ├── hub.go           # (empty)
│   │   │   │   ├── rooms.go         # (empty)
│   │   │   │   ├── broadcast.go     # (empty)
│   │   │   │   ├── pubsub.go        # (empty)
│   │   │   │   └── dto.go           # (empty)
│   │   │   └── streams/             # Phase 8 — scaffolded, not yet built
│   │   │       ├── handler.go       # (empty)
│   │   │       ├── service.go       # (empty)
│   │   │       ├── livekit_client.go# (empty)
│   │   │       ├── webhook.go       # (empty)
│   │   │       ├── dto.go           # (empty)
│   │   │       ├── model.go         # (empty)
│   │   │       └── repository.go    # (empty)
│   │   ├── observability/
│   │   │   └── logger/
│   │   │       ├── logger.go        # slog + tint (colorized text) or JSON logger
│   │   │       └── middleware.go    # request logging middleware with X-Request-ID
│   │   ├── platform/
│   │   │   ├── cache/
│   │   │   │   └── session.go       # Redis-backed JWT blacklist (Revoke, IsRevoked, Ping)
│   │   │   ├── database/
│   │   │   │   ├── postgres.go      # pgx connection pool with retry
│   │   │   │   ├── migrate.go       # custom SQL migration runner (schema_migrations table)
│   │   │   │   └── migrations/
│   │   │   │       ├── 0001_init_extensions.sql
│   │   │   │       ├── 0002_users.sql
│   │   │   │       ├── 0003_pins.sql
│   │   │   │       ├── 0004_reports.sql
│   │   │   │       ├── 0005_streams.sql
│   │   │   │       └── 0006_pin_photo_thumbnails.sql
│   │   │   └── storage/
│   │   │       ├── local.go         # local filesystem storage
│   │   │       ├── local_test.go
│   │   │       └── fileid.go        # server-generated random hex file IDs
│   │   └── endpointtest/
│   │       └── endpoints_test.go    # security regression tests (546 lines)
│   ├── pkg/
│   │   ├── geohash/
│   │   │   └── geohash.go           # encode lat/lng → geohash cell + neighbors
│   │   ├── jwt/
│   │   │   └── jwt.go               # generate + validate HS256 tokens with jti
│   │   ├── password/
│   │   │   └── password.go          # bcrypt hash (cost 12) + verify
│   │   └── validid/
│   │       └── validid.go           # UUID format validation middleware
│   ├── deployments/
│   │   ├── docker/
│   │   │   └── server.Dockerfile    # multi-stage: golang:1.27-alpine → alpine:3.20
│   │   └── k8s/.gitkeep             # placeholder
│   ├── go.mod / go.sum
│   └── Makefile                     # build/run/test/vet/lint/docker-up/docker-down
│
└── client/                          # Next.js frontend
    ├── src/
    │   ├── app/
    │   │   ├── layout.tsx           # root layout (fonts, Navbar, ErrorBoundary)
    │   │   ├── page.tsx             # home page (renders MapApp)
    │   │   ├── login/page.tsx       # login form
    │   │   └── register/page.tsx    # registration form
    │   ├── components/
    │   │   ├── ErrorBoundary.tsx    # React error boundary
    │   │   ├── layout/
    │   │   │   └── Navbar.tsx       # navigation bar (brand, auth, logout)
    │   │   ├── map/
    │   │   │   ├── MapApp.tsx       # main map orchestration (state, hooks, overlays)
    │   │   │   └── MapView.tsx      # MapLibre GL map (GeoJSON source, circle layer)
    │   │   └── pins/
    │   │       ├── PinDetailPanel.tsx  # right-side panel (photo carousel, details)
    │   │       ├── CreatePinButton.tsx # floating "+" button + create-pin form
    │   │       └── CategoryBar.tsx  # horizontal category filter chips
    │   ├── hooks/
    │   │   ├── usePins.ts           # pin fetching hook (bbox + category, AbortController)
    │   │   ├── usePinDetail.ts      # single pin detail hook
    │   │   └── useGeolocation.ts    # browser geolocation wrapper
    │   ├── lib/
    │   │   ├── api/
    │   │   │   ├── client.ts        # Axios instance with auth interceptor
    │   │   │   ├── auth.ts          # auth API calls (login, register, logout)
    │   │   │   ├── pins.ts          # pins API calls (list, detail, create)
    │   │   │   ├── categories.ts    # categories API call
    │   │   │   └── schemas.ts       # Zod validation schemas for API responses
    │   │   ├── types/
    │   │   │   ├── auth.ts          # User, AuthResponse types
    │   │   │   ├── pin.ts           # PinListEntry, PinDetail, PinPhoto, etc.
    │   │   │   └── category.ts      # Category type
    │   │   └── utils/
    │   │       ├── geo.ts           # WKT POINT parser, bbox normalization
    │   │       └── format.ts        # time formatting
    │   └── store/
    │       └── auth.ts              # Zustand auth store (persisted to localStorage)
    ├── package.json                 # pnpm, Next.js 16, React 19, MapLibre, Zustand, Zod
    ├── Dockerfile                   # multi-stage: node:20-alpine
    └── AGENTS.md                    # AI agent coding conventions
```

**Why feature-based:** each feature (auth, pins, users, reports, streams, realtime) is a self-contained folder with its own handler/service/model/repository files, rather than splitting one feature's code across separate `handlers/`, `models/`, `db/` folders. This scales better as features are added and matches Go's `internal/` convention, which the compiler enforces (code under `internal/` can only be imported within this module).

**Why `storage/` is in `platform/`, not nested in `pins/`:** photo storage isn't inherently pins-only — user avatars or stream thumbnails could need it later. Keeping `storage.Local` as a shared platform service means any feature can depend on it without duplicating upload logic or reaching into `pins/`.

**Why `categories` and `pin_photos` live inside the pins migration, not their own files:** both only exist to support pins (no independent use case), so they're created in the same migration as the table that depends on them.

**`reports/` now has a real admin-facing endpoint, not just a repository.** The original MVP plan had no admin role at all — reports were reviewed entirely by hand via direct database queries. That's changed: a lightweight `role` column (`user`/`admin`/`owner`) on `users` now gates a `PATCH /reports/:id` endpoint so admins can review and action reports in-app. The only step that still happens by hand is bootstrapping the very first owner, set directly in the database since there's intentionally no endpoint that can grant it.

---

## 6. MVP Build Order (actual progress)

1. ✅ Auth (register/login/logout, JWT, Redis blacklist) — gates write actions
2. ✅ Roles (`user`/`admin`/`owner`) + `RequireAdmin`/`RequireOwner` middleware with DB re-validation
3. ✅ Pins CRUD + PostGIS viewport query + categories (public read, gated write) + multi-photo upload via `pin_photos`
4. ✅ Reports: create (any logged-in user) + admin review endpoint (hide pin on action, FOR UPDATE locking)
5. ✅ Photo upload with processing: validation (JPG/PNG, ≤8000×8000px), WebP conversion (≤1600px, q80), 400px square thumbnails (libvips via bimg)
6. ✅ Frontend map view: MapLibre GL, category filter chips, pin detail panel, create-pin flow, auth screens
7. ⬜ WebSocket layer: geohash rooms + batching for live pin updates (Phase 5)
8. ⬜ Production storage swap: Cloudflare R2 (Phase 6)
9. ⬜ Deploy MVP (Phase 7)
10. ⬜ Livestreaming integration (LiveKit) (Phase 8)
