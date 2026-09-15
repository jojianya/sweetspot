# sweetspot247-backend

Go + Gin backend for SweetSpot 247.

## Layout

```
cmd/api/                    application entrypoint
internal/app/               bootstrap + graceful shutdown
internal/config/            env-based configuration
internal/di/                dependency wiring (container)
internal/http/              gin engine, router, middleware, response helpers
internal/modules/           domain modules (auth, pins, reports, user, realtime, streams)
internal/observability/     logging (slog)
internal/platform/          infrastructure (database + migrations, cache/redis, storage)
pkg/                        reusable framework-agnostic utilities
api/                        API specifications
deployments/                docker + k8s
test/                       integration/e2e/mocks
scripts/                    helper scripts
```

## Quick start

```sh
# 1. copy env vars
cp .env.example .env

# 2. run postgres + redis
docker compose up -d postgres redis

# 3. run the API
make run
```

Requires libvips (the image processing stack uses `bimg`).

## Commands

```sh
make build   # go build ./cmd/api
make run     # go run ./cmd/api
make test    # go test ./...
make vet     # go vet ./...
make lint    # golangci-lint run ./...
```