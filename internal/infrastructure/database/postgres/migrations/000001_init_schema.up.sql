-- Migration: 000001_init_schema.up.sql
-- Description: Initialize the core database schema for KeyIP-Intelligence platform.
-- Creates all primary tables for patents, claims, molecules, portfolios, lifecycles,
-- and workspaces, along with their indexes and triggers.

-- ─────────────────────────────────────────────────────────────────────────────
-- Extension: Enable pgcrypto for UUID generation
-- ─────────────────────────────────────────────────────────────────────────────

CREATE EXTENSION IF NOT EXISTS "pgcrypto";

-- ─────────────────────────────────────────────────────────────────────────────
-- Table: patents
-- Primary entity for patent documents
-- ─────────────────────────────────────────────────────────────────────────────

CREATE TABLE patents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    patent_number VARCHAR(50) NOT NULL UNIQUE,
    title TEXT NOT NULL,
    abstract TEXT,
    description TEXT,
    applicants TEXT[] NOT NULL DEFAULT '{}',
    inventors TEXT[],
    filing_date DATE NOT NULL,
    publication_date DATE,
    grant_date DATE,
    expiry_date DATE,
    status VARCHAR(20) NOT NULL DEFAULT 'filed',
    legal_status TEXT,
    jurisdiction VARCHAR(10) NOT NULL,
    ipc_codes TEXT[],
    citations TEXT[],
    family_id VARCHAR(100),
    priority TEXT[],
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by UUID,
    version INT NOT NULL DEFAULT 1
);

-- Indexes for patents table
CREATE INDEX idx_patents_tenant_id ON patents(tenant_id);
CREATE INDEX idx_patents_status ON patents(status);
CREATE INDEX idx_patents_jurisdiction ON patents(jurisdiction);
CREATE INDEX idx_patents_filing_date ON patents(filing_date);
CREATE INDEX idx_patents_family_id ON patents(family_id);

-- GIN indexes for array columns
CREATE INDEX idx_patents_ipc_codes ON patents USING GIN(ipc_codes);
CREATE INDEX idx_patents_inventors ON patents USING GIN(inventors);
CREATE INDEX idx_patents_applicants ON patents USING GIN(applicants);

-- Full-text search index for title and abstract
CREATE INDEX idx_patents_fulltext ON patents USING GIN(to_tsvector('english', title || ' ' || COALESCE(abstract, '')));

-- ─────────────────────────────────────────────────────────────────────────────
-- Table: patent_claims
-- Patent claims with dependency tracking
-- ─────────────────────────────────────────────────────────────────────────────

CREATE TABLE patent_claims (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    patent_id UUID NOT NULL REFERENCES patents(id) ON DELETE CASCADE,
    claim_number INT NOT NULL,
    text TEXT NOT NULL,
    claim_type VARCHAR(50),
    parent_id UUID,
    is_independent BOOLEAN NOT NULL DEFAULT TRUE,
    UNIQUE(patent_id, claim_number)
);

-- Indexes for patent_claims table
CREATE INDEX idx_patent_claims_patent_id ON patent_claims(patent_id);

-- ─────────────────────────────────────────────────────────────────────────────
-- Table: markush_structures
-- Markush structure representations for generic chemical disclosures
-- ─────────────────────────────────────────────────────────────────────────────

CREATE TABLE markush_structures (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    patent_id UUID NOT NULL REFERENCES patents(id) ON DELETE CASCADE,
    smarts TEXT NOT NULL,
    description TEXT,
    r_groups JSONB NOT NULL,
    enumerated_count BIGINT
);

-- Indexes for markush_structures table
CREATE INDEX idx_markush_patent_id ON markush_structures(patent_id);

-- ─────────────────────────────────────────────────────────────────────────────
-- Table: molecules
-- Molecular entities extracted from patents
-- ─────────────────────────────────────────────────────────────────────────────

CREATE TABLE molecules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    smiles TEXT NOT NULL,
    canonical_smiles TEXT NOT NULL,
    inchi TEXT,
    inchi_key VARCHAR(27) UNIQUE,
    molecular_formula VARCHAR(200),
    molecular_weight DOUBLE PRECISION,
    name VARCHAR(500),
    synonyms TEXT[],
    type VARCHAR(30) NOT NULL,
    properties JSONB,
    fingerprints JSONB,
    source_patent_ids UUID[],
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by UUID,
    version INT NOT NULL DEFAULT 1
);

-- Indexes for molecules table
CREATE INDEX idx_molecules_tenant_id ON molecules(tenant_id);
CREATE INDEX idx_molecules_canonical_smiles ON molecules(canonical_smiles);
CREATE INDEX idx_molecules_inchi_key ON molecules(inchi_key);
CREATE INDEX idx_molecules_type ON molecules(type);

-- GIN indexes for array columns
CREATE INDEX idx_molecules_synonyms ON molecules USING GIN(synonyms);
CREATE INDEX idx_molecules_source_patent_ids ON molecules USING GIN(source_patent_ids);

-- GIN index for properties JSONB
CREATE INDEX idx_molecules_properties ON molecules USING GIN(properties);

-- ─────────────────────────────────────────────────────────────────────────────
-- Table: portfolios
-- Patent portfolio management
-- ─────────────────────────────────────────────────────────────────────────────

CREATE TABLE portfolios (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    name VARCHAR(200) NOT NULL,
    description TEXT,
    owner_id UUID NOT NULL,
    patent_ids UUID[],
    total_value JSONB,
    tags TEXT[],
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by UUID,
    version INT NOT NULL DEFAULT 1
);

-- Indexes for portfolios table
CREATE INDEX idx_portfolios_tenant_id ON portfolios(tenant_id);
CREATE INDEX idx_portfolios_owner_id ON portfolios(owner_id);
CREATE INDEX idx_portfolios_status ON portfolios(status);

-- GIN indexes for array columns
CREATE INDEX idx_portfolios_patent_ids ON portfolios USING GIN(patent_ids);
CREATE INDEX idx_portfolios_tags ON portfolios USING GIN(tags);

-- ─────────────────────────────────────────────────────────────────────────────
-- Table: lifecycles
-- Patent lifecycle events, deadlines, and annuity tracking
-- ─────────────────────────────────────────────────────────────────────────────

CREATE TABLE lifecycles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    patent_id UUID NOT NULL REFERENCES patents(id) ON DELETE CASCADE UNIQUE,
    phase VARCHAR(50),
    deadlines JSONB,
    annuity_schedule JSONB,
    legal_status JSONB,
    events JSONB,
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by UUID,
    version INT NOT NULL DEFAULT 1
);

-- Indexes for lifecycles table
CREATE INDEX idx_lifecycles_tenant_id ON lifecycles(tenant_id);
CREATE INDEX idx_lifecycles_patent_id ON lifecycles(patent_id);

-- GIN index for JSONB columns
CREATE INDEX idx_lifecycles_deadlines ON lifecycles USING GIN(deadlines);
CREATE INDEX idx_lifecycles_events ON lifecycles USING GIN(events);

-- ─────────────────────────────────────────────────────────────────────────────
-- Table: workspaces
-- Collaborative workspace management
-- ─────────────────────────────────────────────────────────────────────────────

CREATE TABLE workspaces (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    name VARCHAR(200) NOT NULL,
    description TEXT,
    owner_id UUID NOT NULL,
    members JSONB NOT NULL DEFAULT '[]',
    shared_resources JSONB NOT NULL DEFAULT '[]',
    status VARCHAR(20) NOT NULL DEFAULT 'active',
    tags TEXT[],
    settings JSONB DEFAULT '{}',
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by UUID,
    version INT NOT NULL DEFAULT 1
);

-- Indexes for workspaces table
CREATE INDEX idx_workspaces_tenant_id ON workspaces(tenant_id);
CREATE INDEX idx_workspaces_owner_id ON workspaces(owner_id);
CREATE INDEX idx_workspaces_status ON workspaces(status);

-- GIN indexes for JSONB columns
CREATE INDEX idx_workspaces_members ON workspaces USING GIN(members);
CREATE INDEX idx_workspaces_shared_resources ON workspaces USING GIN(shared_resources);

-- ─────────────────────────────────────────────────────────────────────────────
-- Trigger function: update_updated_at_column
-- Automatically updates the updated_at timestamp on row modification
-- ─────────────────────────────────────────────────────────────────────────────

CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Attach triggers to all tables with updated_at column
CREATE TRIGGER trigger_patents_updated_at BEFORE UPDATE ON patents
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER trigger_molecules_updated_at BEFORE UPDATE ON molecules
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER trigger_portfolios_updated_at BEFORE UPDATE ON portfolios
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER trigger_lifecycles_updated_at BEFORE UPDATE ON lifecycles
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER trigger_workspaces_updated_at BEFORE UPDATE ON workspaces
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
