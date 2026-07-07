CREATE TABLE models ( -- use model in mapping system
    id SERIAL PRIMARY KEY,
    channel_id INTEGER NOT NULL REFERENCES channels(id),
    model_name VARCHAR(100) NOT NULL,
    UNIQUE(channel_id, model_name)
);