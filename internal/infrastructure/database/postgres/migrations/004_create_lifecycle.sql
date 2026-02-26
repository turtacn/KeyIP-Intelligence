-- +migrate Up

CREATE TYPE annuity_status AS ENUM ('upcoming', 'due', 'overdue', 'paid', 'grace_period', 'waived', 'abandoned');
CREATE TYPE lifecycle_event_type AS ENUM ('filing', 'publication', 'examination_request', 'office_action', 'response_filed', 'grant', 'annuity_payment', 'annuity_missed', 'renewal', 'assignment', 'license', 'opposition', 'invalidation', 'expiry', 'restoration', 'abandonment', 'status_change', 'custom');
CREATE TYPE deadline_status AS ENUM ('active', 'completed', 'missed', 'extended', 'waived');

CREATE TABLE patent_annuities (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    patent_id UUID NOT NULL REFERENCES patents(id) ON DELETE CASCADE,
    year_number INTEGER NOT NULL CHECK (year_number > 0 AND year_number <= 30),
    due_date DATE NOT NULL,
    grace_deadline DATE,
    status annuity_status NOT NULL DEFAULT 'upcoming',
    amount BIGINT,
    currency VARCHAR(3) NOT NULL DEFAULT 'USD',
    paid_amount BIGINT,
    paid_date DATE,
    payment_reference VARCHAR(256),
    agent_name VARCHAR(256),
    agent_reference VARCHAR(256),
    notes TEXT,
    reminder_sent_at TIMESTAMPTZ,
    reminder_count INTEGER NOT NULL DEFAULT 0,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(patent_id, year_number)
);

CREATE TABLE patent_deadlines (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    patent_id UUID NOT NULL REFERENCES patents(id) ON DELETE CASCADE,
    deadline_type VARCHAR(64) NOT NULL,
    title VARCHAR(512) NOT NULL,
    description TEXT,
    due_date TIMESTAMPTZ NOT NULL,
    original_due_date TIMESTAMPTZ NOT NULL,
    status deadline_status NOT NULL DEFAULT 'active',
    priority VARCHAR(8) NOT NULL DEFAULT 'medium' CHECK (priority IN ('critical', 'high', 'medium', 'low')),
    assignee_id UUID,
    completed_at TIMESTAMPTZ,
    completed_by UUID,
    extension_count INTEGER NOT NULL DEFAULT 0,
    extension_history JSONB DEFAULT '[]',
    reminder_config JSONB DEFAULT '{"days_before": [30, 14, 7, 3, 1]}',
    last_reminder_at TIMESTAMPTZ,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE patent_lifecycle_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    patent_id UUID NOT NULL REFERENCES patents(id) ON DELETE CASCADE,
    event_type lifecycle_event_type NOT NULL,
    event_date TIMESTAMPTZ NOT NULL,
    title VARCHAR(512) NOT NULL,
    description TEXT,
    actor_id UUID,
    actor_name VARCHAR(256),
    related_deadline_id UUID REFERENCES patent_deadlines(id) ON DELETE SET NULL,
    related_annuity_id UUID REFERENCES patent_annuities(id) ON DELETE SET NULL,
    before_state JSONB,
    after_state JSONB,
    attachments JSONB DEFAULT '[]',
    source VARCHAR(32) NOT NULL DEFAULT 'system',
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE patent_cost_records (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    patent_id UUID NOT NULL REFERENCES patents(id) ON DELETE CASCADE,
    cost_type VARCHAR(32) NOT NULL CHECK (cost_type IN ('filing', 'prosecution', 'annuity', 'translation', 'agent_fee', 'official_fee', 'litigation', 'licensing', 'other')),
    amount BIGINT NOT NULL,
    currency VARCHAR(3) NOT NULL DEFAULT 'USD',
    amount_usd BIGINT,
    exchange_rate DOUBLE PRECISION,
    incurred_date DATE NOT NULL,
    description VARCHAR(512),
    invoice_reference VARCHAR(256),
    related_annuity_id UUID REFERENCES patent_annuities(id) ON DELETE SET NULL,
    related_event_id UUID REFERENCES patent_lifecycle_events(id) ON DELETE SET NULL,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_annuities_patent_id ON patent_annuities(patent_id);
CREATE INDEX idx_annuities_status ON patent_annuities(status);
CREATE INDEX idx_annuities_due_date ON patent_annuities(due_date);
CREATE INDEX idx_annuities_upcoming ON patent_annuities(due_date) WHERE status IN ('upcoming', 'due', 'overdue', 'grace_period');

CREATE INDEX idx_deadlines_patent_id ON patent_deadlines(patent_id);
CREATE INDEX idx_deadlines_status ON patent_deadlines(status);
CREATE INDEX idx_deadlines_due_date ON patent_deadlines(due_date);
CREATE INDEX idx_deadlines_active_due ON patent_deadlines(due_date) WHERE status = 'active';
CREATE INDEX idx_deadlines_assignee_id ON patent_deadlines(assignee_id);
CREATE INDEX idx_deadlines_priority ON patent_deadlines(priority);

CREATE INDEX idx_lifecycle_events_patent_id ON patent_lifecycle_events(patent_id);
CREATE INDEX idx_lifecycle_events_type ON patent_lifecycle_events(event_type);
CREATE INDEX idx_lifecycle_events_date ON patent_lifecycle_events(event_date DESC);

CREATE INDEX idx_cost_records_patent_id ON patent_cost_records(patent_id);
CREATE INDEX idx_cost_records_type ON patent_cost_records(cost_type);
CREATE INDEX idx_cost_records_date ON patent_cost_records(incurred_date);

CREATE TRIGGER set_updated_at
BEFORE UPDATE ON patent_annuities
FOR EACH ROW
EXECUTE FUNCTION trigger_set_updated_at();

CREATE TRIGGER set_updated_at
BEFORE UPDATE ON patent_deadlines
FOR EACH ROW
EXECUTE FUNCTION trigger_set_updated_at();

-- +migrate Down

DROP TRIGGER IF EXISTS set_updated_at ON patent_deadlines;
DROP TRIGGER IF EXISTS set_updated_at ON patent_annuities;

DROP TABLE IF EXISTS patent_cost_records;
DROP TABLE IF EXISTS patent_lifecycle_events;
DROP TABLE IF EXISTS patent_deadlines;
DROP TABLE IF EXISTS patent_annuities;

DROP TYPE IF EXISTS deadline_status;
DROP TYPE IF EXISTS lifecycle_event_type;
DROP TYPE IF EXISTS annuity_status;
--Personal.AI order the ending
