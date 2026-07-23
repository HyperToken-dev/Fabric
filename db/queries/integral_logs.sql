-- name: CreateIntegralLog :one
INSERT INTO integral_logs (context, response, key_id)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetIntegralLogByID :one
SELECT * FROM integral_logs WHERE id = $1;

-- name: ListIntegralLogs :many
SELECT * FROM integral_logs
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;

-- name: ListIntegralLogsByKeyID :many
SELECT * FROM integral_logs
WHERE key_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountIntegralLogs :one
SELECT COUNT(*) FROM integral_logs;

-- name: CountIntegralLogsByKeyID :one
SELECT COUNT(*) FROM integral_logs WHERE key_id = $1;

-- name: UpdateIntegralLog :one
UPDATE integral_logs
SET context = $2,
    response = $3
WHERE id = $1
RETURNING *;

-- name: DeleteIntegralLog :exec
DELETE FROM integral_logs WHERE id = $1;
