-- name: CreateIntegralLog :one
INSERT INTO integral_logs (context, response, key_id, owner_openid)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetIntegralLogByID :one
SELECT * FROM integral_logs WHERE id = $1;

-- name: GetIntegralLogByIDAndOwnerOpenID :one
SELECT * FROM integral_logs
WHERE id = $1 AND owner_openid = $2;

-- name: ListIntegralLogs :many
SELECT * FROM integral_logs
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: ListIntegralLogsByKeyID :many
SELECT * FROM integral_logs
WHERE key_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: ListIntegralLogsByOwnerOpenID :many
SELECT * FROM integral_logs
WHERE owner_openid = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: ListIntegralLogsByKeyIDAndOwnerOpenID :many
SELECT * FROM integral_logs
WHERE key_id = $1 AND owner_openid = $2
ORDER BY created_at DESC
LIMIT $3 OFFSET $4;

-- name: CountIntegralLogs :one
SELECT COUNT(*) FROM integral_logs;

-- name: CountIntegralLogsByKeyID :one
SELECT COUNT(*) FROM integral_logs WHERE key_id = $1;

-- name: CountIntegralLogsByOwnerOpenID :one
SELECT COUNT(*) FROM integral_logs WHERE owner_openid = $1;

-- name: CountIntegralLogsByKeyIDAndOwnerOpenID :one
SELECT COUNT(*) FROM integral_logs
WHERE key_id = $1 AND owner_openid = $2;

-- name: UpdateIntegralLog :one
UPDATE integral_logs
SET context = $2,
    response = $3
WHERE id = $1
RETURNING *;

-- name: DeleteIntegralLog :exec
DELETE FROM integral_logs WHERE id = $1;
