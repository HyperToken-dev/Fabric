CREATE TABLE provider_tasks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider TEXT NOT NULL,
    key_id INTEGER NOT NULL REFERENCES api_keys(id) ON DELETE CASCADE,
    channel_id INTEGER NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
    model_id INTEGER NOT NULL REFERENCES models(id) ON DELETE CASCADE,
    provider_task_id TEXT NOT NULL,
    status SMALLINT NOT NULL,
    request JSONB NOT NULL,
    response JSONB NOT NULL,
    usage_recorded BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT provider_tasks_provider_task_unique UNIQUE (provider, provider_task_id)
);

CREATE INDEX idx_provider_tasks_key_id ON provider_tasks(key_id);
CREATE INDEX idx_provider_tasks_channel_id ON provider_tasks(channel_id);
CREATE INDEX idx_provider_tasks_model_id ON provider_tasks(model_id);
CREATE INDEX idx_provider_tasks_status ON provider_tasks(status);
CREATE INDEX idx_provider_tasks_updated_at ON provider_tasks(updated_at DESC);
