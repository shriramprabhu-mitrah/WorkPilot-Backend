-- 1. Add velocity column to sprints
ALTER TABLE sprints ADD COLUMN IF NOT EXISTS velocity INT;

-- 2. Create tasks table
CREATE TABLE IF NOT EXISTS tasks (
    id UUID PRIMARY KEY,
    project_id UUID NOT NULL,
    sprint_id UUID,
    key VARCHAR(50) NOT NULL,
    sequence_number INT NOT NULL,
    title VARCHAR(255) NOT NULL,
    description TEXT,
    type VARCHAR(50) NOT NULL DEFAULT 'task',
    priority VARCHAR(50) NOT NULL DEFAULT 'medium',
    status VARCHAR(50) NOT NULL DEFAULT 'todo',
    assignee_id UUID,
    story_points INT NOT NULL DEFAULT 0,
    due_date TIMESTAMPTZ,
    estimated_hours NUMERIC,
    actual_hours NUMERIC,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ,
    deleted_at TIMESTAMPTZ,

    CONSTRAINT fk_tasks_project
        FOREIGN KEY (project_id)
        REFERENCES projects(id)
        ON UPDATE CASCADE
        ON DELETE CASCADE,

    CONSTRAINT fk_tasks_sprint
        FOREIGN KEY (sprint_id)
        REFERENCES sprints(id)
        ON UPDATE CASCADE
        ON DELETE SET NULL,

    CONSTRAINT fk_tasks_assignee
        FOREIGN KEY (assignee_id)
        REFERENCES users(id)
        ON UPDATE CASCADE
        ON DELETE SET NULL,

    CONSTRAINT uq_tasks_project_key
        UNIQUE (project_id, key)
);

CREATE INDEX IF NOT EXISTS idx_tasks_project_id ON tasks (project_id);
CREATE INDEX IF NOT EXISTS idx_tasks_sprint_id ON tasks (sprint_id);
CREATE INDEX IF NOT EXISTS idx_tasks_assignee_id ON tasks (assignee_id);
CREATE INDEX IF NOT EXISTS idx_tasks_deleted_at ON tasks (deleted_at);

-- 3. Create sprint_snapshots table
CREATE TABLE IF NOT EXISTS sprint_snapshots (
    id UUID PRIMARY KEY,
    sprint_id UUID NOT NULL,
    date DATE NOT NULL,
    total_story_points INT NOT NULL DEFAULT 0,
    remaining_story_points INT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL,

    CONSTRAINT fk_sprint_snapshots_sprint
        FOREIGN KEY (sprint_id)
        REFERENCES sprints(id)
        ON UPDATE CASCADE
        ON DELETE CASCADE,

    CONSTRAINT uq_sprint_snapshots_sprint_date
        UNIQUE (sprint_id, date)
);

CREATE INDEX IF NOT EXISTS idx_sprint_snapshots_sprint_id ON sprint_snapshots (sprint_id);
CREATE INDEX IF NOT EXISTS idx_sprint_snapshots_date ON sprint_snapshots (date);
