CREATE TABLE models ( -- use model in mapping system
    id SERIAL PRIMARY KEY,
    channel_id INTEGER NOT NULL REFERENCES channels(id),
    model_name VARCHAR(100) NOT NULL,
    status SMALLINT NOT NULL DEFAULT 1, -- 1.active 2.ban 3.pending
    UNIQUE(channel_id, model_name)
);

CREATE INDEX idx_models_status ON models(status);
CREATE INDEX idx_models_channel_status ON models(channel_id, status);