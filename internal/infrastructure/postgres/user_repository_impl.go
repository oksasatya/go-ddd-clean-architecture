package postgres

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

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

// map helpers for sqlc rows
func mapCreateRow(u pgstore.CreateUserRow) *entity.User {
	var idStr string
	if u.ID.Valid {
		idStr = uuid.UUID(u.ID.Bytes).String()
	}
	var createdAt time.Time
	if u.CreatedAt.Valid {
		createdAt = u.CreatedAt.Time
	}
	var updatedAt time.Time
	if u.UpdatedAt.Valid {
		updatedAt = u.UpdatedAt.Time
	}
	return &entity.User{
		ID:         idStr,
		Email:      u.Email,
		Password:   u.Password,
		Name:       u.Name,
		AvatarURL:  u.AvatarUrl,
		IsVerified: u.IsVerified,
		CreatedAt:  createdAt,
		UpdatedAt:  updatedAt,
	}
}

func mapGetByIDRow(u pgstore.GetUserByIDRow) *entity.User {
	var idStr string
	if u.ID.Valid {
		idStr = uuid.UUID(u.ID.Bytes).String()
	}
	var createdAt time.Time
	if u.CreatedAt.Valid {
		createdAt = u.CreatedAt.Time
	}
	var updatedAt time.Time
	if u.UpdatedAt.Valid {
		updatedAt = u.UpdatedAt.Time
	}
	return &entity.User{
		ID:         idStr,
		Email:      u.Email,
		Password:   u.Password,
		Name:       u.Name,
		AvatarURL:  u.AvatarUrl,
		IsVerified: u.IsVerified,
		CreatedAt:  createdAt,
		UpdatedAt:  updatedAt,
	}
}

func mapGetByEmailRow(u pgstore.GetUserByEmailRow) *entity.User {
	var idStr string
	if u.ID.Valid {
		idStr = uuid.UUID(u.ID.Bytes).String()
	}
	var createdAt time.Time
	if u.CreatedAt.Valid {
		createdAt = u.CreatedAt.Time
	}
	var updatedAt time.Time
	if u.UpdatedAt.Valid {
		updatedAt = u.UpdatedAt.Time
	}
	return &entity.User{
		ID:         idStr,
		Email:      u.Email,
		Password:   u.Password,
		Name:       u.Name,
		AvatarURL:  u.AvatarUrl,
		IsVerified: u.IsVerified,
		CreatedAt:  createdAt,
		UpdatedAt:  updatedAt,
	}
}

func (r *UserRepository) Create(u *entity.User) error {
	ctx := context.Background()
	created, err := r.queries.CreateUser(ctx, pgstore.CreateUserParams{
		Email:     u.Email,
		Password:  u.Password,
		Name:      u.Name,
		AvatarUrl: u.AvatarURL,
	})
	if err != nil {
		return err
	}
	mapped := mapCreateRow(created)
	u.ID = mapped.ID
	u.CreatedAt = mapped.CreatedAt
	u.UpdatedAt = mapped.UpdatedAt
	return nil
}

func (r *UserRepository) GetByID(id string) (*entity.User, error) {
	ctx := context.Background()
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
	return mapGetByIDRow(row), nil
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
	return mapGetByEmailRow(row), nil
}

func (r *UserRepository) Update(u *entity.User) error {
	ctx := context.Background()
	parsed, err := uuid.Parse(u.ID)
	if err != nil {
		return err
	}
	var pgID pgtype.UUID
	pgID.Bytes = parsed
	pgID.Valid = true
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
	u.UpdatedAt = time.Now()
	return nil
}

func (r *UserRepository) UpdatePassword(userID string, passwordHash string) error {
	ctx := context.Background()
	parsed, err := uuid.Parse(userID)
	if err != nil {
		return err
	}
	var pgID pgtype.UUID
	pgID.Bytes = parsed
	pgID.Valid = true
	rows, err := r.queries.UpdateUserPassword(ctx, pgstore.UpdateUserPasswordParams{
		ID:       pgID,
		Password: passwordHash,
	})
	if err != nil {
		return err
	}
	if rows == 0 {
		return errNotFound
	}
	return nil
}

func (r *UserRepository) IsVerified(userID string) (bool, error) {
	ctx := context.Background()
	parsed, err := uuid.Parse(userID)
	if err != nil {
		return false, err
	}
	var id pgtype.UUID
	id.Bytes = parsed
	id.Valid = true
	v, err := r.queries.GetUserIsVerified(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, errNotFound
		}
		return false, err
	}
	return v, nil
}

func (r *UserRepository) SetVerified(userID string) error {
	ctx := context.Background()
	parsed, err := uuid.Parse(userID)
	if err != nil {
		return err
	}
	var id pgtype.UUID
	id.Bytes = parsed
	id.Valid = true
	rows, err := r.queries.SetUserVerified(ctx, id)
	if err != nil {
		return err
	}
	if rows == 0 {
		return errNotFound
	}
	return nil
}

var _ repository.UserRepository = (*UserRepository)(nil)
