-- PostgreSQL schema for Organization, User, and RefreshToken

CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE organizations (
    id UUID PRIMARY KEY,
 
    -- Basic Info
    name VARCHAR(150) NOT NULL,
    slug VARCHAR(100) NOT NULL UNIQUE,
    industry VARCHAR(100),
 
    -- Branding
    logo_url TEXT,
 
    -- Organization Settings
    domain VARCHAR(255),
 
    -- Team Information
    team_size VARCHAR(20),
    country VARCHAR(100),
    timezone VARCHAR(50) DEFAULT 'UTC',
 
    -- Status
    status VARCHAR(20) DEFAULT 'active',
 
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ,
    deleted_at TIMESTAMPTZ
);

ALTER TABLE organizations ADD CONSTRAINT uq_organizations_name UNIQUE (name);
ALTER TABLE organizations ADD CONSTRAINT uq_organizations_domain UNIQUE (domain);

CREATE UNIQUE INDEX uq_org_slugON organizations(slug);
CREATE INDEX idx_org_slugON organizations(slug);
CREATE INDEX idx_organization_name ON organizations(name);
CREATE INDEX idx_organization_deleted_at ON organizations(deleted_at);

CREATE TYPE user_role AS ENUM (
    'super_admin',
    'org_admin',
    'project_manager',
    'developer',
    'viewer'
);

CREATE TABLE users (
    id UUID PRIMARY KEY,
    organization_id UUID,
    full_name VARCHAR(100) NOT NULL,
    username VARCHAR(30) NOT NULL,
    email VARCHAR(100) NOT NULL,
    password_hash TEXT NOT NULL,
    role user_role NOT NULL DEFAULT 'developer',
    avatar_url VARCHAR(255),
    timezone VARCHAR(50) DEFAULT 'UTC',
    is_active BOOLEAN DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ,
    deleted_at TIMESTAMPTZ,
    CONSTRAINT fk_users_organization FOREIGN KEY (organization_id)
        REFERENCES organizations(id) ON DELETE SET NULL
);

ALTER TABLE users ADD CONSTRAINT uq_users_username UNIQUE(username);
ALTER TABLE users ADD CONSTRAINT uq_users_email UNIQUE(email);

CREATE INDEX idx_users_organization_id ON users(organization_id);
CREATE INDEX idx_users_username ON users(username);
CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_role ON users(role);
CREATE INDEX idx_users_deleted_at ON users(deleted_at);

CREATE TABLE refresh_tokens (
    id UUID PRIMARY KEY,
    user_id UUID NOT NULL,
    token_hash VARCHAR(255) NOT NULL,
    user_agent TEXT,
    ip_address VARCHAR(45),
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ,
    deleted_at TIMESTAMPTZ,
    CONSTRAINT fk_refresh_tokens_user FOREIGN KEY (user_id)
        REFERENCES users(id) ON DELETE CASCADE
);

ALTER TABLE refresh_tokens ADD CONSTRAINT uq_refresh_tokens_user UNIQUE(user_id);
ALTER TABLE refresh_tokens ADD CONSTRAINT uq_refresh_tokens_token_hash UNIQUE(token_hash);

CREATE INDEX idx_refresh_tokens_user_id ON refresh_tokens(user_id);
CREATE INDEX idx_refresh_tokens_deleted_at ON refresh_tokens(deleted_at);