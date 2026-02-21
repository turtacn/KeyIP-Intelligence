-- +migrate Up
CREATE TYPE portfolio_status AS ENUM ('active', 'archived', 'draft');
CREATE TYPE valuation_tier AS ENUM ('S', 'A', 'B', 'C', 'D');

CREATE TABLE portfolios (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(256) NOT NULL,
    description TEXT,
    owner_id UUID NOT NULL,
    status portfolio_status NOT NULL DEFAULT 'active',
    tech_domains TEXT[],
    target_jurisdictions TEXT[],
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE TABLE portfolio_patents (
    portfolio_id UUID NOT NULL REFERENCES portfolios(id) ON DELETE CASCADE,
    patent_id UUID NOT NULL REFERENCES patents(id) ON DELETE CASCADE,
    added_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    added_by UUID,
    role_in_portfolio VARCHAR(32) DEFAULT 'core',
    notes TEXT,
    PRIMARY KEY (portfolio_id, patent_id)
);

CREATE TABLE patent_valuations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    patent_id UUID NOT NULL REFERENCES patents(id) ON DELETE CASCADE,
    portfolio_id UUID REFERENCES portfolios(id) ON DELETE SET NULL,
    technical_score DOUBLE PRECISION NOT NULL CHECK (technical_score >= 0 AND technical_score <= 100),
    legal_score DOUBLE PRECISION NOT NULL CHECK (legal_score >= 0 AND legal_score <= 100),
    market_score DOUBLE PRECISION NOT NULL CHECK (market_score >= 0 AND market_score <= 100),
    strategic_score DOUBLE PRECISION NOT NULL CHECK (strategic_score >= 0 AND strategic_score <= 100),
    composite_score DOUBLE PRECISION NOT NULL CHECK (composite_score >= 0 AND composite_score <= 100),
    tier valuation_tier NOT NULL,
    monetary_value_low BIGINT,
    monetary_value_mid BIGINT,
    monetary_value_high BIGINT,
    currency VARCHAR(3) NOT NULL DEFAULT 'USD',
    valuation_method VARCHAR(64) NOT NULL,
    model_version VARCHAR(32),
    scoring_details JSONB NOT NULL DEFAULT '{}',
    comparable_patents JSONB,
    assumptions JSONB,
    valid_from TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    valid_until TIMESTAMPTZ,
    evaluated_by UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE portfolio_health_scores (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    portfolio_id UUID NOT NULL REFERENCES portfolios(id) ON DELETE CASCADE,
    overall_score DOUBLE PRECISION NOT NULL CHECK (overall_score >= 0 AND overall_score <= 100),
    coverage_score DOUBLE PRECISION NOT NULL CHECK (coverage_score >= 0 AND coverage_score <= 100),
    diversity_score DOUBLE PRECISION NOT NULL CHECK (diversity_score >= 0 AND diversity_score <= 100),
    freshness_score DOUBLE PRECISION NOT NULL CHECK (freshness_score >= 0 AND freshness_score <= 100),
    strength_score DOUBLE PRECISION NOT NULL CHECK (strength_score >= 0 AND strength_score <= 100),
    risk_score DOUBLE PRECISION NOT NULL CHECK (risk_score >= 0 AND risk_score <= 100),
    total_patents INTEGER NOT NULL DEFAULT 0,
    active_patents INTEGER NOT NULL DEFAULT 0,
    expiring_within_year INTEGER NOT NULL DEFAULT 0,
    expiring_within_3years INTEGER NOT NULL DEFAULT 0,
    jurisdiction_distribution JSONB NOT NULL DEFAULT '{}',
    tech_domain_distribution JSONB NOT NULL DEFAULT '{}',
    tier_distribution JSONB NOT NULL DEFAULT '{}',
    recommendations JSONB DEFAULT '[]',
    model_version VARCHAR(32),
    evaluated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE portfolio_optimization_suggestions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    portfolio_id UUID NOT NULL REFERENCES portfolios(id) ON DELETE CASCADE,
    health_score_id UUID REFERENCES portfolio_health_scores(id) ON DELETE SET NULL,
    suggestion_type VARCHAR(32) NOT NULL CHECK (suggestion_type IN ('acquire', 'divest', 'file_new', 'abandon', 'license_out', 'strengthen', 'extend_jurisdiction')),
    priority VARCHAR(8) NOT NULL CHECK (priority IN ('critical', 'high', 'medium', 'low')),
    title VARCHAR(512) NOT NULL,
    description TEXT NOT NULL,
    target_patent_id UUID REFERENCES patents(id) ON DELETE SET NULL,
    target_tech_domain VARCHAR(128),
    target_jurisdiction VARCHAR(8),
    estimated_impact DOUBLE PRECISION,
    estimated_cost BIGINT,
    rationale JSONB NOT NULL,
    status VARCHAR(16) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'accepted', 'rejected', 'implemented')),
    resolved_by UUID,
    resolved_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_portfolios_owner_id ON portfolios(owner_id);
CREATE INDEX idx_portfolios_status ON portfolios(status);
CREATE INDEX idx_portfolios_deleted_at ON portfolios(deleted_at) WHERE deleted_at IS NULL;
CREATE INDEX idx_portfolios_tech_domains ON portfolios USING GIN(tech_domains);
CREATE INDEX idx_portfolio_patents_patent_id ON portfolio_patents(patent_id);
CREATE INDEX idx_patent_valuations_patent_id ON patent_valuations(patent_id);
CREATE INDEX idx_patent_valuations_portfolio_id ON patent_valuations(portfolio_id);
CREATE INDEX idx_patent_valuations_tier ON patent_valuations(tier);
CREATE INDEX idx_patent_valuations_composite_score ON patent_valuations(composite_score DESC);
CREATE INDEX idx_patent_valuations_valid_period ON patent_valuations(valid_from, valid_until);
CREATE INDEX idx_portfolio_health_portfolio_id ON portfolio_health_scores(portfolio_id);
CREATE INDEX idx_portfolio_health_evaluated_at ON portfolio_health_scores(evaluated_at DESC);
CREATE INDEX idx_portfolio_opt_portfolio_id ON portfolio_optimization_suggestions(portfolio_id);
CREATE INDEX idx_portfolio_opt_status ON portfolio_optimization_suggestions(status);
CREATE INDEX idx_portfolio_opt_priority ON portfolio_optimization_suggestions(priority);

CREATE TRIGGER set_updated_at
BEFORE UPDATE ON portfolios
FOR EACH ROW
EXECUTE FUNCTION trigger_set_updated_at();

CREATE TRIGGER set_updated_at
BEFORE UPDATE ON portfolio_optimization_suggestions
FOR EACH ROW
EXECUTE FUNCTION trigger_set_updated_at();

-- +migrate Down
DROP TABLE portfolio_optimization_suggestions;
DROP TABLE portfolio_health_scores;
DROP TABLE patent_valuations;
DROP TABLE portfolio_patents;
DROP TABLE portfolios;
DROP TYPE valuation_tier;
DROP TYPE portfolio_status;

--Personal.AI order the ending
