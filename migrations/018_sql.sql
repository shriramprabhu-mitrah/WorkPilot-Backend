-- Step 1: Add nullable status_id column
ALTER TABLE tasks ADD COLUMN status_id UUID;

-- Step 2: Ensure all projects have at least one custom status.
-- If any project has zero custom statuses (e.g. legacy projects), insert a default "Todo" status.
INSERT INTO custom_statuses (id, project_id, name, color, display_order, is_default, created_at, updated_at)
SELECT 
    gen_random_uuid(), 
    p.id, 
    'Todo', 
    '#808080', 
    0, 
    true, 
    NOW(), 
    NOW()
FROM projects p
WHERE NOT EXISTS (
    SELECT 1 FROM custom_statuses cs WHERE cs.project_id = p.id
);

-- Step 3: Populate status_id by matching lowercase status string to project custom status names
UPDATE tasks t
SET status_id = cs.id
FROM custom_statuses cs
WHERE t.project_id = cs.project_id AND LOWER(t.status) = LOWER(cs.name);

-- Step 4: Fallback 1: Default to the project's default status (with lowest display order)
UPDATE tasks t
SET status_id = (
    SELECT cs.id 
    FROM custom_statuses cs 
    WHERE cs.project_id = t.project_id AND cs.is_default = true 
    ORDER BY cs.display_order ASC 
    LIMIT 1
)
WHERE t.status_id IS NULL;

-- Step 5: Fallback 2: Default to ANY status in that project if no status is marked default
UPDATE tasks t
SET status_id = (
    SELECT cs.id 
    FROM custom_statuses cs 
    WHERE cs.project_id = t.project_id 
    ORDER BY cs.display_order ASC 
    LIMIT 1
)
WHERE t.status_id IS NULL;

-- Step 6: Migration Check: Ensure no status_id remains NULL before setting NOT NULL
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM tasks WHERE status_id IS NULL) THEN
        RAISE EXCEPTION 'Migration failed: Some tasks still have NULL status_id';
    END IF;
END $$;

-- Step 7: Make status_id NOT NULL and add foreign key constraint + index
ALTER TABLE tasks ALTER COLUMN status_id SET NOT NULL;
ALTER TABLE tasks ADD CONSTRAINT fk_tasks_status FOREIGN KEY (status_id) REFERENCES custom_statuses(id) ON DELETE RESTRICT;
CREATE INDEX idx_tasks_status_id ON tasks(status_id);
