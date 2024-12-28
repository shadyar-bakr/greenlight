ALTER TABLE tokens ADD COLUMN IF NOT EXISTS is_refresh BOOLEAN DEFAULT FALSE;
CREATE INDEX IF NOT EXISTS tokens_refresh_idx ON tokens(user_id) WHERE scope = 'refresh';

---- create above / drop below ----

DROP INDEX IF EXISTS tokens_refresh_idx;
ALTER TABLE tokens DROP COLUMN IF EXISTS is_refresh;
