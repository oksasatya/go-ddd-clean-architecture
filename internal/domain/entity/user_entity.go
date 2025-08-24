package entity

import (
	"time"
)

// User is the aggregate root for user domain
// Passwords are stored as bcrypt hashes in Password field
//
// In a real-world app, prefer value objects for Email, etc.
type User struct {
	ID         string
	Email      string
	Password   string
	Name       string
	AvatarURL  string
	IsVerified bool
	CreatedAt  time.Time
	UpdatedAt  time.Time
}
