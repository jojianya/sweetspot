.PHONY: build run test vet fmt lint tidy migrate-docker-up docker-down

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

docker-up:
	docker compose up -d --build

docker-down:
	docker compose down