-- Add project_id column to audit_logs for project-level activity tracking
ALTER TABLE audit_logs ADD COLUMN IF NOT EXISTS project_id UUID;

CREATE INDEX IF NOT EXISTS idx_audit_logs_project_id
    ON audit_logs (project_id);

CREATE INDEX IF NOT EXISTS idx_audit_logs_resource_type
    ON audit_logs (resource_type);
