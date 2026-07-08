CREATE TABLE models ( -- use model in mapping system
    id SERIAL PRIMARY KEY,
    channel_id INTEGER NOT NULL REFERENCES channels(id),
    model_name VARCHAR(100) NOT NULL,
    model_type INTEGER NOT NULL DEFAULT 1, -- 1.text 2.audio 3.video 4.file 5.embedding 6.image
    status SMALLINT NOT NULL DEFAULT 1, -- 1.active 2.ban 3.pending
    UNIQUE(channel_id, model_name)
);

CREATE INDEX idx_models_status ON models(status);
CREATE INDEX idx_models_channel_status ON models(channel_id, status);
