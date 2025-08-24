-- name: CreateUser :one
INSERT INTO users (email, password, name, avatar_url)
VALUES ($1, $2, $3, $4)
RETURNING id, email, password, name, avatar_url, is_verified, created_at, updated_at;

-- name: GetUserByID :one
SELECT id, email, password, name, avatar_url, is_verified, created_at, updated_at
FROM users
WHERE id = $1;

-- name: GetUserByEmail :one
SELECT id, email, password, name, avatar_url, is_verified, created_at, updated_at
FROM users
WHERE email = $1;

-- name: UpdateUser :execrows
UPDATE users
SET email = $2,
    password = $3,
    name = $4,
    avatar_url = $5,
    updated_at = now()
WHERE id = $1;

-- name: UpdateUserPassword :execrows
UPDATE users
SET password = $2,
    updated_at = now()
WHERE id = $1;

-- name: SetUserVerified :execrows
UPDATE users
SET is_verified = true,
    updated_at = now()
WHERE id = $1;

-- name: GetUserIsVerified :one
SELECT is_verified
FROM users
WHERE id = $1;
