-- name: CreateApiKey :one
INSERT INTO api_keys (key_hash, key_name, channel_id)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetApiKeyByHash :one
SELECT * FROM api_keys WHERE key_hash = $1;

-- name: GetApiKeyById :one
SELECT * FROM api_keys WHERE id = $1;

-- name: ListApiKeysByChannelID :many
SELECT * FROM api_keys
WHERE channel_id = $1
ORDER BY created_at DESC;

-- name: ListApiKeysByChannelName :many
SELECT api_keys.* FROM api_keys
JOIN channels ON api_keys.channel_id = channels.id
WHERE channels.channel_name = $1
ORDER BY api_keys.created_at DESC;

-- name: DeleteApiKey :exec
DELETE FROM api_keys WHERE id = $1;

-- name: DeleteApiKeyByHash :exec
DELETE FROM api_keys WHERE key_hash = $1;
