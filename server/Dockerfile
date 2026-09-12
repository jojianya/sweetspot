FROM golang:1.27-alpine AS builder
WORKDIR /app
RUN apk add --no-cache vips-dev build-base pkgconf
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o server .

FROM alpine:3.20
WORKDIR /app
RUN apk add --no-cache vips
COPY --from=builder /app/server .
COPY --from=builder /app/migrations ./migrations
EXPOSE 8080
CMD ["./server"]