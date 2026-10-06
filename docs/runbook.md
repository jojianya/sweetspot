# Runbook — Goodspot production (single VM + Caddy)

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
   (via Caddy → client? No: `/ready` is served by the API; check
   `http://127.0.0.1:8081/ready` on the VM if the API port is reachable, else
   through the proxy path) and `https://[FILL IN]/` loads in a browser.
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

1. **Stop nginx and certbot first** (port 80/443 conflict):
   ```sh
   docker compose -f docker-compose.prod.yml stop proxy certbot
   ```

2. **Restore Caddy compose and Caddyfile:**
   ```sh
   git checkout development -- docker-compose.prod.yml Caddyfile
   ```

3. **Start Caddy** (requires `caddy_data`/`caddy_config` volumes intact):
   ```sh
   docker compose -f docker-compose.prod.yml up -d
   ```
   Caddy will use the existing `caddy_data`/`caddy_config` volumes if they
   still exist and serve the valid certificates immediately. If the volumes
   were removed, Caddy will obtain fresh certificates automatically (causing
   a brief period of self-signed certificates if the cutover copy step was
   skipped).

**Rollback requires `caddy_data` and `caddy_config` volumes to be intact.**
Do not remove these volumes until nginx has run stably for at least one week
(see §0 step 6). The `nginx_certs`, `certbot_data`, and `acme_webroot`
volumes are the new ones; they can be cleaned up after confirming the nginx
migration is stable.

### Cutover from Caddy to nginx (zero-downtime certificate transfer)

**Goal:** Switch from Caddy to nginx without ever serving a self-signed
certificate to real users. The HSTS max-age is one year; a self-signed cert
would pin clients to an invalid certificate and break access.

**Prerequisites:**
- Current production runs Caddy with valid Let's Encrypt certificates in
  `caddy_data` volume.
- You have the `chore/nginx-proxy` branch checked out locally (or the
  equivalent nginx config deployed to the VM).

**Steps:**

1. **Stop Caddy, keep volumes:**
   ```sh
   docker compose -f docker-compose.prod.yml stop proxy
   # caddy_data and caddy_config volumes remain mounted and untouched
   ```

2. **Copy the live certificate from Caddy's volume to nginx's volume:**
   Caddy stores certificates under `/data/caddy/certificates/<issuer-directory>/<domain>/`.
   The issuer directory is typically `acme-v02.api.letsencrypt.org-directory` for Let's Encrypt
   production or `acme-staging-v02.api.letsencrypt.org-directory` for staging.
   File names: `fullchain.pem` and `<domain>.key`.
   ```sh
   # Find the exact certificate paths first
   docker run --rm -v <project>_caddy_data:/caddy_data:ro alpine \
     find /caddy_data -name "fullchain.pem" -o -name "*.key" | grep -E "goodspot|yourdomain"
   ```
   Expected output (replace with your actual domain):
   ```
   /caddy_data/caddy/certificates/acme-v02.api.letsencrypt.org-directory/goodspot.test/fullchain.pem
   /caddy_data/caddy/certificates/acme-v02.api.letsencrypt.org-directory/goodspot.test/goodspot.test.key
   ```
   Then copy them to nginx_certs:
   ```sh
   docker run --rm \
     -v $(docker volume inspect -f '{{.Mountpoint}}' <project>_caddy_data):/caddy_data:ro \
     -v $(docker volume inspect -f '{{.Mountpoint}}' <project>_nginx_certs):/nginx_certs \
     alpine:3.20 sh -c '
       mkdir -p /nginx_certs &&
       cp /caddy_data/caddy/certificates/acme-v02.api.letsencrypt.org-directory/goodspot.test/fullchain.pem /nginx_certs/fullchain.pem &&
       cp /caddy_data/caddy/certificates/acme-v02.api.letsencrypt.org-directory/goodspot.test/goodspot.test.key /nginx_certs/privkey.pem &&
       chmod 644 /nginx_certs/fullchain.pem &&
       chmod 600 /nginx_certs/privkey.pem
     '
   ```
   *Note: If using ZeroSSL or another CA, the issuer directory name will differ.
   Use the `find` command above to locate the exact paths.*

3. **Start nginx + certbot:**
   ```sh
   docker compose -f docker-compose.prod.yml up -d proxy certbot
   ```

4. **Verify in the first 10 minutes that nginx serves the SEEDED certificate (not bootstrap):**
   - `curl -sSf https://$DOMAIN/ready` → `{status: ok}`
   - `curl -skI https://$DOMAIN/ | grep -i "strict-transport-security"` → HSTS present
   - `openssl s_client -connect $DOMAIN:443 -servername $DOMAIN </dev/null 2>/dev/null | openssl x509 -noout -subject -issuer -enddate` → issuer must be "R3" or "E1" (Let's Encrypt), NOT self-signed
   - `docker compose -f docker-compose.prod.yml logs proxy | tail -20` → no TLS errors
   - Check that `acme_webroot` is writable: `docker compose -f docker-compose.prod.yml exec proxy ls /var/www/certbot/.well-known/acme-challenge/`

5. **Confirm the bootstrap script does NOT overwrite the seeded certificate:**
   The nginx entrypoint script (`30-bootstrap-certs.sh`) only generates a self-signed cert
   if `/etc/nginx/certs/fullchain.pem` AND `/etc/nginx/certs/privkey.pem` are BOTH missing
   or empty. If exactly one file exists, the script fails loudly with a clear error.
   Verify:
   ```sh
   docker compose -f docker-compose.prod.yml exec proxy ls -la /etc/nginx/certs/
   # Should show your seeded cert with recent timestamps, not a new self-signed cert
   ```

6. **Watch certbot's first issuance (replaces seeded cert):**
   - certbot starts immediately (no random delay on first issuance), retries every 30s on failure.
   - On failure, logs clearly: `Failed to obtain certificate` and deploy hook does NOT run.
   - **Deadline:** certbot MUST have replaced the certificate before the seeded cert expires.
     The deadline is the seeded cert's `notAfter` minus 14 days.
     Verify:
     ```sh
     openssl s_client -connect $DOMAIN:443 -servername $DOMAIN </dev/null 2>/dev/null | openssl x509 -noout -serial -notAfter
     # notAfter of seeded cert minus 14 days = hard deadline for certbot to succeed
     ```

7. **Monitor for 1 week before cleaning up Caddy volumes:**
   - Verify certbot renewal works: `docker compose -f docker-compose.prod.yml exec certbot certbot renew --dry-run`
   - Check `docker compose -f docker-compose.prod.yml logs certbot` for successful renewals
   - After 1 week of stable operation:
     ```sh
     docker volume rm <project>_caddy_data <project>_caddy_config
     ```

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

## 5. Uptime monitoring

Monitor: `[PROVIDER, FILL IN, or documentation only]`.

* Readiness: `GET /ready` must return 200 with `{status: ok}` (fails 503
  `degraded` with `db`/`redis` fields when a dependency is down). It also
  carries a `quarantine` field (`ok`/`unwritable`): `unwritable` stays HTTP
  200 but means hidden-pin files cannot leave `/uploads` — treat it as an
  ERROR-level signal and fix the volume ownership (see §3). Alert on
  non-200 for 2 consecutive minutes, and on Caddy 5xx rate from its access
  logs. This is the user-facing signal.
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
3. Postgres down: API 503s; Caddy serves errors. Restore §3 if data is at
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
* TLS to Postgres, SSE through Caddy, Secure/HSTS end to end, and the §3
  restore drill: **must be run on the real VM** (no Docker/Postgres/Caddy on
  the authoring machine). Check them off below on first deploy.
