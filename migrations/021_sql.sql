-- Add is_final column to custom_statuses table
ALTER TABLE custom_statuses ADD COLUMN IF NOT EXISTS is_final BOOLEAN NOT NULL DEFAULT FALSE;

-- Update existing custom statuses matching traditionally final status names to have is_final = true
UPDATE custom_statuses
SET is_final = TRUE
WHERE LOWER(name) IN ('completed', 'done', 'closed', 'cancelled', 'resolved');
