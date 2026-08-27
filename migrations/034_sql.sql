-- Create GIN indexes for Full-Text Search on tasks, user_stories, projects, and users tables
-- We combine multiple text fields using coalesce and to_tsvector, then index them with GIN

-- Tasks
CREATE INDEX IF NOT EXISTS idx_tasks_fts ON tasks USING gin(
    to_tsvector('english', coalesce(title, '') || ' ' || coalesce(description, '') || ' ' || coalesce(key, ''))
);

-- User Stories
CREATE INDEX IF NOT EXISTS idx_user_stories_fts ON user_stories USING gin(
    to_tsvector('english', coalesce(title, '') || ' ' || coalesce(description, '') || ' ' || coalesce(key, ''))
);

-- Projects
CREATE INDEX IF NOT EXISTS idx_projects_fts ON projects USING gin(
    to_tsvector('english', coalesce(name, '') || ' ' || coalesce(description, '') || ' ' || coalesce(slug, ''))
);

-- Users (within the context of global user/member search)
CREATE INDEX IF NOT EXISTS idx_users_fts ON users USING gin(
    to_tsvector('english', coalesce(full_name, '') || ' ' || coalesce(email, '') || ' ' || coalesce(username, ''))
);

-- Sprints (within the context of global sprints search)
CREATE INDEX IF NOT EXISTS idx_sprints_fts ON sprints USING gin(
    to_tsvector('english', coalesce(name, '') || ' ' || coalesce(goal, ''))
);