FROM golang:1.27.2-alpine@sha256:85dc1069ac644ea3c527b177303a406eb3358192816cd7f9e5848eb658851673 AS builder
WORKDIR /app
RUN apk add --no-cache vips-dev build-base pkgconf
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN GOMAXPROCS=1 GOFLAGS=-p=1 go build -o server ./cmd/api

FROM alpine:3.20@sha256:d9e853e87e55526f6b2917df91a2115c36dd7c696a35be12163d44e6e2a4b6bc
WORKDIR /app
ENV APP_ENV=production
# Runtime libvips only (no compiler toolchain), plus an unprivileged user.
# The uploads directory is owned by appuser so the named volume inherits
# writable ownership on first mount.
RUN apk add --no-cache vips \
  && addgroup -S appuser \
  && adduser -S -G appuser -h /app appuser \
  && mkdir -p /app/uploads /app/quarantine \
  && chown -R appuser:appuser /app
COPY --from=builder --chown=appuser:appuser /app/server .
COPY --from=builder --chown=appuser:appuser /app/internal/platform/database/migrations ./internal/platform/database/migrations
USER appuser
EXPOSE 8081
HEALTHCHECK --interval=30s --timeout=3s --retries=3 \
  CMD wget -q -O /dev/null http://127.0.0.1:8081/health || exit 1
CMD ["./server"]
