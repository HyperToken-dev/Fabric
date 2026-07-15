CREATE TABLE channels (
    id SERIAL PRIMARY KEY,
    channel_name VARCHAR(20) NOT NULL UNIQUE,
    base_url VARCHAR(100) NOT NULL,
    provider_key TEXT NOT NULL DEFAULT '',
    api_format INTEGER NOT NULL DEFAULT 1, -- 1.openai
    status SMALLINT NOT NULL DEFAULT 1, -- 1.active 2.ban 3.pending
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_channels_status ON channels(status);
CREATE INDEX idx_channels_base_url ON channels(base_url);
CREATE INDEX idx_channels_api_format ON channels(api_format);
