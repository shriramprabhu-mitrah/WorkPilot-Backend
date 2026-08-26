-- Step 1: Add sequence_number and key columns to user_stories
ALTER TABLE user_stories ADD COLUMN IF NOT EXISTS sequence_number INTEGER;
ALTER TABLE user_stories ADD COLUMN IF NOT EXISTS key VARCHAR(50);

-- Step 2: Populate sequence_number and key for existing records partitioned by project_id
WITH numbered_stories AS (
    SELECT id,
           row_number() OVER (PARTITION BY project_id ORDER BY created_at, id) as seq
    FROM user_stories
)
UPDATE user_stories us
SET sequence_number = ns.seq,
    key = 'US-' || ns.seq
FROM numbered_stories ns
WHERE us.id = ns.id AND (us.key IS NULL OR us.key = '');

-- Step 3: Set columns to NOT NULL after populating
ALTER TABLE user_stories ALTER COLUMN sequence_number SET NOT NULL;
ALTER TABLE user_stories ALTER COLUMN key SET NOT NULL;

-- Step 4: Create unique composite index for active user stories per project
CREATE UNIQUE INDEX IF NOT EXISTS idx_project_user_story_key ON user_stories(project_id, key) WHERE deleted_at IS NULL;
