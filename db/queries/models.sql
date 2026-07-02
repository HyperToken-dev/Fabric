-- name: UpsertModel :one
INSERT INTO models (provider, model_name)
VALUES ($1, $2)
ON CONFLICT (provider, model_name) DO NOTHING
RETURNING *;

-- name: GetModelByProviderAndName :one
SELECT * FROM models WHERE provider = $1 AND model_name = $2;

-- name: ListModels :many
SELECT * FROM models ORDER BY provider, model_name;
