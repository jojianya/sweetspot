# Engineering appendix — sweetspot

Project appendix to `good-and-bad-practices.md`. The guide is generic and
tiered; this file records what this project actually adopted, what it
deliberately did not, and where each rule is enforced. Read both before
changing code.

## Declared tier: T2 (Production-critical)

**Why T2.** The project handles authentication (bcrypt passwords, JWTs, session
revocation), personal data (emails, social profile links, uploaded photos), and
payments-adjacent moderation state. Other people depend on it: it is a deployed
web service with a public API. Several developers and AI agents work on it.

**What that means in practice.** T2's service/repository layering, real-database
integration tests, and expand/contract migrations apply. So do T1's MUST rules.
T3 is not adopted; see the exclusions below.

### Adopted above T1

| Practice | Where | Why |
| --- | --- | --- |
| Service and repository layers for real business rules (Part 1) | `server/internal/modules/*/{service.go,repository.go}` | There are real rules: hidden-pin visibility, owner-or-moderator authorization, last-owner protection, session revocation floors. |
| Repository interfaces so logic is testable without a database (1.4) | `Repository` interface per module | The characterization and wiring tests stub the repository and exercise real service code. |
| Architecture tests with an allowlist (1.7, T3) | `server/internal/arch/arch_test.go` | Adopted despite being T3: it already exists, it is cheap, and all three allowlists are **empty**, so there is nothing to shrink. Never add an entry — fix the import. |
| Integration tests against a real database (3.4) | `server/internal/endpointtest/*_db_test.go`, `internal/platform/database/migrations_test.go` | Repositories contain PostGIS, `ST_AsText`, `FOR UPDATE`, window functions and triggers. Mocks cannot catch a wrong query. |
| Health checks for liveness and readiness (9.3) | `GET /health`, `GET /ready` | Compose consumes them (`healthcheck` blocks in both compose files). |
| CI/CD hygiene (6.2) | `.github/workflows/ci.yml` | Push and PR gating on `development`, `main`, `release`. |
| Release verification (6.5, partial) | `docs/runbook.md`, tag-based deploy off `release` | There is a documented deploy procedure and a tag-based rollback point. |

### Deliberately NOT adopted

| Practice | Why not |
| --- | --- |
| Metrics and alerts (9.3, T2) | No metrics registry. Liveness/readiness exist; error rate, latency and saturation do not. The only telemetry is structured logs and optional Sentry. **This is a real T2 gap** — record it here rather than pretending. |
| RTO / RPO targets and restore drills (6.5, T2) | The business has defined no recovery requirement. Storage is local disk on named volumes; the RPO consequence is documented in `docs/runbook.md`. Numbers should be set from a requirement, not invented. |
| Outbox, idempotency keys, lease clocks, fencing tokens, CDC, circuit breakers, stampede protection, contract tests, change-type test gates (2.9, 2.10, 2.13, 3.7, T2/T3) | No measured need. The writes that matter are single-transaction; the fan-out (Redis pub/sub) is best-effort by design and a lost event is recovered by the next full fetch. If a measurement says otherwise, promote the practice with the measurement recorded here. |
| axe / automated accessibility checking in CI (4.4, T2) | The checklist is followed by hand. |
| Bundle-size gates (4.3, T3) | No bundle budget has been set. |

## Layers, modules and checks

```
entry point  cmd/api, cmd/reprocess-exif
     │
handler      internal/http/router.go  +  modules/*/handler.go, routes.go
     │
service      modules/*/service.go  (rules, no driver, no gin, no middleware)
     │
repository   modules/*/repository.go  (SQL, driver types, translates to domain errors)
     │
datastore    PostgreSQL + PostGIS (pgx/v5 pool), Redis (go-redis/v9)
platform     internal/platform/{database,storage,imaging,cache}
wire         internal/di/container.go  builds every dependency
```

Modules: `auth`, `collections`, `comments`, `favorites`, `pins`, `reactions`,
`realtime`, `reports`, `social`, `user`.

Architecture checks, run by `go test ./internal/arch/`:

- `TestModelHasNoInfraImports` — `model.go` must not import `pgtype`, gin,
  `platform/database`, `pgconn`, `pgxpool`.
- `TestServiceHasNoInfraImports` — `service.go` must not import `pgconn`,
  `internal/http/middleware`, gin, `platform/database`.
- Allowlists `allowModelPgtype`, `allowServicePgconn`,
  `allowServiceMiddleware`: **all empty**. Entries may only be removed.

Note the limit stated in the guide (1.6): the test scans `model.go` and
`service.go` by name. The design rule covers every file in a module, so do not
move a forbidden import into `helpers.go` or `entitlements.go`.

## Commands

```
cd server
gofmt -l .                    # must print nothing
go build ./...
go vet ./...
go test ./...                 # add -race -count=1 before merging
go test ./internal/platform/database/ -run TestMigrationUpDownUp   # needs PostGIS

cd client
pnpm lint                     # eslint, 0 errors
pnpm next typegen             # generates .next/types, needed before tsc on a cold tree
pnpm tsc --noEmit
pnpm test                     # vitest run
pnpm build
```

CI runs all of the above (`.github/workflows/ci.yml`), plus
`go run golang.org/x/vuln/cmd/govulncheck@v1.8.0 ./...` as a **blocking** step
and `pnpm audit --prod` as a **non-blocking** one. `gofmt -l .` is enforced by
`test -z "$(gofmt -l .)"`.

Integration and Redis-backed tests gate on the environment and skip when it is
absent (`ENDPOINT_TEST_DB`, `DB_PASSWORD`, `MIGRATION_TEST_DB`, a reachable
Redis). A skip is not a pass: CI wires the services so they run there.

## How each rule is enforced

| Rule | Enforcement |
| --- | --- |
| 1.1/1.2/1.3/1.4 (layering) | `internal/arch/arch_test.go`, empty allowlists |
| 2.1 (no SQL from strings) | Manual; every query in the tree uses pgx placeholders |
| 2.2 (bounded lists) | Manual; `params.ParseLimit`/`ParseOffset` (`MaxOffset = 10000`) plus per-query hard caps |
| 2.3 (no ignored errors) | Manual; `_ =` only in tests and documented best-effort cleanup |
| 2.4 (timeouts) | Manual; per-request contexts, `BodyLimit`, SSE per-write deadline |
| 2.5 (config in one place) | `internal/config/config.go` validates at load and `log.Fatal`s on bad values |
| 2.7 (one rule, many copies) | `pins.VisiblePinExists` is the single "hidden pins do not exist" check, used by comments, favorites, reports, reactions, collections |
| 2.8 (authorization) | Live role read from the DB per request (`users.CurrentRole`, `IsModerator`); no role in the JWT. The `UPDATE ... WHERE owner OR moderator` predicate is the authorization, not a load-then-decide |
| 4.4 (a11y) | Manual checklist |
| 5.6 (env vars, safe defaults) | `.env.example`; `QUARANTINE_SWEEP_DRY_RUN` defaults to `true` |
| 6.1 (branch flow) | `docs/branching.md` |
| 7.1 (XSS) | React escaping; client house rule forbids `dangerouslySetInnerHTML` (`client/AGENTS.md`) |
| 7.4 (sessions) | httpOnly cookie, `SameSite` configurable (`COOKIE_SAMESITE`, strict default), Redis blacklist checked on every request, `sessions_valid_after` floor |
| 8.1 (dependencies) | `pnpm-lock.yaml`, `go.sum`, govulncheck blocking |
| 9.2 (no PII in logs) | Manual; `middleware/report.go` deliberately attaches no client IP |

## Mandatory checks per change type

Not adopted as a gate (3.7 is T2/T3). The working rule is: run the full suite in
`server/` and `client/` for any change, and treat a change that touches
migrations, auth or the visibility rule as needing the DB-backed tests, not just
unit ones.

## Branch and release flow

`feature/*` (or `fix/*`) → `development` → `main` → `release`, fast-forward
only, one task per branch. Every promotion to `release` is a tag (`vX.Y.Z`) and
a push to `release` deploys. Full detail in `docs/branching.md`; rollback is
`reset --hard` to the previous tag on `release`.

## Environments and credentials

| Environment | Compose file | Notes |
| --- | --- | --- |
| local | `docker-compose.yml` | Postgres and Redis published on `127.0.0.1` only. Server published on `0.0.0.0:8081` and client on `0.0.0.0:3000` for LAN phone testing — that bypasses nginx, so the edge body limit and security headers do not apply on those ports. Redis **requires** a password locally. |
| production | `docker-compose.prod.yml` | Only nginx publishes (80/443). Postgres, Redis, server and client stay on the internal bridge. `DATABASE_SSLMODE` defaults to `require`; `COOKIE_SAMESITE` defaults to `strict`; `QUARANTINE_SWEEP_DRY_RUN` defaults to `false`. |
| staging | none | Not defined. |

Credentials come from the repo-root `.env` (`godotenv.Load("../.env")` in
`internal/config/config.go`); the server-local `.env` is a legacy fallback.
`.env` is gitignored and no secrets are tracked. `secrets/db/` holds generated
Postgres TLS assets and is gitignored. The `NEXT_PUBLIC_MAPTILER_API_KEY` is
sent to the browser by design — it must stay domain-restricted in the MapTiler
console.

## Migrations

Numbered SQL files in `server/internal/platform/database/migrations/`, applied
in filename order and recorded in `schema_migrations`. Rollback scripts live in
`migrations/down/` as `<name>.down.sql`.

- Applied migrations are immutable; fix forward.
- One migration may not mix schema change and data backfill (2.14).
- `executeMigration` splits statements when `CREATE INDEX CONCURRENTLY` is
  present, because it cannot run in a transaction.
- `verifySchema` fails boot if a required table is missing, so an empty
  migrations directory cannot pass silently.
- `TestMigrationUpDownUp` applies up/down/up against PostGIS in CI.

## Logging and monitoring

- `log/slog` everywhere; `internal/observability/logger` configures tint (text)
  or JSON, colouring the status only in text mode.
- `middleware.RequestLogger` attaches a request-scoped logger with `request_id`.
- `internal/observability/report` is the single place errors go: slog always,
  Sentry when `SENTRY_DSN` is set.
- `POST /errors` ingests browser crash reports into the same pipeline. It is
  public and rate-limited per IP (30/min) with a 1 MB body cap; the payload is
  truncated and capped before it reaches the reporter.
- No metrics registry. See the exclusions table.

## Dependencies

- Server: `go.mod`/`go.sum`; `govulncheck` is blocking in CI.
- Client: `pnpm-lock.yaml` (pinned `packageManager`); `pnpm audit --prod` is
  non-blocking, and the reason is written in the workflow comment — triage
  advisories explicitly rather than blocking all work.
- Both Dockerfiles pin base images by digest.

## Project-specific rules

- **"Hidden pins do not exist."** `pins.VisiblePinExists` is the only check;
  every module that attaches data to a pin uses it. A hidden pin answers 404
  everywhere, including search, trending, feeds and the SSE stream.
- **Authorization is the UPDATE predicate.** Deletes and updates match
  `WHERE id AND is_hidden = false AND (owner OR moderator)`, so there is no
  TOCTOU between a read and a write.
- **Roles are read live, never from the JWT.** A demotion takes effect on the
  caller's next request. `sessions_valid_after` revokes all tokens on a
  password reset.
- **Absent-account timing.** Login compares against a bcrypt hash generated at
  the same cost as real passwords, so a missing account is not faster than a
  wrong password.
- **Uploads are re-encoded, never stored as received.** libvips re-encodes to
  webp with metadata stripped; size is enforced on actual bytes, not the
  client-claimed header.
- **Quarantined files are never deleted.** Hiding a pin moves its files out of
  the static root; there is no un-hide path.
- **Rate limits are in-process and per-replica.** That tradeoff is documented
  at each limiter. The login lockout is keyed on (IP, identifier) with a
  separate per-identifier backstop.
- **Client house rule:** never `dangerouslySetInnerHTML` on user-generated
  content (`client/AGENTS.md`).
