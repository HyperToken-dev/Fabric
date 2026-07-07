CREATE TABLE usage_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    key_id INTEGER NOT NULL REFERENCES api_keys(id) ON DELETE CASCADE,
    channel_id INTEGER NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
    model_id INTEGER NOT NULL REFERENCES models(id) ON DELETE CASCADE,
    prompt_tokens BIGINT NOT NULL DEFAULT 0,
    completion_tokens BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_usage_logs_api_key_id ON usage_logs(key_id);
CREATE INDEX idx_usage_logs_channel_id ON usage_logs(channel_id);
CREATE INDEX idx_usage_logs_created_at ON usage_logs(created_at);
CREATE INDEX idx_usage_logs_key_time ON usage_logs(key_id, created_at DESC);
CREATE INDEX idx_usage_logs_channel_time ON usage_logs(channel_id, created_at DESC);