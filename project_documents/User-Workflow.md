# GoodSpot247 — User Workflow

This describes the end-to-end path a person takes through the app, from opening it as a guest to seeing their pin go live for others. It complements the PRD's user stories (§6) and Architecture.md's data flow examples (§3) by laying out the full journey in one place.

---

## 1. Guest browsing (no login required)

1. **Open the app** — no account needed. This is the default entry point for everyone, per PRD §5.1 ("Auth ... browsing the map and viewing pin details is open to guests").
2. **Browse the map** — the client fetches pins for the current viewport via `GET /pins?bbox=...&category=...` (public endpoint).
3. **View pin details** — tapping a marker opens a detail view (photo, caption, category, user, timestamp). Still no auth required.
4. **Filter by category** — optional, via `GET /categories` + chip selection; re-fetches pins with `&category=...`.
5. **Live updates while browsing** — the client opens a WebSocket connection, subscribes to the visible geohash cells, and receives `pin_batch` messages as new pins appear nearby — no refresh needed.

At this point, a guest can fully explore the app with zero commitment.

---

## 2. Hitting the login gate

6. **Tap "+" to add a pin** — the first point where auth is required.
7. **Branch on auth state:**
   - **Not logged in** → shown a login/register prompt instead of the pin form.
   - **Already logged in** → goes straight to the create-pin form.
8. **Register or log in** (if needed) — `POST /auth/register` or `POST /auth/login`, JWT stored client-side (memory + secure storage/httpOnly cookie).

This is the only mandatory gate in the whole app — everything before it and the reporting flow below are the only other places auth matters.

---

## 3. Creating a pin

9. **Fill out the create-pin form** — photo picker, category select, optional caption, plus the tapped lat/lng.
10. **Submit** — `POST /pins` with the JWT in the `Authorization` header, as `multipart/form-data` (photo + fields together).
11. **Server-side validation** — lat/lng range, category exists, file type/size checked before anything is saved.
12. **Pin saved** — written to Postgres/PostGIS, photo stored (local disk in dev, Cloudflare R2 in production).
13. **Event published** — the new pin's geohash cell is pushed to Redis; the Realtime Hub picks it up and batches it into the next broadcast window (500ms–1s).
14. **Pin appears live** — the creator sees it immediately (optimistic UI update); anyone else with that geohash cell in view sees it appear via the WebSocket `pin_batch` message, no refresh needed.

---

## 4. Reporting a pin (secondary flow, requires login)

15. **Report a pin** — any logged-in user can flag a pin via `POST /pins/:id/report` with a reason.
16. **Stored for review** — the report lands in the `reports` table (`pending` status).
17. **Reviewed directly via database** — there is no admin role or admin API in this system for MVP. Reports are checked and resolved by querying the `reports` table directly (`psql`/DBeaver), on a regular cadence. See Detailed-Checklist.md §2.5.3 and System-Design.md for the full rationale.

---

## 5. Future workflow extensions (post-MVP, not built yet)

- **Going live**: `POST /streams` → LiveKit room created → broadcaster connects via WebRTC; viewers hit `GET /streams/:id` for a join token; chat/reactions ride the existing WebSocket hub keyed by `streamId` instead of geohash.
- **Likes & comments**: engagement layered onto individual pins.
- **Sponsored pins**: paid placement, separate from the organic pin flow.

---

## Summary diagram (text form)

```
Guest opens app
      │
      ▼
Browse map (public) ──► View pin detail (public) ──► Filter by category (public)
      │                                                        │
      │◄───────────── live pin updates via WebSocket ──────────┘
      │
      ▼
Tap "+" to add a pin
      │
      ├── Not logged in ──► Register/login ──┐
      │                                      │
      └── Already logged in ─────────────────┤
                                              ▼
                                   Fill pin form & submit
                                              │
                                              ▼
                                   Pin saved + published to Redis
                                              │
                                              ▼
                                   Live on other users' maps

(Separate, optional path — any logged-in user, any time)
View pin ──► Report pin ──► Stored in `reports` table ──► Reviewed via direct DB query
```
