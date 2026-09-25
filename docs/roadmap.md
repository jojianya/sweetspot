# Roadmap — GoodSpot247 ("Goodspot") · Checklist

Checklist version of the roadmap from the codebase review (Sep 2026). Items A–K.
Legend: `[ ]` = not started, `[i]`-marked = in progress, `[x]` = done.
Progress: **5 of 11 done** (1 in progress, 5 todo).

## 1. New features users would likely want

- [x] **A. Admin moderation console (reports + role management)** — done · Medium
  - **What:** A `Reports` screen (visible to `admin`/`owner` only) listing pending reports with approve/dismiss, plus a small "Report" action in the pin detail panel, and an owner-only role-management screen can follow.
  - **Why:** The entire backend exists (`GET /reports`, `PATCH /reports/:id`, `PATCH /users/:id/role`) with tests — the client has zero references to reporting. Moderation is currently unusable from the app, and it's a Phase 7 pre-launch blocker. Without it, flagging content requires direct DB access.
  - **Files:** new `client/src/app/reports/page.tsx`, `client/src/lib/api/reports.ts`, per-pin report button in `PinDetailPanel.tsx`/`PinPageClient.tsx`; no server changes needed. *(Notes — implemented Sep 2026: added the owner-only role-management screen at `client/src/app/roles/page.tsx` plus a small owner-gated `GET /users?q=` search endpoint in `server/internal/modules/user/` to power it; the report action also opens a new `ReportSheet`.)*

- [x] **B. Profile editing (avatar, username, socials)** — done · Medium
  - **What:** `PATCH /users/me` (own-profile only) to change username/socials + avatar upload through the existing storage/imaging pipeline.
  - **Why:** `users.avatar_url` and `socials` (jsonb) exist in the schema and are returned in profiles, but there's no endpoint or UI to set them — `avatar_url` is always `null`. Profiles are the identity hub of the social layer; today every avatar defaults to initials and `socials` is dead data.
  - **Files:** `server/internal/modules/user/{handler,service,repository,dto,routes}.go`, `client/src/app/users/[id]/page.tsx` (or a new settings page), avatar upload reusing `pins/imaging`. *(Notes — implemented Sep 2026: `PATCH /users/me` (multipart, `AuthRequired`) accepts any subset of `username`, `socials` (JSON object), and `avatar` (jpg/png, ≤5MB); avatars run through a new `imaging.Avatar` (256px square webp) and are stored via the existing `Local` store, with the superseded file deleted; the repo maps a duplicate username to a 409. Client: the profile page gets an "Edit profile" button (own profile only) with avatar preview, username, and Instagram/Twitter/Website fields (socials merged with existing keys, no-op saves skipped), and the profile header now renders socials as links. Registered on the 64MB upload route group because the payload is multipart.)*

- [ ] **C. Delete-pin UI** — todo · Small
  - **What:** A confirm-to-delete action in `PinEditSheet.tsx` (owner only).
  - **Why:** `DELETE /pins/:id` and the client `deletePin()` both exist, but no component calls it — a user can't remove their own pin from the app. Small, closes an already-built API, avoids "stuck" test content.
  - **Files:** `client/src/components/pins/PinEditSheet.tsx`.

- [x] **D. Trending spots (viewport-level)** — done · Medium (build it after pin views lands)
  - **What:** "Trending around here" — recent pins in the viewport ranked by views/comments/age, surfaced as a small list or highlight layer.
  - **Why:** Listed as a future feature in the PRD and it's the app's core thesis (place-first discovery). The in-flight `pin_views` counter gives you the raw material.
  - **Files:** new SQL in `pins/repository.go`, reuse of `0012_pin_views.sql`, `client/src/components/pins/TrendingList.tsx`. *(Notes — implemented Sep 2026: public `GET /pins/trending?bbox=&limit=` returns the shared list-entry shape plus `comment_count` and a hotness `score = (views + 5*comments) / (age_hours + 2)` so fresh pins stay competitive; bbox validation was extracted into a shared `parseBbox` helper; client adds `useTrending` + a toggleable `TrendingList` overlay on `MapApp` that re-fetches per viewport and flies to/reveals the tapped pin.)*

## 2. Gaps in what already exists

- [ ] **E. Tests for the newer modules** — todo · Medium
  - **What:** Handler/service-level tests matching the existing `*_test.go` pattern; a few hook/component tests (e.g. `usePinStream`, `PinDetailPanel` save/share behavior) using the existing vitest setup.
  - **Why:** Server unit tests cover auth, user, reports, favorites, pins, and endpoint security — but nothing for comments, collections, or social/feed, the three newest server modules. The client has only 3 test files (auth store, `format`, `geo`) — no component/hook tests at all. These modules are the least-tested and most recently added — the highest regression risk.
  - **Files:** `server/internal/modules/{comments,collections,social}/` test files, `client/src/hooks/*.test.ts(x)`.

- [x] **F. Accessibility pass on overlay UI** — done · Medium
  - **What:** Add initial-focus/Escape/focus-restore to `PanelSheet.tsx` and `PhotoLightbox.tsx`; a keyboard-accessible pin list fallback.
  - **Why:** `PanelSheet` has no focus management (no focus trap, no Escape-to-close, no focus restore); the photo lightbox likely shares this. Map markers aren't keyboard-operable. The app is one giant interactive map — currently near-unusable without a mouse, and sheets trap keyboard users once opened.
  - **Files:** `client/src/components/PanelSheet.tsx`, `pins/PhotoLightbox.tsx`, `map/MapView.tsx`. *(Notes — implemented Sep 2026: new shared `client/src/hooks/useDialogFocus.ts` (initial focus on the first focusable control, Tab/Shift+Tab trap, Escape-triggers-`onClose`, focus restored on unmount) wired into `PanelSheet` so all five sheets get it, and into `PhotoLightbox`; `MapView` renders a hidden-but-focusable pin list as the keyboard route to the canvas-drawn markers; `globals.css` adds a theme-aware `:focus-visible` outline and a `prefers-reduced-motion` fallback; removed `PinEditSheet`'s redundant global Escape listener.)*

- [x] **G. Monitoring + error tracking** — done · Small–Medium
  - **What:** Add an error-reporting hook downstream of the existing logger (`internal/observability/logger`) and the client `ErrorBoundary`.
  - **Why:** Checklist Phase 7 explicitly calls for uptime monitoring and Sentry; neither exists. When the app is live, you'll be debugging blind — no way to see broken requests or client crashes.
  - **Files:** `server/cmd/api/main.go` + logger, `client/src/components/ErrorBoundary.tsx`, `client/src/lib/api/client.ts`. *(Notes — implemented Sep 2026: new `server/internal/observability/report` package is the single downstream hook — always logs through slog at error level, and forwards to Sentry when `SENTRY_DSN` is set (`SENTRY_ENV` defaults to `development`; log-only otherwise, identical to today). `middleware.Recover` now reports panics (with request context) instead of `gin.Recovery()`; a new `middleware.ReportErrors` reports 5xx responses, and `response.Internal` now attaches the error to `c.Errors` so reports carry the underlying cause. Client crashes are ingested via a new public best-effort `POST /errors` (always answers 204, payload-capped). Client: `client/src/lib/monitoring.ts` throttles (10s dedupe per kind/url/message, 100 events/session) and fire-and-forget POSTs reports; wired into `ErrorBoundary.componentDidCatch`, the axios 5xx interceptor in `client.ts` (`API_BASE_URL` moved into `monitoring.ts` and re-exported — no import cycle), and window-level `error`/`unhandledrejection` handlers installed by a new `RuntimeErrorReporter` component in the root layout. Uptime monitoring needs no code: `GET /health` already exists for UptimeRobot. Tests: `middleware/report_test.go` (5xx/panic reporting), `report_test.go` smoke test, `TestClientErrorIngest`, `monitoring.test.ts` (5 vitest tests).)*

- [ ] **H. Dependency/secret scanning in CI** — todo · Small
  - **What:** Add a vuln-scan step to `.github/workflows/ci.yml` (and optionally a secrets scanner).
  - **Why:** CI runs build/vet/test/lint/typecheck/build but no `govulncheck ./...` or `npm audit` — both are checklist pre-launch items, and there's a history of secrets having been committed (now rotated). Cheap, catches supply-chain issues before launch rather than after.
  - **Files:** `.github/workflows/ci.yml` only.

## 3. Small quick wins

- [ ] **I. Finish and land the in-flight pin-views work** — in progress · Small
  - **What:** `0012_pin_views.sql`, `usePinView.ts`, `POST /pins/:id/view` and the `TestPinView` test are written but uncommitted — wire it into the pin pages (live view counter + "viewed recently") and it unblocks trending.
  - **Why:** Already written on disk; landing it closes the loop and unblocks item D.
  - **Files:** the diff already on disk.

- [ ] **J. Sync the stale docs** — todo · Small
  - **What:** Update `Architecture.md`, `API.md`, and `TECH_STACK.md` to match the code.
  - **Why:** They still describe favorites/comments/collections/social/feed/SSE/pin-edit as unbuilt, describe realtime as WebSocket (it's SSE), and list migrations only up to `0006` (there are 12).
  - **Files:** `project_documents/*`, `TECH_STACK.md`.

- [ ] **K. Trivial cleanups** — todo · Trivial
  - **What:** Remove the stray "testtt" comment in `client/src/components/layout/Navbar.tsx` line 11; delete the dead `CreatePinRequest` JSON DTO in `server/internal/modules/pins/dto.go`; check whether `client/tsconfig.tsbuildinfo` / `next-env.d.ts` are tracked and gitignore them if so.
  - **Why:** Housekeeping; no behavior change.
  - **Files:** `client/src/components/layout/Navbar.tsx`, `server/internal/modules/pins/dto.go`, `.gitignore`.