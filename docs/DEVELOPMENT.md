# Development workflow (Docker)

Live dev stack. Source is bind-mounted into the containers, so edits hot-reload:

- **Client** (Next.js): full HMR on `http://localhost:3000` via `next dev`.
- **Server** (Go + Air): rebuilds and restarts automatically, ~2s.
- **API** on `http://localhost:8081`, Postgres on `:5432`, Redis on `:6379`.

## Daily loop

```sh
make docker-up        # start everything (already-built images, no rebuild)
make docker-logs      # tail all logs
make docker-up-build  # only when Dockerfile / package.json / go.mod changed
make docker-down      # stop; KEEPS your data
```

Database migrations run automatically on every server start and are
idempotent (tracked in `schema_migrations`). If a required table is missing
the server refuses to boot and tells you why (see `verifySchema`).

## Where your data lives (and what survives)

| Data            | Location                                        | Survives `docker build` / `down` / `up`? |
|-----------------|-------------------------------------------------|------------------------------------------|
| Postgres DB     | named volume `sweetspot_postgres_data`           | Yes                                      |
| Uploaded photos | named volume `sweetspot_server_uploads`          | Yes                                      |

`docker compose build`, `docker compose up -d`, and `docker compose down`
**never delete data**. Building images doesn't touch volumes at all.

## Production media with local-disk storage

The current storage implementation writes every processed photo to `./uploads` and
serves that directory from the API's public `/uploads` route. A deployed instance
must therefore provide all of the following:

- exactly one backend instance (local disk is not shared between replicas);
- a persistent volume mounted at `/app/uploads` (do not rely on container-layer
  storage or an anonymous Docker volume);
- a public HTTPS API origin that proxies `/uploads` with long-lived cache headers;
- `APP_ENV=production`, `STORAGE_BASE_URL=https://api.example.com`, and
  `SITE_URL=https://goodspot.example` in the deployment environment.

The production server rejects missing, non-HTTPS, or localhost media origins. The
production Next.js image also requires the `SITE_URL` build argument. Local Docker
development keeps explicit localhost defaults.

This is sufficient for a single-host deployment, but it is not a CDN or durable
multi-region storage strategy. Cloudflare R2 variables are not wired yet; setting
`STORAGE_BACKEND=r2` fails rather than silently writing to local disk.

## Commands that DESTROY data — never run these casually

| Command                                                   | Effect                                    |
|-----------------------------------------------------------|-------------------------------------------|
| `docker compose down -v`                                  | Deletes BOTH volumes (DB + uploads)       |
| `docker volume prune` / `docker volume rm <name>`          | Deletes volumes, inc. `sweetspot_*`       |
| `docker system prune -a --volumes`                         | Deletes all stopped containers + all volumes |
| Re-placing pg data in anonymous (`./`-declared) volumes    | Orphaned + "wiped" on next container replace |

The intended way to wipe everything is the guarded target:

```sh
make docker-reset        # prints a warning and refuses…
CONFIRM_WIPE=1 make docker-reset   # …then actually wipes
```

(For the record: prior data-loss on this project came from anonymous volumes
being orphaned on rebuild — old Redis/cache volumes were found abandoned
under anonymous hashes. The named volumes above prevent that.)

## Backups

```sh
make backup
# -> backups/goodspotdb-<timestamp>.dump   (Postgres, custom format)
# -> backups/uploads-<timestamp>.tar       (uploaded photos)

make restore DUMP=backups/goodspotdb-<date>.dump \
             UPLOADS=backups/uploads-<date>.tar
```

## RAM-constrained host notes

This box has ~7 GiB RAM. The Go toolchain is capped so rebuilds don't get
OOM-killed (`GOGC=50`, `GOMEMLIMIT=2GiB`, `go build -p 1` in `.air.toml`).
If a build still fails with `signal: killed`, close other apps or add swap;
the server healthcheck (`/health`) will report it as unhealthy until the
service listens on `:8081`.