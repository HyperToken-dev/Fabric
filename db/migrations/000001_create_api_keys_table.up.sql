CREATE TABLE api_keys (
    key_hash VARCHAR(255) PRIMARY KEY,
    key_name VARCHAR(100) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
