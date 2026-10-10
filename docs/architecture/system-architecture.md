# GoodSpot247 — System Architecture

## 1. High-Level Overview

```
                                ┌─────────────────────┐
                                │      CLIENTS         │
                                │  Next.js (Web)        │
                                └──────────┬───────────┘
                                           │
                        ┌───────────────────┼───────────────────┐
                        │                   │
                  REST API (HTTPS)   SSE (`/events`, via Next rewrite)
                        │              [implemented]
                        ▼                   ▼
             ┌─────────────────┐  ┌──────────────────┐
             │   Go API Server  │  │  Go API Server    │
             │  (Gin)           │  │  realtime handler │
             │                  │  │  (SSE, Redis bus) │
             └────────┬─────────┘  └─────────┬─────────┘
                      │                      │
                      │              ┌───────┴────────┐
                      │              │  Redis Pub/Sub  │
                      │              │  (used)         │
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

### 2.3 Realtime — Server-Sent Events over Redis

Implemented as `GET /events` in the same Go binary. One global stream per process, no rooms:

- `Broker.PinCreated` publishes `pins.Event` JSON to Redis channel `goodspot:pins` (`broker.go:9,36-51`).
- Each connection subscribes; `runStreamLoop` (`realtime/handler.go:198`) filters by bbox (`matches`, handler.go:237-250) and category.
- 20 s `: heartbeat` comment keeps intermediaries alive (`handler.go:65`); capped by `MAX_SSE_CONNECTIONS` (`handler.go:78-95`); nginx disables buffering for `/events`.

Used for:

- New pin notifications in a viewport

Deferred designs: no WebSocket, no geohash-cell rooms, no batching, no outbox. The previously planned hub/rooms/ws shapes are obsolete.

### 2.4 Database — PostgreSQL + PostGIS

- Single source of truth: users, pins, pin photos, categories, streams, reports, (later) likes/comments
- Pins support multiple photos via a separate `pin_photos` table (one row per photo, ordered, with `thumbnail_url` for map/list views) rather than a single `photo_url` column
- `GIST` index on `location GEOGRAPHY(POINT, 4326)` for fast viewport/radius queries
- Scales via read replicas later if needed
- Migrations: `0001` PostGIS extension, `0002` users, `0003` categories+pins+pin_photos, `0004` reports, `0005` streams (scaffold), `0006` pin_photos.thumbnail_url

### 2.5 Storage — Local Filesystem (dev) / Cloudflare R2 (planned for production)

- **Currently:** Local filesystem storage (`internal/platform/storage/local.go`) — saves processed photos to `./uploads/`, served via `GET /uploads/*`
- **Planned (Phase 6):** Cloudflare R2 — chosen over AWS S3 specifically for zero egress fees — GoodSpot247's usage pattern (many views per photo upload) is egress-heavy, so this avoids costs scaling with popularity
- Photos uploaded from client → Go backend → validated (JPG/PNG, ≤8000×8000px) → resized (≤1600px WebP q80) + 400px square thumbnail → stored locally
- Cloudflare CDN delivery planned for production
- **R2 config env vars are documented but not yet wired** (`STORAGE_BACKEND`, `R2_*` in `.env.example` are aspirational)

### 2.6 Redis

**Currently:** JWT blacklist only — logout revokes the token's `jti` in Redis for the token's remaining TTL

**Planned:**

- **Pub/Sub** — coordinates real-time broadcasts across multiple Go instances
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

[done] → publish event to Redis
                         → Broker subscribers pick up
                         → filter (bbox/category) per connection
                         → connected viewers see it live
```

### C) User browses the map (no login required)

```
Client → GET /pins?bbox=...&category=... (Go API, public endpoint) → PostGIS bounding-box query (excludes is_hidden pins) → return pins

Client → opens EventSource GET /events?bbox=&category= → server filters by viewport bbox + category live
       → receives new pins as they appear in view
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
2. **Horizontal scaling** — run multiple Go API/Realtime instances behind a load balancer, coordinated via Redis Pub/Sub
3. **Read replicas** for Postgres if read load grows
5. (No planned split; realtime is in-process SSE over Redis — split only if profiling proves it.)

---

## 5. Repo Structure (actual)

```
sweetspot/
├── .env / .env.example              # shared secrets for docker-compose
├── docker-compose.yml               # 4 services: postgres, redis, server, client
├── SECURITY.md                      # security audit findings and controls
│
├── docs/                            # product & engineering docs (see docs/README.md for the index)
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
│   │   │   └── validid/
│   │   │       └── validid.go       # UUID format validation middleware
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
│   │   │   │   └── pins_test.go
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
│   │   │   ├── realtime/            # Phase 5 — implemented as SSE broker + handler
│   │   │   │   ├── broker.go        # Redis pub/sub bridge for pin events
│   │   │   │   └── handler.go       # GET /events SSE stream + limiter + runStreamLoop
│   │   │   └── streams/             # Phase 8 — scaffolded, not yet built
│   │   │       ├── handler.go       # (empty)
│   │   │       ├── service.go       # (empty)
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
│   │   └── imaging/
│   │       ├── imaging.go       # image validation + processing (libvips via bimg)
│   │       ├── strip.go         # EXIF stripping
│   │       └── testdata/        # GPS/orientation fixtures
│   │   └── endpointtest/
│   │       └── endpoints_test.go    # security regression tests (546 lines)
│   ├── pkg/
│   │   ├── geohash/
│   │   │   └── geohash.go           # encode lat/lng → geohash cell + neighbors
│   │   ├── jwt/
│   │   │   └── jwt.go               # generate + validate HS256 tokens with jti
│   │   ├── password/
│   │   │   └── password.go          # bcrypt hash (cost 12) + verify
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
7. ✅ SSE real-time layer: one `GET /events` stream, Redis pub/sub bus; batching and outbox are deferred (documented).
8. ⬜ Production storage swap: Cloudflare R2 (Phase 6)
9. ⬜ Deploy MVP (Phase 7)

---

## 7. Requirements, estimates, fan-out, failures and deferred items

> Merged from the former `System-Design` document (removed in this restructure; the
> exact source path is recorded in the merge commit message).
> The endpoint sketch (its §2) lives in `../api/rest-api.md`, the schema SQL (its §3)
> is summarised in §2.4 above with the full history under `server/internal/migrations/`,
> and the SSE event shape (its §2, SSE part) is covered in §2.3 above — those are not
> repeated. Everything else below is kept verbatim: requirements and capacity estimates,
> fan-out design, schema rationale (including geohash), failure handling (including the
> outbox gap), and deferred items.

## 1. Requirements

### Functional

- Users can register/login (required only to create a pin or take other interactive actions)
- Anyone (including guests, no login) can browse the map and view pin details
- Users can pin a location + upload one or more photos + select a fixed, required category
- Users can view/filter pins within their current map viewport, optionally by category
- Users receive real-time updates as new pins appear nearby — **planned (Phase 5), not yet implemented**
- Users can go live at a pinned location; others can watch + chat — **planned (Phase 8), not yet implemented**
- Admins can review reported pins and hide ones that violate policy; the owner can promote/demote admins
- (Future) Likes, comments, trending spots, sponsored pins, analytics

### Non-functional

- Low-latency real-time updates (near-instant pin/chat delivery)
- Handle concentrated concurrency (many users watching one hotspot/event)
- Horizontally scalable (no single point of failure as usage grows)
- Reasonable cost at MVP scale; scales predictably beyond it

### Back-of-envelope estimates (MVP → early growth)

| Metric                                 | Estimate                                |
| --------------------------------------- | ---------------------------------------- |
| Registered users                       | 10K → 500K                              |
| Concurrent active users                | 500 → 20K                               |
| Peak concurrent viewers on one hotspot | 100 → 5,000                             |
| Pin writes/day                         | 5K → 200K                               |
| Avg photo size                         | ~2–4MB (compressed on upload to ~500KB) |

**Why `photo_url` moved off `pins` into `pin_photos`:** MVP scope expanded to support multiple photos per pin. A single `TEXT` column can't hold more than one URL, so photos became their own table (one row per photo), joined back to `pins` via `pin_id`, ordered by `position`.

**Why `category_id` is `NOT NULL` with `ON DELETE RESTRICT`:** the PRD defines category as required. `RESTRICT` means a category can't be deleted while any pin still references it, so the "required" guarantee can never be silently broken by a category disappearing out from under existing pins.

**Why geohash column, not just PostGIS GIST index:** geohash exists from the first schema (room assignment was planned) and remains a column; the viewport path is PostGIS GIST, and SSE filtering is done in memory from parsed locations.

## 4. Real-Time Fan-Out Design — **implemented as SSE + Redis**

### Connection model

- One `GET /events?bbox=&category=` stream per open client; no subscriptions, no rooms.
- Each connection subscribes once to the Redis channel `goodspot:pins` and filters server-side.

### Broadcast flow

```
1. New pin written to Postgres
2. events.PinCreated → Broker.PinCreated → PUBLISH "goodspot:pins"
3. Every Go instance subscribed receives it
4. Each subscriber's runStreamLoop applies matches(ev, bbox, category) and drops non-matching events
5. Matching event is written as `event: pin` + JSON payload
```

### Why single stream + Redis together (replaces rooms+batching design)

- **No rooms/cells** — one global stream per process; bbox/category filtering happens per subscriber at the handler.
- **Batching** — deferred: each pin create emits one event today.
- **Redis Pub/Sub** — lets this work across multiple horizontally-scaled Go instances.

### Concurrency handling in Go

- Each SSE connection runs a write loop (`runStreamLoop`) with heartbeat ticker; writes carry a 10 s deadline (`handler.go:56-62`).
- A process-wide cap (`MAX_SSE_CONNECTIONS`, default 1000) bounds concurrency via `ConnectionLimiter` (`handler.go:67-95`).

## 6. Failure & Edge Case Handling

| Scenario                                                           | Handling                                                                                                                                                        |
| -------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Go instance crashes mid-broadcast                                  | Client `EventSource` reconnects and re-filters with the same bbox; Redis Pub/Sub means other instances unaffected. No guaranteed replay of missed events. |
| Pin saved but Redis publish fails/is skipped                        | **Phase 5, known gap:** pin exists in Postgres but never broadcasts live. Planned fix: transactional outbox table + poller |
| Photo upload fails mid-way                                          | ✅ Client retries; pin (and its `pin_photos` rows) isn't created until all photos are validated and saved — inserted in one DB transaction |
| Unauthenticated user attempts a gated action (create pin, go live) | ✅ Middleware rejects with 401 before handler logic runs; client shows login/register prompt                                                                       |
| Non-admin attempts to review a report, or non-owner attempts a role change | ✅ Middleware rejects with 403 before handler logic runs                                                                                                     |
| Viewport query on sparse data                                      | ✅ Standard bounding-box query, no special handling needed at this scale                                                                                           |
| Viewport query on dense hotspot (thousands of pins in view)        | ✅ Paginate via `LIMIT` param (default 200, max 200); marker clustering planned for frontend                                                                    |
| Redis goes down                                                    | ✅ Real-time updates degrade to single-instance-only (if only one instance up) or pause; core REST API (pins, auth) keeps working since it doesn't depend on Redis (currently Redis is only used for JWT blacklist, so logout is temporarily affected) |

## 7. What's Deliberately Deferred (not yet implemented)

- ~~WebSocket real-time layer (Phase 5)~~ — superseded by SSE (see §4)
- Cloudflare R2 storage (Phase 6) — env vars documented, not wired
- Multi-region deployment
- Dedicated microservice split (Realtime Hub as separate service from API) — start as one Go binary, split only if profiling shows a real need
- Full Kafka-style event streaming — Redis Pub/Sub is sufficient at MVP/early-growth scale
- Transactional outbox for pin broadcast events — noted as a known gap in §6, not yet implemented
