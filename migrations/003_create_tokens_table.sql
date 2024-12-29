BEGIN;

CREATE TABLE IF NOT EXISTS tokens (
    hash bytea PRIMARY KEY,
    user_id bigint NOT NULL REFERENCES users ON DELETE CASCADE,
    expiry timestamp(0) with time zone NOT NULL,
    scope text NOT NULL,
    is_refresh BOOLEAN NOT NULL DEFAULT FALSE
);

CREATE INDEX IF NOT EXISTS tokens_refresh_idx ON tokens(user_id) WHERE scope = 'refresh';

COMMIT;

---- create above / drop below ----

BEGIN;

DROP TABLE IF EXISTS tokens CASCADE;

COMMIT; 
