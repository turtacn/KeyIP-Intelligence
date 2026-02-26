-- +migrate Up

CREATE TYPE molecule_status AS ENUM ('active', 'archived', 'deleted', 'pending_review');

CREATE TABLE molecules (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    smiles TEXT NOT NULL,
    canonical_smiles TEXT NOT NULL,
    inchi TEXT,
    inchi_key VARCHAR(27) UNIQUE,
    molecular_formula VARCHAR(256),
    molecular_weight DOUBLE PRECISION,
    exact_mass DOUBLE PRECISION,
    logp DOUBLE PRECISION,
    tpsa DOUBLE PRECISION,
    num_atoms INTEGER,
    num_bonds INTEGER,
    num_rings INTEGER,
    num_aromatic_rings INTEGER,
    num_rotatable_bonds INTEGER,
    status molecule_status NOT NULL DEFAULT 'active',
    name VARCHAR(512),
    aliases TEXT[],
    source VARCHAR(32) NOT NULL DEFAULT 'manual',
    source_reference VARCHAR(512),
    metadata JSONB DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ
);

CREATE TABLE molecule_fingerprints (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    molecule_id UUID NOT NULL REFERENCES molecules(id) ON DELETE CASCADE,
    fingerprint_type VARCHAR(32) NOT NULL,
    fingerprint_bits BIT VARYING,
    fingerprint_vector VECTOR(512),
    fingerprint_hash VARCHAR(128),
    parameters JSONB,
    model_version VARCHAR(32),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(molecule_id, fingerprint_type, model_version)
);

CREATE TABLE molecule_properties (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    molecule_id UUID NOT NULL REFERENCES molecules(id) ON DELETE CASCADE,
    property_type VARCHAR(64) NOT NULL,
    value DOUBLE PRECISION NOT NULL,
    unit VARCHAR(32),
    measurement_conditions JSONB,
    data_source VARCHAR(32) NOT NULL,
    confidence DOUBLE PRECISION,
    source_reference VARCHAR(512),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE patent_molecule_relations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    patent_id UUID NOT NULL REFERENCES patents(id) ON DELETE CASCADE,
    molecule_id UUID NOT NULL REFERENCES molecules(id) ON DELETE CASCADE,
    relation_type VARCHAR(32) NOT NULL,
    location_in_patent VARCHAR(256),
    page_reference VARCHAR(64),
    claim_numbers INTEGER[],
    extraction_method VARCHAR(32) NOT NULL DEFAULT 'manual',
    confidence DOUBLE PRECISION DEFAULT 1.0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(patent_id, molecule_id, relation_type)
);

CREATE INDEX idx_molecules_inchi_key ON molecules(inchi_key);
CREATE INDEX idx_molecules_canonical_smiles ON molecules USING HASH(canonical_smiles);
CREATE INDEX idx_molecules_molecular_weight ON molecules(molecular_weight);
CREATE INDEX idx_molecules_status ON molecules(status);
CREATE INDEX idx_molecules_deleted_at ON molecules(deleted_at) WHERE deleted_at IS NULL;
CREATE INDEX idx_molecules_num_aromatic_rings ON molecules(num_aromatic_rings);

CREATE INDEX idx_molecule_fps_molecule_id ON molecule_fingerprints(molecule_id);
CREATE INDEX idx_molecule_fps_type ON molecule_fingerprints(fingerprint_type);

CREATE INDEX idx_molecule_props_molecule_id ON molecule_properties(molecule_id);
CREATE INDEX idx_molecule_props_type ON molecule_properties(property_type);

CREATE INDEX idx_patent_mol_rel_patent_id ON patent_molecule_relations(patent_id);
CREATE INDEX idx_patent_mol_rel_molecule_id ON patent_molecule_relations(molecule_id);
CREATE INDEX idx_patent_mol_rel_type ON patent_molecule_relations(relation_type);

CREATE TRIGGER set_updated_at
BEFORE UPDATE ON molecules
FOR EACH ROW
EXECUTE FUNCTION trigger_set_updated_at();

-- +migrate Down

DROP TRIGGER IF EXISTS set_updated_at ON molecules;

DROP TABLE IF EXISTS patent_molecule_relations;
DROP TABLE IF EXISTS molecule_properties;
DROP TABLE IF EXISTS molecule_fingerprints;
DROP TABLE IF EXISTS molecules;

DROP TYPE IF EXISTS molecule_status;
--Personal.AI order the ending
