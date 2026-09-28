# maps/ — self-hosted map tiles for GoodSpot247 (Philippines)

A **self-contained** vector-tile module. It is deliberately independent of the
app's `client/` and `server/`: its own data, its own `docker-compose` file, its
own release cadence. Nothing here is imported by the app at build time — the
client only needs one browser-facing URL.

Scope: the **Philippines** only (~112.17, 4.38 → 127.07, 21.53), zoom 0–14
(client side overzooms beyond 14).

## What's here

```
maps/
├── data/
│   └── philippines-latest.osm.pbf   # raw OSM extract (579 MB) — gitignored
├── planetiler.jar                   # fetched on demand by build-tiles.sh — gitignored
├── build-tiles.sh                   # .pbf → philippines.mbtiles (Planetiler / OpenMapTiles)
├── build-tiles-native.sh            # native build alternative
├── philippines.mbtiles              # generated tile archive — gitignored
├── style/
│   └── goodspot/
│       ├── style.json               # light MapLibre style (committed)
│       └── style-dark.json          # dark MapLibre style (committed)
├── legacy/                          # unused legacy styles (not served)
│   ├── style.json                   # old AWS Location Service style
│   ├── style-dark.json              # old custom dark style
│   ├── osm-bright-style.json        # old OSM Bright style
│   └── positron-style.json          # old Positron style
├── sprites/
│   ├── sprite.json                  # base sprite metadata (from OSM Bright)
│   ├── sprite.png                   # base sprite sheet
│   ├── sprite@2x.json               # @2x sprite metadata
│   ├── sprite@2x.png                # @2x sprite sheet
│   ├── goodspot/                    # style-specific copies (tileserver-gl expects these)
│   │   ├── sprite.json
│   │   ├── sprite.png
│   │   ├── sprite@2x.json
│   │   └── sprite@2x.png
│   └── goodspot-dark/
│       ├── sprite.json
│       ├── sprite.png
│       ├── sprite@2x.json
│       └── sprite@2x.png
├── tileserver-config.json           # TileServer GL config (committed)
├── docker-compose.tiles.yml         # separate compose file (NOT merged into the app's)
└── README.md
```

## TL;DR

```bash
docker compose -f docker-compose.tiles.yml up -d
# style:  http://localhost:8080/styles/goodspot/style.json
# tiles:  http://localhost:8080/data/philippines/{z}/{x}/{y}.pbf
```

Then point the client at it with `NEXT_PUBLIC_TILES_URL=http://localhost:8080`.

## Tile source: Geofabrik's pre-built Shortbread archive

This module ships with **Geofabrik's pre-generated tiles** rather than a local
Planetiler build:

- URL: <https://download.geofabrik.de/asia/philippines-shortbread-1.0.mbtiles>
- Local file: `philippines.mbtiles` — **491 MB (515,297,280 bytes)**
- 730,059 tiles, z0–14, schema **Shortbread 1.0** (not OpenMapTiles)
- © OpenStreetMap contributors, ODbL 1.0

**Why this instead of Planetiler:** it is a ready-to-serve `.mbtiles`, so TileServer
GL can serve it directly. Building OpenMapTiles tiles locally for a 579 MB
extract needs several GB of RAM and 30–90+ minutes; this box doesn't have the
headroom. The tradeoff is schema: Geofabrik's package is **Shortbread**, so the
styles here are written for Shortbread layer names (`water_polygons`, `streets`,
`place_labels`, `boundaries`, …), not OpenMapTiles/OSM Bright.

`build-tiles.sh` is still provided for when you *do* want a self-generated,
OpenMapTiles-compatible tileset. Note that running it **replaces**
`philippines.mbtiles` with an OpenMapTiles build — you must then swap
`style/style.json` for an OpenMapTiles style (e.g. OSM Bright).

> ⚠️ `philippines.mbtiles` is gitignored. It is **not** in the repo; provision it
> with the download command below or `build-tiles.sh`.

### Provision the pre-built archive (Shortbread)

```bash
curl -L -o philippines.mbtiles \
  https://download.geofabrik.de/asia/philippines-shortbread-1.0.mbtiles
```

### Or build it yourself (Planetiler / OpenMapTiles)

```bash
# raw extract first (gitignored)
curl -L -o data/philippines-latest.osm.pbf \
  https://download.geofabrik.de/asia/philippines-latest.osm.pbf

./build-tiles.sh          # Docker, pinned Planetiler, verified jar, idempotent
# JAVA_MEM=4g PLANETILER_VERSION=v0.10.2 ./build-tiles.sh
```

**Docker memory requirement:** The Planetiler build needs **at least 8 GB of
memory** allocated to Docker Desktop / the Docker daemon (`JAVA_MEM=8g`
recommended for the 579 MB Philippines extract). If the build fails with OOM,
increase the Docker memory limit and retry.

`build-tiles.sh` is idempotent: it removes the previous output, verifies the
pinned Planetiler jar against its published sha256, caches auxiliary sources
under `.planetiler-cache/`, and writes `philippines.mbtiles` fresh. Running it
twice produces a clean regeneration, not an append.

> ⚠️ **Backup:** `philippines.mbtiles` is gitignored and not in the repo.
> Keep a backup of the generated `.mbtiles` file (e.g. in an S3 bucket, NAS,
> or `~/backups/maps/`) so you can restore it without re-running the 30–90
> minute build. The pre-built Geofabrik Shortbread archive can always be
> re-downloaded from `https://download.geofabrik.de/asia/philippines-shortbread-1.0.mbtiles`
> if needed.

## Running it

### Standalone (this directory)

```bash
docker compose -f docker-compose.tiles.yml up -d
curl -s http://localhost:8080/styles/goodspot/style.json | head
```

The port is published on **loopback only** (`127.0.0.1:8080`) — never expose a
single-region tile server directly.

### Quick visual test (no app required)

`test.html` renders the basemap with MapLibre GL JS loaded from a CDN, opens
centered on Manila, and shows a badge stating which tile host it is hitting
(green = `localhost:8080` and rendering, amber = retargeted host, red =
failure).

**It must be served over HTTP.** Browsers block cross-origin requests (the
style JSON, tiles, fonts) from `file://` pages by design, so double-clicking
the file will always fail — this is not a bug in the file. Serve it from this
directory:

```bash
python3 -m http.server 8899
# then open http://localhost:8899/test.html
```

The port (8899) is chosen to avoid every port the stacks use — 3000 client,
5432 postgres, 6379 redis, 8080 tiles, 8081 app API — and in particular the
app stack's Go API on 8081 (see `../sweetspot/docker-compose.yml`). Useful query overrides:
`?tiles=http://192.168.1.10:8080` (another host) and
`?style=goodspot-dark`.

### Alongside the main app stack

If `maps/` lives inside the app repo, merge the override (paths in the override
resolve against the base file's directory, hence `TILES_DIR`):

```bash
TILES_DIR=./maps docker compose \
  -f docker-compose.yml \
  -f maps/docker-compose.tiles.yml \
  up -d
```

This runs the tile server next to postgres/redis/server/client. It is a separate
file on purpose: the app stack should still come up if the maps module is absent.

## Client wiring

The map component (`client/src/components/map/MapView.tsx`) loads its styles
from `NEXT_PUBLIC_TILES_URL`:

```
${NEXT_PUBLIC_TILES_URL}/styles/goodspot/style.json        # light
${NEXT_PUBLIC_TILES_URL}/styles/goodspot-dark/style.json   # dark
```

The styles reference tiles, glyphs, and sprites with **root-relative** URLs
(`/data/philippines/...`, `/fonts/...`, `sprite`) so the same files work whether
served from `localhost:8080`, a Cloudflare hostname, or a tunnel. MapLibre
resolves root-relative URLs against the *page* origin (the app), not the style
origin, so `MapView` passes a `transformRequest` that rewrites them onto
`NEXT_PUBLIC_TILES_URL`. If you open these styles in an external editor
(e.g. Maputnik), serve the editor from the tile origin or tiles will 404.

Sprites are served by TileServer GL from the `maps/sprites/` directory
(`sprite.json`, `sprite.png`, `sprite@2x.json`, `sprite@2x.png`), sourced from
the OpenMapTiles-compatible OSM Bright style. Glyphs come from the
`tileserver-gl-styles` package bundled with the tile server image.

Set it in `.env` (and `.env.local` for non-Docker dev):

```env
NEXT_PUBLIC_TILES_URL=http://localhost:8080
```

In production, use the public HTTPS origin fronted by Cloudflare. Because
`NEXT_PUBLIC_*` is inlined into the bundle, restart/rebuild the client after
changing it.

Address **geocoding** gracefully degrades in self-hosted mode — if
`NEXT_PUBLIC_MAPTILER_API_KEY` is not set, place search returns no results
and the UI shows "place search unavailable" while pin search continues to
work. No **tile** request leaves the deployment.

## Cloudflare (edge caching) — required config step

A single-region tile server has no global presence, so put Cloudflare in front
for edge caching. The origin serves hundreds of thousands of immutable,
hashed-by-coordinate tiles, so caching is highly effective.

1. **Expose the origin.** Either a Cloudflare Tunnel (`cloudflared`) or a
   reverse proxy on a public host:

   ```bash
   cloudflared tunnel --url http://localhost:8080     # quick test only
   ```

   For production, create a named tunnel to this service and a proxied DNS
   record (`tiles.example.com`), keeping the origin firewalled.

1b. **Tell the tile server its public URL.** TileServer GL rewrites some URLs
    (e.g. `glyphs`) absolutely from the request `Host` header. Behind
    Cloudflare, start the service with `--public_url https://tiles.example.com`
    (or `allowedHosts` / `TILESERVER_GL_ALLOWED_HOSTS`) so rewritten URLs use the
    public host — and to close the Host-header spoofing warning it logs.

2. **Cache the tile paths at the edge.** Add a Cloudflare **Cache Rule** for:
   - `/data/*` (vector tiles) — `Cache Eligible`, long edge TTL (e.g. 30 days)
   - `/fonts/*`, `/sprites/*`, `/styles/*` — long edge TTL
   Do *not* cache `/health`.

3. **Turn on Tiered Cache** (Smart Tiered Cache) so edge misses are served from
   a nearby upper tier instead of always hitting the single origin.

4. **Version + purge on data updates.** Tiles are keyed by `z/x/y`, so a new
   extract shadows the old cache. After regenerating, either bump a path prefix
   (e.g. `/data/philippines/v2026-09/...`) or purge the `/data/*` cache tag.

5. Point `NEXT_PUBLIC_TILES_URL` at the public hostname once the edge is in place.

TileServer GL sends `ETag` (and `Access-Control-Allow-Origin: *`) but no
`Cache-Control`, so the Cache Rules above are what give tiles an edge TTL.

## Regenerating tiles when OSM updates

This is **manual** — there is no scheduled refresh (see tradeoffs).

```bash
# 1. refresh the raw extract (if you build locally)
curl -L -o data/philippines-latest.osm.pbf \
  https://download.geofabrik.de/asia/philippines-latest.osm.pbf

# 2a. cheapest: re-fetch Geofabrik's pre-built Shortbread archive
curl -L -o philippines.mbtiles \
  https://download.geofabrik.de/asia/philippines-shortbread-1.0.mbtiles

# 2b. or rebuild from raw with Planetiler (OpenMapTiles — remember to swap styles)
./build-tiles.sh
```

Then restart the tile service and purge the CDN:

```bash
docker compose -f docker-compose.tiles.yml up -d --force-recreate tiles
```

A monthly cadence is usually plenty for basemap purposes.

## Verifying

```bash
# style + config load
curl -s -o /dev/null -w '%{http_code}\n' http://localhost:8080/styles/goodspot/style.json

# a tile over Manila / Cebu / Davao (z12)
curl -s -o /dev/null -w 'manila %{http_code} %{size_download}\n' \
  http://localhost:8080/data/philippines/12/3424/1880.pbf
```

For a rendered view without the app, open `test.html` in a browser (see
"Quick visual test" above — served over HTTP, not `file://`).

In the browser DevTools → Network tab, filter for `/data/`, `/styles/`, `/fonts/`
and confirm **all** tile traffic goes to the tile host — zero requests to
`api.maptiler.com` tiles or any other paid tile provider.

## Tradeoffs

- **No global CDN by default.** Self-hosted tiles serve from one region until a
  CDN (Cloudflare) is configured. Clients far from the origin see higher
  latency. This is the main reason step "Cloudflare" exists.
- **Data freshness is manual.** Geofabrik's Shortbread packages are described as
  "experimental, non-updated"; freshness is whatever is on the download server
  when you fetch it. Nothing auto-regenerates — re-run the commands above
  periodically.
- **Schema lock-in.** The bundled styles target Shortbread. Switching to a
  Planetiler/OpenMapTiles build means writing/replacing the style.
- **Single-region origin = SPOF.** Add health checks, restart policy
  (`restart: unless-stopped` is set) and CDN caching for resilience.

## Attribution

Data © OpenStreetMap contributors, available under the
[Open Database License (ODbL) 1.0](https://opendatacommons.org/licenses/odbl/).
The attribution is embedded in both styles and must remain visible in the app.
