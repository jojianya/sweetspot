.PHONY: build run test vet fmt lint tidy docker-up docker-up-build docker-down docker-logs \
        docker-stats docker-reset backup restore

build:
	mkdir -p bin
	go build -o bin/server ./cmd/api

run:
	go run ./cmd/api

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -w .

lint:
	golangci-lint run ./...

tidy:
	go mod tidy

# --- Docker workflow -------------------------------------------------------
#
# Source is bind-mounted and hot-reloads (Next HMR + Go Air). None of the
# up/down targets below touch data. Data lives in the named volumes
# `sweetspot_postgres_data` and `sweetspot_server_uploads`.

docker-up:
	docker compose up -d

# Only needed when a Dockerfile, package.json, or go.mod/go.sum changed.
docker-up-build:
	docker compose up -d --build

# Stops containers but KEEPS all data.
docker-down:
	docker compose down

docker-logs:
	docker compose logs -f

docker-stats:
	docker stats

# REAL wipe. Refuses to run unless you explicitly opt in, because this
# permanently deletes the database and every uploaded photo.
docker-reset:
	@test "$$CONFIRM_WIPE" = "1" || (echo "Refusing to run. This deletes ALL data (volumes sweetspot_postgres_data, sweetspot_server_uploads). Re-run explicitly with: CONFIRM_WIPE=1 make docker-reset"; exit 1)
	docker compose down -v

# --- Backup / restore ------------------------------------------------------
# Saves the Postgres database and uploads into ./backups.

backup:
	@mkdir -p backups
	docker compose exec -T postgres pg_dump -U postgres -d goodspotdb -Fc -f /tmp/goodspotdb.dump
	docker compose cp postgres:/tmp/goodspotdb.dump backups/goodspotdb-$$(date +%Y%m%d-%H%M%S).dump
	docker compose exec -T server sh -c 'tar cf - -C /app/uploads .' > backups/uploads-$$(date +%Y%m%d-%H%M%S).tar

restore:
	@test -n "$(DUMP)" || (echo "usage: make restore DUMP=backups/goodspotdb-<date>.dump UPLOADS=backups/uploads-<date>.tar"; exit 1)
	docker compose exec -T postgres sh -c 'cat > /tmp/goodspotdb.dump' < "$(DUMP)"
	docker compose exec -T postgres pg_restore -U postgres -d goodspotdb --clean --if-exists /tmp/goodspotdb.dump
	@if test -n "$(UPLOADS)" && test -f "$(UPLOADS)"; then \
		cat "$(UPLOADS)" | docker compose exec -T server sh -c 'rm -rf /app/uploads/*; tar xf - -C /app/uploads'; \
	fi