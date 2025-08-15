package main

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/joho/godotenv"

	"github.com/oksasatya/go-ddd-clean-architecture/configs"
	"github.com/oksasatya/go-ddd-clean-architecture/pkg/helpers"
)

func main() {
	_ = godotenv.Load()
	cfg := configs.Load()

	dsn := cfg.PostgresDSN()
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		log.Fatalf("failed to open db: %v", err)
	}
	defer func() { _ = db.Close() }()

	password := "password123"
	hash, err := helpers.HashPassword(password)
	if err != nil {
		log.Fatalf("failed to hash password: %v", err)
	}

	var id string
	err = db.QueryRow(`
		INSERT INTO users (email, password, name, avatar_url)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (email) DO UPDATE SET name=EXCLUDED.name
		RETURNING id
	`, "admin@example.com", hash, "Demo User", "").Scan(&id)
	if err != nil {
		log.Fatalf("failed to seed user: %v", err)
	}
	fmt.Printf("seeded user: id=%s email=admin@example.com password=%s\n", id, password)
}
