CREATE TABLE IF NOT EXISTS custom_statuses (
    id UUID PRIMARY KEY,
    project_id UUID NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
    name VARCHAR(50) NOT NULL,
    color VARCHAR(7) NOT NULL,
    display_order INTEGER NOT NULL,
    is_default BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    deleted_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_custom_status_project_name ON custom_statuses (project_id, LOWER(name));
CREATE INDEX IF NOT EXISTS idx_custom_statuses_project_id ON custom_statuses(project_id);
CREATE INDEX IF NOT EXISTS idx_custom_statuses_deleted_at ON custom_statuses(deleted_at);
