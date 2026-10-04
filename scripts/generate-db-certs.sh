#!/bin/sh
# generate-db-certs.sh — create a self-signed Postgres server certificate for
# the production compose stack.
#
# Usage (from the repo root):
#   ./scripts/generate-db-certs.sh
#
# Writes ./secrets/db/server.crt and ./secrets/db/server.key (key mode 0600).
# The files are gitignored and mounted read-only into the Postgres container by
# docker-compose.prod.yml, which turns ssl on. The Go server connects with
# DATABASE_SSLMODE=require against this setup.
#
# Why self-signed is acceptable here: the database lives on the same Docker
# network as the API and is never exposed publicly, so `require` (encrypted,
# unauthenticated server) is the right mode. If the database ever moves
# off-host (managed Postgres), switch DATABASE_SSLMODE to verify-full and
# point it at the provider's CA instead of these files.
#
# Regenerate when the certificate expires (validity: 825 days) or when
# rotating: re-run this script, then `docker compose -f
# docker-compose.prod.yml up -d postgres` followed by a server restart.
set -eu

OUT_DIR="${OUT_DIR:-./secrets/db}"
DAYS="${CERT_DAYS:-825}"

mkdir -p "$OUT_DIR"
openssl req -x509 -newkey rsa:2048 -nodes \
  -keyout "$OUT_DIR/server.key" \
  -out "$OUT_DIR/server.crt" \
  -days "$DAYS" \
  -subj "/CN=postgres" \
  -addext "subjectAltName=DNS:postgres,DNS:localhost,IP:127.0.0.1"
chmod 0600 "$OUT_DIR/server.key"
chmod 0644 "$OUT_DIR/server.crt"
echo "wrote $OUT_DIR/server.crt and $OUT_DIR/server.key (valid $DAYS days)"
