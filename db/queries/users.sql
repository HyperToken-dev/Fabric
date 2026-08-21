-- name: GetUserByIssuerSubject :one
SELECT * FROM users
WHERE issuer = $1 AND subject = $2;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1;

-- name: GetSystemUser :one
SELECT * FROM users WHERE issuer = 'fabric' AND subject = 'system';

-- name: CreateUser :one
INSERT INTO users (issuer, subject, email, display_name, avatar_url, role, status, last_login_at)
VALUES ($1, $2, $3, $4, $5, $6, 'active', NOW())
RETURNING *;

-- name: UpdateUserLoginProfile :one
UPDATE users
SET email = $3,
    display_name = $4,
    avatar_url = $5,
    role = $6,
    updated_at = NOW(),
    last_login_at = NOW()
WHERE issuer = $1 AND subject = $2
RETURNING *;
