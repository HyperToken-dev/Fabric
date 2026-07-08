-- name: CreateChannel :one
INSERT INTO channels (channel_name, base_url, api_format)
VALUES ($1, $2, $3)
RETURNING *;

-- name: GetChannelById :one
SELECT * FROM channels WHERE id = $1;

-- name: GetChannelByName :one
SELECT * FROM channels WHERE channel_name = $1;

-- name: ListChannels :many
SELECT * FROM channels ORDER BY created_at DESC;

-- name: DeleteChannel :exec
DELETE FROM channels WHERE id = $1;
