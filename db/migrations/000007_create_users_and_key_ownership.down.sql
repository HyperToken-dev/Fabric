DROP INDEX IF EXISTS idx_api_keys_user_id;
DROP INDEX IF EXISTS idx_users_role;
DROP INDEX IF EXISTS idx_users_email;

ALTER TABLE api_keys DROP COLUMN IF EXISTS user_id;

DROP TABLE IF EXISTS users;
