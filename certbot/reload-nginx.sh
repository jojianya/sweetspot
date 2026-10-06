#!/bin/sh
# Reload nginx after the real certificate has been (re)installed.
# Runs inside the certbot sidecar. The sidecar shares a PID namespace with
# the nginx container (compose: pid: "service:proxy"), so the nginx master
# process is visible here as PID 1 and can be asked to reload with SIGHUP.
set -eu

mkdir -p /etc/nginx/certs
cp -rL "/etc/letsencrypt/live/${DOMAIN}/fullchain.pem" /etc/nginx/certs/fullchain.pem
cp -rL "/etc/letsencrypt/live/${DOMAIN}/privkey.pem" /etc/nginx/certs/privkey.pem
chmod 644 /etc/nginx/certs/fullchain.pem
chmod 600 /etc/nginx/certs/privkey.pem
kill -HUP 1
echo "[certbot] installed real cert and reloaded nginx"
