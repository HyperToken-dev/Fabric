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

-- name: ListActiveChannelNames :many
SELECT channel_name FROM channels WHERE status = 1 ORDER BY channel_name;

-- name: GetActiveChannelByName :one
SELECT * FROM channels WHERE channel_name = $1 AND status = 1;

-- name: UpdateChannelName :one
UPDATE channels
SET channel_name = $2
WHERE id = $1
RETURNING *;

-- name: UpdateChannelStatus :one
UPDATE channels
SET status = $2
WHERE id = $1
RETURNING *;

-- name: UpdateChannelBaseURL :one
UPDATE channels
SET base_url = $2
WHERE id = $1
RETURNING *;

-- name: UpdateChannelAPIFormat :one
UPDATE channels
SET api_format = $2
WHERE id = $1
RETURNING *;

-- name: UpdateChannelProviderKey :one
UPDATE channels
SET provider_key = $2
WHERE id = $1
RETURNING *;

-- name: DeleteChannel :exec
DELETE FROM channels WHERE id = $1;
