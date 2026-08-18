CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    issuer TEXT NOT NULL,
    subject TEXT NOT NULL,
    email TEXT NOT NULL,
    display_name TEXT NOT NULL DEFAULT '',
    avatar_url TEXT NOT NULL DEFAULT '',
    role TEXT NOT NULL DEFAULT 'user',
    status TEXT NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_login_at TIMESTAMPTZ DEFAULT NULL,
    UNIQUE (issuer, subject),
    CHECK (role IN ('admin', 'user')),
    CHECK (status IN ('active', 'disabled'))
);

ALTER TABLE api_keys
ADD COLUMN user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE;

CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_role ON users(role);
CREATE INDEX idx_api_keys_user_id ON api_keys(user_id);

INSERT INTO users (issuer, subject, email, display_name, role, status)
VALUES ('fabric', 'system', 'system@fabric.local', 'Fabric System', 'admin', 'active');
