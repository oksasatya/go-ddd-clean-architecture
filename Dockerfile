# syntax=docker/dockerfile:1

# ---------- Build stage ----------
FROM golang:1.24-bookworm AS builder
WORKDIR /src

ENV CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64

# Copy go files and download deps
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

# Copy rest of the source
COPY . .

# Build the binary
RUN --mount=type=cache,target=/root/.cache/go-build \
    go build -ldflags="-s -w" -o /out/server ./cmd/main.go

# ---------- Runtime stage ----------
FROM alpine:3.20

# Install ca-certificates (for GCS & HTTPS calls) and tzdata
RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

# Copy binary and runtime assets
COPY --from=builder /out/server ./server
COPY db/migrations ./db/migrations

# Drop privileges
RUN adduser -S appuser
USER appuser

# If not set by Railway → fallback to 8080
ENV PORT=8080
ENV GIN_MODE=release

EXPOSE 8080

CMD ["./server"]
