# GoodSpot247 — System Design

## 1. Requirements

### Functional

- Users can register/login (required only to create a pin or take other interactive actions)
- Anyone (including guests, no login) can browse the map and view pin details
- Users can pin a location + upload one or more photos + select a fixed, required category
- Users can view/filter pins within their current map viewport, optionally by category
- Users receive real-time updates as new pins appear nearby
- Users can go live at a pinned location; others can watch + chat
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
POST   /auth/register
POST   /auth/login

GET    /pins?bbox=lat1,lng1,lat2,lng2&category=food   — [public] pins within viewport, optional category filter (hidden pins excluded)
GET    /pins/:id                 — [public]
GET    /categories               — [public] fixed list of categories for the create-pin UI
POST   /pins                     — [auth required] create a pin (1+ photos + lat/lng + caption + category)

POST   /streams                  — [auth required] start a livestream at a pin
GET    /streams/:id              — [public] get stream info + join token
POST   /streams/:id/end          — [auth required]

GET    /users/:id                — [public]

POST   /pins/:id/report          — [auth required] report a pin (content moderation)
PATCH  /reports/:id              — [admin or owner] review a report: approve (hides the pin, sets pins.is_hidden) or dismiss
PATCH  /users/:id/role           — [owner only] promote/demote a user's role (user/admin)
```

Roles are `user` (default), `admin`, and `owner`. There is exactly one owner-bootstrap path: the first owner is set directly against the `users` table via `psql`/DBeaver — no endpoint grants it. From there, the owner promotes trusted users to admin through `PATCH /users/:id/role`. Admins handle day-to-day report review through `PATCH /reports/:id`; nothing about report review requires direct database access anymore, though it remains available as a fallback.

### WebSocket events

```
Client → Server
  subscribe:  { type: "subscribe", cells: ["u4pruy", "u4prux", ...] }
  unsubscribe:{ type: "unsubscribe", cells: [...] }
  chat:       { type: "chat", streamId, message }

Server → Client
  pin_batch:  { type: "pin_batch", cell: "u4pruy", pins: [...] }
  viewer_count: { type: "viewer_count", streamId, count }
  chat:       { type: "chat", streamId, user, message }
```

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
    name TEXT UNIQUE NOT NULL       -- fixed list: Food, Nature, Event, Nightlife, etc.
);

CREATE TABLE pins (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID REFERENCES users(id) ON DELETE SET NULL,
    location    GEOGRAPHY(POINT, 4326) NOT NULL,
    geohash     TEXT NOT NULL,           -- precomputed, for room assignment
    caption     TEXT,
    category_id INT NOT NULL REFERENCES categories(id) ON DELETE RESTRICT,  -- fixed, required category
    is_hidden   BOOLEAN NOT NULL DEFAULT false,   -- set true when an admin actions a report
    created_at  TIMESTAMPTZ DEFAULT now()
);
CREATE INDEX pins_location_idx ON pins USING GIST (location);
CREATE INDEX pins_geohash_idx ON pins (geohash);
CREATE INDEX pins_category_idx ON pins (category_id);

CREATE TABLE pin_photos (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    pin_id     UUID NOT NULL REFERENCES pins(id) ON DELETE CASCADE,
    photo_url  TEXT NOT NULL,
    position   SMALLINT NOT NULL DEFAULT 0,   -- display order; position 0 = cover photo
    created_at TIMESTAMPTZ DEFAULT now()
);
CREATE INDEX pin_photos_pin_id_idx ON pin_photos (pin_id);
CREATE UNIQUE INDEX pin_photos_pin_id_position_idx ON pin_photos (pin_id, position);

CREATE TABLE streams (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    pin_id            UUID REFERENCES pins(id) ON DELETE SET NULL,
    broadcaster_id    UUID REFERENCES users(id) ON DELETE SET NULL,
    livekit_room_name TEXT NOT NULL UNIQUE,
    status            TEXT NOT NULL CHECK (status IN ('live','ended')) DEFAULT 'live',
    peak_viewer_count INT NOT NULL DEFAULT 0,
    started_at        TIMESTAMPTZ DEFAULT now(),
    ended_at          TIMESTAMPTZ
);
CREATE INDEX streams_status_idx ON streams (status) WHERE status = 'live';

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
-- photo_url (on pin_photos) points to local filesystem in dev, Cloudflare R2 in production (see build order)
-- migration order: 0001 extensions, 0002 users, 0003 categories+pins+pin_photos, 0004 reports, 0005 streams
```

**Why `photo_url` moved off `pins` into `pin_photos`:** MVP scope expanded to support multiple photos per pin. A single `TEXT` column can't hold more than one URL, so photos became their own table (one row per photo), joined back to `pins` via `pin_id`, ordered by `position`.

**Why `category_id` is `NOT NULL` with `ON DELETE RESTRICT`:** the PRD defines category as required. `RESTRICT` means a category can't be deleted while any pin still references it, so the "required" guarantee can never be silently broken by a category disappearing out from under existing pins.

**Why geohash column, not just PostGIS GIST index:** the GIST index is for precise "pins within this bounding box" queries (used by the REST viewport endpoint). The `geohash` column is a coarser bucket used purely for **WebSocket room assignment** — cheap to group clients by, avoids running a spatial query on every broadcast.

---

## 4. Real-Time Fan-Out Design (the core scaling piece)

### Room assignment

- Map viewport is divided into geohash cells (e.g., precision 5-6, ~5km cells, tuned by zoom level)
- Client subscribes to the cells currently visible on its screen
- Server maintains: `map[geohashCell][]*Connection`

### Broadcast flow

```
1. New pin written to Postgres
2. Compute its geohash cell
3. Push event to Redis channel: pin_events:{cell}
4. Every Go instance subscribed to that channel receives it
5. Each instance batches events per cell (500ms–1s window)
6. On tick, broadcast batched pins to all local connections in that cell's room
```

### Why batching + rooms + Redis together

- **Rooms** — shrink fan-out from "all connected users" to "users actually viewing this area"
- **Batching** — collapse many rapid events into fewer broadcast operations
- **Redis Pub/Sub** — lets this work correctly across multiple horizontally-scaled Go instances (a pin written on instance A needs to reach a viewer connected to instance B)

### Concurrency handling in Go

- Each WebSocket connection read/write runs in its own goroutine
- Broadcasting to a room's connections is parallelized across goroutines (bounded via worker pool to avoid unbounded goroutine spawn under extreme load)
- `sync.RWMutex` or sharded maps protect the room registry from concurrent access

---

## 5. Livestreaming Design

```
Broadcaster taps "Go Live"
  → POST /streams { pinId }
  → Go API creates a LiveKit room, returns broadcaster token
  → Broadcaster's client connects to LiveKit via WebRTC, starts publishing

Viewer opens the stream
  → GET /streams/:id
  → Go API returns a viewer token (scoped, read-only)
  → Viewer's client connects to LiveKit, subscribes to broadcaster's tracks

Chat/reactions
  → Sent over existing WebSocket Realtime Hub (not through LiveKit)
  → Same room/batching pattern as pin broadcasts, keyed by streamId instead of geohash

Viewer count
  → LiveKit emits webhooks on participant join/leave
  → Go API updates count (and streams.peak_viewer_count if a new high), pushes via WebSocket to the stream's room
```

**Why chat goes through your own WebSocket hub, not LiveKit's data channel:** keeps chat, viewer count, and pin events all on one consistent real-time infrastructure you control and can evolve (e.g., persisting chat history to Postgres) rather than splitting logic across two systems.

---

## 6. Failure & Edge Case Handling

| Scenario                                                           | Handling                                                                                                                                                        |
| -------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Go instance crashes mid-broadcast                                  | Client WebSocket reconnects, re-subscribes to cells; Redis Pub/Sub means other instances unaffected                                                             |
| Pin saved but Redis publish fails/is skipped                        | Known gap: pin exists in Postgres but never broadcasts live. Planned fix: transactional outbox table + poller, so the DB write and the "announce it" step can't get split apart |
| Photo upload fails mid-way                                          | Client retries; pin (and its `pin_photos` rows) isn't created until all photo URLs are confirmed stored in R2 (local disk in dev) — inserted in one DB transaction |
| Unauthenticated user attempts a gated action (create pin, go live) | Middleware rejects with 401 before handler logic runs; client shows login/register prompt                                                                       |
| Non-admin attempts to review a report, or non-owner attempts a role change | Middleware rejects with 403 before handler logic runs                                                                                                     |
| Viewport query on sparse data                                      | Standard bounding-box query, no special handling needed at this scale                                                                                           |
| Viewport query on dense hotspot (thousands of pins in view)        | Paginate / limit + cluster pins client-side (marker clustering) rather than returning all points                                                                |
| LiveKit room fails to start                                        | Return error to broadcaster before they think they're live; don't create a "phantom" stream row                                                                 |
| Redis goes down                                                    | Real-time updates degrade to single-instance-only (if only one instance up) or pause; core REST API (pins, auth) keeps working since it doesn't depend on Redis |

---

## 7. What's Deliberately Deferred (not MVP)

- Multi-region deployment
- Dedicated microservice split (Realtime Hub as separate service from API) — start as one Go binary, split only if profiling shows a real need
- Full Kafka-style event streaming — Redis Pub/Sub is sufficient at MVP/early-growth scale
- Self-hosted LiveKit — start on LiveKit Cloud (managed)
- Transactional outbox for pin broadcast events — noted as a known gap in §6, not yet implemented
