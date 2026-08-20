-- Add url column to user_story_attachments table
ALTER TABLE user_story_attachments ADD COLUMN IF NOT EXISTS url TEXT NOT NULL DEFAULT '';
