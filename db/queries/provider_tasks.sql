-- name: CreateProviderTask :one
INSERT INTO provider_tasks (provider, key_id, channel_id, model_id, provider_task_id, status, request, response)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
ON CONFLICT (provider, provider_task_id) DO NOTHING
RETURNING *;

-- name: GetProviderTaskByProviderTaskID :one
SELECT * FROM provider_tasks
WHERE provider = $1 AND provider_task_id = $2;

-- name: GetProviderTaskForUpdate :one
SELECT * FROM provider_tasks
WHERE provider = $1 AND provider_task_id = $2
FOR UPDATE;

-- name: UpdateProviderTaskPendingResponse :one
UPDATE provider_tasks
SET status = $3,
    response = $4,
    updated_at = NOW()
WHERE provider = $1
  AND provider_task_id = $2
  AND status = 1
RETURNING *;

-- name: UpdateProviderTaskTerminalResponse :one
UPDATE provider_tasks
SET status = $3,
    response = $4,
    updated_at = NOW()
WHERE provider = $1
  AND provider_task_id = $2
  AND status = 1
RETURNING *;

-- name: MarkProviderTaskUsageRecorded :one
UPDATE provider_tasks
SET usage_recorded = TRUE,
    updated_at = NOW()
WHERE provider = $1
  AND provider_task_id = $2
  AND usage_recorded = FALSE
RETURNING *;
