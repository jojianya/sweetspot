# GoodSpot247 — Code Review Checklist

Full review of the `sweetspot` monorepo, prioritized **P0 (critical) → P3 (polish)**. Every item cites a
file/line and a suggested fix. Check items off as you work through them.

**Stack:** Next.js 16.3.4 / React 19.2.8 client (`client/`), Go 1.27 + Gin backend (`server/`),
PostgreSQL + PostGIS, Redis (pub/sub + JWT blacklist).

**Verification baseline at time of review:** `go vet` clean · `go build` clean · `go test ./...` pass ·
`tsc --noEmit` clean · `vitest run` 50/50 pass · `eslint` 0 errors / 1139 warnings.
Everything compiles — the findings below are logic, security, and robustness issues, not build failures.

**Totals:** 4 critical · 10 high · 24 medium · 23 low/polish

---

## How to use this document

- Sections are ordered by severity, not by module. Fix P0 before starting P1.
- Each item is a checkbox. Tick it, and if the fix is non-obvious add a short note or PR link.
- "Verified clean" at the bottom lists what was checked and found **not** to be a problem — don't re-litigate those.

---

## P0 — Critical

### P0.1 HTTP server has no read/idle timeouts → Slowloris DoS

- [ ] **Add `ReadHeaderTimeout` and `IdleTimeout` to the server** — `server/internal/app/app.go:31-34`
  ```go
  srv := &stdhttp.Server{
      Addr:    ":" + cfg.Port,
      Handler: router,
  }
  ```
  All three timeouts are zero = infinite. A handful of connections that never finish their headers pin a
  goroutine each and exhaust the server. `BodyLimit` doesn't help — it only caps body bytes on requests
  that already arrived.
- [ ] Set `ReadHeaderTimeout: 5 * time.Second`
- [ ] Set `IdleTimeout: 60 * time.Second`
- [ ] Do **not** set `WriteTimeout` blindly — it would kill the SSE endpoint at `/events` (`router.go:73`).
      Bound SSE instead with a per-write `http.ResponseController.SetWriteDeadline`.

### P0.2 Process exits 0 when the server fails → orchestrators never restart it

- [ ] **`os.Exit(1)` on server error** — `server/cmd/api/main.go:32-34`
  ```go
  if err := app.Run(cfg, pool, rep); err != nil {
      lg.Error("server error", "error", err.Error())
  }
  ```
  Falls through to a clean `main` return → exit code 0. Docker `restart: unless-stopped` and any k8s
  probe treat the crash as a normal exit.
- [ ] Replace both `panic(err)` blocks (`main.go:20-24` DB connect, `main.go:27-30` migrations) with the
      same `lg.Error(...) + os.Exit(1)` so no Go panic trace is printed in production.

### P0.3 Role escalation window: admin rights live in the JWT for 30 days

- [ ] **Stop authorizing on the stale JWT role** — `server/internal/modules/auth/service.go:14`
      (`tokenExpiry = 30 * 24 * time.Hour`) + `server/internal/http/middleware/auth.go:50`
      (`c.Set(CtxRole, claims.Role)`)

  The role is baked into the token at login and never re-read. Two authorization paths disagree:

  | Path | Source of truth | Demoted admin |
  |---|---|---|
  | `users.RequireAdmin` / `RequireOwner` (`internal/modules/user/routes.go:38-64`) | live `GetByID` DB read | blocked ✅ |
  | `pins.Handler.canViewHidden` (`internal/modules/pins/handler.go:173`) | `middleware.GetRole(c)` → JWT | **still authorized** ❌ |
  | `pins.Handler.Update` (`internal/modules/pins/handler.go:502`) | `middleware.GetRole(c)` → JWT | **still authorized** ❌ |
  | `comments.isModerator` (`internal/modules/comments/handler.go:24-27`) | `middleware.GetRole(c)` → JWT | **still authorized** ❌ |

  A user demoted from `admin` to `user` keeps pin moderation, hidden-pin visibility, and comment
  moderation for up to 30 days. The only mitigation is `useSessionRefresh` on the *client* — cosmetic
  and trivially bypassed.
- [ ] Make `canViewHidden`, `Update`, and `isModerator` consult the DB role (or drop the `role` claim
      from the JWT entirely and route every moderator check through `users.CurrentRole`, which exists)
- [ ] Additionally shorten `tokenExpiry` to ~24h and add refresh-token rotation (narrows the window,
      does not close it)

### P0.4 Password length limit counts runes, bcrypt counts bytes → silent truncation

- [ ] **Validate password length in bytes, not runes** — `server/internal/modules/auth/dto.go`
      (`binding:"min=8,max=72"`) + `server/pkg/password/password.go`

  `go-playground/validator` measures strings in **runes**; bcrypt truncates at **72 bytes**. A 72-rune
  password of non-ASCII (e.g. `"é".repeat(72)` = 144 bytes) passes validation and is silently cut to its
  first 72 bytes. Users set a password, and part of it becomes the actual credential.
- [ ] Use `utf8.RuneCountInString(p) >= 8 && len(p) <= 72`
- [ ] Or pre-hash with SHA-256 before bcrypt (the standard fix) and keep the 72-byte rule

---

## P1 — High

### P1.1 `ListTrending` aggregates the entire comments table on every request

- [ ] **Replace the uncorrelated aggregate join** —
      `server/internal/modules/pins/repository.go:249-252`
      ```sql
      LEFT JOIN (
          SELECT pin_id, COUNT(*) AS comment_count
          FROM comments WHERE is_hidden = false GROUP BY pin_id
      ) ...
      ```
      This subquery is uncorrelated — Postgres scans and hash-groups the whole `comments` table before
      the join, then throws away all but ≤25 rows. Cost grows linearly with total comments, forever.
      Runs on every trending panel open.
- [ ] Invert to `LEFT JOIN LATERAL (SELECT COUNT(*) FROM comments WHERE pin_id = p.id AND is_hidden = false)`
      — uses the existing `comments_pin_idx`, evaluated only for candidate rows
- [ ] (Alternative) maintain a denormalized `comment_count` column on `pins`

### P1.2 `pins` has no index supporting `ORDER BY created_at DESC`

- [ ] **Add the missing sort index** — `server/internal/platform/database/migrations/0003_pins.sql`
      (GIST on `location` only)

      `ListPins` (`repository.go:217`), `ListByUser` (`:242`), `Feed`, and `SearchPins` all
      `ORDER BY p.created_at DESC LIMIT 200`. The GIST index filters the bbox but cannot produce the
      sort order, so Postgres does a full sort of every matching row.
- [ ] New migration: `CREATE INDEX pins_created_at_idx ON pins (created_at DESC);`
- [ ] New migration: `CREATE INDEX pins_visible_created_idx ON pins (created_at DESC) WHERE is_hidden = false;`
      (every read path filters `is_hidden = false`)

### P1.3 Bbox filter uses `ST_DWithin(..., 0)` — obscure and index-hostile

- [ ] **Switch to the documented bbox predicate** —
      `server/internal/modules/pins/repository.go:217` and `:259`
      ```sql
      AND ST_DWithin(p.location, ST_MakeEnvelope($1,$2,$3,$4,4326)::geography, 0)
      ```
      Functionally correct (distance from an interior point to the containing polygon is 0) but nobody
      reading it will know that, and PostGIS's geography distance path with distance 0 will not always
      pick the GiST index.
- [ ] Replace with `AND ST_Intersects(p.location, ST_MakeEnvelope($1,$2,$3,$4,4326)::geography)`
      — same semantics, obviously correct, reliably index-backed

### P1.4 `ListUsers` runs a full `COUNT(*)` on every page

- [ ] **Fold the count into the page query** —
      `server/internal/modules/user/repository.go:138-141`, called from `users.Handler.List`
      alongside `ListUsers`. Two extra round-trips per page of the owner console.
- [ ] Use `SELECT ..., COUNT(*) OVER () AS total FROM users ... LIMIT $1 OFFSET $2` — one query, no
      extra scan
- [ ] Keep the separate count only when the page comes back empty (to distinguish "no results" from
      "past the end")

### P1.5 Axios interceptor throws away the HTTP status, forcing string-matching for control flow

- [ ] **Preserve the status code on rejections** — `client/src/lib/api/client.ts:38-43`
      ```ts
      const serverMessage = error?.response?.data?.error;
      return Promise.reject(new Error(message));
      ```
      `error.response.status` is discarded. Callers then regex the message —
      `client/src/app/users/[id]/page.tsx:145`:
      ```ts
      if (/not found/i.test(message)) setNotFound(true);
      ```
      Rename "user not found" on the server, or add a period, and the profile page silently shows
      "Failed to load" instead of a 404 screen. 403-vs-404 is indistinguishable everywhere.
- [ ] Attach the status: `Object.assign(new Error(message), { status, code })`, or define an
      `ApiError` class
- [ ] Then rewrite `page.tsx:145` to `if (e instanceof ApiError && e.status === 404)`

### P1.6 No `error.tsx`, `not-found.tsx`, `global-error.tsx`, or `loading.tsx` in `app/`

- [ ] **Add the missing Next.js App Router boundary files** — `client/src/app/` contains only
      `layout.tsx`, `page.tsx`, and 5 route folders.
- [ ] `notFound()` in `client/src/app/pin/[id]/page.tsx` currently renders Next.js's **unstyled default** 404
- [ ] A server-component throw in `generateMetadata`/`page` renders the default error UI
- [ ] No route has a skeleton, so every navigation is a full blank flash
- [ ] `components/ErrorBoundary.tsx` is mounted *inside* `layout.tsx`, so it cannot catch layout errors
- [ ] Create `app/error.tsx`, `app/global-error.tsx`, `app/not-found.tsx`, `app/loading.tsx`
- [ ] Optionally add per-segment `loading.tsx` for `/pin/[id]`, `/users/[id]`, `/feed`

### P1.7 `pin/[id]` has no error handling — a 5s API timeout 500s the route

- [ ] **Wrap the server fetch in `try/catch`** — `client/src/app/pin/[id]/page.tsx` — neither
      `generateMetadata` nor the page wraps `getPinServer`, and `client/src/lib/api/server.ts` uses
      `AbortSignal.timeout(5000)` and throws on any non-404 non-ok.
- [ ] Catch, call `notFound()` only on 404
- [ ] Render a retry UI (or delegate to `app/error.tsx` from P1.6) otherwise

### P1.8 `SSR_API_URL` silently falls back to the browser-visible URL → wrong host in Docker

- [ ] **Set `API_INTERNAL_URL` and fail loudly when it's missing** —
      `client/src/lib/api/server.ts:10-12` reads `process.env.API_INTERNAL_URL`, and per
      `docker-compose.yml` the server service environment block does not set it. The fallback is the
      `localhost` URL, which from inside the client container is *itself* — every SSR pin fetch either
      fails or hits the wrong origin.
- [ ] Add `API_INTERNAL_URL=http://server:8080` to the client service env in `docker-compose.yml`
- [ ] In production, throw at module load when unset rather than falling back

### P1.9 JWT is stored in `localStorage` → any XSS exfiltrates a 30-day session

- [ ] **Move the session to an httpOnly cookie** — `client/src/store/auth.ts` (zustand `persist` to
      localStorage), token attached manually at `client/src/lib/api/client.ts:12-16`.

  The Next.js-recommended pattern is an httpOnly, `Secure`, `SameSite=Strict` cookie set by a route
  handler, with the client never touching the raw token. localStorage survives until explicitly cleared
  and is readable by any injected script.
- [ ] Implement the httpOnly cookie flow. `client/src/lib/api/server.ts` already does `cache()`-wrapped
      server fetches, so it slots in naturally
- [ ] Interim mitigation: cut `tokenExpiry` to hours and add refresh-token rotation

### P1.10 `GET /pins/:id` owner gets 404 for their own hidden pin on SSR

- [ ] **Forward caller auth in the SSR fetch** — `client/src/lib/api/server.ts` does not forward the
      caller's auth to the backend. The server gates hidden pins behind `canViewHidden`
      (`internal/modules/pins/handler.go:156-173`), so if a moderator hides their own pin the permalink
      404s on the server render and only "works" after hydration.
- [ ] Forward the incoming `cookie` / `Authorization` header in `fetchPinServer`

---

## P2 — Medium

### P2.1 `MapView` re-renders ~200 DOM nodes on every mouse move over the map

- [ ] **Extract the sr-only pin list into a memoized child** —
      `client/src/components/map/MapView.tsx:194-201` calls `setHover({pin, x, y})` with a **fresh
      object every event**, and `:387` renders `{pins.map(...)}` as an sr-only a11y list of up to 200
      buttons. `setHover` with a new object reference always re-renders, so every `mousemove`
      reconciles the whole list ~60×/sec on the hottest interaction surface in the app.
- [ ] Extract into a `React.memo`'d `<PinList>` child taking only `pins`
- [ ] Bail out in the `mousemove` handler when the target is unchanged:
      ```ts
      setHover(prev => prev && prev.pin.id === id &&
        Math.abs(prev.x-x)<2 && Math.abs(prev.y-y)<2
        ? prev : { pin, x, y });
      ```

### P2.2 SSE connection is torn down and rebuilt on every map pan

- [ ] **Quantize the bbox string** — `client/src/components/map/MapApp.tsx:132-134` → `setBbox(bboxValue)`,
      fed by `boundsToValidBbox` (`client/src/lib/utils/geo.ts`) into `usePinStream` (`MapApp.tsx:76`),
      whose effect deps are `[bbox, category, onPin]` (`client/src/hooks/usePinStream.ts:17-21`).

      Full float-precision bbox strings differ on *every* `moveend`, so every pan/zoom closes the
      `EventSource` and opens a new one — new TCP+HTTP handshake, new Redis pubsub subscription on the
      server, and a burst of reconnect attempts if a pan fires faster than teardown.
- [ ] Round the bbox to ~4 decimal places (~11 m) in `boundsToValidBbox` so small pans produce a
      string-identical bbox
- [ ] Add a ~250 ms debounce to `handleBoundsChange`

### P2.3 `maplibre-gl-shared.mjs` is imported into the client bundle — ~513 KB of vendored source

- [ ] **Add `"public/**"` to `globalIgnores`** — `client/eslint.config.mjs`. This is why **1098 of the
      1139 eslint warnings** come from `client/public/*.mjs|*.js`: the vendored bundles are being linted.
      One-line fix that removes 96% of lint noise.
- [ ] **Stop vendoring MapLibre into `public/`** — `client/public/maplibre-gl-shared.mjs` (513 KB) +
      `maplibre-gl-worker.js` (19 KB), referenced by
      `client/src/components/map/MapView.tsx:31` (`setWorkerUrl("/maplibre-gl-worker.js")`).
      Defeats tree-shaking and hard-codes a hand-managed copy of a dependency `pnpm` also installs.
- [ ] Generate the worker at build time (e.g. `vite-plugin-web-worker-asset` / `maplibre-gl-csp`) or
      bundle maplibre normally and set `workerUrl` from `new Worker(new URL(...))`

### P2.4 `pinLayers.ts` pulls `react-dom/server` into the client bundle and rebuilds 18 icons per theme switch

- [ ] **Memoize the generated icon data** — `client/src/components/map/pinLayers.ts:2`
      (`import { renderToStaticMarkup } from "react-dom/server"`), `:197-206` (`makePinIcon` → markup →
      Blob → `createImageBitmap` → canvas), `:255` (`map.addImage`), `:271-289` (18 images per
      `ensurePinLayers` call). Every `style.load` (map init **and** every light/dark toggle) re-renders
      18 SVGs, decodes 18 blobs, and re-uploads 18 textures. The markup is a pure function of its args
      and never changes.
- [ ] Memoize at module scope keyed by `(size, color, selected)` so a theme toggle only builds the delta
- [ ] Replace `renderToStaticMarkup` with a hand-built SVG string or a bundled static asset

### P2.5 `URL.createObjectURL` called inside `useMemo` during render → object-URL leak

- [ ] **Move blob-URL creation into an effect** — `client/src/components/pins/CreatePinButton.tsx`,
      `client/src/components/pins/PinEditSheet.tsx:34-38`, `client/src/app/users/[id]/page.tsx:171-180`
      all use:
      ```ts
      const previews = useMemo(() => files.map(f => URL.createObjectURL(f)), [files]);
      useEffect(() => () => previews.forEach(URL.revokeObjectURL), [previews]);
      ```
      `useMemo` is a *performance hint*, not a guarantee — React may discard the cached value
      (StrictMode double-invoke, suspense/offscreen re-render, hot reload). Every discarded value is a
      blob URL the cleanup effect, keyed on the memo result, never sees.
- [ ] Fix all three call sites:
      ```ts
      const [previews, setPreviews] = useState<string[]>([]);
      useEffect(() => {
        const urls = files.map(f => URL.createObjectURL(f));
        setPreviews(urls);
        return () => urls.forEach(URL.revokeObjectURL);
      }, [files]);
      ```

### P2.6 `SearchBar` highlights the wrong pin while arrowing through results

- [ ] **Use one flat index for the active-option highlight** —
      `client/src/components/map/SearchBar.tsx:219-227`
      ```tsx
      {results.map((item) => {
        if (item.kind !== "pin") return null;
        pinIdx++;
        const isActive = pinIdx === activeIndex;   // ← per-group index
      ```
      `activeIndex` is a **flat** index into `results` (used correctly by `Enter` at `:120`:
      `select(results[activeIndex])`), but the highlight compares against `pinIdx`, which restarts at 0
      for the pins group. With 3 place results, the first pin reports `isActive` whenever
      `activeIndex === 0` — highlighting pin 1 while place 1 is actually selected.
- [ ] Track one running counter across both loops: `let flat = -1`, then `const isActive = ++flat === activeIndex`
- [ ] Add `aria-activedescendant` on the input (`:147`) with per-option `id`s — the highlight is
      currently invisible to screen readers

### P2.7 `MapNavBar` account menu is a `role="menu"` with no keyboard handling

- [ ] **Wire up keyboard support or drop the ARIA roles** —
      `client/src/components/map/MapNavBar.tsx:64-177`: `role="menu"`, no Escape handler, no
      click-outside handler, no focus move into the menu, no `ArrowUp`/`ArrowDown` navigation, and
      `role="menuitem"` children unreachable by keyboard. It also cannot be closed without clicking
      the trigger. `client/src/hooks/useDialogFocus.ts` already implements focus-trap + focus-restore,
      and `PanelSheet` uses it — the menu hand-rolls a worse version.
- [ ] Wrap the menu in `useDialogFocus`
- [ ] Or drop `role="menu"`/`role="menuitem"` and make it a plain disclosure (`aria-expanded` +
      `aria-controls` on the trigger, plain `<button>`s inside) — honest and accessible with existing code

### P2.8 `CreatePinButton` hand-rolls its dialog — no focus trap, no focus restore

- [ ] **Use `PanelSheet` like every other modal** — `client/src/components/pins/CreatePinButton.tsx:72-73`
      adds its own `window` keydown listener for Escape, while `PinEditSheet` / `ReportSheet` /
      `AddToCollectionSheet` all go through `PanelSheet` + `useDialogFocus`. It therefore has no focus
      trap, no focus restoration on close, and no `aria-modal`.
- [ ] Switch to `PanelSheet` and delete the local listener

### P2.9 `CreatePinButton` keeps stale state when dismissed via the map

- [ ] **Reset draft state on close** — `client/src/components/map/MapApp.tsx:196-197` /
      `CreatePinButton.tsx`: the create-pin dialog is always mounted (hidden with a CSS transform).
      Dismissing via a map click sets `createOpen = false` without calling `handleClose`, so `files` /
      `previews` / `caption` survive into the next open — users can re-open the sheet and find a
      previous draft's images still attached.
- [ ] Drive the reset from an effect on `open === false`, or unmount the dialog when closed

### P2.10 Unauthenticated `POST /errors` with no rate limit → Sentry/log flooding

- [ ] **Add a limiter to the error ingest route** — `server/internal/http/router.go:54` registers
      `ClientErrorIngest` on `jsonRoutes`, which carries only `BodyLimit(1<<20)`. The handler
      (`router.go:73-108`) forwards to Sentry unconditionally, and `Extra map[string]any` is passed
      through to `rep.Report` unsanitized. A bot can push 1 MB × N at line rate. A
      `middleware.New(20, time.Minute)` limiter is already used for login/register in this same file.
- [ ] Add `middleware.New(30, time.Minute).Middleware()` to the `/errors` route
- [ ] Truncate `Stack` and `URL` to a sane length (only `Message` is capped, at `:95-97`)

### P2.11 `Recover` returns a non-JSON body, breaking the API's error envelope

- [ ] **Return JSON from the recover middleware** — `server/internal/http/middleware/recover.go:36`
      uses `c.AbortWithStatus(http.StatusInternalServerError)`, which writes an **empty** body, while
      every other error path goes through `response.Internal` and returns `{"error": "..."}`. Clients
      that `res.json()` on a 500 throw a *parse* error and lose the real cause. If the panic happens
      after headers are written (the SSE handler at `realtime/handler.go:60`), gin also logs a
      "superfluous WriteHeader" warning.
- [ ] Use `c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})`
- [ ] Guard with `if c.Writer.Written() { c.Abort() }`

### P2.12 `Recover` reports an empty `request_id`

- [ ] **Read the generated request ID from the context** —
      `server/internal/http/middleware/recover.go:30` reads `c.GetHeader("X-Request-ID")`, but the
      request logger **generates** the ID and writes it to the *response* header. A client that didn't
      send the header gets `request_id: ""` in Sentry, so panics can't be correlated with logs.
- [ ] Use `c.GetString("request_id")`

### P2.13 SSE `/events` has no heartbeat and no connection cap

- [ ] **Add a keepalive ticker** — `server/internal/modules/realtime/handler.go:31-80` is a
      `for { select { case <-ch: ...; case <-ctx.Done(): return } }` with no ticker. Any intermediary
      (nginx `proxy_read_timeout`, ALB idle timeout, Cloudflare) silently drops the stream after ~60 s of
      inactivity, and `EventSource` reconnects into a fresh Redis pubsub subscription each time.
- [ ] Add `case <-ticker.C: c.Writer.Write([]byte(": ping\n\n")); c.Writer.Flush()` on a 15–25 s ticker
- [ ] **Cap concurrent subscribers** — no limit exists; each holds a Redis channel with a 100-message
      buffer. Add a per-IP counter returning 503 past the cap, in front of `Subscribe`

### P2.14 `sslmode=disable` is hardcoded in the DSN

- [ ] **Make the SSL mode configurable** — `server/internal/config/config.go:136`
      ```go
      return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable", …)
      ```
      Fine for the compose network; sends the DB password in cleartext for any non-local deployment, and
      cannot be overridden.
- [ ] Add `DATABASE_SSLMODE` (default `disable` for dev, `require`/`verify-full` in prod) and read it from env

### P2.15 `docker-compose.yml` silently drops `CORS_ALLOWED_ORIGINS` and `SENTRY_DSN`

- [ ] **Pass both through compose** — the server service `environment:` block doesn't include either,
      so the containerized app always uses the `cors.go:16` localhost fallback, and Sentry can't be
      enabled in Docker at all. Combined with `AllowCredentials: true` (`cors.go:24`), a misconfigured
      production deploy fails closed (safe) but *silently*, which is worse than failing loudly.
- [ ] Add `CORS_ALLOWED_ORIGINS` / `SENTRY_DSN` / `SENTRY_ENV` to the compose server service
- [ ] **Consolidate env examples** — there are **three** `.env.example` files with overlapping,
      conflicting keys and **conflicting ports**: root (`PORT=8081`), `server/.env.example` (`8081`),
      `server/internal/config/.env.example` (`8080`). Reduce to a single source of truth at the repo root.

### P2.16 `Logout` hard-fails with a 500 when Redis is down, and the token is never revoked

- [ ] **Degrade gracefully on `Revoke` failure** — `server/internal/modules/auth/handler.go`:
      `h.bl.Revoke(...)` error → 500, and the client is left holding a still-valid token. There is
      already a precedent: `middleware/auth.go:37-39` logs and *continues* when `IsRevoked` fails.
- [ ] On `Revoke` failure, log a warning and still return 204, or fall back to a short-lived
      client-side discard and warn
- [ ] (Trade-off note: failing *closed* on logout with a 500 just teaches users not to log out)

### P2.17 Two Redis clients, never closed

- [ ] **Add `Close()` to `Blacklist` and `Broker`** — `server/internal/di/container.go:43-44`:
      `cache.New(...)` and `realtime.NewBroker(...)` each open their own `redis.Client`, and
      `cache.Blacklist` exposes `Ping`/`Revoke`/`IsRevoked` but **no `Close`**. `main.go` closes `pool`
      and `rep` but not these → leaked connections on every hot restart (Air in dev, redeploy in prod).
- [ ] Add `Close() error` to both types
- [ ] `defer` them in `app.Run` alongside the pool

### P2.18 Shutdown blocks for the full 10 s on every deploy because SSE connections never finish

- [ ] **Terminate open SSE streams on shutdown** —
      `server/internal/app/graceful_shutdown.go`: `srv.Shutdown(ctx)` waits for active handlers. Each
      open `/events` handler sits in a `select` on `ctx.Done()`, which is the *request* context — it
      does not fire on `Shutdown`, so every deploy eats the full 10 s timeout then force-closes.
- [ ] Keep a `sync.WaitGroup`/registry of open SSE streams
- [ ] On `RegisterOnShutdown`, write a terminating event and return from each handler

### P2.19 `RequireAdmin`/`RequireOwner` add a DB round-trip to every admin request

- [ ] **Reduce the per-request authorization query** —
      `server/internal/modules/user/routes.go:38-44`: `CurrentRole` calls `svc.GetByID` on each request
      purely for authorization, applied to `GET /reports`, `PATCH /reports/:id`, `GET /users`,
      `PATCH /users/:id/role`. Correct (that's why P0.3 is "high" and not "critical"), but it's a
      per-request query for data the JWT already carries.
- [ ] Once P0.3 is fixed (claim removed), cache the role in Redis (client already lives there) for
      ~60 s, accepting at most 60 s of stale-role latency
- [ ] Or accept the query and collapse `RequireAdmin` into a single combined auth middleware

### P2.20 `RequestLogger` has dead code, and `/health` is logged on every probe

- [ ] **Uncomment or delete the skip logic** —
      `server/internal/observability/logger/middleware.go:26-29` builds a `skip` map, and `:51` — its
      only consumer — is **commented out**. Yet `router.go:33` still passes `"/health"`, so health-check
      traffic is logged on every 5 s probe.
- [ ] Uncomment `:51`, or delete the `skipPaths` parameter and the argument at `router.go:33`

### P2.21 `GetByLogin` OR-blocks two columns → full scan, and usernames are case-inconsistent

- [ ] **Split into two index-backed branches and normalize usernames** —
      `server/internal/modules/user/repository.go:98`
      ```sql
      WHERE email = LOWER($1) OR username = $1 LIMIT 1
      ```
      `OR` across two columns can't use a single index — Postgres seq-scans or bitmap-ors both indexes.
      Separately, email is matched case-insensitively but `username` is not, so a user typing `John`
      gets "invalid credentials" for a registered `john`.
- [ ] Use one query with two `UNION ALL` branches, each index-backed
      (`WHERE email = LOWER($1)` / `WHERE username = LOWER($1)`)
- [ ] Normalize usernames to lowercase at registration

### P2.22 `comments.Create` doesn't check the pin exists → 500 instead of 404

- [ ] **Align parent-existence handling across sibling modules** —
      `server/internal/modules/comments/handler.go:55`: `INSERT INTO comments (pin_id, …)`. If
      `comments_pin_fkey` exists, a bad pin id surfaces as a constraint violation →
      `response.Internal` → 500 "internal server error", when the answer is 404.
- [ ] `SELECT 1 FROM pins WHERE id=$1` first, or map the FK violation to `ErrNotFound` in the repository
- [ ] `comments.List` (`:30`) has the mirror problem: returns `[]` for a nonexistent pin, while
      `favorites` 404s. Pick **one** convention — 404 on a missing parent — across `comments`,
      `favorites`, and `collections`

### P2.23 `UpdatePin` treats an absent `caption` field as "clear the caption"

- [ ] **Distinguish absent from empty** — `server/internal/modules/pins/handler.go`:
      `caption := strings.TrimSpace(c.PostForm("caption"))` always yields a non-nil `*string`, and the
      repository does `SET caption = COALESCE($2, caption)`. A PATCH that only swaps photos, sent without
      a `caption` field, **wipes the existing caption**. The contract is "the client always sends the
      whole current state", which is fragile — a partial-update client loses data silently.
- [ ] Use `c.GetPostForm("caption")` (two-value form) and only include `Caption` when present

### P2.24 `UpdatePin` photo cleanup leaks files on moderation

- [ ] **Delete photos on the hide path** — moderation (`reports.ReviewReport` "approve",
      `comments.Hide`) sets `is_hidden = true` / soft-deletes but never calls `store.Delete` on the
      pin's photos. Soft-hidden and deleted pins accumulate `pin_photos` rows and orphaned files in
      `./uploads` forever, served publicly at `/uploads` (`router.go:35`).
- [ ] On the hide path, enqueue photo deletion
- [ ] Or run a periodic reconciliation job comparing `pin_photos` against non-hidden pins

---

## P3 — Low / polish

### P3.1 Leftover debug log on every map move
- [ ] **Delete the `console.log`** — `client/src/components/map/MapView.tsx:254`:
      `console.log("zoom:", map.getZoom())` inside `moveend`. Also confirms `setBbox` fires on
      programmatic moves (see P2.2).

### P3.2 Map silently jumps to the user's location on load
- [ ] **Gate geolocation behind user intent** — `client/src/components/map/MapView.tsx:277-297`:
      `useGeolocation` fires on mount and `map.jumpTo(...)` on success, yanking the map away from the
      default Hyderabad center for every visitor with no prompt. Gate behind the existing
      `LocateButton`, or keep the default center until the user acts.

### P3.3 `usePins` duplicates `useAsyncData` and has no `retry`
- [ ] **Reimplement on the existing abstraction** — `client/src/hooks/usePins.ts:26-49` hand-rolls the
      abort / stale-guard / loading logic that `useAsyncData` already provides, and omits `retry`.
      Replace the body with `useAsyncData`.

### P3.4 `usePins` leaves `loading` stuck `true` when `bbox` goes `null`
- [ ] **Reset `loading` in the `!bbox` branch** — `client/src/hooks/usePins.ts:18-24, 27`. Same issue in
      `useAsyncData` when `enabled` flips `true → false`.

### P3.5 `useCollections.addPin` optimistically bumps `pin_count` even when the pin was already present
- [ ] **Reconcile from the response** — `client/src/hooks/useCollections.ts`. The server does
      `ON CONFLICT DO NOTHING`; the local count drifts permanently.

### P3.6 `useSavedStatus` fetches the entire favorite ID list to check one pin
- [ ] **Guard with a shared cache** — `client/src/hooks/useFavorites.ts`. Acceptable for one panel; if
      it's ever called per-pin in a grid it's N requests.

### P3.7 `useSessionRefresh` fires `GET /users/:id` on every page navigation
- [ ] **Add a TTL** — `client/src/hooks/useSessionRefresh.ts:15-28`, called from **both**
      `Navbar.tsx:28` and `MapNavBar.tsx:40`. Add a ~5 min TTL or a `sessionStorage` timestamp.

### P3.8 `useDialogFocus` has no body scroll lock and no `inert`
- [ ] **Lock `document.body` overflow while open** — `client/src/hooks/useDialogFocus.ts`.

### P3.9 Client caption limit (200) ≠ server limit (500)
- [ ] **Pick one number** — `CreatePinButton.tsx:284`, `PinEditSheet.tsx:114` vs
      `internal/modules/pins/dto.go`. Export it from a shared constant.

### P3.10 `CreatePinButton` defaults `categoryId` to `1` before categories load
- [ ] **Derive from the loaded list** — `CreatePinButton.tsx:43`:
      `useState<number>(categories[0]?.id ?? 1)`. `useState` never re-runs, so the default is
      permanently "Food" by accident. Default to `null` and require a selection.

### P3.11 `pin-default` icon is unreachable dead code
- [ ] **Delete the branch or make categories dynamic** — `pinLayers.ts:285, 374` vs
      `CATEGORY_IDS = [1..8]` (`:62`) and `DOT_COLORS[categoryId % len]`: every id in 1..8 always matches
      a `pin-cat-N` image, so `pin-default` never renders. Make `CATEGORY_IDS` derive from the API's
      `/categories` response so new DB categories get icons without a code change.

### P3.12 `roles/page.tsx` — pointless IIFE wrapping a single button
- [ ] **Inline it** — `client/src/app/roles/page.tsx:218`.

### P3.13 Collections links on the profile page go nowhere
- [ ] **Either filter by it or point at a real route** — `client/src/app/users/[id]/page.tsx:550`:
      `href={`/users/${id}?collection=${c.id}`}`, but the page never reads the `collection` search
      param. Currently a dead link.

### P3.14 `users/[id]` uses `Promise.all` for profile + pins + collections
- [ ] **Use `Promise.allSettled`** — `page.tsx:133-137`. A single failed `fetchUserPins` blanks the
      whole profile. Render partial results instead.

### P3.15 `users/[id]` `return null` (blank screen, no Navbar)
- [ ] **Return an error UI instead of `null`** — `page.tsx:293`. Unreachable today but has no fallback.

### P3.16 `SearchBar` sets `loading=false` from an aborted request's `finally`
- [ ] **Guard `finally` on `signal?.aborted`** — `SearchBar.tsx:64-70`. Otherwise the dropdown flashes
      "No results found" during the 300 ms debounce.

### P3.17 Missing Go tests for most modules
- [ ] **Add unit tests for the security primitives at minimum** — `comments`, `favorites`, `social`,
      `collections`, `pins`, `realtime`, `jwt`, `password`, `validid` all report `[no test files]`.
      `internal/endpointtest` covers features well, but `pkg/jwt` and `pkg/password` — the security
      primitives — have zero unit tests.

### P3.18 CI never runs `go test -race`
- [ ] **Add `-race`** — `.github/workflows/*`. The rate limiter and SSE handler are both
      concurrency-sensitive.

### P3.19 `logger.FromContext(nil)` reads like a bug
- [ ] **Clean it up** — `server/internal/app/app.go:17`. Works (returns the default logger), but use
      `logger.Default()` and pass a real context, or drop the indirection.

### P3.20 Auth is cleared on *any* 401
- [ ] **Whitelist paths instead of blacklisting one** —
      `client/src/lib/api/client.ts:31-36`. If a future anonymous endpoint 401s, every user gets logged
      out.

### P3.21 `database.Connect` uses `log.Printf`, not `slog`
- [ ] **Switch to `slog`** — `server/internal/platform/database/postgres.go`. Startup logs go to a
      different sink/format than everything else.

### P3.22 `server/deployments/k8s/` is an empty tracked directory
- [ ] **Add manifests or drop the path** — `-` (directory only, no files).

### P3.23 Dead scaffolding in the server module tree
- [ ] **Delete or annotate** — 7 one-line files in `server/internal/modules/streams/`
      (`webhook`, `service`, `repository`, `model`, `livekit_client`, `handler`, `dto`) plus
      `realtime/{rooms,pubsub,hub,dto,broadcast}.go`. `migrations/0005_streams.sql` also exists with no
      code behind it. Delete, or add a `// TODO(owner):` header.

---

## Verified clean — do not re-investigate

- [x] **No secret leakage.** `.env` is untracked and ignored at all three levels (`.gitignore`,
      `server/.gitignore`, `client/.gitignore`); only `.env.example` variants are committed.
      `git log --all -p -- .env` shows no committed secrets across 99 commits. `client/.next` and
      `node_modules` are correctly ignored. `server/uploads` is ignored. The
      `NEXT_PUBLIC_MAPTILER_API_KEY` exposure is by design — confirm it's domain-restricted in the
      MapTiler console.
- [x] **SQL injection:** every query uses pgx placeholders. The `SearchPins` `ILIKE` wildcard escape in
      `repository.go` relies on Postgres's default backslash escape character — correct, though worth
      a comment.
- [x] **Path traversal:** `storage.Local.Save` uses `crypto/rand` hex filenames; `Local.Delete`
      (`server/internal/platform/storage/local.go:42-58`) correctly guards with `filepath.Base` +
      `Clean` + `..` rejection.
- [x] **CORS:** `server/internal/http/middleware/cors.go` uses an explicit allow-list with a localhost
      fallback and never reflects `Origin` — safe against the classic
      `AllowOriginFunc: return true` mistake. `*` is not used alongside `AllowCredentials: true`.
- [x] **JWT signing:** HS256 with algorithm-family validation (`server/pkg/jwt/jwt.go:52-57` rejects
      non-HMAC), cryptographic `jti`, expiry enforced, blacklist checked on every request.
- [x] **Image uploads:** content type sniffed via `bimg` (magic bytes), not the client-declared MIME
      type; size bounded by both the multipart part header and a 64 MB body limit.
- [x] **MapLibre worker asset exists** — `client/public/maplibre-gl-worker.js` is present, so the
      `setWorkerUrl` call won't 404.
- [x] **`.tsx` tests correctly declare `// @vitest-environment jsdom`** even though
      `vitest.config.ts` defaults to `node`.
- [x] **Map lifecycle is clean:** `map.remove()` runs in the init effect's cleanup
      (`MapView.tsx:264`), and the async `style.load` handler is guarded by an `active` flag (`:292`)
      so it can't touch a removed map.
- [x] **`Promise.allSettled` in `SearchBar`** (`client/src/components/map/SearchBar.tsx:50`)
      correctly degrades when one of the two searches fails.

---

## Suggested order of work

1. **P0.1, P0.2** — two-line changes that close a DoS and a silent-failure mode. Do them first.
2. **P0.3, P0.4** — the two genuine auth/security defects. P0.3 needs a decision on token
   lifetime / refresh strategy.
3. **P1.5, P1.6, P1.7** — unblocks honest error UI everywhere and deletes the regex-on-message hack.
4. **P1.1, P1.2, P1.4** — three SQL changes (one new index migration, two query rewrites) with
   outsized latency wins.
5. **P2.1, P2.2, P2.5** — the three client perf/memory issues, all localized.
6. Everything else as a backlog pass.
