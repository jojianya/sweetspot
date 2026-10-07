# Cutover: Caddy → nginx + certbot (production VM)

Run on the VM from the repo root. Deploys track tags (`docs/runbook.md` §1–§2).

## 0. Fill in `.env` (placeholders only — put real values on the server)

```sh
DOMAIN=example.com
SITE_URL=https://example.com
PUBLIC_BASE_URL=https://example.com
STORAGE_BASE_URL=https://example.com
POSTGRES_PASSWORD=<generate-with-openssl-rand-hex-32>
REDIS_PASSWORD=<generate-with-openssl-rand-hex-32>
JWT_SECRET=<generate-with-openssl-rand-hex-32>
NEXT_PUBLIC_MAPTILER_API_KEY=<restricted-key-from-maptiler-dashboard>
MAILER_WEBHOOK_URL=<email-provider-webhook-url>
MAILER_WEBHOOK_KEY=<email-provider-secret>
CERTBOT_EMAIL=<acme-contact-email-or-empty>
# Optional: CERTBOT_STAGING=1 for a dry run against the staging CA.
```

`DOMAIN` is required (`docker-compose.prod.yml` uses `${DOMAIN:?}`). The
certbot knobs are all optional: `CERTBOT_EMAIL` (empty →
`--register-unsafely-without-email`), `CERTBOT_STAGING`,
`CERTBOT_SERVER`, `CERTBOT_NO_VERIFY_SSL` (see `certbot/entrypoint.sh`).

## 1. DNS and firewall

1. Point `DOMAIN` at the VM and confirm: `dig +short DOMAIN`.
2. Port 80 must be open to the internet (certbot HTTP-01 needs it); 443 too.
   Nothing else is published — only `proxy` binds `80:80` and `443:443`.

## 2. Free ports 80/443 without touching Caddy's data

The old (`Caddyfile`, commit `cbed149`) and new stacks both name the
container `goodspot-proxy`. Stop it; do NOT remove anything:

```sh
docker stop goodspot-proxy
```

NEVER run `docker volume rm`, `docker compose down -v`, or `docker compose down`
here — the `caddy_data`/`caddy_config` volumes must survive for rollback.
`docker compose stop` is safe; volume deletion is not.

## 3. Seed nginx with Caddy's live certificate (optional but avoids downtime)

History (`cbed149`) mounted `caddy_data:/data`, so Caddy's certs live in that
volume. Find the exact files on the server — issuer directory layout is
VERIFY ON SERVER:

```sh
docker volume inspect -f '{{.Mountpoint}}' <project>_caddy_data
sudo find <mountpoint> -name fullchain.pem
```

Expected layout: `<mountpoint>/caddy/certificates/<acme-directory>/DOMAIN/fullchain.pem`
plus the matching `DOMAIN.key`. Copy them into the nginx volume before first
start (VERIFY ON SERVER for the exact source names):

```sh
CADDY_MOUNT=$(docker volume inspect -f '{{.Mountpoint}}' <project>_caddy_data)
NGINX_MOUNT=$(docker volume inspect -f '{{.Mountpoint}}' <project>_nginx_certs)
sudo cp "$CADDY_MOUNT/caddy/certificates/VERIFY-ON-SERVER/fullchain.pem" "$NGINX_MOUNT/fullchain.pem"
sudo cp "$CADDY_MOUNT/caddy/certificates/VERIFY-ON-SERVER/DOMAIN.key" "$NGINX_MOUNT/privkey.pem"
sudo chmod 644 "$NGINX_MOUNT/fullchain.pem" "$NGINX_MOUNT/privkey.pem"
```

If the source filenames differ, copy whatever `fullchain.pem` + key pair the
`find` above locates. If this step is skipped, the bootstrap script mints a
30-day self-signed cert (`nginx/docker-entrypoint.d/30-bootstrap-certs.sh`)
and certbot replaces it after issuance.

## 4. Start nginx + certbot

```sh
git fetch --tags && git checkout <tag>
docker compose -f docker-compose.prod.yml build
docker compose -f docker-compose.prod.yml up -d
```

Shared volumes: `nginx_certs` (`/etc/nginx/certs`), `certbot_data`
(`/etc/letsencrypt`), `acme_webroot` (`/var/www/certbot`, served at
`/.well-known/acme-challenge/`). Certbot issues on boot, installs via
`certbot/reload-nginx.sh`, then renews every 12h.

## 5. Verify

```sh
docker compose -f docker-compose.prod.yml exec proxy nginx -t
docker compose -f docker-compose.prod.yml logs certbot | tail -20
curl -sSf https://DOMAIN/ready
echo | openssl s_client -connect 127.0.0.1:443 -servername DOMAIN 2>/dev/null | openssl x509 -noout -subject -dates
curl -sI https://DOMAIN/ | grep -iE 'strict-transport|content-type-options|frame-options|referrer'
./scripts/verify-proxy.sh DOMAIN /tmp/cookies.txt
```

## 6. Rollback to Caddy

Volumes were never deleted, so redeploy the previous tag (`docs/runbook.md` §2):

```sh
git checkout <previous-tag>
docker compose -f docker-compose.prod.yml build client server
docker compose -f docker-compose.prod.yml up -d
```

The old stack brings Caddy back with its intact `caddy_data`/`caddy_config`.
