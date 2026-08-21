-- name: InsertUsageLog :one
INSERT INTO usage_logs (key_id, channel_id, model_id, prompt_tokens, completion_tokens)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetUsageLogsByKeyID :many
SELECT * FROM usage_logs
WHERE key_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: GetUsageLogsByKeyHash :many
SELECT usage_logs.* from usage_logs
JOIN api_keys ON usage_logs.key_id = api_keys.id
WHERE api_keys.key_hash = $1
ORDER BY usage_logs.created_at DESC
LIMIT $2 OFFSET $3;

-- name: GetUsageLogsByUser :many
SELECT usage_logs.* FROM usage_logs
JOIN api_keys ON usage_logs.key_id = api_keys.id
WHERE api_keys.user_id = $1
ORDER BY usage_logs.created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountUsageLogsByKeyID :one
SELECT COUNT(*) FROM usage_logs WHERE key_id = $1;

-- name: GetUsageLogsByChannel :many
SELECT * FROM usage_logs
WHERE channel_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: GetUsageLogsByModelID :many
SELECT * FROM usage_logs
WHERE model_id = $1
ORDER BY created_at DESC
LIMIT $2 OFFSET $3;

-- name: CountUsageLogsByChannel :one
SELECT COUNT(*) FROM usage_logs WHERE channel_id = $1;

-- name: GetUsageStatsByKey :many
SELECT
    model_id,
    SUM(prompt_tokens)::bigint AS total_prompt_tokens,
    SUM(completion_tokens)::bigint AS total_completion_tokens,
    COUNT(*)::bigint AS request_count
FROM usage_logs
WHERE key_id = $1
  AND ($2::timestamptz IS NULL OR created_at >= $2)
  AND ($3::timestamptz IS NULL OR created_at <= $3)
GROUP BY model_id
ORDER BY request_count DESC;

-- name: GetUsageStatsByKeyHash :many
SELECT
    usage_logs.model_id,
    SUM(usage_logs.prompt_tokens)::bigint AS total_prompt_tokens,
    SUM(usage_logs.completion_tokens)::bigint AS total_completion_tokens,
    COUNT(*)::bigint AS request_count
FROM usage_logs
JOIN api_keys ON usage_logs.key_id = api_keys.id
WHERE api_keys.key_hash = $1
  AND ($2::timestamptz IS NULL OR usage_logs.created_at >= $2)
  AND ($3::timestamptz IS NULL OR usage_logs.created_at <= $3)
GROUP BY usage_logs.model_id
ORDER BY request_count DESC;

-- name: GetUsageStatsByChannel :many
SELECT
    model_id,
    SUM(prompt_tokens)::bigint AS total_prompt_tokens,
    SUM(completion_tokens)::bigint AS total_completion_tokens,
    COUNT(*)::bigint AS request_count
FROM usage_logs
WHERE channel_id = $1
  AND ($2::timestamptz IS NULL OR created_at >= $2)
  AND ($3::timestamptz IS NULL OR created_at <= $3)
GROUP BY model_id
ORDER BY request_count DESC;

-- name: GetUsageStatsGlobal :many
SELECT
    channel_id,
    model_id,
    SUM(prompt_tokens)::bigint AS total_prompt_tokens,
    SUM(completion_tokens)::bigint AS total_completion_tokens,
    COUNT(*)::bigint AS request_count
FROM usage_logs
WHERE ($1::timestamptz IS NULL OR created_at >= $1)
  AND ($2::timestamptz IS NULL OR created_at <= $2)
GROUP BY channel_id, model_id
ORDER BY request_count DESC;

-- name: GetUsageTimelineByKey :many
SELECT
    DATE(created_at)::date AS date,
    SUM(prompt_tokens)::bigint AS total_prompt_tokens,
    SUM(completion_tokens)::bigint AS total_completion_tokens,
    COUNT(*)::bigint AS request_count
FROM usage_logs
WHERE key_id = $1
  AND ($2::timestamptz IS NULL OR created_at >= $2)
  AND ($3::timestamptz IS NULL OR created_at <= $3)
GROUP BY DATE(created_at)
ORDER BY date;

-- name: GetUsageTimelineByChannel :many
SELECT
    DATE(created_at)::date AS date,
    SUM(prompt_tokens)::bigint AS total_prompt_tokens,
    SUM(completion_tokens)::bigint AS total_completion_tokens,
    COUNT(*)::bigint AS request_count
FROM usage_logs
WHERE channel_id = $1
  AND ($2::timestamptz IS NULL OR created_at >= $2)
  AND ($3::timestamptz IS NULL OR created_at <= $3)
GROUP BY DATE(created_at)
ORDER BY date;

-- name: GetUsageTimelineGlobal :many
SELECT
    DATE(created_at)::date AS date,
    SUM(prompt_tokens)::bigint AS total_prompt_tokens,
    SUM(completion_tokens)::bigint AS total_completion_tokens,
    COUNT(*)::bigint AS request_count
FROM usage_logs
WHERE ($1::timestamptz IS NULL OR created_at >= $1)
  AND ($2::timestamptz IS NULL OR created_at <= $2)
GROUP BY DATE(created_at)
ORDER BY date;

-- name: GetUsageDashboardTotals :one
SELECT
    COALESCE(SUM(prompt_tokens), 0)::bigint AS total_prompt_tokens,
    COALESCE(SUM(completion_tokens), 0)::bigint AS total_completion_tokens,
    COUNT(*)::bigint AS request_count
FROM usage_logs
WHERE created_at >= sqlc.arg(start_at)
  AND created_at < sqlc.arg(end_at);

-- name: GetUsageDashboardTimeline :many
SELECT
    DATE(created_at AT TIME ZONE sqlc.arg(time_zone)::text)::date AS date,
    SUM(prompt_tokens)::bigint AS total_prompt_tokens,
    SUM(completion_tokens)::bigint AS total_completion_tokens,
    COUNT(*)::bigint AS request_count
FROM usage_logs
WHERE created_at >= sqlc.arg(start_at)
  AND created_at < sqlc.arg(end_at)
GROUP BY DATE(created_at AT TIME ZONE sqlc.arg(time_zone)::text)
ORDER BY date;

-- name: GetUsageDashboardTotalsByUser :one
SELECT
    COALESCE(SUM(usage_logs.prompt_tokens), 0)::bigint AS total_prompt_tokens,
    COALESCE(SUM(usage_logs.completion_tokens), 0)::bigint AS total_completion_tokens,
    COUNT(*)::bigint AS request_count
FROM usage_logs
JOIN api_keys ON usage_logs.key_id = api_keys.id
WHERE api_keys.user_id = sqlc.arg(user_id)
  AND usage_logs.created_at >= sqlc.arg(start_at)
  AND usage_logs.created_at < sqlc.arg(end_at);

-- name: GetUsageDashboardTimelineByUser :many
SELECT
    DATE(usage_logs.created_at AT TIME ZONE sqlc.arg(time_zone)::text)::date AS date,
    SUM(usage_logs.prompt_tokens)::bigint AS total_prompt_tokens,
    SUM(usage_logs.completion_tokens)::bigint AS total_completion_tokens,
    COUNT(*)::bigint AS request_count
FROM usage_logs
JOIN api_keys ON usage_logs.key_id = api_keys.id
WHERE api_keys.user_id = sqlc.arg(user_id)
  AND usage_logs.created_at >= sqlc.arg(start_at)
  AND usage_logs.created_at < sqlc.arg(end_at)
GROUP BY DATE(usage_logs.created_at AT TIME ZONE sqlc.arg(time_zone)::text)
ORDER BY date;
