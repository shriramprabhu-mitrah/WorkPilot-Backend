-- Alter comment_id column to be nullable
ALTER TABLE comment_attachments ALTER COLUMN comment_id DROP NOT NULL;

-- Add task_id and user_story_id columns to comment_attachments
ALTER TABLE comment_attachments ADD COLUMN IF NOT EXISTS task_id UUID REFERENCES tasks(id) ON DELETE CASCADE;
ALTER TABLE comment_attachments ADD COLUMN IF NOT EXISTS user_story_id UUID REFERENCES user_stories(id) ON DELETE CASCADE;

-- Create indexes on task_id and user_story_id
CREATE INDEX IF NOT EXISTS idx_comment_attachments_task_id ON comment_attachments(task_id);
CREATE INDEX IF NOT EXISTS idx_comment_attachments_user_story_id ON comment_attachments(user_story_id);

-- Alter task_id column in task_attachments to be nullable
ALTER TABLE task_attachments ALTER COLUMN task_id DROP NOT NULL;

-- Alter user_story_id column in user_story_attachments to be nullable
ALTER TABLE user_story_attachments ALTER COLUMN user_story_id DROP NOT NULL;

-- Add project_id column to task_attachments and user_story_attachments
ALTER TABLE task_attachments ADD COLUMN IF NOT EXISTS project_id UUID REFERENCES projects(id) ON DELETE CASCADE;
ALTER TABLE user_story_attachments ADD COLUMN IF NOT EXISTS project_id UUID REFERENCES projects(id) ON DELETE CASCADE;

-- Create indexes on project_id
CREATE INDEX IF NOT EXISTS idx_task_attachments_project_id ON task_attachments(project_id);
CREATE INDEX IF NOT EXISTS idx_user_story_attachments_project_id ON user_story_attachments(project_id);
