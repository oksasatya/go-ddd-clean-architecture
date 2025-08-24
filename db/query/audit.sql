-- name: InsertAuditLog :exec
INSERT INTO audit_logs (user_id, email, action, ip, user_agent, metadata)
VALUES ($1, $2, $3, $4, $5, $6);

