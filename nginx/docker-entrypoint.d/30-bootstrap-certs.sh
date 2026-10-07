#!/bin/sh
# Bootstrap self-signed cert on first run; minted cert lives at
# /etc/nginx/certs/fullchain.pem + privkey.pem. If both missing, generates
# a 30‑day self‑signed cert for ${DOMAIN:-localhost} via openssl. If both
# exist, verifies key matches cert; mismatch kills startup. If exactly one
# file exists, aborts startup with error.

# First-boot cert bootstrap, runs inside the nginx container entrypoint
# (/docker-entrypoint.d scripts run before nginx starts).
#
# nginx needs a real, readable cert/key at the paths referenced by
# default.conf.template before it can start. On a brand-new nginx_certs
# volume there is no cert yet, so we mint a short-lived self-signed one; the
# certbot sidecar later obtains the real certificate and swaps it in via
# SIGHUP reload. Both files live on the shared nginx_certs volume, so the
# self-signed bootstrap persists and is only overwritten by a successful
# issuance.
set -eu

CERT_DIR=/etc/nginx/certs
FULLCHAIN="$CERT_DIR/fullchain.pem"
PRIVKEY="$CERT_DIR/privkey.pem"

# Check for inconsistent state: exactly one file exists
if [ -e "$FULLCHAIN" ] && [ ! -e "$PRIVKEY" ]; then
  echo "[bootstrap] ERROR: $FULLCHAIN exists but $PRIVKEY is missing — refusing to start"
  exit 1
fi
if [ ! -e "$FULLCHAIN" ] && [ -e "$PRIVKEY" ]; then
  echo "[bootstrap] ERROR: $PRIVKEY exists but $FULLCHAIN is missing — refusing to start"
  exit 1
fi

# Both missing: generate bootstrap self-signed cert
if [ ! -s "$FULLCHAIN" ] && [ ! -s "$PRIVKEY" ]; then
  echo "[bootstrap] no cert found; generating self-signed bootstrap cert for ${DOMAIN:-unknown}"
  mkdir -p "$CERT_DIR"
  openssl req -x509 -newkey rsa:2048 -nodes \
    -keyout "$PRIVKEY" \
    -out "$FULLCHAIN" \
    -days 30 -sha256 \
    -subj "/CN=${DOMAIN:-localhost}" \
    -addext "subjectAltName=DNS:${DOMAIN:-localhost}"
  chmod 644 "$FULLCHAIN"
  chmod 600 "$PRIVKEY"
fi

# Both exist: verify they are a matching pair
if [ -s "$FULLCHAIN" ] && [ -s "$PRIVKEY" ]; then
  echo "[bootstrap] existing certificate found, verifying key matches cert"
  CERT_PUB=$(openssl x509 -noout -pubkey -in "$FULLCHAIN" 2>/dev/null | openssl pkey -pubin -outform der 2>/dev/null | openssl dgst -sha256 | cut -d' ' -f2)
  KEY_PUB=$(openssl pkey -pubout -in "$PRIVKEY" -outform der 2>/dev/null | openssl dgst -sha256 | cut -d' ' -f2)
  if [ "$CERT_PUB" != "$KEY_PUB" ]; then
    echo "[bootstrap] ERROR: certificate and private key do not match — refusing to start"
    exit 1
  fi
  echo "[bootstrap] certificate and key match"
fi