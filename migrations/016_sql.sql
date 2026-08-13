-- Alter user_stories table to add backlog_order
ALTER TABLE user_stories ADD COLUMN IF NOT EXISTS backlog_order INTEGER NOT NULL DEFAULT 0;

-- Alter tasks table to add user_story_id referencing user_stories
ALTER TABLE tasks ADD COLUMN IF NOT EXISTS user_story_id UUID REFERENCES user_stories(id) ON DELETE SET NULL;

-- Create index on tasks.user_story_id
CREATE INDEX IF NOT EXISTS idx_tasks_user_story_id ON tasks(user_story_id);
