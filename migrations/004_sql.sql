-- Email verification support

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS is_verified BOOLEAN NOT NULL DEFAULT FALSE;

-- User joined at time support

ALTER TABLE users
    ADD COLUMN IF NOT EXISTS joined_at TIMESTAMP with time zone NOT NULL DEFAULT NOW();
