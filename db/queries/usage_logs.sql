-- name: InsertUsageLog :one
INSERT INTO usage_logs (api_key_hash, provider, model_name, prompt_tokens, completion_tokens)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetUsageLogsByKey :many
SELECT * FROM usage_logs
WHERE api_key_hash = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountUsageLogsByKey :one
SELECT COUNT(*) FROM usage_logs WHERE api_key_hash = $1;

-- name: GetUsageStats :many
SELECT
    provider,
    model_name,
    SUM(prompt_tokens)::bigint AS total_prompt_tokens,
    SUM(completion_tokens)::bigint AS total_completion_tokens,
    COUNT(*)::bigint AS request_count
FROM usage_logs
WHERE api_key_hash = $1
  AND ($2::timestamptz IS NULL OR created_at >= $2)
  AND ($3::timestamptz IS NULL OR created_at <= $3)
GROUP BY provider, model_name
ORDER BY request_count DESC;

-- name: GetUsageStatsByProvider :many
SELECT
    provider,
    model_name,
    SUM(prompt_tokens)::bigint AS total_prompt_tokens,
    SUM(completion_tokens)::bigint AS total_completion_tokens,
    COUNT(*)::bigint AS request_count
FROM usage_logs
WHERE api_key_hash = $1
  AND provider = $2
  AND ($3::timestamptz IS NULL OR created_at >= $3)
  AND ($4::timestamptz IS NULL OR created_at <= $4)
GROUP BY provider, model_name
ORDER BY request_count DESC;
