CREATE TABLE channels (
    id SERIAL PRIMARY KEY,
    channel_name VARCHAR(20) NOT NULL,
    base_url VARCHAR(100) NOT NULL,
    status SMALLINT NOT NULL DEFAULT 1, -- 1.active 2.ban 3.pending
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_channels_channel_name ON channels(channel_name);
CREATE INDEX idx_channels_status ON channels(status);
CREATE INDEX idx_channels_base_url ON channels(base_url);
