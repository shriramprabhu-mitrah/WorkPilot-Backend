-- Remove blocked_reason column from tasks table
ALTER TABLE tasks DROP COLUMN IF EXISTS blocked_reason;
