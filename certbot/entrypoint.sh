#!/bin/sh
# Certbot sidecar entrypoint:
#   1. Wait for nginx, then obtain the first certificate via HTTP-01 webroot.
#   2. Install it and HUP nginx to replace the self-signed bootstrap cert.
#   3. Every 12h: renew; on renewal certbot fires --deploy-hook to install +
#      HUP nginx again.
#
# Env knobs (all optional):
#   CERTBOT_EMAIL        registration contact (omitted -> --register-unsafely-without-email)
#   CERTBOT_STAGING=1    use the Let's Encrypt staging CA
#   CERTBOT_SERVER       custom ACME directory URL (e.g. local test CA)
#   CERTBOT_NO_VERIFY_SSL=1  allow a self-signed CA directory cert (local tests)
set -u

EXTRA_ARGS=""
if [ -n "${CERTBOT_EMAIL:-}" ]; then
  EXTRA_ARGS="$EXTRA_ARGS --email ${CERTBOT_EMAIL} --no-eff-email"
else
  EXTRA_ARGS="$EXTRA_ARGS --register-unsafely-without-email"
fi
[ -n "${CERTBOT_SERVER:-}" ] && EXTRA_ARGS="$EXTRA_ARGS --server ${CERTBOT_SERVER}"
[ "${CERTBOT_STAGING:-}" = "1" ] && EXTRA_ARGS="$EXTRA_ARGS --staging"
[ "${CERTBOT_NO_VERIFY_SSL:-}" = "1" ] && EXTRA_ARGS="$EXTRA_ARGS --no-verify-ssl"

echo "[certbot] starting (DOMAIN=${DOMAIN})"

# Initial issuance, with retry: nginx may not be serving the webroot yet.
# Retry every 1 hour on failure, log clearly.
if [ ! -e "/etc/letsencrypt/live/${DOMAIN}/fullchain.pem" ]; then
  while true; do
    # shellcheck disable=SC2086
    if certbot certonly --webroot -w /var/www/certbot -d "${DOMAIN}" \
      --non-interactive --agree-to $EXTRA_ARGS; then
      echo "[certbot] initial issuance succeeded"
      break
    else
      echo "[certbot] ERROR: initial issuance failed, retrying in 1 hour"
      sleep 1h
    fi
  done
fi

# Install + reload right away (also covers "nginx_certs volume was wiped but
# certbot_data survived" restarts).
/usr/local/bin/reload-nginx.sh || echo "[certbot] WARN: reload failed (nginx mid-start?), continuing"

echo "[certbot] entering 12h renewal loop"
while true; do
  sleep 12h
  # shellcheck disable=SC2086
  if certbot renew --webroot -w /var/www/certbot $EXTRA_ARGS \
    --deploy-hook /usr/local/bin/reload-nginx.sh; then
    echo "[certbot] renewal succeeded"
  else
    echo "[certbot] ERROR: renewal failed, will retry in 12h"
  fi
done