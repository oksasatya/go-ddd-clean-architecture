-- name: CreateRole :one
INSERT INTO roles (name)
VALUES ($1)
ON CONFLICT (name) DO UPDATE SET name = EXCLUDED.name, updated_at = now()
RETURNING id, name, created_at, updated_at;

-- name: GetRoleByName :one
SELECT id, name, created_at, updated_at
FROM roles
WHERE name = $1;

-- name: ListRoles :many
SELECT id, name, created_at, updated_at
FROM roles
ORDER BY name ASC;

-- name: AssignRoleToUser :execrows
INSERT INTO user_roles (user_id, role_id)
VALUES ($1, $2)
ON CONFLICT (user_id, role_id) DO NOTHING;

-- name: RevokeRoleFromUser :execrows
DELETE FROM user_roles
WHERE user_id = $1 AND role_id = $2;

-- name: GetUserRoles :many
SELECT r.id, r.name, r.created_at, r.updated_at
FROM roles r
JOIN user_roles ur ON ur.role_id = r.id
WHERE ur.user_id = $1
ORDER BY r.name ASC;

