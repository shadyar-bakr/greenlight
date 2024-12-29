-- Add trusted clients support

-- Migrate Up
CREATE TABLE IF NOT EXISTS trusted_clients (
    id bigserial PRIMARY KEY,
    name text NOT NULL,
    description text,
    api_key_hash bytea NOT NULL UNIQUE,
    rate_limit_rps integer NOT NULL DEFAULT 1000,
    rate_limit_burst integer NOT NULL DEFAULT 2000,
    enabled boolean NOT NULL DEFAULT true,
    created_at timestamp(0) with time zone NOT NULL DEFAULT NOW(),
    version integer NOT NULL DEFAULT 1
);

CREATE INDEX IF NOT EXISTS trusted_clients_api_key_idx ON trusted_clients(api_key_hash);

-- Add audit logging for API key usage
CREATE TABLE IF NOT EXISTS trusted_client_logs (
    id bigserial PRIMARY KEY,
    client_id bigint NOT NULL REFERENCES trusted_clients ON DELETE CASCADE,
    endpoint text NOT NULL,
    method text NOT NULL,
    status_code integer NOT NULL,
    timestamp timestamp(0) with time zone NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS trusted_client_logs_client_idx ON trusted_client_logs(client_id);
CREATE INDEX IF NOT EXISTS trusted_client_logs_timestamp_idx ON trusted_client_logs(timestamp);

---- create above / drop below ----

DROP TABLE IF EXISTS trusted_client_logs;
DROP TABLE IF EXISTS trusted_clients; 
