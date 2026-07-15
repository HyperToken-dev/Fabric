-- name: CreateModel :one
INSERT INTO models (channel_id, model_name, status, model_type)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: UpsertModel :one
INSERT INTO models (channel_id, model_name, status, model_type)
VALUES ($1, $2, $3, $4)
ON CONFLICT (channel_id, model_name) DO UPDATE SET
    model_name = models.model_name
RETURNING *;

-- name: GetModelById :one
SELECT * FROM models WHERE id = $1;

-- name: GetModelByChannelAndName :one
SELECT * FROM models WHERE channel_id = $1 AND model_name = $2;

-- name: GetActiveModelByChannelAndName :one
SELECT * FROM models WHERE channel_id = $1 AND model_name = $2 AND status = 1;

-- name: ListModels :many
SELECT * FROM models ORDER BY channel_id, model_name;

-- name: ListModelsByChannel :many
SELECT * FROM models WHERE channel_id = $1 ORDER BY model_name;

-- name: ListActiveModelsByChannel :many
SELECT * FROM models WHERE channel_id = $1 AND status = 1 ORDER BY model_name;

-- name: DeleteModel :exec
DELETE FROM models WHERE id = $1;
