-- Create user_stories table
CREATE TABLE IF NOT EXISTS user_stories (
    id UUID PRIMARY KEY,
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    sprint_id UUID REFERENCES sprints(id) ON DELETE SET NULL,
    title VARCHAR(255) NOT NULL,
    description TEXT,
    priority VARCHAR(50) NOT NULL DEFAULT 'medium',
    status VARCHAR(50) NOT NULL DEFAULT 'todo',
    story_points INTEGER NOT NULL DEFAULT 0,
    assignee_id UUID REFERENCES users(id) ON DELETE SET NULL,
    reporter_id UUID NOT NULL REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    deleted_at TIMESTAMPTZ
);

-- Indexes for performance and lookup
CREATE INDEX IF NOT EXISTS idx_user_stories_project ON user_stories(project_id);
CREATE INDEX IF NOT EXISTS idx_user_stories_sprint ON user_stories(sprint_id);
CREATE INDEX IF NOT EXISTS idx_user_stories_assignee ON user_stories(assignee_id);
CREATE INDEX IF NOT EXISTS idx_user_stories_reporter ON user_stories(reporter_id);
CREATE INDEX IF NOT EXISTS idx_user_stories_deleted_at ON user_stories(deleted_at);
