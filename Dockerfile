# syntax=docker/dockerfile:1

# ---------- Build stage ----------
FROM golang:1.24-bookworm AS builder
WORKDIR /src

# Enable static build for minimal final image
ENV CGO_ENABLED=0 GOOS=linux GOARCH=amd64

# Pre-cache deps
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Copy the rest of the source
COPY . .

# Build the binary
RUN --mount=type=cache,target=/root/.cache/go-build \
    go build -ldflags="-s -w" -o /out/server ./cmd

# ---------- Runtime stage ----------
FROM alpine:3.20

# Install certificates and timezone data
RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

# Copy binary and runtime assets (migrations)
COPY --from=builder /out/server /app/server
COPY db/migrations /app/db/migrations

# Non-root user
RUN addgroup -S app && adduser -S -G app appuser
USER appuser

ENV GIN_MODE=release
# Railway sets PORT; ensure default for local run
ENV PORT=8080

EXPOSE 8080

# Start the service
CMD ["/app/server"]

