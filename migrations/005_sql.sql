-- Create the countries table
CREATE TABLE IF NOT EXISTS countries (
    id              UUID PRIMARY KEY,
    name            VARCHAR(100) NOT NULL,
    iso2            CHAR(2) NOT NULL UNIQUE,
    iso3            CHAR(3) NOT NULL UNIQUE,
    phone_code      VARCHAR(10),
    timezone        TEXT[] NOT NULL,
    flag_emoji      VARCHAR(10),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE EXTENSION IF NOT EXISTS pg_trgm;
CREATE INDEX idx_countries_name_trgm ON countries USING GIN (name gin_trgm_ops);