-- name: CreateApiKey :one
INSERT INTO api_keys (key_hash, key_name)
VALUES ($1, $2)
RETURNING *;

-- name: GetApiKeyByHash :one
SELECT * FROM api_keys WHERE key_hash = $1;

-- name: ListApiKeys :many
SELECT * FROM api_keys ORDER BY created_at DESC;

-- name: DeleteApiKey :exec
DELETE FROM api_keys WHERE key_hash = $1;
