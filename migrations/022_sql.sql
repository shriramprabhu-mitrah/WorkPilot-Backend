-- Add url column to user_story_attachments table
ALTER TABLE user_story_attachments ADD COLUMN IF NOT EXISTS url TEXT NOT NULL DEFAULT '';


-- Add user_story_id column to audit_logs table
ALTER TABLE audit_logs ADD COLUMN IF NOT EXISTS user_story_id UUID;
CREATE INDEX IF NOT EXISTS idx_audit_logs_user_story_id ON audit_logs(user_story_id);