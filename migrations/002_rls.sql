ALTER TABLE users ENABLE ROW LEVEL SECURITY;
ALTER TABLE users FORCE ROW LEVEL SECURITY;

DROP POLICY IF EXISTS tenant_isolation_users ON users;
CREATE POLICY tenant_isolation_users ON users
    USING (
        organization_id = current_setting('app.current_org_id', true)::uuid
        OR current_setting('app.rls_bypass_reason', true) = 'auth_lookup'
    )
    WITH CHECK (
        organization_id = current_setting('app.current_org_id', true)::uuid
        OR current_setting('app.rls_bypass_reason', true) = 'auth_lookup'
    );

-- every query now carries an organization_id predicate, so index for it
-- (idx_users_organization_id already exists from 001_sql.sql)
CREATE INDEX IF NOT EXISTS idx_users_org_active
ON users (organization_id, is_active);