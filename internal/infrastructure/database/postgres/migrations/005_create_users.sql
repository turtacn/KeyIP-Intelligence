-- +migrate Up

CREATE TYPE user_status AS ENUM ('active', 'inactive', 'suspended', 'pending_verification');

CREATE TABLE users (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email VARCHAR(320) NOT NULL UNIQUE,
    username VARCHAR(64) NOT NULL UNIQUE,
    display_name VARCHAR(256) NOT NULL,
    password_hash VARCHAR(256),
    status user_status NOT NULL DEFAULT 'pending_verification',
    avatar_url VARCHAR(1024),
    locale VARCHAR(16) NOT NULL DEFAULT 'zh-CN',
    timezone VARCHAR(64) NOT NULL DEFAULT 'Asia/Shanghai',
    last_login_at TIMESTAMPTZ,
    last_login_ip VARCHAR(45),
    login_count INTEGER NOT NULL DEFAULT 0,
    failed_login_count INTEGER NOT NULL DEFAULT 0,
    locked_until TIMESTAMPTZ,
    email_verified_at TIMESTAMPTZ,
    mfa_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    mfa_secret VARCHAR(256),
    preferences JSONB DEFAULT '{}',
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE TABLE organizations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(256) NOT NULL,
    slug VARCHAR(128) NOT NULL UNIQUE,
    description TEXT,
    logo_url VARCHAR(1024),
    plan VARCHAR(32) NOT NULL DEFAULT 'free' CHECK (plan IN ('free', 'starter', 'professional', 'enterprise')),
    max_members INTEGER NOT NULL DEFAULT 5,
    max_patents INTEGER NOT NULL DEFAULT 100,
    settings JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE TABLE organization_members (
    organization_id UUID NOT NULL REFERENCES organizations(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role VARCHAR(32) NOT NULL DEFAULT 'member' CHECK (role IN ('owner', 'admin', 'member', 'viewer', 'billing')),
    joined_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    invited_by UUID REFERENCES users(id) ON DELETE SET NULL,
    PRIMARY KEY (organization_id, user_id)
);

CREATE TABLE roles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(64) NOT NULL UNIQUE,
    display_name VARCHAR(128) NOT NULL,
    description TEXT,
    is_system BOOLEAN NOT NULL DEFAULT FALSE,
    permissions JSONB NOT NULL DEFAULT '[]',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE user_roles (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role_id UUID NOT NULL REFERENCES roles(id) ON DELETE CASCADE,
    organization_id UUID REFERENCES organizations(id) ON DELETE CASCADE,
    granted_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    granted_by UUID REFERENCES users(id) ON DELETE SET NULL,
    PRIMARY KEY (user_id, role_id, organization_id)
);

CREATE TABLE api_keys (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    organization_id UUID REFERENCES organizations(id) ON DELETE CASCADE,
    name VARCHAR(128) NOT NULL,
    key_hash VARCHAR(256) NOT NULL UNIQUE,
    key_prefix VARCHAR(16) NOT NULL,
    scopes TEXT[] NOT NULL DEFAULT '{}',
    rate_limit INTEGER NOT NULL DEFAULT 1000,
    expires_at TIMESTAMPTZ,
    last_used_at TIMESTAMPTZ,
    last_used_ip VARCHAR(45),
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE audit_logs (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID REFERENCES users(id) ON DELETE SET NULL,
    organization_id UUID REFERENCES organizations(id) ON DELETE SET NULL,
    action VARCHAR(64) NOT NULL,
    resource_type VARCHAR(64) NOT NULL,
    resource_id VARCHAR(128),
    ip_address VARCHAR(45),
    user_agent VARCHAR(512),
    request_id VARCHAR(64),
    before_state JSONB,
    after_state JSONB,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Add foreign keys for tables created in previous migrations
ALTER TABLE patents ADD CONSTRAINT fk_patents_assignee FOREIGN KEY (assignee_id) REFERENCES users(id) ON DELETE SET NULL;
ALTER TABLE portfolios ADD CONSTRAINT fk_portfolios_owner FOREIGN KEY (owner_id) REFERENCES users(id) ON DELETE RESTRICT;

CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_username ON users(username);
CREATE INDEX idx_users_status ON users(status);
CREATE INDEX idx_users_deleted_at ON users(deleted_at) WHERE deleted_at IS NULL;

CREATE INDEX idx_orgs_slug ON organizations(slug);
CREATE INDEX idx_orgs_deleted_at ON organizations(deleted_at) WHERE deleted_at IS NULL;

CREATE INDEX idx_org_members_user_id ON organization_members(user_id);

CREATE INDEX idx_user_roles_role_id ON user_roles(role_id);
CREATE INDEX idx_user_roles_org_id ON user_roles(organization_id);

CREATE INDEX idx_api_keys_user_id ON api_keys(user_id);
CREATE INDEX idx_api_keys_key_hash ON api_keys(key_hash);
CREATE INDEX idx_api_keys_active ON api_keys(is_active) WHERE is_active = TRUE;

CREATE INDEX idx_audit_logs_user_id ON audit_logs(user_id);
CREATE INDEX idx_audit_logs_org_id ON audit_logs(organization_id);
CREATE INDEX idx_audit_logs_action ON audit_logs(action);
CREATE INDEX idx_audit_logs_resource ON audit_logs(resource_type, resource_id);
CREATE INDEX idx_audit_logs_created_at ON audit_logs(created_at DESC);

-- Seed roles
INSERT INTO roles (name, display_name, description, is_system, permissions) VALUES
('super_admin', '超级管理员', '系统最高权限', TRUE, '["*"]'),
('org_admin', '组织管理员', '组织级管理权限', TRUE, '["org:*", "patent:*", "portfolio:*", "molecule:*", "user:read", "user:invite"]'),
('patent_analyst', '专利分析师', '专利分析与估值权限', TRUE, '["patent:read", "patent:write", "patent:analyze", "portfolio:read", "portfolio:analyze", "molecule:read"]'),
('researcher', '研究员', '分子研究与专利检索权限', TRUE, '["patent:read", "patent:search", "molecule:*", "portfolio:read"]'),
('viewer', '只读用户', '只读访问权限', TRUE, '["patent:read", "portfolio:read", "molecule:read"]');

CREATE TRIGGER set_updated_at
BEFORE UPDATE ON users
FOR EACH ROW
EXECUTE FUNCTION trigger_set_updated_at();

CREATE TRIGGER set_updated_at
BEFORE UPDATE ON organizations
FOR EACH ROW
EXECUTE FUNCTION trigger_set_updated_at();

CREATE TRIGGER set_updated_at
BEFORE UPDATE ON api_keys
FOR EACH ROW
EXECUTE FUNCTION trigger_set_updated_at();

-- +migrate Down

DROP TRIGGER IF EXISTS set_updated_at ON api_keys;
DROP TRIGGER IF EXISTS set_updated_at ON organizations;
DROP TRIGGER IF EXISTS set_updated_at ON users;

ALTER TABLE portfolios DROP CONSTRAINT IF EXISTS fk_portfolios_owner;
ALTER TABLE patents DROP CONSTRAINT IF EXISTS fk_patents_assignee;

DROP TABLE IF EXISTS audit_logs;
DROP TABLE IF EXISTS api_keys;
DROP TABLE IF EXISTS user_roles;
DROP TABLE IF EXISTS roles;
DROP TABLE IF EXISTS organization_members;
DROP TABLE IF EXISTS organizations;
DROP TABLE IF EXISTS users;

DROP TYPE IF EXISTS user_status;
--Personal.AI order the ending
