-- name: CreateChannel :one
INSERT INTO channels (channel_name, base_url, provider_key, api_format)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetChannelById :one
SELECT * FROM channels WHERE id = $1;

-- name: GetChannelByName :one
SELECT * FROM channels WHERE channel_name = $1;

-- name: ListChannels :many
SELECT * FROM channels ORDER BY created_at DESC;

-- name: ListActiveChannels :many
SELECT * FROM channels WHERE status = 1 ORDER BY created_at DESC;

-- name: DeleteChannel :exec
DELETE FROM channels WHERE id = $1;
