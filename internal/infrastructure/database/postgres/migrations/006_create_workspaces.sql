-- +migrate Up

CREATE TYPE workspace_type AS ENUM ('personal', 'team', 'project');
CREATE TYPE notification_type AS ENUM ('deadline_reminder', 'annuity_due', 'task_assigned', 'comment_mention', 'analysis_complete', 'portfolio_alert', 'system_announcement', 'invitation');

CREATE TABLE workspaces (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(256) NOT NULL,
    description TEXT,
    workspace_type workspace_type NOT NULL DEFAULT 'personal',
    organization_id UUID REFERENCES organizations(id) ON DELETE CASCADE,
    owner_id UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    settings JSONB DEFAULT '{}',
    icon VARCHAR(64),
    color VARCHAR(7),
    is_archived BOOLEAN NOT NULL DEFAULT FALSE,
    max_collaborators INTEGER NOT NULL DEFAULT 10,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE TABLE workspace_members (
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role VARCHAR(16) NOT NULL DEFAULT 'editor' CHECK (role IN ('owner', 'admin', 'editor', 'commenter', 'viewer')),
    joined_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    invited_by UUID REFERENCES users(id) ON DELETE SET NULL,
    last_accessed_at TIMESTAMPTZ,
    PRIMARY KEY (workspace_id, user_id)
);

CREATE TABLE workspace_projects (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name VARCHAR(256) NOT NULL,
    description TEXT,
    project_type VARCHAR(32) NOT NULL DEFAULT 'analysis' CHECK (project_type IN ('analysis', 'monitoring', 'valuation', 'landscape', 'freedom_to_operate', 'infringement', 'prior_art_search', 'custom')),
    status VARCHAR(16) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'completed', 'paused', 'archived')),
    lead_id UUID REFERENCES users(id) ON DELETE SET NULL,
    start_date DATE,
    target_date DATE,
    completed_at TIMESTAMPTZ,
    config JSONB DEFAULT '{}',
    results_summary JSONB,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE project_patents (
    project_id UUID NOT NULL REFERENCES workspace_projects(id) ON DELETE CASCADE,
    patent_id UUID NOT NULL REFERENCES patents(id) ON DELETE CASCADE,
    added_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    added_by UUID REFERENCES users(id) ON DELETE SET NULL,
    tags TEXT[] DEFAULT '{}',
    annotations JSONB DEFAULT '{}',
    analysis_status VARCHAR(16) DEFAULT 'pending' CHECK (analysis_status IN ('pending', 'in_progress', 'completed', 'skipped')),
    PRIMARY KEY (project_id, patent_id)
);

CREATE TABLE project_molecules (
    project_id UUID NOT NULL REFERENCES workspace_projects(id) ON DELETE CASCADE,
    molecule_id UUID NOT NULL REFERENCES molecules(id) ON DELETE CASCADE,
    added_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    added_by UUID REFERENCES users(id) ON DELETE SET NULL,
    tags TEXT[] DEFAULT '{}',
    annotations JSONB DEFAULT '{}',
    PRIMARY KEY (project_id, molecule_id)
);

CREATE TABLE comments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    author_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    resource_type VARCHAR(32) NOT NULL CHECK (resource_type IN ('patent', 'molecule', 'portfolio', 'project', 'claim', 'valuation')),
    resource_id UUID NOT NULL,
    parent_comment_id UUID REFERENCES comments(id) ON DELETE CASCADE,
    content TEXT NOT NULL,
    mentions UUID[] DEFAULT '{}',
    attachments JSONB DEFAULT '[]',
    is_resolved BOOLEAN NOT NULL DEFAULT FALSE,
    resolved_by UUID REFERENCES users(id) ON DELETE SET NULL,
    resolved_at TIMESTAMPTZ,
    edited_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE TABLE notifications (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    notification_type notification_type NOT NULL,
    title VARCHAR(512) NOT NULL,
    body TEXT,
    resource_type VARCHAR(32),
    resource_id UUID,
    action_url VARCHAR(1024),
    sender_id UUID REFERENCES users(id) ON DELETE SET NULL,
    is_read BOOLEAN NOT NULL DEFAULT FALSE,
    read_at TIMESTAMPTZ,
    is_email_sent BOOLEAN NOT NULL DEFAULT FALSE,
    email_sent_at TIMESTAMPTZ,
    priority VARCHAR(8) NOT NULL DEFAULT 'normal' CHECK (priority IN ('urgent', 'high', 'normal', 'low')),
    expires_at TIMESTAMPTZ,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE saved_searches (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    workspace_id UUID REFERENCES workspaces(id) ON DELETE CASCADE,
    name VARCHAR(256) NOT NULL,
    description TEXT,
    search_type VARCHAR(32) NOT NULL CHECK (search_type IN ('patent', 'molecule', 'portfolio', 'combined')),
    query_params JSONB NOT NULL,
    filters JSONB DEFAULT '{}',
    sort_config JSONB DEFAULT '{}',
    is_alert_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    alert_frequency VARCHAR(16) DEFAULT 'daily' CHECK (alert_frequency IN ('realtime', 'daily', 'weekly', 'monthly')),
    last_alert_at TIMESTAMPTZ,
    result_count INTEGER NOT NULL DEFAULT 0,
    last_executed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_workspaces_org_id ON workspaces(organization_id);
CREATE INDEX idx_workspaces_owner_id ON workspaces(owner_id);
CREATE INDEX idx_workspaces_type ON workspaces(workspace_type);
CREATE INDEX idx_workspaces_deleted_at ON workspaces(deleted_at) WHERE deleted_at IS NULL;

CREATE INDEX idx_ws_members_user_id ON workspace_members(user_id);

CREATE INDEX idx_ws_projects_workspace_id ON workspace_projects(workspace_id);
CREATE INDEX idx_ws_projects_status ON workspace_projects(status);
CREATE INDEX idx_ws_projects_lead_id ON workspace_projects(lead_id);

CREATE INDEX idx_project_patents_patent_id ON project_patents(patent_id);
CREATE INDEX idx_project_molecules_molecule_id ON project_molecules(molecule_id);

CREATE INDEX idx_comments_resource ON comments(resource_type, resource_id);
CREATE INDEX idx_comments_author_id ON comments(author_id);
CREATE INDEX idx_comments_parent_id ON comments(parent_comment_id);
CREATE INDEX idx_comments_deleted_at ON comments(deleted_at) WHERE deleted_at IS NULL;

CREATE INDEX idx_notifications_user_id ON notifications(user_id);
CREATE INDEX idx_notifications_unread ON notifications(user_id, is_read) WHERE is_read = FALSE;
CREATE INDEX idx_notifications_type ON notifications(notification_type);
CREATE INDEX idx_notifications_created_at ON notifications(created_at DESC);

CREATE INDEX idx_saved_searches_user_id ON saved_searches(user_id);
CREATE INDEX idx_saved_searches_workspace_id ON saved_searches(workspace_id);
CREATE INDEX idx_saved_searches_alert ON saved_searches(is_alert_enabled) WHERE is_alert_enabled = TRUE;

CREATE TRIGGER set_updated_at
BEFORE UPDATE ON workspaces
FOR EACH ROW
EXECUTE FUNCTION trigger_set_updated_at();

CREATE TRIGGER set_updated_at
BEFORE UPDATE ON workspace_projects
FOR EACH ROW
EXECUTE FUNCTION trigger_set_updated_at();

CREATE TRIGGER set_updated_at
BEFORE UPDATE ON comments
FOR EACH ROW
EXECUTE FUNCTION trigger_set_updated_at();

CREATE TRIGGER set_updated_at
BEFORE UPDATE ON saved_searches
FOR EACH ROW
EXECUTE FUNCTION trigger_set_updated_at();

-- +migrate Down

DROP TRIGGER IF EXISTS set_updated_at ON saved_searches;
DROP TRIGGER IF EXISTS set_updated_at ON comments;
DROP TRIGGER IF EXISTS set_updated_at ON workspace_projects;
DROP TRIGGER IF EXISTS set_updated_at ON workspaces;

DROP TABLE IF EXISTS saved_searches;
DROP TABLE IF EXISTS notifications;
DROP TABLE IF EXISTS comments;
DROP TABLE IF EXISTS project_molecules;
DROP TABLE IF EXISTS project_patents;
DROP TABLE IF EXISTS workspace_projects;
DROP TABLE IF EXISTS workspace_members;
DROP TABLE IF EXISTS workspaces;

DROP TYPE IF EXISTS notification_type;
DROP TYPE IF EXISTS workspace_type;
--Personal.AI order the ending
