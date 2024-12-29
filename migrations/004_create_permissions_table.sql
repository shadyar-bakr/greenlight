BEGIN;

CREATE TABLE IF NOT EXISTS permissions (
    id bigserial PRIMARY KEY,
    code text NOT NULL UNIQUE
);

COMMIT;

---- create above / drop below ----

BEGIN;

DROP TABLE IF EXISTS permissions CASCADE;

COMMIT; 
