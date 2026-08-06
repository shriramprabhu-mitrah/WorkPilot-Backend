-- 1. Create labels table
CREATE TABLE IF NOT EXISTS labels (
    id UUID PRIMARY KEY,
    project_id UUID NOT NULL,
    name VARCHAR(30) NOT NULL,
    color VARCHAR(7) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ,

    CONSTRAINT fk_labels_project
        FOREIGN KEY (project_id)
        REFERENCES projects(id)
        ON UPDATE CASCADE
        ON DELETE CASCADE
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_project_label_name ON labels (project_id, name);
CREATE INDEX IF NOT EXISTS idx_labels_project_id ON labels (project_id);

-- 2. Create task_labels join table
CREATE TABLE IF NOT EXISTS task_labels (
    task_id UUID NOT NULL,
    label_id UUID NOT NULL,
    PRIMARY KEY (task_id, label_id),

    CONSTRAINT fk_task_labels_task
        FOREIGN KEY (task_id)
        REFERENCES tasks(id)
        ON UPDATE CASCADE
        ON DELETE CASCADE,

    CONSTRAINT fk_task_labels_label
        FOREIGN KEY (label_id)
        REFERENCES labels(id)
        ON UPDATE CASCADE
        ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_task_labels_task_id ON task_labels (task_id);
CREATE INDEX IF NOT EXISTS idx_task_labels_label_id ON task_labels (label_id);
