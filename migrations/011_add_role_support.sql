CREATE TABLE IF NOT EXISTS roles (
    id bigserial PRIMARY KEY,
    name text NOT NULL UNIQUE,
    description text,
    parent_id bigint REFERENCES roles(id) ON DELETE SET NULL,
    created_at timestamp(0) with time zone NOT NULL DEFAULT NOW(),
    version integer NOT NULL DEFAULT 1
);

CREATE TABLE IF NOT EXISTS roles_permissions (
    role_id bigint NOT NULL REFERENCES roles ON DELETE CASCADE,
    permission_id bigint NOT NULL REFERENCES permissions ON DELETE CASCADE,
    granted_at timestamp(0) with time zone NOT NULL DEFAULT NOW(),
    PRIMARY KEY (role_id, permission_id)
);

CREATE TABLE IF NOT EXISTS users_roles (
    user_id bigint NOT NULL REFERENCES users ON DELETE CASCADE,
    role_id bigint NOT NULL REFERENCES roles ON DELETE CASCADE,
    granted_at timestamp(0) with time zone NOT NULL DEFAULT NOW(),
    granted_by bigint REFERENCES users ON DELETE SET NULL,
    PRIMARY KEY (user_id, role_id)
);

CREATE INDEX IF NOT EXISTS roles_parent_idx ON roles(parent_id);
CREATE INDEX IF NOT EXISTS roles_permissions_role_idx ON roles_permissions(role_id);
CREATE INDEX IF NOT EXISTS roles_permissions_permission_idx ON roles_permissions(permission_id);
CREATE INDEX IF NOT EXISTS users_roles_user_idx ON users_roles(user_id);
CREATE INDEX IF NOT EXISTS users_roles_role_idx ON users_roles(role_id);

INSERT INTO roles (name, description) VALUES
    ('admin', 'System administrator with full access'),
    ('manager', 'Content manager with write access'),
    ('user', 'Regular user with read access')
ON CONFLICT DO NOTHING;

---- create above / drop below ----

DROP TABLE IF EXISTS users_roles;
DROP TABLE IF EXISTS roles_permissions;
DROP TABLE IF EXISTS roles; 
