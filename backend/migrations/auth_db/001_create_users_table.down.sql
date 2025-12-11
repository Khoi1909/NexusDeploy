-- Rollback users table creation
DROP INDEX IF EXISTS idx_oauth_states_expires_at;
DROP TABLE IF EXISTS oauth_states;

DROP INDEX IF EXISTS idx_jwt_blacklist_expired_at;
DROP TABLE IF EXISTS jwt_blacklist;

DROP INDEX IF EXISTS idx_users_username;
DROP INDEX IF EXISTS idx_users_github_id;
DROP TABLE IF EXISTS users;

