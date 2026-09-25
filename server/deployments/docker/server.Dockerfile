FROM golang:1.27-alpine AS builder
WORKDIR /app
RUN apk add --no-cache vips-dev build-base pkgconf
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN GOMAXPROCS=1 GOFLAGS=-p=1 go build -o server ./cmd/api

FROM alpine:3.20
WORKDIR /app
ENV APP_ENV=production
RUN apk add --no-cache vips
COPY --from=builder /app/server .
COPY --from=builder /app/internal/platform/database/migrations ./internal/platform/database/migrations
EXPOSE 8081
CMD ["./server"]