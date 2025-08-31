package main

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/joho/godotenv"

	"github.com/oksasatya/go-ddd-clean-architecture/config"
	"github.com/oksasatya/go-ddd-clean-architecture/pkg/helpers"
)

func main() {
	_ = godotenv.Load()
	cfg := config.Load()

	dsn := cfg.PostgresDSN()
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		log.Fatalf("failed to open db: %v", err)
	}
	defer func() { _ = db.Close() }()

	email := "oksasatyaa@gmail.com"
	password := "password123"
	user := "demoUser"
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
	`, email, hash, user, "").Scan(&id)
	if err != nil {
		log.Fatalf("failed to seed user: %v", err)
	}
	fmt.Printf("seeded user: id=%s email=%s name=%s password=%s\n", id, email, user, password)

	// Ensure base roles exist
	var adminRoleID, userRoleID string
	if err := db.QueryRow(`
		INSERT INTO roles (name) VALUES ('admin')
		ON CONFLICT (name) DO UPDATE SET updated_at = now()
		RETURNING id
	`).Scan(&adminRoleID); err != nil {
		log.Fatalf("failed to upsert admin role: %v", err)
	}
	if err := db.QueryRow(`
		INSERT INTO roles (name) VALUES ('user')
		ON CONFLICT (name) DO UPDATE SET updated_at = now()
		RETURNING id
	`).Scan(&userRoleID); err != nil {
		log.Fatalf("failed to upsert user role: %v", err)
	}
	fmt.Printf("roles ensured: admin=%s user=%s\n", adminRoleID, userRoleID)

	// Assign admin role to seeded user
	if _, err := db.Exec(`
		INSERT INTO user_roles (user_id, role_id)
		VALUES ($1, $2)
		ON CONFLICT (user_id, role_id) DO NOTHING
	`, id, adminRoleID); err != nil {
		log.Fatalf("failed to assign admin role: %v", err)
	}
	fmt.Println("assigned admin role to seeded user (if not already)")
}
