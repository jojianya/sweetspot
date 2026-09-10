# GoodSpot247 — System Architecture

## 1. High-Level Overview

```
                                ┌─────────────────────┐
                                │      CLIENTS         │
                                │  Next.js (Web)        │
                                │  React Native (Mobile)│
                                └──────────┬───────────┘
                                           │
                       ┌───────────────────┼───────────────────┐
                       │                   │                   │
                 REST API (HTTPS)   WebSocket (real-time)  Media (WebRTC)
                       │                   │                   │
                       ▼                   ▼                   ▼
            ┌─────────────────┐  ┌──────────────────┐  ┌──────────────────┐
            │   Go API Server  │  │  Go Realtime Hub  │  │  LiveKit SFU      │
            │  (Gin/Chi)       │  │  (goroutines +     │  │ (managed or       │
            │                  │  │   geohash rooms)   │  │  self-hosted, Go) │
            └────────┬─────────┘  └─────────┬─────────┘  └─────────┬────────┘
                     │                      │                      │
                     │              ┌───────┴────────┐             │
                     │              │  Redis Pub/Sub  │◄────────────┘
                     │              │ (cross-instance  │  (stream events,
                     │              │  broadcast sync) │   viewer presence)
                     │              └────────────────┘
                     │
        ┌────────────┼─────────────────┐
        ▼            ▼                 ▼
 ┌────────────┐ ┌───────────┐   ┌──────────────┐
 │ PostgreSQL  │ │ Cloudflare │   │  Auth Service │
 │ + PostGIS   │ │    R2      │   │ (JWT-based,   │
 │             │ │  (photos)  │   │  gated actions │
 │             │ │            │   │  + role checks │
 │             │ │            │   │  for admin/    │
 │             │ │            │   │  owner actions)│
 └────────────┘ └───────────┘   └──────────────┘
        │              │
        ▼              ▼
 ┌────────────┐ ┌───────────┐
 │  Geo index  │ │    CDN     │
 │ (GIST)      │ │ (delivery) │
 └────────────┘ └───────────┘
```

---

## 2. Core Components

### 2.1 Client Layer

- **Web:** Next.js + MapLibre GL JS
- **Mobile:** React Native + MapLibre Native SDK
- Both talk to the same Go backend via REST + WebSocket + WebRTC (via LiveKit SDKs)

### 2.2 Go API Server (REST)

Handles standard CRUD + business logic:

- Auth (register/login, JWT issuance) — required only for write/interactive actions (creating a pin, later: going live, liking, commenting). Browsing pins and viewing pin details is open to unauthenticated/guest requests.
- Roles & moderation — `role` (`user`/`admin`/`owner`) gates report review and role management; enforced via `RequireAdmin`/`RequireOwner` middleware, layered on top of the base JWT auth middleware
- Pins (create with one or more photos, viewport query, get by id, filter by category)
- Categories (fixed list — e.g. Food, Nature, Event, Nightlife)
- Photos (upload handling, multiple files per pin → Cloudflare R2)
- Reports (create, admin review/action)
- Streams (create session, get stream metadata)
- Future: likes/comments, sponsored pins, analytics endpoints

Framework: **Gin**

### 2.3 Go Realtime Hub (WebSocket layer)

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

### 2.4 Livestreaming (WebRTC + SFU)

- **LiveKit** (Go-based, open source) — managed via LiveKit Cloud to start, self-hostable later
- Go API server creates "rooms" via LiveKit's API, issues join tokens
- Actual video routing handled entirely by LiveKit — your backend never touches raw video

### 2.5 Database — PostgreSQL + PostGIS

- Single source of truth: users, pins, pin photos, categories, streams, reports, (later) likes/comments
- Pins support multiple photos via a separate `pin_photos` table (one row per photo, ordered) rather than a single `photo_url` column
- `GIST` index on `location GEOGRAPHY(POINT, 4326)` for fast viewport/radius queries
- Scales via read replicas later if needed

### 2.6 Storage — Cloudflare R2 + CDN

- Chosen over AWS S3 specifically for zero egress fees — GoodSpot247's usage pattern (many views per photo upload) is egress-heavy, so this avoids costs scaling with popularity
- Photos uploaded from client → Go backend → resized (e.g. `imaging` lib or a worker) → stored in R2, one object per photo
- Served through Cloudflare's CDN for fast delivery
- **Deferred for local development** — MVP build starts with local filesystem storage, swapped to R2 before deployment/public use (see build order)

### 2.7 Redis

Two jobs:

- **Pub/Sub** — coordinates real-time broadcasts across multiple Go instances
- Optional: cache hot queries (e.g., trending spots) later

---

## 3. Data Flow Examples

### A) User posts a new pin (requires login)

```
Client → (must have valid JWT) → POST /pins (Go API) → validate → save pin + pin_photos rows
                                       to Postgres/PostGIS in one transaction
                                       → upload photo(s) to R2
                                       → publish event to Redis
                                             → Realtime Hub picks up
                                             → broadcasts to geohash room
                                             → connected viewers see it live
```

### B) User goes live at a location

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

1. **Single Go instance** — fine for MVP and early growth
2. **Horizontal scaling** — run multiple Go API/Realtime instances behind a load balancer, coordinated via Redis Pub/Sub
3. **Read replicas** for Postgres if read load grows
4. **Self-host LiveKit** if managed pricing becomes a bottleneck at scale
5. Only if a genuine, measured bottleneck appears in the WebSocket layer specifically (rare) — consider splitting Realtime Hub into its own dedicated service/cluster

---

## 5. Suggested Repo Structure

```
goodspot247-backend/
├── go.mod
├── go.sum
├── main.go
├── .env
├── .env.example
├── .gitignore
├── docker-compose.yml
│
├── config/
│   └── config.go                # env loading, app config struct
│
├── db/
│   └── postgres.go               # connection pool setup
│
├── migrations/
│   ├── 0001_init_extensions.sql  # postgis extension
│   ├── 0002_users.sql            # role, avatar_url, socials, updated_at + trigger
│   ├── 0003_pins.sql             # categories + pins + pin_photos
│   ├── 0004_reports.sql          # resolved_by, unique(pin_id, reporter_id)
│   └── 0005_streams.sql          # peak_viewer_count, unique room name
│
├── pkg/
│   ├── jwt/
│   │   └── jwt.go                # generate + validate tokens
│   ├── password/
│   │   └── password.go           # bcrypt hash + compare
│   ├── geohash/
│   │   └── geohash.go            # encode lat/lng -> geohash cell
│   └── ratelimit/
│       └── ratelimit.go          # Redis-backed limiter middleware
│
├── storage/
│   ├── storage.go                # interface (shared contract)
│   ├── local.go                  # dev: filesystem
│   └── r2.go                     # production: Cloudflare R2
│
└── internal/
    ├── users/
    │   ├── get_user.go           # GET /users/:id (public)
    │   ├── update_role.go        # PATCH /users/:id/role (owner only)
    │   ├── model.go
    │   ├── dto.go                # PublicUser (strips PasswordHash)
    │   └── repository.go
    │
    ├── auth/
    │   ├── register.go           # POST /auth/register
    │   ├── login.go              # POST /auth/login
    │   ├── dto.go
    │   ├── service.go
    │   ├── middleware.go         # JWT auth middleware + RequireAdmin/RequireOwner role checks
    │   └── jwt.go
    │
    ├── pins/
    │   ├── upload_pin.go         # POST /pins (auth required) — one or more photos, uses storage.Storage
    │   ├── get_pin.go            # GET /pins, GET /pins/:id, GET /categories
    │   ├── dto.go
    │   ├── model.go              # Pin + Category + PinPhoto structs
    │   └── repository.go
    │
    ├── reports/
    │   ├── create_report.go      # POST /pins/:id/report (auth required)
    │   ├── review_report.go      # PATCH /reports/:id (admin or owner) — actions the report, may hide the pin
    │   ├── dto.go
    │   ├── model.go
    │   └── repository.go
    │
    ├── realtime/
    │   ├── handler.go            # GET /ws upgrade + message routing
    │   ├── dto.go
    │   ├── hub.go                # connection registry, geohash rooms
    │   ├── rooms.go
    │   ├── broadcast.go          # batching logic
    │   └── pubsub.go             # Redis pub/sub, cross-instance sync
    │
    └── streams/                  # Phase 8 — scaffolded, not built yet
        ├── handler.go
        ├── webhook.go            # LiveKit participant join/leave → updates viewer_count, peak_viewer_count
        ├── dto.go
        ├── service.go
        ├── model.go
        ├── repository.go
        └── livekit_client.go
```

**Why feature-based:** each feature (auth, pins, users, reports, streams, realtime) is a self-contained folder with its own handler/service/model files, rather than splitting one feature's code across separate `handlers/`, `models/`, `db/` folders. This scales better as features are added and matches Go's `internal/` convention, which the compiler enforces (code under `internal/` can only be imported within this module).

**Why `storage/` is shared, not nested in `pins/`:** photo storage isn't inherently pins-only — user avatars or stream thumbnails could need it later. Keeping `storage.Storage` as a shared top-level interface means any feature can depend on it without duplicating upload logic or reaching into `pins/`.

**Why `categories` and `pin_photos` live inside the pins migration, not their own files:** both only exist to support pins (no independent use case), so they're created in the same migration as the table that depends on them.

**`reports/` now has a real admin-facing endpoint, not just a repository.** The original MVP plan had no admin role at all — reports were reviewed entirely by hand via direct database queries. That's changed: a lightweight `role` column (`user`/`admin`/`owner`) on `users` now gates a `PATCH /reports/:id` endpoint so admins can review and action reports in-app. The only step that still happens by hand is bootstrapping the very first owner, set directly in the database since there's intentionally no endpoint that can grant it.

---

## 6. MVP Build Order (recommended)

1. Auth (register/login, JWT) — used only to gate write actions
2. Roles (`user`/`admin`/`owner`) + `RequireAdmin`/`RequireOwner` middleware
3. Pins CRUD + PostGIS viewport query + categories (public read, gated write) + multi-photo upload via `pin_photos`
4. Reports: create (any logged-in user) + admin review endpoint (hide pin on action)
5. Basic map view (frontend) showing pins, with category filter chips
6. WebSocket layer: geohash rooms + batching for live pin updates
7. Livestreaming integration (LiveKit) — once core loop is validated
