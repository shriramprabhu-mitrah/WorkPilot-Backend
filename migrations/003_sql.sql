-- organization_invitations
CREATE TABLE organization_invitations (
    id UUID PRIMARY KEY,
    organization_id UUID NOT NULL,
    email VARCHAR(100) NOT NULL,
    role VARCHAR(30) NOT NULL,
    token VARCHAR(255) NOT NULL,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    expires_at TIMESTAMPTZ NOT NULL,
    accepted_at TIMESTAMPTZ,
    created_by UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ,
    deleted_at TIMESTAMPTZ,

    CONSTRAINT fk_organization_invitations_organization
        FOREIGN KEY (organization_id)
        REFERENCES organizations(id)
        ON UPDATE CASCADE
        ON DELETE RESTRICT,

    CONSTRAINT uq_organization_invitations_token
        UNIQUE (token)
);

CREATE INDEX idx_org_invites_org_id
    ON organization_invitations (organization_id);

CREATE INDEX idx_org_invites_email
    ON organization_invitations (email);

CREATE INDEX idx_org_invites_token
    ON organization_invitations (token);

CREATE INDEX idx_org_invites_deleted_at
    ON organization_invitations (deleted_at);


-- audit_logs
CREATE TABLE audit_logs (
    id UUID PRIMARY KEY,
    user_id UUID,
    organization_id UUID,
    action VARCHAR(100) NOT NULL,
    resource_type VARCHAR(50) NOT NULL,
    resource_id VARCHAR(255),
    details TEXT,
    created_at TIMESTAMPTZ NOT NULL,

    CONSTRAINT fk_audit_logs_user
        FOREIGN KEY (user_id)
        REFERENCES users(id)
        ON UPDATE CASCADE
        ON DELETE SET NULL,

    CONSTRAINT fk_audit_logs_organization
        FOREIGN KEY (organization_id)
        REFERENCES organizations(id)
        ON UPDATE CASCADE
        ON DELETE SET NULL
);

CREATE INDEX idx_audit_logs_user_id
    ON audit_logs (user_id);

CREATE INDEX idx_audit_logs_org_id
    ON audit_logs (organization_id);

CREATE INDEX idx_audit_logs_action
    ON audit_logs (action);