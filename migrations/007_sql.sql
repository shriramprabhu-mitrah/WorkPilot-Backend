-- projects
CREATE TABLE IF NOT EXISTS projects (
    id UUID PRIMARY KEY,
    organization_id UUID NOT NULL,
    name VARCHAR(150) NOT NULL,
    description TEXT,
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    created_by UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ,
    deleted_at TIMESTAMPTZ,

    CONSTRAINT fk_projects_organization
        FOREIGN KEY (organization_id)
        REFERENCES organizations(id)
        ON UPDATE CASCADE
        ON DELETE RESTRICT,

    CONSTRAINT fk_projects_creator
        FOREIGN KEY (created_by)
        REFERENCES users(id)
        ON UPDATE CASCADE
        ON DELETE RESTRICT
);

CREATE INDEX IF NOT EXISTS idx_projects_organization_id
    ON projects (organization_id);

CREATE INDEX IF NOT EXISTS idx_projects_created_by
    ON projects (created_by);

CREATE INDEX IF NOT EXISTS idx_projects_deleted_at
    ON projects (deleted_at);

-- project_members
CREATE TABLE IF NOT EXISTS project_members (
    id UUID PRIMARY KEY,
    project_id UUID NOT NULL,
    user_id UUID NOT NULL,
    joined_at TIMESTAMPTZ NOT NULL,
    added_by_id UUID NOT NULL,

    CONSTRAINT fk_project_members_project
        FOREIGN KEY (project_id)
        REFERENCES projects(id)
        ON UPDATE CASCADE
        ON DELETE CASCADE,

    CONSTRAINT fk_project_members_user
        FOREIGN KEY (user_id)
        REFERENCES users(id)
        ON UPDATE CASCADE
        ON DELETE CASCADE,

    CONSTRAINT fk_project_members_added_by
        FOREIGN KEY (added_by_id)
        REFERENCES users(id)
        ON UPDATE CASCADE
        ON DELETE RESTRICT,

    CONSTRAINT uq_project_members_project_user
        UNIQUE (project_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_project_members_project_id
    ON project_members (project_id);

CREATE INDEX IF NOT EXISTS idx_project_members_user_id
    ON project_members (user_id);

CREATE INDEX IF NOT EXISTS idx_project_members_added_by_id
    ON project_members (added_by_id);

-- sprints
CREATE TABLE IF NOT EXISTS sprints (
    id UUID PRIMARY KEY,
    project_id UUID NOT NULL,
    name VARCHAR(100) NOT NULL,
    goal VARCHAR(500),
    status VARCHAR(20) NOT NULL DEFAULT 'planning',
    start_date DATE NOT NULL,
    end_date DATE NOT NULL,
    created_by_id UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ,
    deleted_at TIMESTAMPTZ,

    CONSTRAINT fk_sprints_project
        FOREIGN KEY (project_id)
        REFERENCES projects(id)
        ON UPDATE CASCADE
        ON DELETE CASCADE,

    CONSTRAINT fk_sprints_created_by
        FOREIGN KEY (created_by_id)
        REFERENCES users(id)
        ON UPDATE CASCADE
        ON DELETE RESTRICT,

    CONSTRAINT uq_sprints_name
        UNIQUE (name)
);

CREATE INDEX IF NOT EXISTS idx_sprints_project_id
    ON sprints (project_id);

CREATE INDEX IF NOT EXISTS idx_sprints_created_by_id
    ON sprints (created_by_id);

CREATE INDEX IF NOT EXISTS idx_sprints_deleted_at
    ON sprints (deleted_at);
