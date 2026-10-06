#!/bin/sh
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

if [ ! -s "$CERT_DIR/fullchain.pem" ] || [ ! -s "$CERT_DIR/privkey.pem" ]; then
  echo "[bootstrap] no cert found; generating self-signed bootstrap cert for ${DOMAIN:-unknown}"
  mkdir -p "$CERT_DIR"
  openssl req -x509 -newkey rsa:2048 -nodes \
    -keyout "$CERT_DIR/privkey.pem" \
    -out "$CERT_DIR/fullchain.pem" \
    -days 30 -sha256 \
    -subj "/CN=${DOMAIN:-localhost}" \
    -addext "subjectAltName=DNS:${DOMAIN:-localhost}"
  chmod 644 "$CERT_DIR/fullchain.pem"
  chmod 600 "$CERT_DIR/privkey.pem"
fi
