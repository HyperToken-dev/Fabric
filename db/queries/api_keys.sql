-- name: CreateApiKey :one
INSERT INTO api_keys (key_hash, key_name, channel_id, user_id)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetApiKeyByHash :one
SELECT * FROM api_keys WHERE key_hash = $1;

-- name: GetApiKeyWithChannelByHash :one
SELECT
    api_keys.id AS key_id,
    api_keys.channel_id,
    channels.base_url,
    channels.provider_key,
    channels.api_format AS channel_api_format,
    channels.status AS channel_status
FROM api_keys
JOIN channels ON api_keys.channel_id = channels.id
WHERE api_keys.key_hash = $1;

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

-- name: ListApiKeysByUser :many
SELECT api_keys.* FROM api_keys
WHERE user_id = $1
ORDER BY created_at DESC;

-- name: ListApiKeysWithChannelByUser :many
SELECT
    api_keys.id,
    api_keys.key_hash,
    api_keys.key_name,
    api_keys.channel_id,
    api_keys.created_at,
    api_keys.user_id,
    channels.channel_name
FROM api_keys
JOIN channels ON api_keys.channel_id = channels.id
WHERE api_keys.user_id = $1
ORDER BY api_keys.created_at DESC;

-- name: DeleteApiKeyByHashAndUser :exec
DELETE FROM api_keys
WHERE key_hash = $1 AND user_id = $2;

-- name: DeleteApiKey :exec
DELETE FROM api_keys WHERE id = $1;

-- name: DeleteApiKeyByHash :exec
DELETE FROM api_keys WHERE key_hash = $1;
