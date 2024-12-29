BEGIN;

CREATE TABLE IF NOT EXISTS resource_permissions (
    id bigserial PRIMARY KEY,
    user_id bigint NOT NULL REFERENCES users ON DELETE CASCADE,
    resource_type text NOT NULL,
    resource_id bigint NOT NULL,
    permission text NOT NULL,
    granted_by bigint REFERENCES users ON DELETE SET NULL,
    created_at timestamp(0) with time zone NOT NULL DEFAULT NOW(),
    UNIQUE (user_id, resource_type, resource_id, permission)
);

CREATE INDEX IF NOT EXISTS resource_permissions_user_idx ON resource_permissions(user_id);
CREATE INDEX IF NOT EXISTS resource_permissions_resource_idx ON resource_permissions(resource_type, resource_id);

COMMIT;

---- create above / drop below ----

BEGIN;

DROP TABLE IF EXISTS resource_permissions CASCADE;

COMMIT; 
