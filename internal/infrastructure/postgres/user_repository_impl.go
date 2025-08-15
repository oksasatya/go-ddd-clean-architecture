package postgres

import (
	"boilerplate-go-pgsql/internal/domain/repository"
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"boilerplate-go-pgsql/internal/domain/entity"
)

var (
	errNotFound = errors.New("not found")
)

type UserRepository struct {
	pool *pgxpool.Pool
}

func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
	return &UserRepository{pool: pool}
}

func (r *UserRepository) Create(u *entity.User) error {
	ctx := context.Background()
	row := r.pool.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, name, avatar_url)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at, updated_at
	`, u.Email, u.Password, u.Name, u.AvatarURL)

	return row.Scan(&u.ID, &u.CreatedAt, &u.UpdatedAt)
}

func (r *UserRepository) GetByID(id string) (*entity.User, error) {
	ctx := context.Background()
	u := &entity.User{}

	row := r.pool.QueryRow(ctx, `
		SELECT id, email, password_hash, name, avatar_url, created_at, updated_at
		FROM users
		WHERE id = $1
	`, id)

	if err := row.Scan(&u.ID, &u.Email, &u.Password, &u.Name, &u.AvatarURL,
		&u.CreatedAt, &u.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errNotFound
		}
		return nil, err
	}

	return u, nil
}

func (r *UserRepository) GetByEmail(email string) (*entity.User, error) {
	ctx := context.Background()
	u := &entity.User{}

	row := r.pool.QueryRow(ctx, `
		SELECT id, email, password_hash, name, avatar_url, created_at, updated_at
		FROM users
		WHERE email = $1
	`, email)

	if err := row.Scan(&u.ID, &u.Email, &u.Password, &u.Name, &u.AvatarURL,
		&u.CreatedAt, &u.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, errNotFound
		}
		return nil, err
	}

	return u, nil
}

func (r *UserRepository) Update(u *entity.User) error {
	ctx := context.Background()
	u.UpdatedAt = time.Now()

	res, err := r.pool.Exec(ctx, `
		UPDATE users
		SET email = $1, password_hash = $2, name = $3, avatar_url = $4, updated_at = $5
		WHERE id = $6
	`, u.Email, u.Password, u.Name, u.AvatarURL, u.UpdatedAt, u.ID)
	if err != nil {
		return err
	}

	if res.RowsAffected() == 0 {
		return errNotFound
	}

	return nil
}

var _ repository.UserRepository = (*UserRepository)(nil)
