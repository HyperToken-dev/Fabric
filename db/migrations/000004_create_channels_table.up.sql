CREATE TABLE channels (
    id SERIAL PRIMARY KEY,
    channel_name VARCHAR(20) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
