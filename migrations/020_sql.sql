-- Add is_closed column to user_stories table
ALTER TABLE user_stories ADD COLUMN IF NOT EXISTS is_closed BOOLEAN NOT NULL DEFAULT FALSE;
