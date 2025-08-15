package repository

import "boilerplate-go-pgsql/internal/domain/entity"

// UserRepository defines the interface for user-related database operations.
type UserRepository interface {
	Create(u *entity.User) error
	GetByID(id string) (*entity.User, error)
	GetByEmail(email string) (*entity.User, error)
	Update(u *entity.User) error
}
