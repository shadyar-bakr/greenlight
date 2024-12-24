BEGIN;

INSERT INTO movies (title, year, runtime, genres) VALUES
    ('The Shawshank Redemption', 1994, 142, '{"Drama"}'),
    ('The Godfather', 1972, 175, '{"Crime", "Drama"}'),
    ('The Dark Knight', 2008, 152, '{"Action", "Crime", "Drama"}'),
    ('Pulp Fiction', 1994, 154, '{"Crime", "Drama"}'),
    ('Inception', 2010, 148, '{"Action", "Sci-Fi", "Thriller"}');

COMMIT;

---- create above / drop below ----

BEGIN;

TRUNCATE movies RESTART IDENTITY CASCADE;

COMMIT; 
