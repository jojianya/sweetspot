#!/usr/bin/env bash
#
# build-tiles.sh — regenerate maps/philippines.mbtiles from the raw OSM extract
# using Planetiler's built-in OpenMapTiles schema.
#
# NOTE: the module currently ships with Geofabrik's *pre-built Shortbread*
# tiles (see README.md) because they're ready to serve and need no local
# rendering. Running this script replaces philippines.mbtiles with an
# OpenMapTiles-schema build, which requires an OpenMapTiles style (the bundled
# Shortbread style.json will NOT render against it). It is kept for when you
# want a self-generated, OpenMapTiles/OSM-Bright-compatible tileset.
#
# Usage:
#   ./build-tiles.sh                 # regenerate cleanly (idempotent)
#
# Env overrides:
#   PLANETILER_VERSION=v0.10.2       # pinned Planetiler release
#   JAVA_MEM=4g                      # JVM heap for Planetiler (needs ~1.5x pbf size)
#   DOCKER_IMAGE=eclipse-temurin:21-jdk
#
set -euo pipefail

MAPS_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
INPUT="${MAPS_DIR}/data/philippines-latest.osm.pbf"
OUTPUT="${MAPS_DIR}/philippines.mbtiles"
JAR="${MAPS_DIR}/planetiler.jar"
CACHE_DIR="${MAPS_DIR}/.planetiler-cache"

PLANETILER_VERSION="${PLANETILER_VERSION:-v0.10.2}"
JAVA_MEM="${JAVA_MEM:-4g}"
DOCKER_IMAGE="${DOCKER_IMAGE:-eclipse-temurin:21-jdk}"
RELEASE_URL="https://github.com/onthegomap/planetiler/releases/download/${PLANETILER_VERSION}"

log() { printf '\033[1;34m[build-tiles]\033[0m %s\n' "$*"; }
die() { printf '\033[1;31m[build-tiles] error:\033[0m %s\n' "$*" >&2; exit 1; }

command -v docker >/dev/null 2>&1 || die "docker is required (Planetiler runs in a container; the host has no guaranteed JDK)."
[ -f "$INPUT" ] || die "missing input '$INPUT'. Download it first:
  curl -L -o data/philippines-latest.osm.pbf https://download.geofabrik.de/asia/philippines-latest.osm.pbf"

# 1. Fetch + verify the pinned Planetiler jar (cached between runs).
if [ ! -f "$JAR" ]; then
  log "Downloading Planetiler ${PLANETILER_VERSION} ..."
  curl -fL --retry 3 -o "${JAR}.tmp" "${RELEASE_URL}/planetiler.jar"
  curl -fL --retry 3 -o "${JAR}.sha256" "${RELEASE_URL}/planetiler.jar.sha256"
  expected="$(awk '{print $1}' "${JAR}.sha256")"
  actual="$(sha256sum "${JAR}.tmp" | awk '{print $1}')"
  [ "$expected" = "$actual" ] || { rm -f "${JAR}.tmp" "${JAR}.sha256"; die "planetiler.jar sha256 mismatch (expected $expected, got $actual)"; }
  mv "${JAR}.tmp" "$JAR"
  rm -f "${JAR}.sha256"
  log "planetiler.jar verified (sha256 ${actual:0:12}…)"
else
  log "Using cached planetiler.jar"
fi

# 2. Remove any previous output so a re-run regenerates deterministically.
rm -f "$OUTPUT"
mkdir -p "$CACHE_DIR"

# 3. Run Planetiler. Its default profile IS the OpenMapTiles schema, so no
#    external profile jar is needed. Auxiliary sources are cached under
#    .planetiler-cache/ via the pinned --download flag.
log "Generating $(basename "$OUTPUT") with heap ${JAVA_MEM} — this can take 30-90+ min ..."
docker run --rm \
  -v "${MAPS_DIR}:/data" \
  -v "${CACHE_DIR}:/root/.cache/planetiler" \
  -w /data \
  "$DOCKER_IMAGE" \
  java -Xmx"${JAVA_MEM}" -jar /data/planetiler.jar \
    --osm-path=/data/data/philippines-latest.osm.pbf \
    --output=/data/philippines.mbtiles \
    --force \
    --download

log "Done: $(du -h "$OUTPUT" | cut -f1) ($(stat -c%s "$OUTPUT") bytes)"
log "Reminder: switch style/style.json to an OpenMapTiles style before serving this build."
