-- Add is_final column to user_story_statuses table
ALTER TABLE user_story_statuses ADD COLUMN IF NOT EXISTS is_final BOOLEAN NOT NULL DEFAULT FALSE;

-- Sync is_final with the existing is_closed values
UPDATE user_story_statuses
SET is_final = is_closed;
