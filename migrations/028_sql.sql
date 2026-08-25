ALTER TABLE projects ADD COLUMN IF NOT EXISTS slug VARCHAR(150);

-- Step 2: Populate slug for existing records using a native Postgres slugify query
WITH slugified AS (
    SELECT id, 
           COALESCE(NULLIF(trim(both '-' from regexp_replace(LOWER(name), '[^a-z0-9]+', '-', 'g')), ''), 'project') as base_slug
    FROM projects
),
numbered AS (
    SELECT id, 
           base_slug,
           row_number() OVER (PARTITION BY base_slug ORDER BY id) as rn
    FROM slugified
)
UPDATE projects p
SET slug = CASE 
    WHEN n.rn = 1 THEN n.base_slug 
    ELSE n.base_slug || '-' || (n.rn - 1)
END
FROM numbered n
WHERE p.id = n.id AND (p.slug IS NULL OR p.slug = '');

-- Step 3: Create unique index for active projects
CREATE UNIQUE INDEX IF NOT EXISTS idx_projects_slug ON projects(slug) WHERE deleted_at IS NULL;