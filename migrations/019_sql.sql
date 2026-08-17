-- Create user_story_attachments table
CREATE TABLE IF NOT EXISTS user_story_attachments (
    id UUID PRIMARY KEY,
    user_story_id UUID NOT NULL REFERENCES user_stories(id) ON DELETE CASCADE,
    original_filename VARCHAR(255) NOT NULL,
    stored_filename VARCHAR(255) NOT NULL,
    mime_type VARCHAR(100) NOT NULL,
    file_size BIGINT NOT NULL,
    storage_path TEXT NOT NULL,
    uploaded_by UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    uploaded_at TIMESTAMPTZ NOT NULL
);

-- Index on user_story_id
CREATE INDEX IF NOT EXISTS idx_user_story_attachments_user_story_id ON user_story_attachments(user_story_id);

-- Alter comments table to support user stories comments
ALTER TABLE comments ALTER COLUMN task_id DROP NOT NULL;

ALTER TABLE comments ADD COLUMN IF NOT EXISTS user_story_id UUID REFERENCES user_stories(id) ON DELETE CASCADE;

ALTER TABLE comments DROP CONSTRAINT IF EXISTS chk_comment_target;

ALTER TABLE comments ADD CONSTRAINT chk_comment_target CHECK (
    (task_id IS NOT NULL AND user_story_id IS NULL) OR
    (task_id IS NULL AND user_story_id IS NOT NULL)
);

-- Index on comments.user_story_id
CREATE INDEX IF NOT EXISTS idx_comments_user_story_id ON comments(user_story_id);
