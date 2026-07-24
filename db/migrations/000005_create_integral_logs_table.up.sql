CREATE TABLE integral_logs (
    id SERIAL PRIMARY KEY,
    context JSON NOT NULL,
    response TEXT DEFAULT NULL,
    key_id INTEGER NOT NULL REFERENCES api_keys(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_integral_logs_api_key_id ON integral_logs(key_id);
CREATE INDEX idx_integral_logs_key_time ON integral_logs(key_id, created_at DESC);