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
    applicant VARCHAR(500) NOT NULL,
    inventors TEXT[],
    filing_date DATE NOT NULL,
    publication_date DATE,
    grant_date DATE,
    expiry_date DATE,
    status VARCHAR(20) NOT NULL DEFAULT 'filed' CHECK (status IN ('filed', 'published', 'granted', 'expired', 'abandoned')),
    jurisdiction VARCHAR(10) NOT NULL,
    ipc_codes TEXT[],
    cpc_codes TEXT[],
    family_id VARCHAR(100),
    priority JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    created_by UUID,
    version INT NOT NULL DEFAULT 1
);

-- Indexes for patents table
CREATE INDEX idx_patents_tenant_id ON patents(tenant_id);
CREATE INDEX idx_patents_status ON patents(status);
CREATE INDEX idx_patents_jurisdiction ON patents(jurisdiction);
CREATE INDEX idx_patents_applicant ON patents(applicant);
CREATE INDEX idx_patents_filing_date ON patents(filing_date);
CREATE INDEX idx_patents_family_id ON patents(family_id);

-- GIN indexes for array columns
CREATE INDEX idx_patents_ipc_codes ON patents USING GIN(ipc_codes);
CREATE INDEX idx_patents_cpc_codes ON patents USING GIN(cpc_codes);
CREATE INDEX idx_patents_inventors ON patents USING GIN(inventors);

-- Full-text search index for title and abstract
CREATE INDEX idx_patents_fulltext ON patents USING GIN(to_tsvector('english', title || ' ' || COALESCE(abstract, '')));

-- ─────────────────────────────────────────────────────────────────────────────
-- Table: claims
-- Patent claims with dependency tracking
-- ─────────────────────────────────────────────────────────────────────────────

CREATE TABLE claims (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    patent_id UUID NOT NULL REFERENCES patents(id) ON DELETE CASCADE,
    number INT NOT NULL,
    text TEXT NOT NULL,
    type VARCHAR(20) NOT NULL CHECK (type IN ('independent', 'dependent')),
    parent_claim_number INT,
    elements JSONB,
    UNIQUE(patent_id, number)
);

-- Indexes for claims table
CREATE INDEX idx_claims_patent_id ON claims(patent_id);
CREATE INDEX idx_claims_type ON claims(type);
CREATE INDEX idx_claims_parent_claim_number ON claims(patent_id, parent_claim_number);

-- ─────────────────────────────────────────────────────────────────────────────
-- Table: markush_structures
-- Markush structure representations for generic chemical disclosures
-- ─────────────────────────────────────────────────────────────────────────────

CREATE TABLE markush_structures (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    patent_id UUID NOT NULL REFERENCES patents(id) ON DELETE CASCADE,
    claim_id UUID NOT NULL REFERENCES claims(id) ON DELETE CASCADE,
    core_structure TEXT NOT NULL,
    r_groups JSONB NOT NULL,
    description TEXT,
    enumerated_count BIGINT
);

-- Indexes for markush_structures table
CREATE INDEX idx_markush_patent_id ON markush_structures(patent_id);
CREATE INDEX idx_markush_claim_id ON markush_structures(claim_id);

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
    type VARCHAR(30) NOT NULL CHECK (type IN ('small_molecule', 'polymer', 'oled_material', 'catalyst', 'intermediate')),
    properties JSONB,
    fingerprints JSONB,
    source_patent_ids UUID[],
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
    status VARCHAR(20) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'inactive', 'archived')),
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
-- Table: patent_lifecycles
-- Patent lifecycle events, deadlines, and annuity tracking
-- ─────────────────────────────────────────────────────────────────────────────

CREATE TABLE patent_lifecycles (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    tenant_id UUID NOT NULL,
    patent_id UUID NOT NULL REFERENCES patents(id) ON DELETE CASCADE UNIQUE,
    patent_number VARCHAR(50) NOT NULL,
    jurisdiction VARCHAR(10) NOT NULL,
    filing_date DATE NOT NULL,
    grant_date DATE,
    expiry_date DATE NOT NULL,
    deadlines JSONB,
    annuity_schedule JSONB,
    legal_status JSONB,
    events JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Indexes for patent_lifecycles table
CREATE INDEX idx_lifecycles_tenant_id ON patent_lifecycles(tenant_id);
CREATE INDEX idx_lifecycles_patent_id ON patent_lifecycles(patent_id);
CREATE INDEX idx_lifecycles_jurisdiction ON patent_lifecycles(jurisdiction);
CREATE INDEX idx_lifecycles_expiry_date ON patent_lifecycles(expiry_date);

-- GIN index for JSONB columns
CREATE INDEX idx_lifecycles_deadlines ON patent_lifecycles USING GIN(deadlines);
CREATE INDEX idx_lifecycles_events ON patent_lifecycles USING GIN(events);

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
    status VARCHAR(20) NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'inactive', 'archived')),
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

CREATE TRIGGER trigger_lifecycles_updated_at BEFORE UPDATE ON patent_lifecycles
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER trigger_workspaces_updated_at BEFORE UPDATE ON workspaces
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

