-- Migration to add color field to users table

-- Step 1: Add the color column as nullable first
ALTER TABLE users ADD COLUMN IF NOT EXISTS color VARCHAR(7);

-- Step 2: Assign random color to existing users who don't have one
UPDATE users
SET color = '#' || upper(substring(md5(random()::text) from 1 for 6))
WHERE color IS NULL OR color = '';

-- Step 3: Set NOT NULL constraint and default value
ALTER TABLE users ALTER COLUMN color SET NOT NULL;
ALTER TABLE users ALTER COLUMN color SET DEFAULT '#3498DB';
