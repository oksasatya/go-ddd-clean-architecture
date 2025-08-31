package entity

import "time"

// Role represents an authorization role
// Many-to-many with User via user_roles
// kept minimal for domain use
type Role struct {
	ID        string
	Name      string
	CreatedAt time.Time
	UpdatedAt time.Time
}
