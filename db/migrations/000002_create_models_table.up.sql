CREATE TABLE models (
    id SERIAL PRIMARY KEY,
    provider VARCHAR(50) NOT NULL, -- 'openai', 'google', 'anthropic', etc
    model_name VARCHAR(100) NOT NULL,
    UNIQUE(provider, model_name)
)