-- PostgreSQL schema migration for configurable roles and permissions system

-- Step 1: Create roles table
CREATE TABLE IF NOT EXISTS roles (
    id UUID PRIMARY KEY,
    organization_id UUID REFERENCES organizations(id) ON DELETE CASCADE,
    name VARCHAR(100) NOT NULL,
    description TEXT,
    is_system BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX IF NOT EXISTS uq_roles_system_name ON roles (LOWER(name)) WHERE organization_id IS NULL AND deleted_at IS NULL;
CREATE UNIQUE INDEX IF NOT EXISTS uq_roles_org_name ON roles (organization_id, LOWER(name)) WHERE organization_id IS NOT NULL AND deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_roles_org_id ON roles(organization_id);
CREATE INDEX IF NOT EXISTS idx_roles_deleted_at ON roles(deleted_at);

-- Step 2: Create permissions table
CREATE TABLE IF NOT EXISTS permissions (
    id UUID PRIMARY KEY,
    resource VARCHAR(50) NOT NULL,
    action VARCHAR(50) NOT NULL,
    CONSTRAINT uq_permissions_res_act UNIQUE (resource, action)
);

ALTER TABLE permissions DROP CONSTRAINT IF EXISTS uq_permissions_res_act;
ALTER TABLE permissions ADD CONSTRAINT uq_permissions_res_act UNIQUE (resource, action);

-- Step 3: Create role_permissions table
CREATE TABLE IF NOT EXISTS role_permissions (
    role_id UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    permission_id UUID NOT NULL REFERENCES permissions(id) ON DELETE CASCADE,
    PRIMARY KEY (role_id, permission_id)
);

-- Step 4: Seed permissions
INSERT INTO permissions (id, resource, action) VALUES
(gen_random_uuid(), 'projects', 'view'),
(gen_random_uuid(), 'projects', 'add'),
(gen_random_uuid(), 'projects', 'modify'),
(gen_random_uuid(), 'projects', 'delete'),

(gen_random_uuid(), 'sprints', 'view'),
(gen_random_uuid(), 'sprints', 'add'),
(gen_random_uuid(), 'sprints', 'modify'),
(gen_random_uuid(), 'sprints', 'delete'),

(gen_random_uuid(), 'user_stories', 'view'),
(gen_random_uuid(), 'user_stories', 'add'),
(gen_random_uuid(), 'user_stories', 'modify'),
(gen_random_uuid(), 'user_stories', 'delete'),

(gen_random_uuid(), 'tasks', 'view'),
(gen_random_uuid(), 'tasks', 'add'),
(gen_random_uuid(), 'tasks', 'modify'),
(gen_random_uuid(), 'tasks', 'delete'),

(gen_random_uuid(), 'comments', 'view'),
(gen_random_uuid(), 'comments', 'add'),
(gen_random_uuid(), 'comments', 'modify'),
(gen_random_uuid(), 'comments', 'delete'),
(gen_random_uuid(), 'comments', 'comment')
ON CONFLICT (resource, action) DO NOTHING;

-- Step 5: Seed default system roles
INSERT INTO roles (id, name, description, is_system, created_at, updated_at)
SELECT gen_random_uuid(), 'super_admin', 'System administrator with global access', true, NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM roles WHERE name = 'super_admin' AND organization_id IS NULL AND deleted_at IS NULL);

INSERT INTO roles (id, name, description, is_system, created_at, updated_at)
SELECT gen_random_uuid(), 'org_admin', 'Organization administrator with full access to organization resources', true, NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM roles WHERE name = 'org_admin' AND organization_id IS NULL AND deleted_at IS NULL);

INSERT INTO roles (id, name, description, is_system, created_at, updated_at)
SELECT gen_random_uuid(), 'project_manager', 'Project manager with access to manage projects, sprints, and team activities', true, NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM roles WHERE name = 'project_manager' AND organization_id IS NULL AND deleted_at IS NULL);

INSERT INTO roles (id, name, description, is_system, created_at, updated_at)
SELECT gen_random_uuid(), 'developer', 'Software developer with access to view and modify user stories and tasks', true, NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM roles WHERE name = 'developer' AND organization_id IS NULL AND deleted_at IS NULL);

INSERT INTO roles (id, name, description, is_system, created_at, updated_at)
SELECT gen_random_uuid(), 'qa', 'Quality assurance engineer with access to manage issues and test tasks', true, NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM roles WHERE name = 'qa' AND organization_id IS NULL AND deleted_at IS NULL);

INSERT INTO roles (id, name, description, is_system, created_at, updated_at)
SELECT gen_random_uuid(), 'stakeholder', 'Read-only stakeholder with basic viewing and commenting privileges', true, NOW(), NOW()
WHERE NOT EXISTS (SELECT 1 FROM roles WHERE name = 'stakeholder' AND organization_id IS NULL AND deleted_at IS NULL);

-- Step 6: Seed role_permissions for system roles

-- org_admin gets everything
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r, permissions p
WHERE r.name = 'org_admin' AND r.organization_id IS NULL
ON CONFLICT DO NOTHING;

-- project_manager gets all except projects:add and projects:delete
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r, permissions p
WHERE r.name = 'project_manager' AND r.organization_id IS NULL
  AND NOT (p.resource = 'projects' AND p.action IN ('add', 'delete'))
ON CONFLICT DO NOTHING;

-- developer permissions
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r, permissions p
WHERE r.name = 'developer' AND r.organization_id IS NULL
  AND (
    (p.resource = 'projects' AND p.action = 'view')
    OR (p.resource = 'sprints' AND p.action = 'view')
    OR (p.resource = 'user_stories' AND p.action IN ('view', 'add', 'modify'))
    OR (p.resource = 'tasks' AND p.action IN ('view', 'add', 'modify', 'delete'))
    OR (p.resource = 'comments' AND p.action IN ('view', 'add', 'modify', 'delete', 'comment'))
  )
ON CONFLICT DO NOTHING;

-- qa permissions
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r, permissions p
WHERE r.name = 'qa' AND r.organization_id IS NULL
  AND (
    (p.resource = 'projects' AND p.action = 'view')
    OR (p.resource = 'sprints' AND p.action = 'view')
    OR (p.resource = 'user_stories' AND p.action IN ('view', 'modify'))
    OR (p.resource = 'tasks' AND p.action IN ('view', 'add', 'modify'))
    OR (p.resource = 'comments' AND p.action IN ('view', 'add', 'comment'))
  )
ON CONFLICT DO NOTHING;

-- stakeholder permissions
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r, permissions p
WHERE r.name = 'stakeholder' AND r.organization_id IS NULL
  AND (
    (p.resource = 'projects' AND p.action = 'view')
    OR (p.resource = 'sprints' AND p.action = 'view')
    OR (p.resource = 'user_stories' AND p.action = 'view')
    OR (p.resource = 'tasks' AND p.action = 'view')
    OR (p.resource = 'comments' AND p.action IN ('view', 'comment'))
  )
ON CONFLICT DO NOTHING;


-- Step 7: Migrate existing users, project_members, and organization_invitations

-- 1. Migrate users table
-- 1. Migrate users table
ALTER TABLE users ADD COLUMN IF NOT EXISTS role_id UUID REFERENCES roles(id);

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 
        FROM information_schema.columns 
        WHERE table_name='users' AND column_name='role'
    ) THEN
        UPDATE users SET role_id = (SELECT id FROM roles WHERE name = 'super_admin' AND organization_id IS NULL LIMIT 1) WHERE role::text = 'super_admin';
        UPDATE users SET role_id = (SELECT id FROM roles WHERE name = 'org_admin' AND organization_id IS NULL LIMIT 1) WHERE role::text = 'org_admin';
        UPDATE users SET role_id = (SELECT id FROM roles WHERE name = 'developer' AND organization_id IS NULL LIMIT 1) WHERE role::text = 'member' OR role_id IS NULL;
        
        ALTER TABLE users DROP COLUMN role;
    END IF;
END $$;

-- Default fallback
UPDATE users SET role_id = (SELECT id FROM roles WHERE name = 'developer' AND organization_id IS NULL LIMIT 1) WHERE role_id IS NULL;

ALTER TABLE users ALTER COLUMN role_id SET NOT NULL;
CREATE INDEX IF NOT EXISTS idx_users_role_id ON users(role_id);
DROP TYPE IF EXISTS user_role;

-- 2. Migrate project_members table
ALTER TABLE project_members ADD COLUMN IF NOT EXISTS role_id UUID REFERENCES roles(id);

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 
        FROM information_schema.columns 
        WHERE table_name='project_members' AND column_name='project_role'
    ) THEN
        UPDATE project_members SET role_id = (SELECT id FROM roles WHERE name = 'org_admin' AND organization_id IS NULL LIMIT 1) WHERE project_role = 'org_admin';
        UPDATE project_members SET role_id = (SELECT id FROM roles WHERE name = 'project_manager' AND organization_id IS NULL LIMIT 1) WHERE project_role = 'project_manager';
        UPDATE project_members SET role_id = (SELECT id FROM roles WHERE name = 'developer' AND organization_id IS NULL LIMIT 1) WHERE project_role = 'developer';
        UPDATE project_members SET role_id = (SELECT id FROM roles WHERE name = 'qa' AND organization_id IS NULL LIMIT 1) WHERE project_role = 'tester';
        UPDATE project_members SET role_id = (SELECT id FROM roles WHERE name = 'stakeholder' AND organization_id IS NULL LIMIT 1) WHERE project_role = 'viewer';
        
        ALTER TABLE project_members DROP COLUMN project_role;
    END IF;
END $$;

-- Default fallback
UPDATE project_members SET role_id = (SELECT id FROM roles WHERE name = 'developer' AND organization_id IS NULL LIMIT 1) WHERE role_id IS NULL;

ALTER TABLE project_members ALTER COLUMN role_id SET NOT NULL;
CREATE INDEX IF NOT EXISTS idx_project_members_role_id ON project_members(role_id);

-- 3. Migrate organization_invitations table
ALTER TABLE organization_invitations ADD COLUMN IF NOT EXISTS role_id UUID REFERENCES roles(id);

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 
        FROM information_schema.columns 
        WHERE table_name='organization_invitations' AND column_name='role'
    ) THEN
        UPDATE organization_invitations SET role_id = (SELECT id FROM roles WHERE name = 'org_admin' AND organization_id IS NULL LIMIT 1) WHERE role = 'org_admin';
        UPDATE organization_invitations SET role_id = (SELECT id FROM roles WHERE name = 'developer' AND organization_id IS NULL LIMIT 1) WHERE role = 'member';
        
        ALTER TABLE organization_invitations DROP COLUMN role;
    END IF;
END $$;

-- Default fallback
UPDATE organization_invitations SET role_id = (SELECT id FROM roles WHERE name = 'developer' AND organization_id IS NULL LIMIT 1) WHERE role_id IS NULL;

ALTER TABLE organization_invitations ALTER COLUMN role_id SET NOT NULL;
CREATE INDEX IF NOT EXISTS idx_organization_invitations_role_id ON organization_invitations(role_id);
