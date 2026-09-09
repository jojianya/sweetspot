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
 │             │ │            │   │  only — not    │
 │             │ │            │   │  browsing)     │
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
- Pins (create, viewport query, get by id, filter by category)
- Categories (fixed list — e.g. Food, Nature, Event, Nightlife)
- Photos (upload handling → Cloudflare R2)
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

- Single source of truth: users, pins, categories, photos, streams, (later) likes/comments
- `GIST` index on `location GEOGRAPHY(POINT, 4326)` for fast viewport/radius queries
- Scales via read replicas later if needed

### 2.6 Storage — Cloudflare R2 + CDN

- Chosen over AWS S3 specifically for zero egress fees — GoodSpot247's usage pattern (many views per photo upload) is egress-heavy, so this avoids costs scaling with popularity
- Photos uploaded from client → Go backend → resized (e.g. `imaging` lib or a worker) → stored in R2
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
Client → (must have valid JWT) → POST /pins (Go API) → validate → save to Postgres/PostGIS
                                       → upload photo to R2
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
```

### C) User browses the map (no login required)

```
Client → GET /pins?bbox=...&category=... (Go API, public endpoint) → PostGIS bounding-box query → return pins
Client → opens WebSocket → subscribes to visible geohash cells
       → receives live updates as new pins appear in view
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
│   ├── 0002_users.sql
│   ├── 0003_pins.sql             # categories table + pins table (categories created first)
│   ├── 0004_reports.sql
│   └── 0005_streams.sql
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
    │   ├── model.go
    │   ├── dto.go                # PublicUser (strips PasswordHash)
    │   └── repository.go
    │
    ├── auth/
    │   ├── register.go           # POST /auth/register
    │   ├── login.go              # POST /auth/login
    │   ├── dto.go
    │   ├── service.go
    │   ├── middleware.go         # JWT auth middleware
    │   └── jwt.go
    │
    ├── pins/
    │   ├── upload_pin.go         # POST /pins (auth required) — uses storage.Storage interface
    │   ├── get_pin.go            # GET /pins, GET /pins/:id, GET /categories
    │   ├── dto.go
    │   ├── model.go              # Pin + Category structs
    │   └── repository.go
    │
    ├── reports/
    │   ├── create_report.go      # POST /pins/:id/report (auth required)
    │   ├── dto.go
    │   ├── model.go
    │   └── repository.go         # review/resolve happens via direct DB query, no HTTP handlers
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
        ├── webhook.go            # LiveKit participant join/leave
        ├── dto.go
        ├── service.go
        ├── model.go
        ├── repository.go
        └── livekit_client.go
```

**Why feature-based:** each feature (auth, pins, users, reports, streams, realtime) is a self-contained folder with its own handler/service/model files, rather than splitting one feature's code across separate `handlers/`, `models/`, `db/` folders. This scales better as features are added and matches Go's `internal/` convention, which the compiler enforces (code under `internal/` can only be imported within this module).

**Why `storage/` is shared, not nested in `pins/`:** photo storage isn't inherently pins-only — user avatars or stream thumbnails could need it later. Keeping `storage.Storage` as a shared top-level interface means any feature can depend on it without duplicating upload logic or reaching into `pins/`.

**Why `categories` lives inside the pins migration, not its own file:** categories is a small reference/lookup table that only exists to support pins (no independent use case), so it's created in the same migration as the table that depends on it.

**`reports/` covers the Content Moderation Policy's technical side** — user-submitted reports on pins, built as a first-class feature from the start rather than retrofitted later. There is no admin role or admin-only API in this system for MVP: review and resolution happen via direct database queries against the `reports` table, not through HTTP endpoints. This keeps the auth model (JWT gates writes only) simple and avoids introducing a user-role concept for a single-operator MVP.

---

## 6. MVP Build Order (recommended)

1. Auth (register/login, JWT) — used only to gate write actions
2. Pins CRUD + PostGIS viewport query + categories (public read, gated write)
3. Photo upload → local filesystem (dev) → swap to Cloudflare R2 before deployment
4. Basic map view (frontend) showing pins, with category filter chips
5. WebSocket layer: geohash rooms + batching for live pin updates
6. Livestreaming integration (LiveKit) — once core loop is validated
