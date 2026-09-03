-- Migration to drop legacy 'role' columns and set NOT NULL constraint on 'role_id'

-- 1. organization_invitations table cleanup
ALTER TABLE organization_invitations DROP COLUMN IF EXISTS role;
ALTER TABLE organization_invitations ALTER COLUMN role_id SET NOT NULL;

-- 2. users table cleanup
ALTER TABLE users DROP COLUMN IF EXISTS role;
