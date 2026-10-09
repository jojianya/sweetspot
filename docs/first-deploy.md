# First Deploy on a Fresh Linux VPS (Production)

Run on the VM from the repo root. Deploys track tags (`docs/runbook.md` §1–§2).

## 1. Prerequisites

- **VPS**: 2 vCPU / 4 GiB RAM minimum; 20 GiB+ disk for DB and uploads.
- **Docker + Compose plugin** installed.
- **Domain**: an A record pointing at the VPS public IP.
- **Firewall**: only ports 22 (SSH), 80 (HTTP, required for ACME HTTP-01 challenge), and 443 (HTTPS) open to the internet. Nothing else published.

## 2. Get the Code

```sh
git clone https://github.com/jojianya/sweetspot.git
cd sweetspot
git checkout release
```

## 3. Create `.env` from `.env.example`

```sh
cp .env.example .env
```

Edit `.env` and fill in **all required values** (no secrets in this doc — use placeholders only):

**Required (must be set):**

- `DOMAIN` — public hostname (e.g., `app.example.com`). Required by `docker-compose.prod.yml` (`${DOMAIN:?}`).
- `POSTGRES_PASSWORD` — strong random value (e.g., `openssl rand -hex 32`).
- `REDIS_PASSWORD` — strong random value.
- `JWT_SECRET` — strong random value (e.g., `openssl rand -hex 32`).
- `NEXT_PUBLIC_MAPTILER_API_KEY` — restricted key from MapTiler dashboard (tiles + geocoding APIs only).
- `MAILER_WEBHOOK_URL` — production refuses to boot without it (email provider webhook endpoint).
- `MAILER_WEBHOOK_KEY` — secret for the webhook `Authorization: Bearer <key>`.
- `SITE_URL` — public HTTPS origin (e.g., `https://app.example.com`).
- `PUBLIC_BASE_URL` — same as `SITE_URL` (used for password reset links).
- `STORAGE_BASE_URL` — public HTTPS origin serving `/uploads` (e.g., `https://app.example.com`).
- `NEXT_PUBLIC_MAPTILER_API_KEY` — restricted key from MapTiler dashboard.

**CORS (critical):**

- `CORS_ALLOWED_ORIGINS` — must be the **real https domain only** (e.g., `https://app.example.com`). Must **not** contain `localhost`, `127.0.0.1`, or `goodspot.test`.

**Optional certbot knobs (see `certbot/entrypoint.sh`):**

- `CERTBOT_EMAIL` — ACME contact email (empty → `--register-unsafely-without-email`).
- `CERTBOT_STAGING` — set to `true` **only for debugging** (uses Let's Encrypt staging CA, avoids rate limits). Do **not** use for first deploy; the real cert is obtained directly.
- `CERTBOT_SERVER` — custom ACME directory URL (rarely needed).
- `CERTBOT_NO_VERIFY_SSL` — set to `true` for local test CA.

**First-run flags (required for first deploy):**

- `QUARANTINE_SWEEP_DRY_RUN=true`

**Database/Redis (already set by compose defaults, but can be overridden):**

- `DATABASE_SSLMODE=require` (production requires at least `require`).
- `DB_POOL_MAX_CONNS=10`, `DB_POOL_MAX_LIFETIME=30m`, `DB_POOL_MAX_IDLE=5m`, `DB_POOL_HEALTH_CHECK=1m`.
- `LOG_LEVEL=info`, `LOG_FORMAT=json`.
- `SENTRY_DSN=` (empty keeps log-only), `SENTRY_ENV=production`.

**Other (keep defaults unless you know you need to change them):**

- `COOKIE_SAMESITE=strict`
- `TRUSTED_PROXIES=` (leave empty; the compose network is handled by the proxy).
- `NEXT_ALLOWED_DEV_ORIGINS=localhost,127.0.0.1,goodspot.test` (dev only).
- `SENTRY_DSN=` (empty keeps log-only), `SENTRY_ENV=production`.
- `MAX_SSE_CONNECTIONS=1000`, `LOG_LEVEL=info`, `LOG_FORMAT=json`.

## 4. Validate

```sh
docker compose -f docker-compose.prod.yml config
```

This validates the compose file and interpolates `.env`. Fix any errors before proceeding.

## 5. Start

```sh
docker compose -f docker-compose.prod.yml up -d --build
docker compose -f docker-compose.prod.yml ps
```

All services should show `Up` (healthy). The `proxy` service depends on `client` being healthy.

## 6. Certificate (bootstrap + certbot)

**How it works:**

- On first boot, the `proxy` container's entrypoint (`nginx/docker-entrypoint.d/30-bootstrap-certs.sh`) checks `/etc/nginx/certs/`. If missing or empty, it generates a **self-signed certificate** (valid 30 days) so nginx can start immediately.
- The `certbot` sidecar waits for `proxy` to be healthy, then requests a **real Let's Encrypt certificate** via HTTP-01 challenge using the shared `acme_webroot` volume (`/.well-known/acme-challenge/`). On success, `certbot/reload-nginx.sh` copies the real cert/key into `nginx_certs` and sends `SIGHUP` to nginx (via shared PID namespace).
- Certbot renews every 12h automatically via its loop.

**Watch the logs:**

```sh
docker compose -f docker-compose.prod.yml logs -f certbot
```

**Common failures:**

- **DNS not pointing at VPS**: certbot will fail with connection timeout. Fix the A record and wait for propagation.
- **Port 80 blocked**: certbot HTTP-01 challenge requires inbound port 80. Check firewall/security group.

**Troubleshooting note:**

- `CERTBOT_STAGING=true` is **only for debugging** (uses Let's Encrypt staging CA to avoid rate limits). It issues a **staging certificate** that browsers don't trust. Do **not** use it for production deploys.
- If you used `CERTBOT_STAGING=true` and need to switch to the real certificate, you **must clear the `certbot_data` volume** so certbot sees no existing certificate and requests a fresh one from the production CA. The certbot entrypoint only does initial issuance when `/etc/letsencrypt/live/${DOMAIN}/fullchain.pem` is missing; otherwise it skips to the renewal loop with the same CA.

  **VERIFY ON SERVER** — exact steps to force reissue:

  ```sh
  # 1. Stop certbot
  docker compose -f docker-compose.prod.yml stop certbot

  # 2. Remove the certbot data (this deletes the ACME account and staging cert)
  docker volume rm <project>_certbot_data

  # 3. Ensure CERTBOT_STAGING is unset in .env
  # 4. Restart certbot to trigger fresh issuance against production CA
  docker compose -f docker-compose.prod.yml up -d certbot
  ```

  After step 4, watch `docker compose -f docker-compose.prod.yml logs -f certbot` for "initial issuance succeeded" from the production CA.

## 7. Verify

```sh
# HTTP → HTTPS redirect (should be 301 to https://DOMAIN:443)
curl -s -o /dev/null -w "%{http_code} -> %{redirect_url}\n" http://DOMAIN/

# Readiness
curl -sSf https://DOMAIN/ready

# Certificate details
echo | openssl s_client -connect 127.0.0.1:443 -servername DOMAIN 2>/dev/null | openssl x509 -noout -subject -dates

# Security headers
curl -sI https://DOMAIN/ | grep -iE 'strict-transport|content-type-options|frame-options|referrer'

# Proxy verification (if scripts/verify-proxy.sh exists and you have a session cookie)
./scripts/verify-proxy.sh DOMAIN /tmp/cookies.txt
```

All checks should pass.

## 8. Switch from Staging to Production Certificate

If you deployed with `CERTBOT_STAGING=true` and now need the real certificate:

1. Edit `.env` and remove `CERTBOT_STAGING` (unset it completely).
2. Stop certbot and clear its data so it requests a fresh production cert:

   ```sh
   docker compose -f docker-compose.prod.yml stop certbot
   docker volume rm <project>_certbot_data
   ```

3. Restart certbot to trigger fresh issuance against the production CA:

   ```sh
   docker compose -f docker-compose.prod.yml up -d certbot
   ```

4. Watch the logs for "initial issuance succeeded" from the production CA:

   ```sh
   docker compose -f docker-compose.prod.yml logs -f certbot
   ```

**VERIFY ON SERVER** — the exact reissue behavior depends on the certbot entrypoint logic; if the cert already exists in `certbot_data`, certbot skips initial issuance and goes straight to the renewal loop with the same CA. Clearing the volume is the only reliable way to force a fresh production issuance.

## 9. Quarantine Sweep

Dry-run is opt-in for the first sweep against existing data: keep
`QUARANTINE_SWEEP_DRY_RUN=true` until the log shows clean `checked`/`moved` counts (no unexpected moves). Then:

1. Edit `.env`: `QUARANTINE_SWEEP_DRY_RUN=false`.
2. Recreate the server to pick up the change:

   ```sh
   docker compose -f docker-compose.prod.yml up -d server
   ```

## 10. Updating (New Release)

```sh
git pull origin release
git checkout <new-tag>
docker compose -f docker-compose.prod.yml up -d --build
```

The stack rebuilds only changed images and restarts affected services.

## 11. Backups

Nightly (or before every deploy):

```sh
# Database
docker compose -f docker-compose.prod.yml exec -T postgres \
  pg_dump -U postgres -d goodspotdb -Fc -f /tmp/goodspotdb.dump
docker compose -f docker-compose.prod.yml cp postgres:/tmp/goodspotdb.dump \
  backups/goodspotdb-$(date +%Y%m%d-%H%M%S).dump

# Uploads volume
docker compose -f docker-compose.prod.yml exec -T server \
  sh -c 'tar cf - -C /app/uploads .' > backups/uploads-$(date +%Y%m%d-%H%M%S).tar

# Quarantine volume
docker compose -f docker-compose.prod.yml exec -T server \
  sh -c 'tar cf - -C /app/quarantine .' > backups/quarantine-$(date +%Y%m%d-%H%M%S).tar
```

## 12. Rollback

```sh
git checkout <previous-tag>
docker compose -f docker-compose.prod.yml build client server
docker compose -f docker-compose.prod.yml up -d
```

**⚠️ WARNING — Never run on the server:**

- `docker compose down -v`
- `docker volume prune`
- `docker system prune`

These delete the named volumes (`postgres_data`, `server_uploads`, `quarantine_data`, `nginx_certs`, `certbot_data`, `acme_webroot`) and destroy your data irreversibly.

---

## Notes

- The `certbot_data` volume stores the ACME account and certificates. Do not delete it.
- The `nginx_certs` volume holds the live cert/key that nginx reads. Certbot writes there on renewal.
- The `acme_webroot` volume is served at `/.well-known/acme-challenge/` for HTTP-01 challenges.
- Postgres runs with TLS (`ssl=on`); the self-signed certs are generated by `./scripts/generate-db-certs.sh` (run before first deploy).
- Database backups use the `postgres` service user `postgres` and DB `goodspotdb` (from compose).
- The `server` container runs as non-root (Air in dev, `./server` binary in prod).
- `QUARANTINE_SWEEP_DRY_RUN=true` logs checked/moved counts without moving files; set to `false` after the first clean sweep.