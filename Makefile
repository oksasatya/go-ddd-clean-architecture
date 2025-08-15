# Load variables from .env if present
ifneq (,$(wildcard .env))
include .env
export
endif

# Defaults
MIGRATIONS_DIR ?= db/migrations
DB_USER ?= postgres
DB_PASSWORD ?= postgres
DB_HOST ?= localhost
DB_PORT ?= 5432
DB_NAME ?= appdb
DB_SSLMODE ?= disable
DB_DSN := postgres://$(DB_USER):$(DB_PASSWORD)@$(DB_HOST):$(DB_PORT)/$(DB_NAME)?sslmode=$(DB_SSLMODE)

.PHONY: tidy build run sqlc-generate migrate-up migrate-down migrate-drop seed

# Go module helpers
tidy:
	go mod tidy

build:
	go build ./...

run:
	go run cmd/main.go

# sqlc
sqlc-generate:
	sqlc generate

# Migrations (requires golang-migrate CLI: https://github.com/golang-migrate/migrate/tree/master/cmd/migrate)
migrate-up:
	migrate -path $(MIGRATIONS_DIR) -database "$(DB_DSN)" up

migrate-down:
	migrate -path $(MIGRATIONS_DIR) -database "$(DB_DSN)" down 1

migrate-drop:
	migrate -path $(MIGRATIONS_DIR) -database "$(DB_DSN)" drop -f

# Seed sample data (one demo user)
seed:
	go run cmd/seed/main.go

