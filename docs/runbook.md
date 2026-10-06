# Runbook — Goodspot production (single VM + nginx)

Audience: the on-call operator. All commands run from the repo root on the
production VM unless noted. Domain and provider names marked `[FILL IN]`
must be replaced during first deploy.

## 0. First deploy (one time)

1. Provision the VM (2 vCPU / 4 GiB RAM minimum; 20 GiB+ disk for DB and
   uploads). Open ports 80/443 to the internet; nothing else.
2. Install Docker + compose plugin. Clone the repo, check out the release tag.
3. Create `.env` from `.env.example`. Required: `DOMAIN`, `SITE_URL`
   (`https://[FILL IN]`), `PUBLIC_BASE_URL` (same public origin; reset links
   are built from it), `STORAGE_BASE_URL` (`https://[FILL IN]`, serving
   `/uploads`), `POSTGRES_PASSWORD`, `REDIS_PASSWORD`, `JWT_SECRET`
   (`openssl rand -hex 32`), `NEXT_PUBLIC_MAPTILER_API_KEY` (restricted in the
   MapTiler dashboard), `MAILER_WEBHOOK_URL` (production refuses to boot
   without it; email provider [FILL IN]). Optional: `SENTRY_DSN`,
   `SENTRY_ENV=production`.
4. Generate Postgres TLS assets: `./scripts/generate-db-certs.sh` (writes
   gitignored `./secrets/db/`; never commit them).
5. `docker compose -f docker-compose.prod.yml build`
6. `docker compose -f docker-compose.prod.yml up -d` (migrations run
   automatically on server boot).
7. Verify: `curl -sSf https://[FILL IN]/ready` reports `{status: ok}`
   (via nginx → client; the API itself has no published port so always
   go through the proxy) and `https://[FILL IN]/` loads in a browser.
   Confirm the session cookie carries `Secure`, HSTS is present, and
   `GET /events` streams (`curl -N https://[FILL IN]/events?bbox=...` shows
   `: heartbeat` lines).
8. Register the uptime monitor (see §5).

## 1. Deploy a new release

1. `git fetch --tags && git checkout <tag>` on the VM (deploys track tags;
   `main` is protected, see §6).
2. `docker compose -f docker-compose.prod.yml build client server`
3. `docker compose -f docker-compose.prod.yml up -d` (rolling: server first,
   then client; SSE connections drain via `CloseAllSSE` + 5s shutdown).
   On the first deploy after the M1 quarantine upgrade, set
   `QUARANTINE_SWEEP_DRY_RUN=true`, read the logged `checked`/`moved`
   counts, then set it back to `false` and redeploy.
4. Watch `docker compose -f docker-compose.prod.yml ps` until all
   `healthy`, then smoke-test §0 step 7.

## 2. Rollback

1. `git checkout <previous-tag>`, rebuild and `up -d` as above.
2. Database: migrations run forward-only on boot. Only `0015/0016/0017` have
   down scripts (`RollbackLastMigration` skips the rest), so a rollback that
   must undo a migration requires restoring from a backup (§3) instead.
   Prefer forward fixes for schema mistakes.

### Rollback to Caddy (if the nginx migration must be reverted)

The `Caddyfile` and Caddy service definition remain in git history (branch
`development`). To revert:

1. `git checkout development -- docker-compose.prod.yml Caddyfile`
2. `docker compose -f docker-compose.prod.yml up -d` (Caddy will use the
   existing `caddy_data`/`caddy_config` volumes if they still exist; if not,
   it will obtain fresh certificates automatically).
3. If you already removed the old volumes, the old Caddy data is gone; the
   new Caddy will issue fresh certificates.

The named volumes `caddy_data` and `caddy_config` are not deleted by the
nginx compose file, so they persist unless explicitly removed. The `nginx_certs`,
`certbot_data`, and `acme_webroot` volumes are the new ones; they can be
cleaned up after confirming the nginx migration is stable.

## 3. Backup and restore

Backups (nightly cron + before every deploy):

```sh
docker compose -f docker-compose.prod.yml exec -T postgres \
  pg_dump -U postgres -d goodspotdb -Fc -f /tmp/goodspotdb.dump
docker compose -f docker-compose.prod.yml cp postgres:/tmp/goodspotdb.dump \
  backups/goodspotdb-$(date +%Y%m%d-%H%M%S).dump
docker compose -f docker-compose.prod.yml exec -T server \
  sh -c 'tar cf - -C /app/uploads .' > backups/uploads-$(date +%Y%m%d-%H%M%S).tar
docker compose -f docker-compose.prod.yml exec -T server \
  sh -c 'tar cf - -C /app/quarantine .' > backups/quarantine-$(date +%Y%m%d-%H%M%S).tar
```

### Quarantine volume (hidden pins)

The server mounts two data volumes: `server_uploads:/app/uploads` and
`quarantine_data:/app/quarantine`. Both must exist; Compose creates them on
first `up -d`. When a pin is hidden, its files move from `/app/uploads` to the
same relative path under `/app/quarantine`, which nothing serves
(`GET /uploads/<file>` then 404s). A startup sweep finishes moves left over
from before this wiring existed; it is idempotent and never deletes.

Root-owned volume fix: images before this change never created
`/app/quarantine`, so a pre-existing `quarantine_data` volume can be
root-owned while the server runs as `appuser` — every move then fails
permission-denied (`/ready` reports `"quarantine":"unwritable"` and boot logs
an ERROR). Fix from a root shell in the container:

```sh
docker compose -f docker-compose.prod.yml exec -T -u root server \
  chown -R appuser:appuser /app/quarantine
```

Manual restore (operator only; there is no API un-hide path): move the file
back preserving the relative path, then un-hide the row:

```sh
docker compose -f docker-compose.prod.yml exec -T server \
  sh -c 'mv /app/quarantine/<name>.webp /app/uploads/<name>.webp'
docker compose -f docker-compose.prod.yml exec -T postgres \
  psql -U postgres -d goodspotdb -c "UPDATE pins SET is_hidden = false WHERE id = '<pin-uuid>'"
```

Copy `backups/` off-host (object storage or a second machine); a backup that
lives only on the VM is not a backup.

Restore drill (run against a disposable database and scratch volume before
you need it for real; procedure, not yet executed here — see §7):

```sh
# 1. Create a scratch database and restore into it (never --clean on live first).
docker compose -f docker-compose.prod.yml exec -T postgres \
  psql -U postgres -c 'CREATE DATABASE restore_drill'
cat backups/goodspotdb-<date>.dump | docker compose -f docker-compose.prod.yml exec -T postgres \
  pg_restore -U postgres -d restore_drill --clean --if-exists
# 2. Verify: table/row counts match live (users, pins, pin_photos, reports).
# 3. Untar uploads to a scratch dir, spot-check images open and carry no EXIF.
# 4. Drop the scratch database.
```

Point-in-time recovery is not configured (no WAL archiving); RPO is the last
backup. **Host-loss RPO: all data written since the most recent off-host
backup is lost** (database + uploads volume). If that RPO is unacceptable,
that is the trigger to move to managed Postgres + R2 object storage
(post-launch work, Tier 3).

## 4. Secret rotation

* `JWT_SECRET`: rotating invalidates **every session immediately** (all users
  are logged out; logged-out-token blacklist entries keyed by old `jti` become
  irrelevant). Procedure: set the new value in `.env`, `up -d server`, announce
  a forced re-login. There is no grace period; do it in a maintenance window.
* `POSTGRES_PASSWORD` / `REDIS_PASSWORD`: change in `.env` **and** in the
  service (`ALTER USER` / `CONFIG SET` or recreate the container after updating
  the stored secret), then restart dependents. Mismatched values fail fast at
  boot (compose `:?` guards) or as connection errors.
* MapTiler key: rotate in the dashboard (restrict to your domains + tiles and
  geocoding APIs), update `.env`, rebuild `client` (the key is baked in at
  build time), redeploy. Never commit the real value (`.env.example` carries a
  placeholder).
* DB TLS certs: re-run `./scripts/generate-db-certs.sh`, restart `postgres`
  then `server`. `verify-full` is required if the DB ever moves off-host.
* TLS certificates (Let's Encrypt via certbot): the `certbot` sidecar handles
  issuance and renewal automatically. On first boot (empty `nginx_certs`
  volume), nginx starts with a self-signed bootstrap cert and certbot obtains
  the real certificate via HTTP-01 webroot, then reloads nginx. Renewal runs
  every 12h; the deploy hook copies the new cert to the shared volume and
  HUPs nginx (zero-downtime reload). To test renewal without touching the
  production CA, set `CERTBOT_STAGING=1` in `.env` and restart the certbot
  container. To force a renewal: `docker compose -f docker-compose.prod.yml
  exec certbot certbot renew --force-renewal`. The real cert/key live in the
  `nginx_certs` named volume; the ACME webroot is in `acme_webroot`. The old
  `caddy_data` and `caddy_config` volumes are no longer used; remove them
  manually when you no longer need rollback data:
  ```
  docker volume rm <project>_caddy_data <project>_caddy_config
  ```

## 5. Uptime monitoring

Monitor: `[PROVIDER, FILL IN, or documentation only]`.

* Readiness: `GET /ready` must return 200 with `{status: ok}` (fails 503
  `degraded` with `db`/`redis` fields when a dependency is down). It also
  carries a `quarantine` field (`ok`/`unwritable`): `unwritable` stays HTTP
  200 but means hidden-pin files cannot leave `/uploads` — treat it as an
  ERROR-level signal and fix the volume ownership (see §3). Alert on
  non-200 for 2 consecutive minutes, and on nginx 5xx rate from its access
  logs (`docker compose -f docker-compose.prod.yml logs proxy`). This is the
  user-facing signal.
* Liveness: `GET /health` returns 200 whenever the process is alive
  (dependency-free by design). Container healthchecks and any restart policy
  key off liveness, never readiness.
* Server Sentry: set `SENTRY_DSN`/`SENTRY_ENV=production` in `.env` (already
  wired through `docker-compose.prod.yml`; empty default keeps log-only
  behavior). Without a DSN the server logs errors only. No client Sentry SDK
  in Tier 1: browser crashes arrive via throttled `POST /errors` into the
  same pipeline.

## 6. Branch and deploy policy

`main` is protected; releases are tags cut from `main`. Required checks per
tag: `go build`, `go vet`, `go test -race` (including the DB-backed endpoint
tests), `pnpm lint`, `tsc`, `pnpm test`, `pnpm build`, `pnpm audit` (no new
high/critical), migration test. `development` remains the integration branch.

## 7. Incident steps

1. Triage via `docker compose -f docker-compose.prod.yml logs --tail=200
   server client proxy` and `/ready` output (`db`/`redis` fields name the
   dependency).
2. Redis down: auth fails closed (401 `session verification unavailable`) —
   users cannot log in until Redis recovers; logged-out tokens stay revoked
   only after recovery. Restart `redis`; sessions resume without a deploy.
3. Postgres down: API 503s; nginx serves errors. Restore §3 if data is at
   risk, otherwise restart and watch migrations.
4. Flood/abuse: per-IP limits (register/login/pins/comments/`/errors`) plus
   global caps and the login identifier lockout hold; all users behind one
   egress share a bucket (documented in `docs/SECURITY.md`). Block at the
   firewall for L3/L4 abuse.
5. Bad deploy: §2 rollback. Bad migration: forward fix preferred; restore §3
   only if the schema must go back.

## 8. Verification status of this document

* Compose render (`docker compose config` with dummy env): verified.
* Caddyfile syntax: brace balance + directive review only — `caddy validate`
  needs the binary; run `docker run caddy:2-alpine caddy validate` on the VM.
* TLS to Postgres, SSE through nginx, Secure/HSTS end to end, and the §3
  restore drill: **must be run on the real VM** (no Docker/Postgres/nginx on
  the authoring machine). Check them off below on first deploy.
