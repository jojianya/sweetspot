# GoodSpot247 (`sweetspot`) — Codebase Onboarding Guide

**Audience:** an engineer who has never seen this repository.
**Scope:** everything under `client/`, `server/`, and the `maps/` tile module (where it currently lives).
**How to read it:** sections 1–8 describe how the system *works today*. Section 9 is the honest list of what is
*not* finished or is fragile. Section 10 is how to actually get it running.

Throughout, file paths and identifiers are real. Where I could not verify something by reading the code, I say
so explicitly rather than guessing. Where I verified something empirically (I ran it), I say that too.

---

## 0. Read this first: the branch situation is unusual

Before anything else, because it will confuse you otherwise:

| Ref | Commit | Notes |
|---|---|---|
| `origin/main` | `cb60e82` | Same commit as `origin/development` and local `main`. |
| `origin/development` | `cb60e82` | Identical to `origin/main` — nothing extra on the remote. |
| local `main` | `cb60e82` | |
| local `development` | `6322c89` | **26 commits ahead of `origin/development`, none of them pushed.** |

Two things follow:

1. **There is no `test` branch and no `self-hosted-map` branch** — not locally, not on `origin`. Those
   branches existed while the self-hosted tile stack was built and have since been deleted. See
   [§7.5](#75-the-self-hosted-map-branch-is-dangling--this-is-the-biggest-open-risk-in-the-repo).
2. **26 commits of real work (SSE heartbeat/connection cap/bbox validation + tests, JWT-secret length
   enforcement, `Skeleton` UI primitive and its rollout) exist only on this machine.** Nothing is on the
   remote. If this disk is lost, they are gone.

Other remote branches: `chore/remove-tiles-env`, `chore/remove-tiles-env-dev`, `feature/search-locations`,
`fix/p2-redis-shutdown`, `refactor/modular-structure`. `fix/p2-redis-shutdown` (`87e5df7`) consolidates the
Redis client and fixes logout-during-outage; that work is **already merged into `main`** (see
[§6](#6-auth-and-session-flow-end-to-end) — the consolidated `di.Build` in the current tree is the result).

`origin/HEAD` → `origin/main`.

---

## 1. What the app does

Goodspot is a **location-based social pinning app**. The entire product surface is one big interactive map
with panels hanging off it.

A user can:

- **Drop a pin** at a map coordinate with 1–5 photos, a caption (≤500 chars) and a category
  (Food, Nature, Event, Nightlife, Art, Sports, Travel, Other).
- **Browse pins in the current viewport.** The map is clustered; zooming in past
  `clusterMaxZoom: 8` shows individual markers. There is no global pin list — the map *is* the index.
- **Open a pin** either as a side panel (`PinDetailPanel`) or as a permalink page (`/pin/[id]`, server-rendered
  with OpenGraph metadata so links preview).
- **Search** in one box: MapTiler **places** plus local **pins** (`GET /pins/search`, `ILIKE` over caption and
  username). Results are a grouped listbox with keyboard nav.
- **Favorite** pins, and file them into **collections** (ordered pin lists per user).
- **Comment** on pins. Comment authors can hard-delete their own; moderators soft-hide.
- **Follow** other users and read a **feed** of pins from people you follow (follow-graph only, no bbox).
- **See "trending"** for the viewport, ranked by `(views + 5 × comments) / (age_hours + 2)`.
- **Report** a pin. Reports go to an admin-only console at `/reports` with approve/dismiss.
- **See live pins appear** while someone else creates one, via an SSE stream (see [§8](#8-real-time-sse)).
- **Manage roles** as the `owner`: `/roles` promotes/demotes `user` / `admin` / `owner`.

Non-features that are easy to assume exist but don't: there is no messaging, no live video, no search-by-location-bbox on the feed, and **no reverse geocoding stored on
the pin** (see [§9.4](#94-reverse-geocoding-at-creation-is-not-done)). The stub `streams/` package was deleted in B1; only its orphan `streams` DB table (from `0005_streams.sql`) remains.

Domain: the app's default map centre and the self-hosted tileset both point at **different countries** —
the client defaults to Hyderabad, India (`17.385, 78.4867` in `MapApp.tsx`) while `maps/` is built for the
**Philippines**. The product is nominally PH-facing; the default centre is stale. Flagged in
[§9.6](#96-domain-inconsistency--india-defaults-philippine-tiles).

---

## 2. High-level architecture

```
                        ┌───────────────────────────────────────────┐
   browser  ──────────► │  Next.js 16 (App Router, standalone)      │
      │                 │  client/ — SSR for /pin/[id], rest is CSR  │
      │                 └───────────────┬───────────────────────────┘
      │                                 │  /api/*  (same-origin rewrite,
      │                                 │            server-side only)
      │                                 ▼
      │                 ┌───────────────────────────────────────────┐
      │                 │  Go 1.27 + Gin — server/                   │
      │                 │  :8081                                     │
      │                 └───┬───────────────┬───────────────┬───────┘
      │                     │               │               │
      │                     ▼               ▼               ▼
      │                 Postgres 16   Redis 7          ./uploads
      │                 + PostGIS 3.4  (JWT blacklist,   (local disk,
      │                 (pgxpool)       pin pub/sub)     served at /uploads)
      │                     ▲
      │                     │
      │                 MapTiler        ← geocoding (search + reverse
      │   ─────────────────────────────     geocode for display) AND
      │   ─────────────────────────────     basemap tiles + style + glyphs
      │
      └── EventSource GET /api/events ──► SSE stream of newly created pins

   optional, self-hosted tiles path (see §7):
      MapLibre ──► TileServer GL (maptiler/tileserver-gl image) on :8080
                    serving an OpenMapTiles-schema philippines.mbtiles
```

Key structural decisions worth internalising:

- **The browser only ever talks to one origin.** `client/next.config.ts` rewrites `/api/:path*` →
  `${API_INTERNAL_URL}/:path*`, resolved *server-side* by Next. The browser calls `/api/...` and never learns
  the Go API's address. This is not cosmetic — it is what makes the `SameSite=Strict` session cookie usable
  ([§6.4](#64-the-samesite--proxy-history-read-this-before-touching-auth)).
- **Redis is not optional.** It is the JWT revocation store *and* the realtime pub/sub bus. A Redis outage is
  an authentication outage ([§6.3](#63-the-redis-fail-closed-fix)).
- **Storage is local disk only.** `storage.NewLocal("./uploads", cfg.StorageBase)` writes processed photos to
  `./uploads` and `router.go` serves that directory at `/uploads`. `STORAGE_BACKEND` accepts only `local`;
  anything else is a hard `log.Fatal` at startup (`config.validateStorageBackend`).
- **Spatial queries use PostGIS**, not geohash. The `pins.geohash` column is *written* on create
  (`geohash.Encode(lat, lng)`) and indexed, but **no query filters on it** — bbox filtering is
  `ST_Intersects(p.location, ST_MakeEnvelope(...)::geography)` against the `GEOGRAPHY(POINT,4326)` column.
  Geohash is currently decorative.

### 2.1 Which map setup is on which branch

| Branch | Basemap tiles | Geocoding | `maps/` present? |
|---|---|---|---|
| `main`, `development` (both at `cb60e82`) | **MapTiler** `streets-v2` / `streets-v2-dark`, loaded from `api.maptiler.com` | **MapTiler** `api.maptiler.com/geocoding` | **No** |
| `test/self-hosted-map` — **deleted; dangling chain ending at `a4c8f34`** | **Self-hosted** TileServer GL, `philippines.mbtiles` | MapTiler, but **gracefully degrades to "unavailable"** with no key | **Yes** (whole module) |

On `main`/`development` the client uses `NEXT_PUBLIC_MAPTILER_API_KEY` for **both** tiles and geocoding:

```ts
// client/src/components/map/MapView.tsx:32-35
const MAPTILER_KEY = process.env.NEXT_PUBLIC_MAPTILER_API_KEY ?? "";
const LIGHT_STYLE = `https://api.maptiler.com/maps/streets-v2/style.json?key=${MAPTILER_KEY}`;
const DARK_STYLE  = `https://api.maptiler.com/maps/streets-v2-dark/style.json?key=${MAPTILER_KEY}`;
```

**Heads-up:** the root `.env.example` still carries two artefacts of the deleted self-hosted branch — a
`NEXT_PUBLIC_TILES_URL` line that **nothing on this branch reads**, and a comment on
`NEXT_PUBLIC_MAPTILER_API_KEY` claiming "base map tiles are self-hosted (see below)", which is **false on
this branch**. Both are stale; see [§9.2](#92-stale-env-example--documentation-drift).

---

## 3. Client structure (`client/`)

Next.js **16.3.4**, React **19.2.8**, TypeScript, Tailwind v4, `output: "standalone"`, pnpm 10.28.0, Node 20.
Packages: `axios`, `maplibre-gl` ^6.9, `zod` ^4.5, `zustand` ^5. (`react-icons` removed in B1 — no importers.)

House rule from `client/AGENTS.md`: **never use `dangerouslySetInnerHTML`** (or any raw-HTML rendering) on
user-generated content — captions, usernames, socials JSON. React escapes by default; only break that
deliberately.

### 3.1 App Router (`client/src/app/`)

`app/layout.tsx` is a **server** component (`RootLayout`, no `"use client"`). It sets
`metadataBase: getSiteUrl()`, loads Geist/Geist Mono, imports `globals.css`, and injects a
`beforeInteractive` inline `<Script id="theme-init">` that reads `localStorage["goodspot-theme"]` (falling
back to `prefers-color-scheme`) and sets `.dark` + `colorScheme` on `<html>`. `<body>` is
`flex h-dvh flex-col overflow-hidden`. It mounts two **client leaves that render nothing** —
`<RuntimeErrorReporter />` and `<SessionSync />` — precisely so the root layout stays a server component.

| Route | Component kind | What it does |
|---|---|---|
| `/` | server, async | Awaits `searchParams`, reads `?category` (only when it's a single string), renders `<MapApp initialCategory={...} />`. |
| `/login` | client | `login(identifier, password)` → `setUser` → `router.push("/")`. |
| `/register` | client | `EMAIL_RE` check, username ≥3, `checkPassword()` from `lib/password`, then `register()`. |
| `/feed` | client | `useAsyncData(() => fetchFeed(), [user], { enabled: !!user })`. `SkeletonRegion` while loading. |
| `/pin/[id]` | **server**, async | `resolvePin(id)` → `getPinServer(id)`; returns `null` **only** for `ApiError` 404, rethrows everything else. Exports `generateMetadata` (404 ⇒ `robots: {index:false,follow:false}`; else `buildPinMetadata(pin, getSiteUrl(), NEXT_PUBLIC_API_URL)`) and renders `<PinPageClient initialPin={pin}/>`, else `notFound()`. |
| `/users/[id]` | client | `Promise.all([fetchUser, fetchUserPins, fetchUserCollections])` in one `useEffect` keyed `[id, attempt]`. Own profile is editable (avatar/username/socials). Follow button via `useFollow(id)`. |
| `/reports` | client | `isModerator = role === "admin" \|\| "owner"`; `fetchReports({status})`; `reviewReport(id, "approve" \| "dismiss")`. |
| `/roles` | client | Owner-only. `PAGE_SIZE = 50`; `fetchUsers({limit,offset})` with load-more, `searchUsers(q)`, `updateUserRole()`. |
| `error.tsx` / `global-error.tsx` | client | Error boundaries; `global-error.tsx` re-declares its own `<html>/<body>`. Both call `reportError()`. |
| `loading.tsx` / `not-found.tsx` | server | Gradient+spinner shell; 404 with "Back to map". |

There is **no** `export const dynamic`, `revalidate`, or `generateStaticParams` anywhere in `client/`. The one
`cache()` call in the whole client is `export const getPinServer = cache(fetchPinServer)` in `lib/api/server.ts`,
so `generateMetadata` and the page share one API read per SSR request.

### 3.2 The map (`client/src/components/map/`)

**`MapView.tsx`** — the MapLibre surface. Props:
`{ pins, flyTo, highlightId, theme, onBoundsChange, onSelectPin, onMapClick }`.

- Module scope: `setWorkerUrl("/maplibre-gl-worker.js")` (a vendored copy in `client/public/`, alongside
  `maplibre-gl-shared.mjs`).
- Map is created once in a `[]`-deps effect: `zoom: 10`, `minZoom: 5`, `maxZoom: 21`, style chosen from
  `theme`. **No `transformRequest`, no `maxBounds`, no explicit `center`** — MapLibre's defaults apply on
  load. There is an explicit comment saying no `transformRequest` is needed because MapTiler's hosted style
  has absolute tile/glyph/sprite URLs. (The self-hosted branch needed one; see
  [§7.3](#73-transformrequest-and-root-relative-urls).)
- Theme swap: `styleUrlRef` guard, then `map.setStyle(next, { diff: false })`.
- `style.load` → `ensurePinLayers(map, () => active)` → bumps `styleVersion` so the data and filter effects
  re-run against the fresh style. An `active` flag prevents touching a removed map.
- **Pins are GeoJSON, not markers.** One `GeoJSONSource` named `"pins"` with `cluster: true`,
  `clusterRadius: 35`, `clusterMaxZoom: 8`, and four layers: `pins-cluster`, `pins-cluster-label`,
  `pins-base`, `pins-selected`. Data comes from `parsePoint(p.location)` (`POINT(lng lat)` regex).
- Selection: the `pins-base` filter stays at `notCluster` so neighbours remain visible; `pins-selected`
  gets `["all", notCluster, ["==", ["get","id"], highlightId]]`.
- Hover is guarded by `hoverTargetRef` so a `mousemove` over the same pin doesn't `setState`.
- Pin click → `onSelectPin(id)` + `fitPinNeighborhood(...)` (`selectNearbyPins`, `PIN_FIT_MAX_ZOOM=15`,
  `PIN_FIT_MIN_SPAN_DEGREES=0.01`, `PIN_FIT_DURATION_MS=650`, asymmetric `PIN_FIT_PADDING`).
  Cluster click → `getClusterExpansionZoom` → `easeTo`.
- Background click: `queryRenderedFeatures` over the pin layers + `pins-cluster`; empty hit ⇒ `onMapClick()`.
- `moveend` **and** the initial `load` both emit bounds via `boundsToValidBbox(map.getBounds())` formatted as
  `` `${south},${west},${north},${east}` ``.
- The only `Marker` is the blue user-location dot.
- `const PinList = memo(...)` renders an `sr-only` `<ul aria-label="Pins on the map">` of focusable buttons —
  the keyboard route to canvas-drawn markers.
- If `MAPTILER_KEY` is empty it renders a "Map unavailable" panel naming the env var, instead of the map.

**`pinLayers.ts`** — `ensurePinLayers(map, isActive)` builds **2 images** (`pin-post`,
`pin-post-selected`) from an inline SVG teardrop (`pinSvg(fill)` → Blob → `Image` → 28x40 canvas →
`ImageData`, `PIN_FILL = #EA4335`), then `map.addImage`. The per-category ids (`pin-cat-1..8`,
`pin-cat-N-selected`, `pin-default`, `pin-selected`) are stale ids released on hot-reload only; nothing
references them. Exports `GeoFeature`,
`EMPTY_GEOJSON`, `CLUSTER_MAX_ZOOM = 8`, `CLUSTERING_ENABLED = true`, `notCluster`. Existing layers are
reconfigured with `setLayoutProperty` rather than re-added.

**`MapApp.tsx`** — the orchestrator for `/`. Owns the `bbox` string, overlays, and every panel. The bbox path
is the important part:

```
MapView.moveend → onBoundsChange(bbox, center)
  → MapApp.handleBoundsChange: sets center, clears flyTo, then a 250 ms debounce (boundsTimerRef) → setBbox
  → bbox feeds THREE consumers:
      usePins(bbox, effectiveCategory)         → REST
      usePinStream(bbox, effectiveCategory, onPin) → SSE
      useTrending(trendingOpen ? bbox : null) → REST
```

Mounted children: `MapView`, the posting crosshair, `MapNavBar`, `LocateButton`, `CreatePinButton` (always
mounted, hidden via CSS transform), and conditionally `PinDetailPanel`, `SavedPinsPanel`, `TrendingList`,
plus a `{n} new pins in view` toast that clears after 4 s.

**`MapNavBar.tsx`** — fixed header: `SearchBar`, a scrolling category chip row (`aria-pressed`, toggle-off on
re-click), and an account menu with `ThemeToggle`, Saved, My profile, Feed, Reports (admin/owner), Roles
(owner), Log out. Calls `useAuth()`, `useLogout()`, `useSessionRefresh()`.

**`SearchBar.tsx`** — 300 ms debounce, `AbortController` per keystroke, min 2 chars,
`Promise.allSettled([searchPlaces(q, proximity, signal), searchPins(q, 5, signal)])`, grouped listbox with
arrow/Enter/Escape.

**`LocateButton.tsx`** — `useSyncExternalStore`-based geolocation feature detection; returns `null` if
unsupported.

**`CategoryIcons.tsx`** — `ChipIcon({ name })`, a regex-mapped inline-SVG picker. Server-safe (no `"use client"`).

**`ThemeToggle.tsx`** — `useTheme().toggleTheme()`, `role="menuitem"`, `aria-pressed`.

### 3.3 Pin UI (`client/src/components/pins/`)

`PinPageClient.tsx` (permalink body; photo hero, thumbnail strip, save/share/directions, `reverseGeocode`
address line, comments, and the `PhotoLightbox`/`PinEditSheet`/`AddToCollectionSheet`/`ReportSheet`),
`PinDetailPanel.tsx` (same content, side-panel, built on `PanelSheet`), `CreatePinButton.tsx` (FAB + slide-in
dialog; `MAX_PHOTOS = 5`, `MAX_PHOTO_SIZE = 10MB`, JPG/PNG only, "Use current location"),
`PinEditSheet.tsx`, `SavedPinsPanel.tsx` (saved/collections tabs + collection drill-in),
`TrendingList.tsx`, `CommentsSection.tsx`, `PhotoLightbox.tsx`, `AddToCollectionSheet.tsx`,
`ReportSheet.tsx` (reason 3–1000 chars, `QUICK_REASONS` chips). (`CategoryDropdown.tsx` removed in B1 — had no importers.)

Shared: `components/PanelSheet.tsx` (bottom sheet + `useDialogFocus`), `Avatar.tsx`, `SessionSync.tsx`,
`RuntimeErrorReporter.tsx`, `icons.tsx` (16 inline SVGs), `layout/Navbar.tsx`, `ui/Skeleton.tsx`
(`Skeleton` + `SkeletonRegion`, the latter being `role="status" aria-busy="true"` with an `sr-only` label).

`Skeleton`/`SkeletonRegion` call sites: `app/feed/page.tsx:62`, `app/reports/page.tsx:132`,
`app/users/[id]/page.tsx:247`, `components/map/MapApp.tsx:262` (bare `Skeleton`),
`components/pins/{TrendingList:79, SavedPinsPanel:260,365, CommentsSection:76}`.

### 3.4 Hooks (`client/src/hooks/`)

`useAsyncData` is the shared fetch-state abstraction (`{data, loading, error, retry, setData}`, own
`AbortController` per run, `enabled` flag, `attempt` counter).

`usePins(bbox, category)` → `{pins, loading, error, addPin, removePin}` — hand-rolled abort/stale-guard over
`fetchPins`; `addPin` de-dupes and prepends.
`usePinStream(bbox, category, onPin)` — opens `EventSource`, deps `[bbox, category, onPin]`, cleanup
`es.close()`. **No manual reconnect** — it relies on `EventSource`'s own retry, so `onPin` must be referentially
stable (`MapApp` uses `useCallback`). Skipped entirely while `bbox === null`.
`useGeolocation()`, `useDialogFocus(ref, onClose?)`, `useFavorites()` / `useSavedStatus(pinId, enabled)`,
`useCollections()`, `useComments(pinId)`, `useTrending(bbox)`,
`useFollow(userId)`, `useCategories()`, `usePinDetail(id)`, `useLogout()`, `useSessionSync()`,
`useSessionRefresh()`. (`usePinView(pinId)` removed in B1 — had no callers.)

### 3.5 The API layer (`client/src/lib/api/`)

- **`client.ts`** — one axios instance: `baseURL: API_BASE_URL`, `timeout: 20000`, `withCredentials: true`.
  Exports `class ApiError extends Error { status, code }` — `status === 0` means the request never got a
  response. The response interceptor: reports 5xx via `reportError`, and on **401** clears the auth store
  *unless* `isSessionEnded(url)` says otherwise.
- **`session.ts`** — `isSessionEnded(requestUrl)`; `CREDENTIAL_PATHS = ["/auth/login", "/auth/register"]`.
  A failed login is a 401 but must not log you out.
- **`server.ts`** — `fetchPinServer(id)` + `cache()`-wrapped `getPinServer`. `resolveSSRUrl()` prefers
  `API_INTERNAL_URL`, falls back to `NEXT_PUBLIC_API_URL` **with a `console.warn` in dev** and **throws in
  production**. Forwards the incoming `cookie` header so SSR can authenticate (without it, an owner viewing
  their own hidden pin gets a 404 server-side). `cache: "no-store"`, `AbortSignal.timeout(5000)`.
- **`realtime.ts`** — `openPinStream(bbox, category, onPin): EventSource`; one `addEventListener("pin")`,
  zod-parses with `pinListEntrySchema`, `catch { /* skip */ }`.
- **`geocoding.ts`** — `searchPlaces()` and `reverseGeocode()` straight to `api.maptiler.com/geocoding` with
  `NEXT_PUBLIC_MAPTILER_API_KEY!`. Not on this branch yet: an `isGeocodingAvailable()` guard.
- **`schemas.ts`** — zod schemas for every response: `pinBaseSchema`, `pinListEntrySchema`, `trendingPinSchema`,
  `pinPhotoSchema`, `pinDetailSchema`, `newPinPhotoSchema`, `commentSchema`, `collectionEntrySchema`,
  `collectionDetailSchema`, `userStatsSchema`, `publicUserSchema`, `privateUserSchema`, `reportSchema`,
  `reportEntrySchema`.
- **`auth.ts`** — `login`, `register`, `logout`, and `fetchSession()` → `GET /me` (documented at length: `GET
  /users/:id` is behind `OptionalAuth` and answers 200 for anonymous callers, so it can never detect an ended
  session).
- **`index.ts`** — barrel re-exporting ~40 functions.
- Also `pins.ts`, `users.ts`, `social.ts`, `collections.ts`, `comments.ts`, `favorites.ts`, `reports.ts`,
  `categories.ts`. Note `fetchCategories` is a bare typed cast with **no zod validation**.

### 3.6 State (`client/src/store/`)

Two zustand stores, both `persist`-ed:

- **`auth.ts`** — `useAuth`, storage key **`goodspot-auth`**, default `localStorage`. `partialize` keeps only
  `{ id, username, avatar_url, role }` (`PersistedUser`). Email, socials and timestamps are dropped by
  `pruneUser()` on write. **The token is not here** — it is an httpOnly cookie.
- **`theme.ts`** — `useTheme`, storage key `goodspot-theme`, whole state persisted. `getInitialTheme()` runs at
  store creation (server ⇒ `"light"`; client ⇒ stored value or `prefers-color-scheme`), and `applyTheme()`
  toggles `.dark` on `<html>`.

### 3.7 `client/src/lib/` (non-API)

`types/{auth,category,pin,report,social,index}.ts`; `utils/{geo,format,category,errorMessage,index}.ts`
(`geo.ts` holds `parsePoint`, `distanceMeters`, `selectNearbyPins`, `geolocationAvailable`,
`getCurrentPosition`, `toGeoCoords`, and `boundsToValidBbox`, which clamps latitude, wraps longitude, collapses
a full-world span to `[-180,180]`, and **quantises every coordinate to 4 decimals (~11 m)**);
`metadata/pin.ts` (`resolvePublicMediaUrl`, `buildPinMetadata` — title ≤70, description ≤200); `site.ts`
(`resolveSiteUrl`, `getSiteUrl`); `password.ts` (`PASSWORD_MIN_CHARS = 8`, `PASSWORD_MAX_BYTES = 72`,
`checkPassword`); `monitoring.ts` (**owns `API_BASE_URL = "/api"`**, `reportError`, `initGlobalErrorReporting`).

---

## 4. Server structure (`server/`)

Go **1.27.1**, Gin **1.12**, pgx/v5, go-redis/v9, golang-jwt/v5, bcrypt (cost 12), bimg (libvips),
geohash, godotenv, lmittmann/tint, sentry-go. Module path `github.com/jojianya/sweetspot247-backend`.

### 4.1 Layout

```
server/
├── cmd/api/main.go            run() → config.Load → logger.Init → report.New
│                              → database.Connect → RunMigrations → app.Run
│                              errors → slog.Error + os.Exit(1) (defers must run)
├── internal/
│   ├── app/                   app.go (timeouts), graceful_shutdown.go (serve/signal)
│   ├── config/                env → Config, with hard validations
│   ├── di/container.go        Build(cfg, pool) *Container
│   ├── http/                  router.go + middleware/ + response/ + params/ + validid/
│   ├── modules/               auth, pins, collections, comments, favorites,
│   │                          realtime, reports, social, user
│   ├── observability/         logger/ (slog + request logger), report/ (slog + Sentry)
│   └── platform/              cache/ (Redis blacklist), database/ (pool + migrations),
│                              storage/ (local disk)
├── pkg/                       geohash, jwt, password
├── deployments/docker/        server.Dockerfile(.dev)
├── .air.toml                  hot reload: go build -p 1 -o ./tmp/server ./cmd/api
└── Makefile                   build/run/test/vet/fmt/lint/tidy + docker-*/backup/restore
```

### 4.2 Middleware stack (`internal/http/middleware/`)

Applied globally in `router.go:33`, in this order:

```go
r.Use(middleware.Recover(rep),                       // panic → 500 + report
      middleware.RequestLogger(lg, "/health"),       // X-Request-ID, one line per request
      middleware.ReportErrors(rep),                   // any 5xx → report
      middleware.CORS(cfg.CORSAllowedOrigins...),
      middleware.SecurityHeaders())
```

Per-group: `jsonRoutes` gets `BodyLimit(1 << 20)`; `uploadRoutes` gets `BodyLimit(64 << 20)`.

Per-route: `middleware.New(n, window)` limiters, `middleware.AuthRequired` / `OptionalAuth`,
`validid.Middleware()` (UUID check on `:id`) and `validid.MiddlewareParam("pinId")`.

- **`auth.go`** — `parseBearerClaims` tries the **`session_token` cookie first**, then falls back to
  `Authorization: Bearer` for non-browser clients. `validateToken` checks the JWT then the Redis blacklist and
  **fails closed** on blacklist error. `applyClaims` sets only `user_id` and `jwt_claims` — **no role** (see
  [§6.2](#62-roles-are-never-read-from-the-token)).
- **`rate_limit.go`** — `Limiter` is a **plain in-memory `map[string]*entry` guarded by a mutex**, with
  `maxKeysBounds = 10000` triggering `sweepExpired`. `Middleware()` keys on `c.ClientIP()`;
  `MiddlewareKeyed(fn)` takes any key function (auth uses a constant key for a global counter). `Locked(key)`
  + `Reset(key)` drive the explicit login lockout. **Limits do not hold across replicas.**
- **`cors.go`** — wraps `gin-contrib/cors` with an explicit origin allow-list, `AllowCredentials: true`,
  `MaxAge: 12h`, never reflecting `Origin`.
- **`recover.go`** — declares `type ErrorReporter interface{ Report(ctx, err, attrs...) }` so tests can stub it.
- **`report.go`** — `ReportErrors`: after `c.Next()`, reports any status ≥500 using `c.Errors.Last().Err`
  (set by `response.Internal`).
- **`security_headers.go`** — nosniff, `X-Frame-Options: DENY`,
  `CSP: default-src 'none'; frame-ancestors 'none'; img-src 'self'`, `Referrer-Policy: no-referrer`, HSTS.
- **`body_limit.go`** — `http.MaxBytesReader`.

`gin.SetTrustedProxies(nil)` is called in `router.go:32`, so `ClientIP()` is the real peer and
`X-Forwarded-For` cannot be spoofed to evade per-IP limits.

### 4.3 DI container (`internal/di/container.go`)

```go
func Build(cfg *config.Config, pool *pgxpool.Pool) *Container
```

One **shared** `*redis.Client` feeds both `cache.NewWithClient` and `realtime.NewBrokerWithClient`, and the
container exposes it as `Redis` so `app.Run` can `Close()` it after `serve` returns (the earlier two-client,
never-closed version was fixed). `Store: storage.NewLocal("./uploads", cfg.StorageBase)`.

Note: `New(addr, password)` constructors still exist on both `Blacklist` and `Broker` for their own client;
the doc comments say "prefer `NewWithClient`". Nothing in the app calls them.

### 4.4 Routing (`internal/http/router.go`)

```
GET  /health                     pings Postgres AND Redis → 503 {"status":"degraded"} if either is down
GET  /uploads/*                  r.Static("./uploads")
POST /errors                     ClientErrorIngest(rep) — public, always 204, never fails

jsonRoutes  (BodyLimit 1 MiB):  POST /errors
                             →  auth.RegisterRoutes      POST /auth/register, /auth/login, /auth/logout, GET /me
                             →  GET /events                realtime (registered BEFORE /pins/:id)
                             →  reports, favorites, comments, social, collections

uploadRoutes (BodyLimit 64 MiB): PATCH /users/me (multipart avatar)
                             →  pins  (multipart create/update)
```

`GET /events` is deliberately registered before the parameterised pin routes so it can never collide with
`/pins/:id`.

Full route inventory (module `routes.go` files):

| Module | Routes |
|---|---|
| `auth` | `POST /auth/register` (5/min IP + 20/min global), `POST /auth/login` (20/min IP + 60/min global), `POST /auth/logout` (auth), `GET /me` (auth) |
| `pins` | `GET /categories`, `GET /pins`, `GET /pins/search`, `GET /pins/trending`, `GET /users/:id/pins`, `GET /pins/:id` (optional auth), `POST /pins` (10/min, auth), `POST /pins/:id/view` (public, unthrottled), `PATCH /pins/:id`, `DELETE /pins/:id` (auth) |
| `favorites` | `GET /favorites`, `GET /favorites/ids`, `PUT /favorites/:id`, `DELETE /favorites/:id` (auth) |
| `comments` | `GET /pins/:id/comments`, `POST /pins/:id/comments` (30/min, auth), `DELETE /comments/:id` (auth) |
| `social` | `GET /users/:id/stats` (optional auth), `PUT`/`DELETE /users/:id/follow` (auth), `GET /feed` (auth) |
| `collections` | `GET /users/:id/collections`, `GET /collections`, `POST /collections`, `GET /collections/:id`, `PATCH`/`DELETE /collections/:id`, `PUT`/`DELETE /collections/:id/pins/:pinId` (auth) |
| `reports` | `POST /pins/:id/report` (auth), `GET /reports` (auth + `RequireAdmin`), `PATCH /reports/:id` (auth + `RequireAdmin`) |
| `user` | `GET /users` (auth + `RequireOwner`), `GET /users/:id` (optional auth), `PATCH /users/me` (auth), `PATCH /users/:id/role` (auth + `RequireOwner`) |

### 4.5 Modules (`internal/modules/`)

Each is `model.go` / `dto.go` / `repository.go` / `service.go` / `handler.go` / `routes.go`, with a
`RouteOptions{JWTSecret, Blacklist}` struct passed into `RegisterRoutes`. Handlers return `response.*` helpers;
`gin.H` bodies are hand-built per endpoint (`{"pin":…}`, `{"pins":…}`, `{"ids":[…]}`, `{"users":…,"total":n}`,
…). There is no response wrapper — the only convention is `{"error": "<message>"}` on failure.

Notable per-module behaviour:

- **`pins`** — `CreatePin` is multipart: `UserExists` else 401, lat/lng range check, caption ≤500 runes,
  1–5 photos ≤10 MB each, `imaging.Validate` (magic-byte sniff + ≤8000 px) then `imaging.Process`
  (1600 px WEBP q80 + 400 px square thumbnail, capped at 2 concurrent libvips ops via `processSem`). **On DB
  failure it deletes the files it just wrote.** On success it broadcasts `Event{Location: "POINT(lng lat)"}`.
  `DeletePin` reads the pin first (to learn filenames) then soft-deletes (`is_hidden = true`) and
  best-effort deletes files. `UpdatePin` requires owner or moderator; deletes replaced photos on success and
  new files on failure. `GetPin` returns **404, not 403**, for hidden pins the viewer can't see.
- **`collections`** — `requireOwner`: owner, else `users.IsModerator`, else 403. `AddPin` uses
  `ON CONFLICT (collection_id, pin_id) DO NOTHING` and `COALESCE(MAX(position)+1, 0)`.
- **`comments`** — `maxCommentLength = 500`. Delete: moderator → `Hide` (soft, keeps an audit trail), author →
  hard `Delete`, else 403.
- **`favorites`** — `Save` is `ON CONFLICT DO NOTHING`; `Unsave` 404s when not saved.
- **`social`** — `feedDefaultLimit=50`, `feedMaxLimit=100`. `/feed` is `follows JOIN pins` — **no bbox, no
  geohash**. Self-follow is 400.
- **`reports`** — `ReviewReport` runs in one transaction: `SELECT status … FOR UPDATE`, then `status =
  'actioned'` on approve **and `UPDATE pins SET is_hidden = true WHERE id=$1`**; dismiss changes nothing;
  finally `UPDATE reports SET status, resolved_by, resolved_at`. Already-resolved ⇒ 409.
- **`user`** (`package users`) — `GetByLogin` is `WHERE email = LOWER($1) OR username = $1` (email
  case-insensitive, username **not**). `ListUsers` uses `COUNT(*) OVER ()` with a `CountUsers` fallback on an
  empty page. `UpdateRole` refuses self-change and refuses to demote the last owner. `UpdateMe` (multipart,
  on the 64 MB group) accepts any subset of `username` / `socials` (≤20 keys, key ≤64, value ≤500, string/bool/
  number only) / `avatar` (≤5 MB → `imaging.Avatar` → 256 px WEBP); deletes the new avatar on failure and the
  old avatar on success. `User.PasswordHash` is `json:"-"`, and `ToPrivate()` (which includes `email`) is used
  only when the viewer *is* the subject.
- **`realtime`** — see [§8](#8-real-time-sse). Live files are `broker.go` + `handler.go` (+ tests); the old one-line placeholder stubs were deleted in B1.
  Migration `0005_streams.sql` creates a `streams` table, but there is no code behind it.

### 4.6 Platform layer (`internal/platform/`)

- **`cache/session.go`** — `Blacklist` over one Redis client. Keys are `jwt:blacklist:<jti>`.
  `Revoke(ctx, jti, ttl)` clamps `ttl` to ≥1 s and no-ops on ≤0; `IsRevoked` is an `EXISTS`; `Ping`.
- **`database/postgres.go`** — `Connect(dsn)` retries up to **10 times at 3 s intervals** and pings, so a cold
  Postgres doesn't crash the container. (It logs via `log.Printf`, not `slog` — a known nit.)
- **`database/migrate.go`** — plain SQL files in `internal/platform/database/migrations`, tracked in
  `schema_migrations(filename, applied_at)`. `executeMigration` detects `CONCURRENTLY` and splits the file
  statement-by-statement (it can't run in a transaction), using a hand-rolled `splitStatements` that
  understands line comments, block comments and string literals (but **not** dollar-quoted bodies).
  `verifySchema` then asserts `users, categories, pins, pin_photos, reports` exist via `to_regclass`, so a wiped
  database fails loudly at boot instead of running empty.
- **`storage/`** — `Local{dir, base}`. `Save(data, ext)` → `MkdirAll`, filename
  `hex(16 crypto/rand bytes) + "." + ext`, mode `0o644`, returns `"<base>/uploads/<name>"`.
  `Delete(url)` takes `filepath.Base` only and rejects `..` / anything `filepath.Clean` alters;
  `os.ErrNotExist` is treated as success.

### 4.7 Observability (`internal/observability/`)

- **`logger/`** — `logger.Init(level, format)` builds an `slog.Logger` with `tint` for text output,
  `logger.FromContext(ctx)`, and `RequestLogger(base, skipPaths...)` which generates/propagates
  `X-Request-ID`, echoes it as a response header, attaches a `request_id`-scoped logger, and emits one line
  after `c.Next()` (Error ≥500 including `c.Errors.Last().Error()`, Warn ≥400, else Info).
- **`report/`** — the single downstream error hook. `New(lg, dsn, env)`; empty DSN ⇒ slog-only. `Report`
  always logs at error level, and with a DSN also captures to Sentry (`TracesSampleRate: 0`, attrs under the
  `"report"` context key). `Close()` flushes for 2 s only when a DSN is configured.
- Client-side crashes flow back through `POST /errors` → `ClientErrorIngest` → the same reporter.
  `lib/monitoring.ts` throttles (100 events/session, 10 s dedupe per kind+url+message) and fire-and-forgets.

### 4.8 `pkg/`

- **`jwt`** — `Generate` signs HS256; `Validate` pins the exact method (`SigningMethodHS256`), rejecting HS384/512, `none` and RSA.
  `Claims` = `{ UserID, RegisteredClaims }` with a UUID-v4-shaped `jti` (`newJTI()`). **No `Role` field.**
- **`password`** — owns bcrypt's 72-**byte** limit so it cannot drift from validation: `MinRunes = 8`,
  `MaxLen = 72`, `Validate(p)` checks both bounds in the units that matter, `ErrTooLong.Is()` satisfies
  `errors.Is(err, bcrypt.ErrPasswordTooLong)` so callers answer 400 rather than 500.
- **`validid`** — `IsUUID` (length 36, dashes at 8/13/18/23, hex elsewhere), `Middleware()` (checks `:id`),
  `MiddlewareParam(name)`. Route ordering matters: on `/collections/:id/pins/:pinId`, `id` is the collection,
  so the pin needs `MiddlewareParam("pinId")`.
- **`geohash`** — `Encode(lat, lng)` at precision 7 (used on pin create). (`Neighbors` removed in B1 — had no callers.)

---

## 5. Data model

From `server/internal/platform/database/migrations/`, in order:

| Migration | Contents |
|---|---|
| `0001_init_extensions.sql` | `CREATE EXTENSION postgis` |
| `0002_users.sql` | `users` |
| `0003_pins.sql` | `categories`, `pins`, `pin_photos` + seeds the 8 categories |
| `0004_reports.sql` | `reports` |
| `0005_streams.sql` | `streams` (**no code behind it**) |
| `0006_pin_photo_thumbnails.sql` | `pin_photos.thumbnail_url` + backfill |
| `0007_favorites.sql` | `favorites` |
| `0008_pin_edit.sql` | `pins.updated_at` |
| `0009_comments.sql` | `comments` |
| `0010_follows.sql` | `follows` |
| `0011_collections.sql` | `collections`, `collection_pins` |
| `0012_pin_views.sql` | `pins.views` |
| `0013_category_slug.sql` | `categories.slug` (derived from name, then NOT NULL + UNIQUE) |
| `0014_pins_indexes.sql` | `pins_created_at_idx`, and a partial `pins_visible_created_idx ... WHERE is_hidden = false`, both `CONCURRENTLY` |
| `0015_pin_photo_thumbnail_not_null.sql` | backfills `pin_photos.thumbnail_url` from its own `photo_url`, then `NOT NULL` |
| `0016_pins_updated_at_trigger.sql` | `set_pins_updated_at()` + `BEFORE UPDATE` trigger on `pins`, exempting `views` |
| `0017_streams_drop_room_name.sql` | drops the vendor-specific identifier column 0005 added to `streams`, plus its UNIQUE constraint |

### 5.1 Entities and relationships

```
users ──┬─< pins.user_id            (ON DELETE SET NULL)
        ├─< comments.user_id        (ON DELETE SET NULL)
        ├─< reports.reporter_id     (ON DELETE SET NULL)   ┈ UNIQUE(pin_id, reporter_id)
        ├─< reports.resolved_by     (ON DELETE SET NULL)
        ├─< favorites.user_id       (ON DELETE CASCADE)    ┐ composite PK
        ├─< collections.user_id     (ON DELETE CASCADE)    │
        ├─< follows.follower_id     (ON DELETE CASCADE)   ┘ composite PK
        └─< follows.followee_id     (ON DELETE CASCADE)

categories ──< pins.category_id     (ON DELETE RESTRICT)

pins ──┬─< pin_photos.pin_id        (ON DELETE CASCADE)
       ├─< comments.pin_id          (ON DELETE CASCADE)
       ├─< reports.pin_id           (ON DELETE CASCADE)
       ├─< favorites.pin_id         (ON DELETE CASCADE)
       ├─< collection_pins.pin_id   (ON DELETE CASCADE)
       └─< streams.pin_id           (ON DELETE SET NULL)

collections ──< collection_pins.collection_id (ON DELETE CASCADE)
```

Key column shapes:

- `users` — `id UUID PK`, `email UNIQUE`, `password_hash`, `username UNIQUE`, `avatar_url TEXT`,
  `socials JSONB DEFAULT '{}'`, `role TEXT CHECK (role IN ('user','admin','owner')) DEFAULT 'user'`,
  `created_at`, `updated_at` (maintained by a `set_updated_at()` BEFORE UPDATE trigger).
  **There is no role table** — role is a CHECK-constrained column on `users`.
- `pins` — `location GEOGRAPHY(POINT,4326) NOT NULL`, `geohash TEXT NOT NULL`, `caption TEXT` (nullable),
  `category_id INT NOT NULL REFERENCES categories(id) ON DELETE RESTRICT`, `is_hidden BOOLEAN DEFAULT false`,
  `views BIGINT DEFAULT 0`, `created_at`, `updated_at`. Indexes: GIST on `location`, btree on `geohash`,
  `category_id`, plus the two sort indexes from `0014`.
- `pin_photos` — `photo_url`, `thumbnail_url`, `position SMALLINT DEFAULT 0`, with a UNIQUE index on
  `(pin_id, position)`.
- `comments` — `body TEXT NOT NULL`, `is_hidden BOOLEAN DEFAULT false`, index `(pin_id, created_at ASC)`.
- `reports` — `reason TEXT NOT NULL`, `status CHECK (status IN ('pending','reviewed','actioned')) DEFAULT
  'pending'`, `resolved_by`, `resolved_at`, `UNIQUE (pin_id, reporter_id)`.
- `favorites` — `PRIMARY KEY (user_id, pin_id)`, index `(user_id, created_at DESC)`.
- `follows` — `PRIMARY KEY (follower_id, followee_id)`, indexes on both directions.
- `collections` / `collection_pins` — `name TEXT NOT NULL`, `description`, and a join table with a `position INT`
  giving manual ordering; index `(collection_id, position)`.

### 5.2 Soft delete and the 404-not-403 convention

`is_hidden = true` is the soft-delete for both pins and comments. Most read paths filter it and surface hidden
content as **404**, never 403. Three queries deliberately do **not** filter it — `pins.GetPin` (the handler
enforces), `favorites.PinExists` / `favorites.ListIDs`, and `reports.PinExists`. That's intentional for
favorites ("you can unsave a hidden pin") but it means `GET /favorites/ids` will return IDs for pins that
`GET /pins/:id` will 404.

### 5.3 Cover-photo query, repeated four times

Every list query that returns a `pins.PinListEntry` resolves a cover image with a
`LEFT JOIN LATERAL (SELECT photo_url, thumbnail_url FROM pin_photos WHERE pin_id=p.id ORDER BY position LIMIT 1)
pp ON true` plus `LEFT JOIN users u`. It is factored into a `pinListEntrySelect` constant inside `pins`, and
**re-inlined** in `collections`, `favorites` and `social`. `collections` uses a correlated scalar subquery
instead of LATERAL for its cover. This is duplication, not a design.

Also: **`views` is only selected by `pins.ListPins`, `ListByUser`, `SearchPins` and `ListTrending`.** The
collections, favorites and feed queries omit it, so `views` serialises as `0` on those endpoints. Cosmetic, but
confusing if you trust the number.

### 5.4 The bbox convention — get this wrong and you get silent empties

- Client: `boundsToValidBbox` returns `[south, west, north, east]` and `MapApp` formats it as
  `"south,west,north,east"`.
- Server: `params.ParseBbox` parses to **`[minLat, minLng, maxLat, maxLng]`** and rejects equal min/max
  (zero area). `realtime.validateBbox` double-checks ranges.
- `pins.ListPins` re-orders into `ST_MakeEnvelope($1,$2,$3,$4,4326)` as
  `$1=bbox[1], $2=bbox[0], $3=bbox[3], $4=bbox[2]` (lon/lat envelope), then
  `AND ST_Intersects(p.location, ST_MakeEnvelope(...)::geography)`.
- `realtime.matches` `Sscanf`s `"POINT(lng lat)"` into `&lng, &lat` and tests lat/lng against the bbox.

`server/internal/endpointtest/integration_test.go` has a long comment explaining that swapping lat/lng pairs
asks for longitude 37.75 / latitude −122.45 — "a point in the Indian Ocean" — and the query then returns
nothing *for the right-looking-but-wrong reason*. Read that comment before touching bbox code.

---

## 6. Auth and session flow (end to end)

### 6.1 The happy path

```
POST /api/auth/login  { identifier, password }
  → auth.Handler.Login
      limiter check: emailLim.Locked(identifier)  → 429
      → auth.service.Login → users.Service.GetByLogin (email = LOWER($1) OR username = $1)
      → password.Verify (bcrypt cost 12)
      → on failure: emailLim.AllowKey(identifier) + 401 "invalid email, username, or password"
      → on success: emailLim.Reset(identifier)
  → SetSessionCookie(w, r, token, 30 days)
  → 200 {"user": {...}}          ← the TOKEN IS NOT IN THE BODY

browser stores Set-Cookie: session_token=<jwt>; HttpOnly; SameSite=Strict; Secure iff HTTPS; Path=/
  → zustand persists only {id, username, avatar_url, role} under "goodspot-auth"
```

`tokenExpiry = 30 * 24 * time.Hour` (`auth/service.go`). There is **no refresh token** and no rotation.

`Secure` is computed per request by `auth.isSecureRequest(r)`: `r.TLS != nil` **or**
`X-Forwarded-Proto == "https"`. The comment is explicit that forcing `Secure` on plain HTTP would make the
browser silently drop the cookie and break auth entirely.

Client reconciliation, two separate concerns:

- **`useSessionSync` (via `<SessionSync/>` in the root layout)** — once per page load, `GET /me`. Clears the
  cached user **only** on `ApiError` with `status === 401`; keeps it on offline/timeout/5xx.
- **`useSessionRefresh`** — `GET /users/:id` on mount, so role changes appear without re-login. Called from both
  `Navbar.tsx` and `MapNavBar.tsx`, i.e. on every navigation. (No TTL — see [§9.5](#95-smaller-still-open-client-items).)
- **The axios interceptor** clears auth on a 401 unless `isSessionEnded(url)` says it's a credential endpoint.
  This is the mid-visit backstop.

`GET /me` returns `{user_id, role}` where the role comes from a live DB read via `auth.service.Role`, not the
token.

### 6.2 Roles are never read from the token

`pkg/jwt.Claims` has **no `Role` field**, and `middleware.applyClaims` deliberately stores no role. Every
authorization decision calls through `users.RoleReader` — a one-method interface
(`GetByID(ctx, id) (User, error)`) so tests can stub it — to reach `users.CurrentRole`, `users.IsModerator`,
`users.RequireAdmin` and `users.RequireOwner`.

This was the fix for the audit item "admin rights live in the JWT for 30 days". The cost is a **DB round-trip
per moderated request**. That's a deliberate trade, documented in the code.

### 6.3 The Redis fail-closed fix

`Blacklist.Revoke` / `IsRevoked` live under `jwt:blacklist:<jti>`. Logout revokes with
`ttl = time.Until(claims.ExpiresAt)` — i.e. a revocation entry only as long as the token could still be valid.

Commit `77013f3` ("fix(auth): fail closed when the session blacklist is unreachable, and report Redis in
`/health`") changed `middleware.validateToken` from:

```go
slog.Warn("blacklist check failed", "error", err)   // …and then FALL THROUGH AND ACCEPT
```

to:

```go
slog.Default().Error("blacklist check failed, denying request", "error", err, "jti", claims.ID)
return nil, "session verification unavailable, please try again", false
```

The reason, verbatim from the commit: a logged-out token was accepted again whenever Redis was unreachable, so
*any* Redis blip silently un-revoked every logged-out session for the remaining 30-day life of the cookie.

Three consequences worth internalising:

1. **Redis is a hard dependency of authentication.** That is why `GET /health` pings Redis too and returns
   `503 {"status":"degraded","db":...,"redis":...}` when either dependency is unreachable, so the container
   healthcheck and any load balancer stop routing to a server that cannot verify sessions.
2. **The denial message is deliberately distinct** from `"session was logged out, please sign in again"`, so the
   client does not conflate an outage with a genuine logout and nuke the cached user.
3. **`Logout` deliberately fails open.** `auth.Handler.Logout` clears the cookie *first*, then calls
   `h.bl.Revoke(...)`; on error it only `slog.Warn`s and still answers 200. Failing the logout would teach users
   not to log out, and the cookie is gone either way. (`ClearSessionCookie` sets `MaxAge: -1`.) The token will
   simply expire on its own.

Regression tests live in `server/internal/http/middleware/auth_test.go` (fail-closed on outage,
accept-with-no-blacklist, bad signature) and `server/internal/modules/auth/auth_test.go`.

**Do not "simplify" this back into a warn-and-continue.** It has been reverted into an outage before.

### 6.4 The SameSite / proxy history — read this before touching auth

There are three distinct eras here, and only the third one works:

**Era 1 — JWT in `localStorage` (`Bearer` only).** `client/src/store/auth.ts` persisted the token and
`client.ts` attached it manually. No cookie, so no SameSite issue, but any XSS exfiltrated a 30-day session
(audit item P1.9). `docs/SECURITY.md` still says "there is no cookie-based session, so CSRF does not apply" —
**that statement is obsolete**; re-read the file with that in mind.

**Era 2 — `httpOnly` cookie, still cross-origin (commit `27e35e7`, "feat(auth): move JWT from localStorage to
httpOnly cookie").** This is where it broke. The cookie is `SameSite=Strict` **and host-only**. Loading the app
from `http://127.0.0.1:3000` while the API sat at `http://localhost:8081` makes *every* authenticated request
cross-site: the browser silently withholds the cookie, each protected page answers 401, and the client
correctly interprets 401 as "session ended" and logs the user out. From the outside it looks like the app
"randomly logs you out", and it looks like a race or a caching bug, and it isn't.

The trap: `localhost` and `127.0.0.1` are **different hosts** to a browser. Also note that `SameSite=Strict`
is stricter than needed once the API is same-origin, but it is what makes the cross-origin failure total rather
than partial.

**Era 3 — same-origin proxy (commit `698c2be`, merged in `91ed870`, "feat(proxy): same-origin `/api/*` rewrite so
the session cookie is first-party").** `client/next.config.ts` now rewrites:

```ts
{ source: "/api/:path*", destination: `${apiProxyTarget()}/:path*` }
```

The **destination path is masked**, so the browser sees an ordinary same-origin request and receives an ordinary
same-origin `Set-Cookie`. `API_BASE_URL` is now the literal string `"/api"` (it lives in `lib/monitoring.ts`).

`apiProxyTarget()` prefers `API_INTERNAL_URL`, then `NEXT_PUBLIC_API_URL`, then `http://localhost:8081` —
stripping trailing slashes. `API_INTERNAL_URL` must be a **server-side** address: in Docker that is the compose
service name `http://server:8081`, because inside the client container `localhost` is the container itself.
It is also a **build argument** in `client/Dockerfile` (`ARG`/`ENV` in both `builder` and `runner`), because
Next may resolve rewrites while building the routes manifest — setting it only at runtime is not enough.

`NEXT_PUBLIC_API_URL` still exists for two *unrelated* jobs and must not be confused with `API_BASE_URL`:
absolute URLs in SSR metadata (`lib/site.ts`) and the SSR fetch fallback (`lib/api/server.ts`).

The whole saga is written out at length in three comments you'll hit immediately if you touch these files:
`client/next.config.ts:3-24`, `client/src/lib/monitoring.ts` (above `API_BASE_URL`), and
`client/src/lib/api/server.ts`.

---

## 7. Map / tiles

### 7.1 The MapTiler path (live on `main` / `development`)

`client/src/components/map/MapView.tsx`:

- `NEXT_PUBLIC_MAPTILER_API_KEY` supplies both style URLs
  (`maps/streets-v2/style.json` and `maps/streets-v2-dark/style.json`) **and** geocoding
  (`lib/api/geocoding.ts`).
- Style swap on theme toggle is `map.setStyle(next, { diff: false })`, guarded by a `styleUrlRef`.
- A `styleimagemissing` handler registers a 1×1 transparent image for any id not prefixed `pin-`, which
  suppresses two warnings emitted by the hosted style (commit `4f5d43d`).
- Glyphs come from MapTiler's hosted fonts; sprites too. Nothing is self-hosted here.
- **The key is inlined into the browser bundle.** It must be domain-restricted in the MapTiler dashboard. The
  committed value in `.env.example` is a real-looking key; treat it as public.
- If the key is empty, `MapView` renders a "Map unavailable" panel instead of the map — so a missing key
  degrades to a readable message, not a blank screen.
- **Geocoding is not degradable on this branch.** `geocoding.ts` has `NEXT_PUBLIC_MAPTILER_API_KEY!` (non-null
  assertion) and no availability check, so with no key every place search fails loudly.
- `client/public/maplibre-gl-shared.mjs` (~513 KB) + `maplibre-gl-worker.js` are **vendored copies** of a
  dependency that `pnpm` also installs. `setWorkerUrl("/maplibre-gl-worker.js")` points at the vendored copy.

### 7.2 The self-hosted path (`maps/`, from the deleted branch)

The `maps/` module exists **only in the dangling history** ending at `a4c8f34`; read it with
`git show a4c8f34:maps/README.md`. Its own README is the best single description; the essentials:

```
maps/
├── data/philippines-latest.osm.pbf   # 579 MB raw Geofabrik extract — gitignored
├── philippines.mbtiles                # ~533 MB generated tile archive — gitignored
├── planetiler.jar, .planetiler-cache/ # gitignored, fetched+checksummed by build-tiles.sh
├── build-tiles.sh                     # .pbf → .mbtiles via Planetiler (OpenMapTiles schema), idempotent
├── build-tiles-native.sh              # native alternative
├── style/goodspot/{style.json, style-dark.json}    # committed MapLibre styles
├── sprites/{sprite.json,sprite.png,sprite@2x.*, goodspot/, goodspot-dark/}
├── tileserver-config.json             # committed TileServer GL config
├── docker-compose.tiles.yml           # separate compose file, deliberately NOT merged into the app's
├── docker-compose.yml                 # shim: `include: [docker-compose.tiles.yml]`
└── test.html                          # standalone visual smoke test
```

- **Tiles:** Philippines only, ~`112.17, 4.38 → 127.07, 21.53`, zoom 0–14 (the client overzooms beyond 14 via
  MapLibre's built-in overzoom — verified in commit `a4c8f34` with zero z15+ requests).
- **Schema:** OpenMapTiles layer names (`transportation`, `waterway`, `poi`, …). The README explicitly warns
  the styles will **not** render against a Shortbread-schema tileset, and notes that the provenance of the
  specific bundled `philippines.mbtiles` is unverified (its layer names and metadata match OpenMapTiles).
- **Config** (`tileserver-config.json`): `paths.root=/data`, `styles=/data/styles`, `mbtiles=/data`,
  `fonts=/usr/src/app/node_modules/tileserver-gl-styles/fonts` (glyphs come from the image's bundled
  `tileserver-gl-styles` package), `sprites=/data/sprites`, `serveAllStyles: true`, styles `goodspot` and
  `goodspot-dark`, dataset `philippines`.
- **Compose** (`docker-compose.tiles.yml`): image `maptiler/tileserver-gl:latest`, command that exits with a
  clear message if `/data/philippines.mbtiles` is missing, four read-only bind mounts parameterised by
  `${TILES_DIR:-.}`, published on **`127.0.0.1:8080` only**, and a healthcheck that fetches the real style URL.
- **Serving alongside the app** (from the README):
  ```bash
  TILES_DIR=./maps docker compose \
    -f docker-compose.yml \
    -f maps/docker-compose.tiles.yml up -d
  ```
- **Cloudflare in front is a required config step, not a nicety** — a single-region tile origin otherwise means
  high latency everywhere else. The README covers `--public_url` (TileServer GL rewrites some URLs — e.g.
  glyphs — absolutely from the `Host` header, so behind Cloudflare they come out wrong without it), cache
  rules for `/data/*`, `/fonts/*`, `/sprites/*`, `/styles/*` (TileServer GL sends `ETag` and
  `Access-Control-Allow-Origin: *` but **no `Cache-Control`**, so the edge rules are what give tiles a TTL),
  Tiered Cache, and a purge/versioning strategy.
- **Regeneration is manual.** No scheduled refresh. The README's "Tradeoffs" section is candid: no global CDN by
  default, manual data freshness, schema lock-in, single-region SPOF.
- Known limitation recorded in the merge commit `e771df2`: a TileServer GL v5.6.0 bug made the dark style 404
  (`serveAllStyles` served only the first style). The final config sets `serveAllStyles: true`, so this was
  presumably worked around — **I did not run the tile server, so I can't tell you whether the dark style
  currently serves.**

### 7.3 `transformRequest` and root-relative URLs

On the self-hosted branch the styles reference tiles/glyphs/sprites with **root-relative** URLs
(`/data/philippines/...`, `/fonts/...`, `sprite`) so the same files work from `localhost:8080`, a Cloudflare
hostname, or a tunnel. MapLibre resolves root-relative URLs against the **page** origin, not the style origin —
so `MapView` passes:

```ts
transformRequest: (url) =>
  url.startsWith("/") ? { url: `${TILES_URL}${url}` } : { url },
```

This is required in self-hosted mode and **deliberately absent** in the MapTiler path (where every URL in the
hosted style is already absolute). Note the branch's `.env.example` also documents that opening these styles in
an external editor like Maputnik will 404 tiles unless you serve the editor from the tile origin.

### 7.4 Client wiring in self-hosted mode

```
NEXT_PUBLIC_TILES_URL=http://localhost:8080
${NEXT_PUBLIC_TILES_URL}/styles/goodspot/style.json
${NEXT_PUBLIC_TILES_URL}/styles/goodspot-dark/style.json
```

Plus `PHILIPPINES_BOUNDS = [116.5, 4.5, 127, 21.5]` (commit `5f719aa`, "enforce Philippines bounds on map") and
`MANILA_CENTER = [120.98, 14.6]` (`a4c8f34`) — added because without an explicit centre MapLibre defaults to
`[0,0]` and `maxBounds` clamps the view to the south-west corner of the Philippines.

Geocoding degrades gracefully there: commit `a33a25e` added `isGeocodingAvailable()` to `geocoding.ts` so
`searchPlaces()` returns `[]` and `reverseGeocode()` returns `null` without a key; `SearchBar` swaps its
placeholder to "place search only" and pin search (local `/pins/search`) keeps working. It also removed the
`NEXT_PUBLIC_MAPTILER_API_KEY` *requirement* from the Dockerfile, CI and compose. **None of this is on
`main`/`development`.**

### 7.5 The self-hosted map branch is dangling — this is the biggest open risk in the repo

There is **no `test` branch and no `self-hosted-map` branch**, locally or on `origin`. The work is a chain
ending at `a4c8f34`, reachable **only through `refs/stash`** (the stash entries are literally named
`WIP on test/self-hosted-map` / `On test/self-hosted-map`).

Concretely:
- `git branch -a --contains a4c8f34` → nothing.
- `git for-each-ref --contains a4c8f34` → only `refs/stash`.
- **`git stash drop` or `git stash clear` orphans the entire self-hosted map line.** So would a `git gc` after
  the reflog expiry window.
- A large part of the tile payload is *not* in git at all: `philippines.mbtiles` (~533 MB) and the raw PBF
  (579 MB) are gitignored by design. The README says to back the `.mbtiles` up separately (S3/NAS) — **I have
  no evidence that backup exists.**

Also note that branch's `docker-compose.yml` is a **regression** against `main`: it drops the Redis/Postgres
healthchecks, the `GOGC`/`GOMEMLIMIT` caps, the server healthcheck, the named `server_uploads` volume (it
reverts to an anonymous `./uploads` mount — exactly the class of bug that caused the documented past data
loss), and it re-adds the obsolete top-level `version: '3.8'` key. Merging that file as-is would undo real
work. Take the `maps/` directory from it, not the compose file.

---

## 8. Real-time (SSE)

### 8.1 The path

```
pins.Handler.CreatePin
  → events.PinCreated(ctx, pins.Event{...})          // the interface, nil-safe via nopEvents
  → realtime.Broker.PinCreated
  → json.Marshal → PUBLISH "goodspot:pins"           // Redis pub/sub
                              │
                              ▼
realtime.Handler.Stream  GET /events?bbox=&category=
  → ConnectionLimiter.TryAcquire()                    // 503 + Retry-After: 5 at capacity
  → track(ctx, cancel)                                // so graceful shutdown can cancel it
  → params.ParseBbox + validateBbox                   // 400 on bad input
  → broker.Subscribe(ctx) → *redis.PubSub
  → headers: Content-Type: text/event-stream,
             Cache-Control: no-cache,
             Connection: keep-alive,
             X-Accel-Buffering: no
  → runStreamLoop(ctx, w, ch, 20s, bbox, category)
```

### 8.2 What it emits

Exactly **one** event type: `event: pin`, `data: <json>` of `pins.Event`:

```json
{"id","user_id","location","caption","category_id","cover_url","created_at"}
```

Plus a bare `: heartbeat\n\n` comment every **20 s** (`sseHeartbeatInterval`) to keep intermediaries from
dropping an idle stream, and a per-write `http.ResponseController.SetWriteDeadline` of **10 s**
(`sseWriteTimeout`) — the server-wide `WriteTimeout` is deliberately left unset (`app.go`) because any write
deadline would sever every open stream.

Per-connection filtering happens in `realtime.matches`: `category_id` equality when `?category=` was given,
then `Sscanf(ev.Location, "POINT(%f %f)", &lng, &lat)` and a lat/lng range test against the bbox. A malformed
`location` fails the Sscanf and the event is dropped **whenever a bbox filter is active**.

`MAX_SSE_CONNECTIONS` (default 1000, `0`/empty ⇒ default) caps concurrency globally per process.

`CloseAllSSE()` is called first in `app.graceful_shutdown` on SIGINT/SIGTERM, so a deploy doesn't eat the full
shutdown timeout waiting on long-lived streams.

### 8.3 The client side, and a real bug in it

`lib/api/realtime.openPinStream(bbox, category, onPin)` opens
`new EventSource("/api/events?bbox=…&category=…")`, attaches one `pin` listener, and **validates the payload
with `pinListEntrySchema`**, wrapping everything in `try { … } catch { /* skip */ }`. `hooks/usePinStream` opens
and closes it with effect deps `[bbox, category, onPin]`; `MapApp` supplies a `useCallback`-stable handler.

**The schema does not match the event.** `pinBaseSchema` requires `geohash`, `is_hidden` and `views`;
`pinListEntrySchema` adds `username`. `pins.Event` carries **none** of those four. So every streamed pin fails
validation and is silently discarded by the `catch`.

I verified this rather than inferring it — I ran the exact schemas against a payload shaped like
`pins.Event` using the repo's own zod:

```
PARSE THREW:
  - geohash   Invalid input: expected string,  received undefined
  - is_hidden Invalid input: expected boolean, received undefined
  - views     Invalid input: expected number,  received undefined
  - username  Invalid input: expected string,  received undefined
```

Consequence: **live pins never appear on other users' maps.** The plumbing (broker, pub/sub, SSE handler,
heartbeat, limiter, filters, shutdown, client hook, `addPin` optimistic insert, the "{n} new pins in view"
toast) is all present and exercised by tests — but the payload contract between
`server/internal/modules/pins/events.go` and `client/src/lib/api/schemas.ts` doesn't line up. See
[§9.1](#91-sse-payload-doesn't-match-the-client-schema--live-pins-never-arrive).

---

## 9. Known issues and open items

This is the honest list. Where `docs/CODE_REVIEW.md` disagrees with the code, **the code wins** — that document
is a from-scratch audit whose checkboxes were never ticked, so its `[ ]` markers are stale in both directions.

### 9.1 SSE payload doesn't match the client schema — live pins never arrive

As proven in [§8.3](#83-the-client-side-and-a-real-bug-in-it). Fix is one of: extend `pins.Event` with
`geohash`, `is_hidden`, `views` and `username` (requires a join at publish time), or give the client a
dedicated, looser schema for stream events. Right now the failure is invisible — the `catch` swallows it, so
"realtime works" and "realtime silently does nothing" look identical from the outside.

### 9.2 Stale `.env.example` / documentation drift

- Root `.env.example` documents `NEXT_PUBLIC_TILES_URL`, which **nothing on this branch reads**.
- Its comment says `NEXT_PUBLIC_MAPTILER_API_KEY` is "for address geocoding only; base map tiles are
  self-hosted (see below)" — **false on `main`/`development`**, where the same key serves the tiles.
- **Three `.env.example` files with conflicting keys and conflicting ports:** root (`PORT` unset / default
  8080), `server/.env.example` (`PORT=8081`), `server/internal/config/.env.example` (`PORT=8080`).
  `docker-compose.yml` only reads the root one.
- `docker-compose.yml` does **not** pass `CORS_ALLOWED_ORIGINS`, `SENTRY_DSN`, or `SENTRY_ENV` to the server
  service, so a containerised deployment silently falls back to the localhost CORS list and can never enable
  Sentry.
- `docs/TECH_STACK.md`, `docs/project_documents/{Architecture,API}.md` and `docs/DEVELOPMENT.md` are behind the
  code (`docs/roadmap.md` item **J** tracks this).
- `docs/SECURITY.md` still states "there is no cookie-based session, so CSRF does not apply". There **is** now.
- `docs/DEVELOPMENT.md` tells you to run `make docker-up` / `make backup`, but **there is no root
  `Makefile`** — those targets live in `server/Makefile`, and running `docker compose` from `server/` finds no
  compose file. Run compose from the repo root.

### 9.3 Production tile-URL hardcoding

Two distinct things, both real:

1. **On `main`/`development` there is no tile URL to hardcode** — the style URLs are built from
   `NEXT_PUBLIC_MAPTILER_API_KEY` at module scope. But because `NEXT_PUBLIC_*` is inlined into the bundle,
   changing it requires a client **rebuild**, not just a restart. Compose only supplies it as a runtime
   `environment:` entry on the dev image; the production `client/Dockerfile` takes it as a **build `ARG`**.
   Deploy the wrong key and you get MapTiler's 401 with no runtime override path.
2. **On the self-hosted line, `docker-compose.yml` hardcodes `NEXT_PUBLIC_TILES_URL: "http://localhost:8080"`**
   with a literal string — no `${...}` interpolation and no `.env` fallback. Production would need that edited
   by hand, and the value is baked into the bundle, so a stale value silently points every map at
   `localhost:8080` on a user's machine.

The self-hosted README also says production should front the tiles with Cloudflare and then set
`NEXT_PUBLIC_TILES_URL` to the public hostname — but that value is only injectable at build time, which the
README does not call out.

### 9.4 Reverse geocoding at creation is not done

There is **no `address` column on `pins`** and no address field anywhere in the schema. `lib/api/geocoding.reverseGeocode`
exists and is used **only for display**, in exactly two places: `PinDetailPanel.tsx` and `PinPageClient.tsx`,
both in a client-side `useEffect` with an `AbortController`.

That means:
- Every pin view fires a fresh MapTiler reverse-geocode request from the browser (rate-limit exposure, and a
  hard dependency on a third party for a core piece of UI).
- Pins have no stored place name, so there is no server-side search by place, no address in metadata/OG tags,
  and no offline/printable address.
- On the self-hosted branch `reverseGeocode` returns `null` when geocoding is unavailable — so self-hosted
  mode has **no address display at all**, by design.

If you want addresses, the work is: a migration adding e.g. `pins.address TEXT`, reverse-geocoding either
server-side at create time (needs a server-side geocoder + a key in the Go service) or client-side and posting
it back, plus a backfill story for existing pins.

### 9.5 Smaller, still-open client items

Verified still present in the tree:

| Item | Evidence |
|---|---|
| **Blob-URL leak** — `URL.createObjectURL` inside `useMemo` in all three upload previews | `CreatePinButton.tsx:50`, `PinEditSheet.tsx:34`, `app/users/[id]/page.tsx:173`. `useMemo` is a hint, not a guarantee; a discarded value is a blob URL the cleanup never sees. |
| **`SearchBar` highlights the wrong option** — `activeIndex` is a flat index but the highlight compares per-group counters (`placeIdx`, `pinIdx`), so with places present the first pin highlights whenever `activeIndex === 0`. `Enter` uses the flat index correctly. Also no `aria-activedescendant`. | `SearchBar.tsx:137-138, 200-201, 226-227` |
| **`MapNavBar` account menu is `role="menu"` with no keyboard handling** — no Escape, no click-outside, no arrow navigation. `useDialogFocus` exists and is used by `PanelSheet`. | `MapNavBar.tsx:67` onward |
| **`CreatePinButton` hand-rolls its dialog** — its own `window` keydown listener, own `role="dialog"`, no focus trap/restore/`aria-modal`. Every other sheet goes through `PanelSheet`. | `CreatePinButton.tsx:72-73, 159` |
| **Stale draft on map-click dismissal** — `MapApp.handleMapClick` does `setCreateOpen(false)` directly instead of the child's `handleClose`, so `files`/`caption` survive into the next open. | `MapApp.tsx:156-167` |
| **Pin views never registered from the UI** — `usePinView` was removed in B1 (had no callers), so `pins.views` only moves when something external hits `POST /pins/:id/view`. This undercuts trending's main input. | `POST /pins/:id/view` (server route kept; client hook removed) |
| **`CategoryDropdown.tsx` removed in B1** (had no importers); `pin-default` icon is unreachable (`CATEGORY_IDS` hardcoded 1–8). | |
| **`useSessionRefresh` fires on every navigation** from both navbars, no TTL. | `Navbar.tsx:28`, `MapNavBar.tsx:40` |
| **`roles/page.tsx` has no skeleton** — the only loading page left without `SkeletonRegion`. | |
| **Unused exports (remaining)** — `storage.ErrUnsupportedContentType`, `server/deployments/k8s/` (empty tracked dir). Removed in B1: `geohash.Neighbors`, `CategoryDropdown`, `usePinView`, the 5 stub `realtime/*.go` files, all `streams/*.go` files. | |
| **Vendored MapLibre in `client/public/`** defeats tree-shaking and hand-pins a dependency `pnpm` also installs. The lint-noise half of this was fixed (`public/**` in `globalIgnores`); the vendoring itself was not. | `client/eslint.config.mjs` |
| **New engineer trap: `client/AGENTS.md`** tells agents to read `node_modules/next/dist/docs/` because Next 16 has breaking changes vs. training data. Take it seriously. | |

### 9.6 Smaller, still-open server items

| Item | Evidence |
|---|---|
| **`Recover` returns an empty body on panic** — `c.AbortWithStatus(500)` instead of `AbortWithStatusJSON`, so a client that `res.json()`s a 500 throws a *parse* error and loses the cause. | `middleware/recover.go` |
| **`Recover` reports an empty `request_id`** — reads `c.GetHeader("X-Request-ID")`, but the logger *generates* the ID and writes it to the response. Panics can't be correlated with logs. Should be `c.GetString("request_id")`. | `middleware/recover.go` |
| **`POST /errors` is unauthenticated and unthrottled** — only `BodyLimit(1 MiB)`. A bot can push 1 MB × N at line rate, and `Extra map[string]any` is passed to the reporter unsanitised. A limiter is one line; `Stack` and `URL` are also uncapped (only `Message` is, at 2048). | `router.go:81` |
| **`sslmode=disable` is hardcoded in the DSN** — fine on the compose network, cleartext password for any non-local deployment, not overridable. | `config.DSN()` |
| **`GetByLogin` OR-blocks two columns** — `WHERE email = LOWER($1) OR username = $1` can't use a single index. Email is case-insensitive, username is not, so `John` fails against a registered `john`. | `user/repository.go:96-100` |
| **`UpdatePin` treats an absent `caption` field as "clear the caption"** — `strings.TrimSpace(c.PostForm("caption"))` always yields non-nil, and the repo does `SET caption = COALESCE($2, caption)`. A PATCH that only swaps photos and omits `caption` **wipes it**. | `pins/handler.go:547` |
| **`comments.Create` doesn't check the pin exists** — an FK violation surfaces as 500 where 404 is right. `comments.List` returns `[]` for a nonexistent pin while `favorites` 404s. The convention isn't picked consistently across `comments`/`favorites`/`collections`. | `comments/repository.go` |
| **Moderation doesn't delete photos** — hiding a pin (`DeletePin`, report approval, comment `Hide`) leaves `pin_photos` rows and files in `./uploads` forever, served publicly. Only an actual `DELETE /pins/:id` cleans up, and that is itself a soft delete. | |
| **`RequestLogger`'s `skipPaths` is dead** — the skip map is built, `router.go` passes `"/health"`, and the check is commented out, so every 5 s health probe is logged. | `observability/logger/middleware.go:51-53` |
| **Rate limiting is per-process** — correct for one instance; does not hold across replicas. `docs/SECURITY.md` accepts this. | |
| **One DB read per moderated request** — `users.CurrentRole` is a deliberate trade for not trusting the JWT. | |
| **`local.go` stores `/uploads/` hardcoded** in the returned URL; `newFileID()` discards its `rand.Read` error. | `storage/` |
| **26 unpushed commits.** | see [§0](#0-read-this-first-the-branch-situation-is-unusual) |

### 9.7 The audit documents are not a reliable status board

`docs/CODE_REVIEW.md` is a 636-line review with **zero checkboxes ticked**, but a lot of its findings *have*
been fixed. Spot-checked against the current tree:

- **Fixed:** P0.1 (timeouts), P0.2 (`os.Exit(1)`), P0.3 (JWT role), P0.4 (password bytes), P1.1 (LATERAL),
  P1.2 (`0014` sort indexes), P1.3 (`ST_Intersects`), P1.4 (`COUNT(*) OVER ()`), P1.5 (`ApiError`),
  P1.6 (boundary files), P1.7 (pin-page error handling), P1.8 (`API_INTERNAL_URL`), P1.9 (httpOnly cookie),
  P1.10 (SSR cookie forwarding), P2.1 (memoized `PinList`), P2.2 (bbox quantisation + 250 ms debounce),
  P2.3 (lint ignores), P2.13 (SSE heartbeat + connection cap), P2.16 (logout degrades), P2.17 (single Redis
  client, closed), P2.18 (SSE closed on shutdown).
- **Still open:** P2.4–P2.12 (except P2.10-adjacent work), P2.14, P2.15, P2.19–P2.24, and most of P3.

Also note two things that were attempted and then **reverted** on `development`, so don't reintroduce them
without knowing why:

- `646ec0e` `router: add explicit BodyLimit to /errors endpoint` — reverted by `bd30db9`.
- `7f4a081` `pins: move authorization before multipart parsing in UpdatePin` — reverted by `6a30aff`.

Separately, the SSE test guards were deleted on purpose in `b50751d` (it removed
`TestSSEHeartbeatTiming` and `TestSSEWriteTimeoutIsShort`, which asserted the 10–30 s heartbeat window
and the short write deadline). The production behaviour was not removed: `ee7a23e` refactored the
heartbeat into `runStreamLoop` behind a `heartbeatInterval` field and the tests there assert the comment
still fires. So there is currently **no test pinning the 20 s interval or the 10 s write deadline** —
those constants are only protected by convention.

`scripts/review-issues/` has a `manifest.json` (base branch `development`, milestones P0–P3) plus a generator
and a `mark-done.py`. Whether the corresponding GitHub issues are open or closed, I can't tell from here.

### 9.8 Things I'm not certain about

Stated plainly rather than guessed:

- Whether the MapTiler key committed in `.env.example` / `client/.env.local` is domain-restricted. It should be;
  the key is in the browser bundle either way.
- Whether the dark MapTiler style currently loads cleanly (two `styleimagemissing` warnings were patched;
  I did not run the app).
- Whether the self-hosted TileServer GL dark style 404 is actually resolved.
- Whether the `philippines.mbtiles` backup the README asks for exists anywhere.
- Whether any GitHub issues from `scripts/review-issues/` are still open.
- Whether production is deployed at all. There is a production `client/Dockerfile` and
  `server/deployments/docker/server.Dockerfile`, and `config.validateStorageBase` has explicit production
  rules, but `docs/DEVELOPMENT.md` describes a single-host local-disk deployment and there are no k8s
  manifests (`server/deployments/k8s/` is an empty tracked directory).

---

## 10. Running it locally

### 10.1 The app stack (Docker — the supported path)

```bash
cp .env.example .env
# then fill in, at minimum, the three secrets compose refuses to start without:
openssl rand -hex 16   # -> POSTGRES_PASSWORD
openssl rand -hex 16   # -> REDIS_PASSWORD
openssl rand -hex 32   # -> JWT_SECRET   (config.Load hard-fails under 32 chars)
# and a real NEXT_PUBLIC_MAPTILER_API_KEY

docker compose up -d --build   # first time only
docker compose up -d           # afterwards (bind mounts hot-reload)
```

Hot reload: `./client` and `./server` are bind-mounted; the client runs `next dev` (HMR), the server runs
Air (`server/deployments/docker/server.Dockerfile.dev`, config `server/.air.toml`,
`go build -p 1 -o ./tmp/server ./cmd/api`). Rebuild only when you change a Dockerfile, `package.json` or
`go.mod`.

| Service | URL | Notes |
|---|---|---|
| client | http://localhost:3000 | `NEXT_PUBLIC_MAPTILER_API_KEY` is **required** (`:?` in compose) |
| API | http://localhost:8081 | healthcheck hits `/health` |
| Postgres | `127.0.0.1:5432` | `postgis/postgis:16-3.4`, db `goodspotdb` |
| Redis | `127.0.0.1:6379` | `requirepass` |

Both databases publish on loopback only. Migrations run automatically on every server start and are
idempotent (`schema_migrations`).

**The `make` targets in `docs/DEVELOPMENT.md` don't work from the repo root** (there is no root `Makefile`).
`make build/run/test/vet/lint/fmt` do work from `server/`. For the compose lifecycle and backups, use plain
`docker compose` from the repo root, or run `make docker-up` from `server/` with `COMPOSE_FILE=../docker-compose.yml`
— but I'd just use the plain command.

### 10.2 Env vars the stack actually needs

Read by the **Go server** (`server/internal/config/config.go`): `PORT` (compose: 8081), `APP_ENV`
(`development` | `production`, validated), `DB_HOST/PORT/USER/PASSWORD/NAME`, `LOG_LEVEL`, `LOG_FORMAT`,
`JWT_SECRET` (**≥32 chars, else `log.Fatal`**), `STORAGE_BACKEND` (**only `local`**, else `log.Fatal`),
`STORAGE_BASE_URL` (absolute http/https, no credentials/query/fragment, https unless loopback, must be
publicly reachable when `APP_ENV=production`), `REDIS_ADDR`, `REDIS_PASSWORD`, `CORS_ALLOWED_ORIGINS`,
`SENTRY_DSN`, `SENTRY_ENV`, `MAX_SSE_CONNECTIONS`.

Read by the **Next server**: `API_INTERNAL_URL` (**required in production or SSR throws**; use
`http://server:8081` in Docker), `NEXT_PUBLIC_API_URL` (SSR metadata + SSR fallback only),
`SITE_URL` (`client/Dockerfile` does `RUN test -n "$SITE_URL"`), `NEXT_PUBLIC_MAPTILER_API_KEY`.

The server also does `godotenv.Load()` and logs `no .env file found, reading from environment` if there isn't
one — so env-only works, but compose interpolates from the **root** `.env`.

### 10.3 Running the pieces without Docker

```bash
# Postgres + Redis only
docker compose up -d postgres redis

# Server (needs libvips for bimg)
cd server && cp .env.example .env && make run      # or: go run ./cmd/api

# Client
cd client && pnpm install
NEXT_PUBLIC_MAPTILER_API_KEY=<key> NEXT_PUBLIC_API_URL=http://localhost:8081 pnpm dev
```

When `next dev` runs on the host instead of in a container, `API_INTERNAL_URL` should be the published
loopback port (`http://localhost:8081`), not the service name.

### 10.4 Tests

```bash
cd server && go test ./...          # some endpoint tests need a real DB
cd client && pnpm test              # vitest, environment: node by default; .tsx tests opt into jsdom
```

`server/internal/endpointtest/integration_test.go` skips itself unless `DB_PASSWORD` is set. `go test -race`
is **not** in CI ([§9.5](#95-smaller-still-open-client-items)).

### 10.5 The self-hosted map stack (only from the dangling history)

There is no `maps/` directory on this branch, so this is not a "run it now" path — it's a "recover it first"
path.

**Recover the code (do this before anything else, and push it somewhere):**

```bash
git log --oneline refs/stash | head        # confirm the stash still holds test/self-hosted-map
git branch saved/self-hosted-map a4c8f34   # a4c8f34 = "feat: self-hosted map stack"
git checkout -b saved/self-hosted-map saved/self-hosted-map
git push -u origin saved/self-hosted-map   # so it stops depending on a stash entry
```

**Then provision the tile archive** (it is gitignored; nothing in the repo ships it):

```bash
cd maps
curl -L -o data/philippines-latest.osm.pbf \
  https://download.geofabrik.de/asia/philippines-latest.osm.pbf     # 579 MB
./build-tiles.sh                                                      # ~30-90 min, needs ~8 GB Docker memory
```

`build-tiles.sh` is idempotent, verifies the pinned Planetiler jar against its published sha256, and caches
auxiliary sources under `.planetiler-cache/`. Override with `JAVA_MEM=8g PLANETILER_VERSION=v0.10.2`.

**Then run it:**

```bash
# standalone
docker compose -f docker-compose.tiles.yml up -d
curl -s http://localhost:8080/styles/goodspot/style.json | head

# or alongside the app stack
TILES_DIR=./maps docker compose \
  -f docker-compose.yml -f maps/docker-compose.tiles.yml up -d
```

The tile port is **loopback-only by design** — put Cloudflare (or another edge) in front for anything public.
Set `NEXT_PUBLIC_TILES_URL` and rebuild the client (it is inlined).

Quick visual check without the app: serve `maps/test.html` over HTTP (`python3 -m http.server 8899`) — it must
not be opened from `file://`, browsers block the cross-origin style/tile/font requests by design. It shows a
badge for which tile host it is hitting.

**Remember [§7.5](#75-the-self-hosted-map-branch-is-dangling--this-is-the-biggest-open-risk-in-the-repo):** the
`.mbtiles` file is not in git, so recovery of the code does not recover the data. Keep your own backup.