-- Step 1: Create user_story_statuses table
CREATE TABLE IF NOT EXISTS user_story_statuses (
    id UUID PRIMARY KEY,
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name VARCHAR(50) NOT NULL,
    color VARCHAR(7) NOT NULL,
    display_order INTEGER NOT NULL,
    is_default BOOLEAN NOT NULL DEFAULT FALSE,
    is_closed BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    deleted_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_user_story_status_project_name ON user_story_statuses (project_id, LOWER(name));
CREATE INDEX IF NOT EXISTS idx_user_story_statuses_project_id ON user_story_statuses(project_id);
CREATE INDEX IF NOT EXISTS idx_user_story_statuses_deleted_at ON user_story_statuses(deleted_at);

-- Step 2: Create new User Story status records from custom_statuses, but with new UUIDs!
-- Do NOT copy task status UUIDs.
INSERT INTO user_story_statuses (id, project_id, name, color, display_order, is_default, is_closed, created_at, updated_at, deleted_at)
SELECT gen_random_uuid(), project_id, name, color, display_order, is_default, is_final, created_at, updated_at, deleted_at
FROM custom_statuses;

-- Step 2b: Fallback for projects with no statuses
INSERT INTO user_story_statuses (id, project_id, name, color, display_order, is_default, is_closed, created_at, updated_at)
SELECT 
    gen_random_uuid(), 
    p.id, 
    'Todo', 
    '#808080', 
    0, 
    true, 
    false,
    NOW(), 
    NOW()
FROM projects p
WHERE NOT EXISTS (
    SELECT 1 FROM user_story_statuses uss WHERE uss.project_id = p.id
);

-- Step 3: Add status_id column to user_stories
ALTER TABLE user_stories ADD COLUMN IF NOT EXISTS status_id UUID;

-- Step 4: Map existing User Stories by project_id + status name
UPDATE user_stories us
SET status_id = uss.id
FROM user_story_statuses uss
WHERE us.project_id = uss.project_id AND LOWER(us.status) = LOWER(uss.name);

-- Fallback 1: Default to the project's default status (with lowest display order)
UPDATE user_stories us
SET status_id = (
    SELECT uss.id 
    FROM user_story_statuses uss 
    WHERE uss.project_id = us.project_id AND uss.is_default = true 
    ORDER BY uss.display_order ASC 
    LIMIT 1
)
WHERE us.status_id IS NULL;

-- Fallback 2: Default to ANY status in that project if no status is marked default
UPDATE user_stories us
SET status_id = (
    SELECT uss.id 
    FROM user_story_statuses uss 
    WHERE uss.project_id = us.project_id 
    ORDER BY uss.display_order ASC 
    LIMIT 1
)
WHERE us.status_id IS NULL;

-- Step 5: Make status_id NOT NULL and add foreign key constraint + index
ALTER TABLE user_stories ALTER COLUMN status_id SET NOT NULL;
ALTER TABLE user_stories ADD CONSTRAINT fk_user_stories_status FOREIGN KEY (status_id) REFERENCES user_story_statuses(id) ON DELETE RESTRICT;
CREATE INDEX IF NOT EXISTS idx_user_stories_status_id ON user_stories(status_id);

-- Step 6: Drop the old status column from user_stories
ALTER TABLE user_stories DROP COLUMN IF EXISTS status;
