# GoodSpot247 — Product Requirements Document (PRD)

## 1. Overview

**GoodSpot247** is a real-time, location-based social platform where users pin a specific location on a map and post a photo at that exact spot. Other users can explore the map and discover photos shared in specific geographic areas — turning social discovery into a place-first, map-based experience rather than a chronological feed.

## 2. Problem Statement

1. **Lack of real-time local discovery** — most social platforms are user-centric (following people), not place-centric (exploring locations).
2. **Hard to find what's happening nearby** — no easy way to see current activity in a specific geographic area.
3. **No visual, map-based social experience** — traditional feeds are chronological lists, not spatial/interactive.

## 3. Goals

- Let users pin a location and attach a photo to it
- Let users discover pins by exploring a map, not scrolling a feed
- Deliver real-time updates as new pins/activity appear nearby
- Build a foundation that supports live, in-the-moment engagement (live event pins, livestreaming) at high concurrency

## 4. Target Users

- Travelers
- Local explorers
- Event attendees
- Food hunters
- Content creators
- Small businesses

## 5. Scope

### 5.1 MVP Features

| Feature                  | Description                                                                                                                              |
| ------------------------ | ---------------------------------------------------------------------------------------------------------------------------------------- |
| Pin a location           | User selects a point on the map to attach content to                                                                                     |
| Upload a photo           | Photo tied to that specific pin                                                                                                          |
| Viewport-based discovery | View pins within the current map view                                                                                                    |
| Auth                     | Register / login required only to create a pin; browsing the map and viewing pin details is open to guests                               |
| Real-time updates        | New pins appear live via WebSocket, no refresh needed                                                                                    |
| Location-based discovery | Core browsing model is the map, not a feed                                                                                               |
| Categories               | Each pin has a fixed category (e.g. Food, Nature, Event, Nightlife), for filtering/discovery                                             |
| Content reporting        | Users can report a pin; no admin role in the system — reports are reviewed directly against the database (baseline moderation mechanism) |

### 5.2 Future Features (post-MVP)

| Feature                 | Description                                                            |
| ----------------------- | ---------------------------------------------------------------------- |
| Likes & comments        | Engagement on individual pins                                          |
| Trending spots          | Surface locations with high recent activity                            |
| Live event pins         | Real-time pins tied to live happenings (high-concurrency broadcast)    |
| Livestreaming           | Users broadcast live video from a pinned location; others watch + chat |
| Business/sponsored pins | Paid placement for businesses                                          |
| Analytics               | Engagement data for locations/businesses                               |

### 5.3 Out of Scope (for now)

- Multi-region infrastructure
- Recorded/replay video (VOD) for streams
- Monetization/payments beyond sponsored pin placeholders
- Native desktop apps

## 6. User Stories

- _As a traveler_, I want to see what other people have pinned nearby, so I can discover interesting spots without prior research.
- _As a content creator_, I want to pin a photo at a specific location, so my content is discoverable by anyone exploring that area.
- _As an event attendee_, I want to see live activity at an event in real time, so I know what's happening right now, not what happened hours ago.
- _As a small business_, I want to eventually promote a sponsored pin at my location, so I can attract nearby users.
- _As any user_, I want new pins to appear on my map automatically while I'm browsing, without needing to refresh.

## 7. Success Metrics (suggested)

- # of pins created / day
- # of active viewport sessions / day (map opens + browsing duration)
- Real-time update latency (pin post → visible to nearby viewers)
- Retention: % of users who pin or view again within 7 days
- (Post-launch, live features) peak concurrent viewers per hotspot handled without degradation

## 8. Technical Constraints & Direction

- Backend: **Go** — chosen for real-time concurrency demands (live pins, livestreaming) over Node/Express; see [[goodspot247]] architecture notes for full reasoning
- Database: PostgreSQL + PostGIS for geospatial queries
- Real-time: WebSocket-based, geohash-partitioned rooms, batched broadcasts, Redis pub/sub for horizontal scaling
- Livestreaming (future): WebRTC via LiveKit (Go-based SFU)
- Frontend: Next.js (web) + React Native (mobile), MapLibre GL for map rendering

## 9. Open Questions

- Photo moderation approach beyond user reporting (automated pre-screening) — not yet defined; MVP ships with user reports reviewed directly via the database (no admin role/endpoints) as the baseline
- Monetization model for sponsored/business pins — not yet defined
- Whether livestreaming ships as MVP+1 or later, pending validation of core pin/discovery loop first
