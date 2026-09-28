#!/usr/bin/env bash
set -euo pipefail

MAPS_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
INPUT="${MAPS_DIR}/data/philippines-latest.osm.pbf"
OUTPUT="${MAPS_DIR}/philippines.mbtiles"
JAR="${MAPS_DIR}/planetiler.jar"
CACHE_DIR="${MAPS_DIR}/.planetiler-cache"

PLANETILER_VERSION="v0.10.2"
RELEASE_URL="https://github.com/onthegomap/planetiler/releases/download/${PLANETILER_VERSION}"

log() { printf '\033[1;34m[build-tiles]\033[0m %s\n' "$*"; }
die() { printf '\033[1;31m[build-tiles] error:\033[0m %s\n' "$*" >&2; exit 1; }

command -v java >/dev/null 2>&1 || die "java is required"
[ -f "$INPUT" ] || die "missing input '$INPUT'"
[ -f "$JAR" ] || die "missing planetiler.jar at '$JAR'"

rm -f "$OUTPUT"
mkdir -p "$CACHE_DIR"

log "Generating $(basename "$OUTPUT") natively with maxzoom=12..."
java -Xmx6g -jar "$JAR" \
  --osm-path="$INPUT" \
  --output="$OUTPUT" \
  --maxzoom=12 \
  --render_maxzoom=12 \
  --force \
  --download \
  --download_dir=data/sources \
  --tmpdir=data/tmp \
  --cache_dir="$CACHE_DIR" \
  --compact=true \
  --no_index=false \
  --vacuum_analyze=false

log "Done: $(du -h "$OUTPUT" | cut -f1) ($(stat -c%s "$OUTPUT") bytes)"
log "Reminder: switch style/style.json to an OpenMapTiles style before serving this build."
