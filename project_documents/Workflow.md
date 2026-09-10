# GoodSpot247 — User Workflow

This describes the end-to-end path a person takes through the app, from opening it as a guest to seeing their pin go live for others. It complements the PRD's user stories (§6) and Architecture.md's data flow examples (§3) by laying out the full journey in one place.

---

## 1. Guest browsing (no login required)

1. **Open the app** — no account needed. This is the default entry point for everyone, per PRD §5.1 ("Auth ... browsing the map and viewing pin details is open to guests").
2. **Browse the map** — the client fetches pins for the current viewport via `GET /pins?bbox=...&category=...` (public endpoint, hidden pins excluded).
3. **View pin details** — tapping a marker opens a detail view (photo(s), caption, category, user, timestamp). Still no auth required.
4. **Filter by category** — optional, via `GET /categories` + chip selection; re-fetches pins with `&category=...`.
5. **Live updates while browsing** — the client opens a WebSocket connection, subscribes to the visible geohash cells, and receives `pin_batch` messages as new pins appear nearby — no refresh needed.

At this point, a guest can fully explore the app with zero commitment.

---

## 2. Hitting the login gate

6. **Tap "+" to add a pin** — the first point where auth is required.
7. **Branch on auth state:**
   - **Not logged in** → shown a login/register prompt instead of the pin form.
   - **Already logged in** → goes straight to the create-pin form.
8. **Register or log in** (if needed) — `POST /auth/register` or `POST /auth/login`, JWT stored client-side (memory + secure storage/httpOnly cookie). New accounts default to `role = 'user'`.

This is the only mandatory gate for regular use of the app — everything before it, and the reporting flow below, are the only other places auth matters.

---

## 3. Creating a pin

9. **Fill out the create-pin form** — photo picker (one or more photos), category select, optional caption, plus the tapped lat/lng.
10. **Submit** — `POST /pins` with the JWT in the `Authorization` header, as `multipart/form-data` (photos + fields together).
11. **Server-side validation** — lat/lng range, category exists, each file's type/size checked, photo count checked against the max, before anything is saved.
12. **Pin saved** — pin row and its `pin_photos` rows written to Postgres/PostGIS in one transaction; photos stored (local disk in dev, Cloudflare R2 in production).
13. **Event published** — the new pin's geohash cell is pushed to Redis; the Realtime Hub picks it up and batches it into the next broadcast window (500ms–1s).
14. **Pin appears live** — the creator sees it immediately (optimistic UI update); anyone else with that geohash cell in view sees it appear via the WebSocket `pin_batch` message, no refresh needed.

---

## 4. Reporting a pin (secondary flow, requires login)

15. **Report a pin** — any logged-in user can flag a pin via `POST /pins/:id/report` with a reason (one report per user per pin).
16. **Stored for review** — the report lands in the `reports` table (`pending` status).
17. **Reviewed by an admin, in-app** — a user with `role = admin` or `role = owner` reviews pending reports and actions them via `PATCH /reports/:id`:
    - **Approve** → the reported pin is hidden (`pins.is_hidden = true`), the report is marked `actioned`, and `resolved_by`/`resolved_at` are recorded.
    - **Dismiss** → the report is marked `reviewed`, `resolved_by`/`resolved_at` recorded, the pin is untouched.
    - This replaces the original MVP plan of reviewing every report by hand via `psql`/DBeaver — that path still exists as a fallback, but is no longer the primary flow now that roles exist. See Detailed-Checklist.md §2.5.3 and System-Design.md for the fuller rationale.

---

## 5. Becoming an admin (one-time setup, not a normal user flow)

18. **The first owner is set manually** — directly in the database (`UPDATE users SET role = 'owner' WHERE email = ...`), since no signup flow or endpoint can grant it. This is a deliberate, one-time operational step, not something that happens through the app.
19. **The owner promotes trusted users to admin** — via `PATCH /users/:id/role`, in-app, any time after that. Only the owner can do this.

---

## 6. Future workflow extensions (post-MVP, not built yet)

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
                                   Fill pin form (1+ photos) & submit
                                              │
                                              ▼
                                   Pin + photos saved + published to Redis
                                              │
                                              ▼
                                   Live on other users' maps

(Separate, optional path — any logged-in user, any time)
View pin ──► Report pin ──► Stored in `reports` table ──► Admin reviews via PATCH /reports/:id ──► Pin hidden (approved) or report dismissed

(One-time setup, not a user-facing flow)
Operator sets first owner directly in DB ──► Owner promotes users to admin via PATCH /users/:id/role
```
