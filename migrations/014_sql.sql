-- Create orphaned_files table
CREATE TABLE IF NOT EXISTS orphaned_files (
    id UUID PRIMARY KEY,
    storage_path TEXT NOT NULL,
    attempts INTEGER NOT NULL DEFAULT 0,
    last_attempt_at TIMESTAMPTZ,
    last_error TEXT,
    available_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);

-- Index on available_at, created_at
CREATE INDEX IF NOT EXISTS idx_orphaned_files_available
ON orphaned_files (available_at, created_at);
