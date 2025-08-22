package postgres

import (
	"context"
	"errors"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"time"

	"github.com/oksasatya/go-ddd-clean-architecture/internal/domain/entity"
	"github.com/oksasatya/go-ddd-clean-architecture/internal/domain/repository"
	"github.com/oksasatya/go-ddd-clean-architecture/internal/infrastructure/postgres/pgstore"
)

var (
	errNotFound = errors.New("not found")
)

type UserRepository struct {
	pool    *pgxpool.Pool
	queries *pgstore.Queries
}

func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
	return &UserRepository{pool: pool, queries: pgstore.New(pool)}
}

// mapPgUser converts a pgstore.User to the domain entity.User
func mapPgUser(u pgstore.User) *entity.User {
	// Convert UUID to string
	var idStr string
	if u.ID.Valid {
		idStr = uuid.UUID(u.ID.Bytes).String()
	}

	// Convert timestamps to time.Time (zero value if invalid)
	var createdAt time.Time
	if u.CreatedAt.Valid {
		createdAt = u.CreatedAt.Time
	}
	var updatedAt time.Time
	if u.UpdatedAt.Valid {
		updatedAt = u.UpdatedAt.Time
	}

	return &entity.User{
		ID:        idStr,
		Email:     u.Email,
		Password:  u.Password,
		Name:      u.Name,
		AvatarURL: u.AvatarUrl,
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	}
}

func (r *UserRepository) Create(u *entity.User) error {
	ctx := context.Background()
	// sqlc handles updated_at via now() on update, but for insert we return both timestamps
	created, err := r.queries.CreateUser(ctx, pgstore.CreateUserParams{
		Email:     u.Email,
		Password:  u.Password,
		Name:      u.Name,
		AvatarUrl: u.AvatarURL,
	})
	if err != nil {
		return err
	}

	mapped := mapPgUser(created)
	// copy back generated fields
	u.ID = mapped.ID
	u.CreatedAt = mapped.CreatedAt
	u.UpdatedAt = mapped.UpdatedAt
	return nil
}

func (r *UserRepository) GetByID(id string) (*entity.User, error) {
	ctx := context.Background()

	// Parse string UUID to pgtype.UUID
	parsed, err := uuid.Parse(id)
	if err != nil {
		return nil, err
	}
	var pgID pgtype.UUID
	pgID.Bytes = parsed
	pgID.Valid = true

	row, err := r.queries.GetUserByID(ctx, pgID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errNotFound
		}
		return nil, err
	}

	return mapPgUser(row), nil
}

func (r *UserRepository) GetByEmail(email string) (*entity.User, error) {
	ctx := context.Background()

	row, err := r.queries.GetUserByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errNotFound
		}
		return nil, err
	}

	return mapPgUser(row), nil
}

func (r *UserRepository) Update(u *entity.User) error {
	ctx := context.Background()

	// Parse string UUID to pgtype.UUID for Update
	parsed, err := uuid.Parse(u.ID)
	if err != nil {
		return err
	}
	var pgID pgtype.UUID
	pgID.Bytes = parsed
	pgID.Valid = true

	// updated_at handled by query (now())
	rows, err := r.queries.UpdateUser(ctx, pgstore.UpdateUserParams{
		ID:        pgID,
		Email:     u.Email,
		Password:  u.Password,
		Name:      u.Name,
		AvatarUrl: u.AvatarURL,
	})
	if err != nil {
		return err
	}
	if rows == 0 {
		return errNotFound
	}

	// Fetch the updated row to return accurate UpdatedAt, or set to now locally
	// Keep behavior similar to previous code: set UpdatedAt locally
	u.UpdatedAt = time.Now()
	return nil
}

var _ repository.UserRepository = (*UserRepository)(nil)
