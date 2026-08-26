-- Migration to add require_password_change column to users table
ALTER TABLE users ADD COLUMN IF NOT EXISTS require_password_change BOOLEAN NOT NULL DEFAULT FALSE;
