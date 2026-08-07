-- Drop unique constraints and indexes on sprints to allow duplicate names and date ranges
ALTER TABLE sprints DROP CONSTRAINT IF EXISTS uq_sprints_name;
DROP INDEX IF EXISTS idx_project_sprint_name;
