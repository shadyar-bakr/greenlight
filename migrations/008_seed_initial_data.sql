BEGIN;

-- Seed movies
INSERT INTO movies (title, year, runtime, genres) VALUES
    ('The Shawshank Redemption', 1994, 142, '{"Drama"}'),
    ('The Godfather', 1972, 175, '{"Crime", "Drama"}'),
    ('The Dark Knight', 2008, 152, '{"Action", "Crime", "Drama"}'),
    ('Pulp Fiction', 1994, 154, '{"Crime", "Drama"}'),
    ('Inception', 2010, 148, '{"Action", "Sci-Fi", "Thriller"}');

-- Seed basic permissions
INSERT INTO permissions (code) VALUES 
    ('movies:read'),
    ('movies:write');

-- Seed default roles
INSERT INTO roles (name, description) VALUES
    ('admin', 'System administrator with full access'),
    ('manager', 'Content manager with write access'),
    ('user', 'Regular user with read access')
ON CONFLICT DO NOTHING;

COMMIT;

---- create above / drop below ----

BEGIN;

TRUNCATE roles CASCADE;
TRUNCATE permissions CASCADE;
TRUNCATE movies RESTART IDENTITY CASCADE;

COMMIT; 
