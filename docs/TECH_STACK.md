# Tech Stack — SweetSpot247 ("Goodspot")

A monorepo with two runtimes (Go + Node) and one docker-compose stack. Product name "Goodspot"; backend module path `sweetspot247-backend`.

## Repo layout

| Path | Content |
|---|---|
| `client/` | Next.js 16 frontend (map app) |
| `server/` | Go 1.27 backend (Gin API) |
| `docker-compose.yml` | PostGIS + Redis + server + client |
| `project_documents/` | PRD, System-Design, Architecture, API docs |

## 1. Infrastructure & Deployment

| Service | Image / Runtime | Version | Notes |
|---|---|---|---|
| Database | `postgis/postgis` | 16-3.4 | Postgres 16 + PostGIS 3.4 |
| Cache | `redis` | 7-alpine | JWT blacklist store |
| Server | `golang:1.27-alpine` -> `alpine:3.20` | Go 1.27.1 | Multi-stage; installs `libvips` |
| Client | `node:20-alpine` | Node 20 -> Next 16.3.4, pnpm 10.28.0 | Standalone output, `node server.js` |

- All ports (`5432`, `6379`, `8081`, `3000`) published loopback-only (`127.0.0.1`).
- Compose reads 5 secrets from root `.env` with `??` fail-fast:
  - `POSTGRES_PASSWORD`, `REDIS_PASSWORD`, `JWT_SECRET`, `NEXT_PUBLIC_MAPTILER_API_KEY`, `NEXT_PUBLIC_API_URL`

## 2. Backend — Go 1.27.1 (`server/go.mod`)

### Framework / web stack
| Library | Version | Purpose |
|---|---|---|
| `github.com/gin-gonic/gin` | v1.12.0 | HTTP framework, routing, middleware |
| `github.com/gin-contrib/cors` | v1.7.8 | CORS middleware |
| `github.com/go-playground/validator/v10` | v10.30.3 | DTO struct validation (`required`, `max=72`, `oneof`) |
| `github.com/bytedance/sonic` | v1.15.2 | High-performance JSON serialization (Gin default engine) |

### Database / persistence
| Library | Version | Purpose |
|---|---|---|
| `github.com/jackc/pgx/v5` | v5.10.0 | Postgres driver + `pgxpool` |
| `github.com/jackc/puddle/v2` | v2.2.2 | Connection pool internals |
| **PostGIS** | 3.4 | `ST_DWithin`, `ST_MakeEnvelope(...)::geography`, `GEOGRAPHY(POINT,4326)` spatial queries |
| **Custom migrations runner** | — | `internal/platform/database/migrate.go`, plain `.sql` tracked in `schema_migrations`; no golang-migrate/tern |

### Auth / security
| Library | Version | Purpose |
|---|---|---|
| `github.com/golang-jwt/jwt/v5` | v5.3.1 | JWT HS256, 30-day expiry, `jti` revocation |
| `github.com/redis/go-redis/v9` | v9.22.0 | JWT blacklist store |
| `golang.org/x/crypto` | v0.57.0 | `bcrypt` (cost 12) password hashing |

### Imaging
| Library | Version | Purpose |
|---|---|---|
| `github.com/h2non/bimg` | v1.1.9 | libvips bindings; dimension pre-check (<=8000x8000), webp re-encode (1600px, q80) + 400px square thumbnail, 2-op concurrency semaphore |

### Utility / infrastructure
| Library | Version | Purpose |
|---|---|---|
| `github.com/mmcloughlin/geohash` | v0.10.0 | Geohash encoding for pins |
| `github.com/lmittmann/tint` | v1.2.0 | Pretty terminal logging for `log/slog` |
| `github.com/joho/godotenv` | v1.5.1 | `.env` loading |

## 3. Frontend — Next.js 16.3.4 / React 19

### Framework / rendering
| Library | Version | Purpose |
|---|---|---|
| `next` | 16.3.4 | App Router, typed routes, `output: "standalone"` |
| `react` / `react-dom` | 19.2.8 | UI runtime |
| `typescript` | ^5 | Strict mode, `@/* -> ./src/*` alias |

### Map engine
| Library | Version | Purpose |
|---|---|---|
| `maplibre-gl` | ^6.9.0 | MapLibre GL JS; MapTiler `toner-lite` style; canvas-drawn pin icons via `map.addImage`; GeoJSON source + 2 symbol layers with filter-based selection |
| Vendored worker | 6.9.0 | `public/maplibre-gl-worker.js` + `maplibre-gl-shared.mjs` (manual pin) |

### State / data
| Library | Version | Purpose |
|---|---|---|
| `zustand` | ^5.0.15 | Auth store with `persist` (localStorage `goodspot-auth`, user pruned to public fields) |
| `axios` | ^1.20.0 | API client, auth header interceptor, 401 -> auto-logout, 20s timeout |
| `zod` | ^4.5.4 | Runtime validation of API responses at the boundary |

### Styling
| Library | Version | Purpose |
|---|---|---|
| `tailwindcss` + `@tailwindcss/postcss` | ^4 | CSS-first config (no `tailwind.config.*`), `@import "tailwindcss"` + `@theme inline` |
| Geist / Geist Mono | — | Self-hosted fonts via Next font |

### Dev / tooling
| Library | Version | Purpose |
|---|---|---|
| `eslint` + `eslint-config-next` | ^9 / 16.3.4 | Flat config, `core-web-vitals` + `/typescript` |
| `vitest` + `jsdom` | ^5 / ^30 | 10 tests across 3 files |
| `pnpm` | 10.28.0 | Package manager |

## 4. Third-party APIs

| API | Where | Key | Purpose |
|---|---|---|---|
| **MapTiler Maps** | `MapView.tsx` | `NEXT_PUBLIC_MAPTILER_API_KEY` (build-time, inlined) | Vector basemap style `toner-lite` |
| **MapTiler Geocoding** | `lib/api/geocoding.ts` | same key | `/geocoding/{query}.json`, proximity bias, bbox fit |
| **MapLibre GL JS** | frontend | none (open source, BSD-3) | Map rendering; tiles served by MapTiler |
| **PostGIS** | Postgres extension | none (open source, GPLv2) | Geospatial functions |
| **libvips / bimg** | server image pipeline | none (open source) | Image processing |

### Planned / stubbed (not live)
- **LiveKit** — `streams` module stubs (`livekit_client.go`, migration `0005_streams.sql`) for live streaming.
- **WebSocket + Redis pub/sub** — `realtime` module stubs for Phase 5 geohash-room fan-out.
- **Cloudflare R2** — R2 env vars (`R2_ACCOUNT_ID`, `R2_ACCESS_KEY_ID`, `R2_SECRET_ACCESS_KEY`, `R2_BUCKET`, `R2_PUBLIC_URL`) documented but never wired; storage is local-disk only.

## 5. Service contract (ports)

- Client -> `http://localhost:8081` (Go API)
- Go API -> `postgres:5432` (PostGIS), `redis:6379`, serves `/uploads` statically
- Browser -> `https://api.maptiler.com` (basemap + geocoding) with the MapTiler key baked into the JS bundle