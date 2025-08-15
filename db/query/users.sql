-- name: CreateUser :one
INSERT INTO users (email, password, name, avatar_url)
VALUES ($1, $2, $3, $4)
RETURNING id, email, password, name, avatar_url, created_at, updated_at;

-- name: GetUserByID :one
SELECT id, email, password, name, avatar_url, created_at, updated_at
FROM users
WHERE id = $1;

-- name: GetUserByEmail :one
SELECT id, email, password, name, avatar_url, created_at, updated_at
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
