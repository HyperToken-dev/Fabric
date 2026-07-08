INSERT INTO channels (channel_name, base_url, provider_key, api_format, status)
VALUES ('OpenAI', 'https://api.openai.com', '', 1, 1)
ON CONFLICT (channel_name) DO UPDATE SET
    base_url = EXCLUDED.base_url,
    provider_key = EXCLUDED.provider_key,
    api_format = EXCLUDED.api_format,
    status = EXCLUDED.status;

WITH seed_openai_channel AS (
    SELECT id FROM channels WHERE channel_name = 'OpenAI'
),
seed_openai_models(model_name) AS (
    VALUES
        ('babbage-002'),
        ('chat-latest'),
        ('davinci-002'),
        ('gpt-3.5-turbo'),
        ('gpt-3.5-turbo-16k'),
        ('gpt-3.5-turbo-instruct'),
        ('gpt-4.1'),
        ('gpt-4.1-mini'),
        ('gpt-4.1-nano'),
        ('gpt-4o'),
        ('gpt-4o-mini'),
        ('gpt-4o-mini-search-preview'),
        ('gpt-4o-search-preview'),
        ('gpt-5'),
        ('gpt-5-chat-latest'),
        ('gpt-5-codex'),
        ('gpt-5-mini'),
        ('gpt-5-nano'),
        ('gpt-5-pro'),
        ('gpt-5-search-api'),
        ('gpt-5.1'),
        ('gpt-5.1-chat-latest'),
        ('gpt-5.1-codex'),
        ('gpt-5.1-codex-max'),
        ('gpt-5.1-codex-mini'),
        ('gpt-5.2'),
        ('gpt-5.2-chat-latest'),
        ('gpt-5.2-codex'),
        ('gpt-5.2-pro'),
        ('gpt-5.3-chat-latest'),
        ('gpt-5.3-codex'),
        ('gpt-5.4'),
        ('gpt-5.4-mini'),
        ('gpt-5.4-nano'),
        ('gpt-5.4-pro'),
        ('gpt-5.5'),
        ('gpt-5.5-pro'),
        ('o1'),
        ('o3'),
        ('o3-mini'),
        ('o4-mini')
)
INSERT INTO models (channel_id, model_name, model_type, status)
SELECT seed_openai_channel.id, seed_openai_models.model_name, 1, 1
FROM seed_openai_channel
CROSS JOIN seed_openai_models
ON CONFLICT (channel_id, model_name) DO UPDATE SET
    model_type = EXCLUDED.model_type,
    status = EXCLUDED.status;
