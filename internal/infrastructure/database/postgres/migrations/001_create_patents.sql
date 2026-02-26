-- +migrate Up

-- Enable extensions
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pgcrypto";
CREATE EXTENSION IF NOT EXISTS "vector";

-- Create trigger function for updated_at
CREATE OR REPLACE FUNCTION trigger_set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
  NEW.updated_at = NOW();
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Create ENUM types
CREATE TYPE patent_status AS ENUM ('draft', 'filed', 'published', 'under_examination', 'granted', 'rejected', 'withdrawn', 'expired', 'lapsed', 'invalidated');
CREATE TYPE patent_type AS ENUM ('invention', 'utility_model', 'design', 'plant', 'provisional');

-- Create patents table
CREATE TABLE patents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    patent_number VARCHAR(64) NOT NULL UNIQUE,
    title TEXT NOT NULL,
    title_en TEXT,
    abstract TEXT,
    abstract_en TEXT,
    patent_type patent_type NOT NULL DEFAULT 'invention',
    status patent_status NOT NULL DEFAULT 'draft',
    filing_date DATE,
    publication_date DATE,
    grant_date DATE,
    expiry_date DATE,
    priority_date DATE,
    assignee_id UUID,
    assignee_name VARCHAR(512),
    jurisdiction VARCHAR(8) NOT NULL,
    ipc_codes TEXT[],
    cpc_codes TEXT[],
    keyip_tech_codes TEXT[],
    family_id VARCHAR(64),
    application_number VARCHAR(64),
    full_text_hash VARCHAR(64),
    source VARCHAR(32) NOT NULL DEFAULT 'manual',
    raw_data JSONB,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

-- Create patent_claims table
CREATE TABLE patent_claims (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    patent_id UUID NOT NULL REFERENCES patents(id) ON DELETE CASCADE,
    claim_number INTEGER NOT NULL,
    claim_type VARCHAR(16) NOT NULL CHECK (claim_type IN ('independent', 'dependent')),
    parent_claim_id UUID REFERENCES patent_claims(id),
    claim_text TEXT NOT NULL,
    claim_text_en TEXT,
    elements JSONB,
    markush_structures JSONB,
    scope_embedding VECTOR(768),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(patent_id, claim_number)
);

-- Create patent_inventors table
CREATE TABLE patent_inventors (
    patent_id UUID NOT NULL REFERENCES patents(id) ON DELETE CASCADE,
    inventor_name VARCHAR(256) NOT NULL,
    inventor_name_en VARCHAR(256),
    sequence INTEGER NOT NULL DEFAULT 1,
    affiliation VARCHAR(512),
    PRIMARY KEY (patent_id, inventor_name, sequence)
);

-- Create patent_priority_claims table
CREATE TABLE patent_priority_claims (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    patent_id UUID NOT NULL REFERENCES patents(id) ON DELETE CASCADE,
    priority_number VARCHAR(64) NOT NULL,
    priority_date DATE NOT NULL,
    priority_country VARCHAR(8) NOT NULL
);

-- Create indexes
CREATE INDEX idx_patents_patent_number ON patents(patent_number);
CREATE INDEX idx_patents_status ON patents(status);
CREATE INDEX idx_patents_jurisdiction ON patents(jurisdiction);
CREATE INDEX idx_patents_filing_date ON patents(filing_date);
CREATE INDEX idx_patents_assignee_id ON patents(assignee_id);
CREATE INDEX idx_patents_family_id ON patents(family_id);
CREATE INDEX idx_patents_ipc_codes ON patents USING GIN(ipc_codes);
CREATE INDEX idx_patents_keyip_tech_codes ON patents USING GIN(keyip_tech_codes);
CREATE INDEX idx_patents_metadata ON patents USING GIN(metadata jsonb_path_ops);
CREATE INDEX idx_patents_deleted_at ON patents(deleted_at) WHERE deleted_at IS NULL;

CREATE INDEX idx_patent_claims_patent_id ON patent_claims(patent_id);
CREATE INDEX idx_patent_claims_type ON patent_claims(claim_type);

CREATE INDEX idx_patent_priority_claims_patent_id ON patent_priority_claims(patent_id);

-- Create triggers
CREATE TRIGGER set_updated_at
BEFORE UPDATE ON patents
FOR EACH ROW
EXECUTE FUNCTION trigger_set_updated_at();

CREATE TRIGGER set_updated_at
BEFORE UPDATE ON patent_claims
FOR EACH ROW
EXECUTE FUNCTION trigger_set_updated_at();

-- +migrate Down

DROP TRIGGER IF EXISTS set_updated_at ON patent_claims;
DROP TRIGGER IF EXISTS set_updated_at ON patents;

DROP TABLE IF EXISTS patent_priority_claims;
DROP TABLE IF EXISTS patent_inventors;
DROP TABLE IF EXISTS patent_claims;
DROP TABLE IF EXISTS patents;

DROP TYPE IF EXISTS patent_type;
DROP TYPE IF EXISTS patent_status;
DROP FUNCTION IF EXISTS trigger_set_updated_at;
DROP EXTENSION IF EXISTS "vector";
--Personal.AI order the ending
