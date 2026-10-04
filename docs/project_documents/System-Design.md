# GoodSpot247 — System Design

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

---

## 2. API Design

### REST endpoints

```
POST   /auth/register           — [rate-limited] create account, returns JWT
POST   /auth/login              — [rate-limited] authenticate, returns JWT
POST   /auth/logout             — [auth required] blacklist JWT

GET    /pins?bbox=lat1,lng1,lat2,lng2&category=1       — [public] pins within viewport, optional numeric category ID filter (hidden pins excluded)
GET    /pins/:id                — [public, owner/admin for hidden pins]
GET    /categories              — [public] fixed list of categories for the create-pin UI
POST   /pins                    — [auth required, rate-limited] create a pin (1–5 photos via multipart + lat/lng + caption + category)

GET    /users/:id               — [optional auth] public profile (email only visible to the account owner)
PATCH  /users/:id/role          — [owner only] promote/demote a user's role (user/admin/owner)

POST   /pins/:id/report         — [auth required] report a pin (content moderation)
GET    /reports                  — [admin or owner] list reports (filters + pagination)
PATCH  /reports/:id             — [admin or owner] review a report: approve (hides the pin) or dismiss

GET    /health                  — liveness + DB connectivity check
GET    /uploads/*               — static photo files (public)

POST   /streams                 — [Phase 8, planned] start a livestream at a pin
GET    /streams/:id             — [Phase 8, planned] get stream info + join token
POST   /streams/:id/end         — [Phase 8, planned]
```

Roles are `user` (default), `admin`, and `owner`. There is exactly one owner-bootstrap path: the first owner is set directly against the `users` table via `psql`/DBeaver — no endpoint grants it. From there, the owner promotes trusted users to admin through `PATCH /users/:id/role`. Admins handle day-to-day report review through `PATCH /reports/:id`; nothing about report review requires direct database access anymore, though it remains available as a fallback.

### SSE events — **implemented**

```
Client → GET /events?bbox=...&category=...
Server → Client:  event: pin\ndata: <pins.Event JSON>\n\n
Server → Client:  : heartbeat\n\n   (every 20 s)
```

Each connection filters server-side: category equality, then bbox range match via `matches` (handler.go:237-250).

---

## 3. Database Schema

```sql
CREATE EXTENSION IF NOT EXISTS postgis;

CREATE TABLE users (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email         TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    username      TEXT UNIQUE NOT NULL,
    avatar_url    TEXT,
    socials       JSONB DEFAULT '{}',
    role          TEXT NOT NULL DEFAULT 'user' CHECK (role IN ('user', 'admin', 'owner')),
    created_at    TIMESTAMPTZ DEFAULT now(),
    updated_at    TIMESTAMPTZ DEFAULT now()
);
-- updated_at kept current via a BEFORE UPDATE trigger (set_updated_at)

CREATE TABLE categories (
    id   SERIAL PRIMARY KEY,
    name TEXT UNIQUE NOT NULL       -- fixed list (finalized): Food, Nature, Event, Nightlife, Art, Sports, Travel, Other
);

CREATE TABLE pins (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID REFERENCES users(id) ON DELETE SET NULL,
    location    GEOGRAPHY(POINT, 4326) NOT NULL,
    geohash     TEXT NOT NULL,           -- precomputed, for room assignment (Phase 5)
    caption     TEXT,
    category_id INT NOT NULL REFERENCES categories(id) ON DELETE RESTRICT,  -- fixed, required category
    is_hidden   BOOLEAN NOT NULL DEFAULT false,   -- set true when an admin actions a report
    created_at  TIMESTAMPTZ DEFAULT now()
);
CREATE INDEX pins_location_idx ON pins USING GIST (location);
CREATE INDEX pins_geohash_idx ON pins (geohash);
CREATE INDEX pins_category_idx ON pins (category_id);

CREATE TABLE pin_photos (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    pin_id        UUID NOT NULL REFERENCES pins(id) ON DELETE CASCADE,
    photo_url     TEXT NOT NULL,
    thumbnail_url TEXT,                  -- added in migration 0006; 400px square thumbnail for map/list views
    position      SMALLINT NOT NULL DEFAULT 0,   -- display order; position 0 = cover photo
    created_at    TIMESTAMPTZ DEFAULT now()
);
CREATE INDEX pin_photos_pin_id_idx ON pin_photos (pin_id);
CREATE UNIQUE INDEX pin_photos_pin_id_position_idx ON pin_photos (pin_id, position);

-- Migrations 0005_streams.sql + 0015 (planned, no code behind them yet):
-- CREATE TABLE streams (
--     id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
--     pin_id            UUID REFERENCES pins(id) ON DELETE SET NULL,
--     broadcaster_id    UUID REFERENCES users(id) ON DELETE SET NULL,
--     status            TEXT NOT NULL CHECK (status IN ('live','ended')) DEFAULT 'live',
--     peak_viewer_count INT NOT NULL DEFAULT 0,
--     started_at        TIMESTAMPTZ DEFAULT now(),
--     ended_at          TIMESTAMPTZ
-- );
-- CREATE INDEX streams_status_idx ON streams (status) WHERE status = 'live';
-- 0015 dropped the identifier column 0005 had added, which named a specific
-- media vendor and was never read or written by any code.

CREATE TABLE reports (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    pin_id      UUID REFERENCES pins(id) ON DELETE CASCADE,
    reporter_id UUID REFERENCES users(id) ON DELETE SET NULL,
    reason      TEXT NOT NULL,
    status      TEXT NOT NULL CHECK (status IN ('pending','reviewed','actioned')) DEFAULT 'pending',
    resolved_by UUID REFERENCES users(id) ON DELETE SET NULL,   -- which admin/owner actioned it
    resolved_at TIMESTAMPTZ,
    created_at  TIMESTAMPTZ DEFAULT now(),
    UNIQUE (pin_id, reporter_id)   -- one report per user per pin
);

-- future tables: likes, comments, sponsored_pins, analytics_events
-- photo_url / thumbnail_url on pin_photos points to local filesystem in dev, Cloudflare R2 in production (see build order)
-- migration order: 0001 extensions, 0002 users, 0003 categories+pins+pin_photos, 0004 reports, 0005 streams (scaffold), 0006 pin_photos.thumbnail_url
```

**Why `photo_url` moved off `pins` into `pin_photos`:** MVP scope expanded to support multiple photos per pin. A single `TEXT` column can't hold more than one URL, so photos became their own table (one row per photo), joined back to `pins` via `pin_id`, ordered by `position`.

**Why `category_id` is `NOT NULL` with `ON DELETE RESTRICT`:** the PRD defines category as required. `RESTRICT` means a category can't be deleted while any pin still references it, so the "required" guarantee can never be silently broken by a category disappearing out from under existing pins.

**Why geohash column, not just PostGIS GIST index:** geohash exists from the first schema (room assignment was planned) and remains a column; the viewport path is PostGIS GIST, and SSE filtering is done in memory from parsed locations.

---

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

---

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

---

## 7. What's Deliberately Deferred (not yet implemented)

- ~~WebSocket real-time layer (Phase 5)~~ — superseded by SSE (see §4)
- Cloudflare R2 storage (Phase 6) — env vars documented, not wired
- Multi-region deployment
- Dedicated microservice split (Realtime Hub as separate service from API) — start as one Go binary, split only if profiling shows a real need
- Full Kafka-style event streaming — Redis Pub/Sub is sufficient at MVP/early-growth scale
- Transactional outbox for pin broadcast events — noted as a known gap in §6, not yet implemented
