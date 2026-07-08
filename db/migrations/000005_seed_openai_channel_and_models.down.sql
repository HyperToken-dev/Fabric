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
DELETE FROM models
WHERE channel_id IN (SELECT id FROM seed_openai_channel)
  AND model_name IN (SELECT model_name FROM seed_openai_models);

DELETE FROM channels
WHERE channel_name = 'OpenAI'
  AND NOT EXISTS (SELECT 1 FROM models WHERE models.channel_id = channels.id)
  AND NOT EXISTS (SELECT 1 FROM api_keys WHERE api_keys.channel_id = channels.id)
  AND NOT EXISTS (SELECT 1 FROM usage_logs WHERE usage_logs.channel_id = channels.id);
