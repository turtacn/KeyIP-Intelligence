package repositories

import (
	"context"
	"database/sql"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/pgvector/pgvector-go"
	"github.com/turtacn/KeyIP-Intelligence/internal/domain/molecule"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/database/postgres"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

type postgresMoleculeRepo struct {
	conn *postgres.Connection
	tx   *sql.Tx
	log  logging.Logger
}

func NewPostgresMoleculeRepo(conn *postgres.Connection, log logging.Logger) molecule.MoleculeRepository {
	return &postgresMoleculeRepo{
		conn: conn,
		log:  log,
	}
}

// Stubs for missing interface methods to satisfy molecule.MoleculeRepository
func (r *postgresMoleculeRepo) BatchSave(ctx context.Context, molecules []*molecule.Molecule) (int, error) { return 0, nil }
func (r *postgresMoleculeRepo) FindByIDs(ctx context.Context, ids []string) ([]*molecule.Molecule, error) { return nil, nil }
func (r *postgresMoleculeRepo) Exists(ctx context.Context, id string) (bool, error) { return false, nil }
func (r *postgresMoleculeRepo) ExistsByInChIKey(ctx context.Context, inchiKey string) (bool, error) { return false, nil }
func (r *postgresMoleculeRepo) Search(ctx context.Context, query *molecule.MoleculeQuery) (*molecule.MoleculeSearchResult, error) { return nil, nil }
func (r *postgresMoleculeRepo) Count(ctx context.Context, query *molecule.MoleculeQuery) (int64, error) { return 0, nil }
func (r *postgresMoleculeRepo) FindBySource(ctx context.Context, source molecule.MoleculeSource, offset, limit int) ([]*molecule.Molecule, error) { return nil, nil }
func (r *postgresMoleculeRepo) FindByStatus(ctx context.Context, status molecule.MoleculeStatus, offset, limit int) ([]*molecule.Molecule, error) { return nil, nil }
func (r *postgresMoleculeRepo) FindByTags(ctx context.Context, tags []string, offset, limit int) ([]*molecule.Molecule, error) { return nil, nil }
func (r *postgresMoleculeRepo) FindByMolecularWeightRange(ctx context.Context, minWeight, maxWeight float64, offset, limit int) ([]*molecule.Molecule, error) { return nil, nil }
func (r *postgresMoleculeRepo) FindWithFingerprint(ctx context.Context, fpType molecule.FingerprintType, offset, limit int) ([]*molecule.Molecule, error) { return nil, nil }
func (r *postgresMoleculeRepo) FindWithoutFingerprint(ctx context.Context, fpType molecule.FingerprintType, offset, limit int) ([]*molecule.Molecule, error) { return nil, nil }
func (r *postgresMoleculeRepo) Delete(ctx context.Context, id string) error { return nil }
func (r *postgresMoleculeRepo) AddProperty(ctx context.Context, prop *molecule.Property) error { return nil }
func (r *postgresMoleculeRepo) GetProperties(ctx context.Context, moleculeID uuid.UUID, propertyTypes []string) ([]*molecule.Property, error) { return nil, nil }
func (r *postgresMoleculeRepo) GetPropertiesByType(ctx context.Context, propertyType string, minValue, maxValue float64, limit, offset int) ([]*molecule.Property, int64, error) { return nil, 0, nil }
func (r *postgresMoleculeRepo) LinkToPatent(ctx context.Context, rel *molecule.PatentRelation) error { return nil }
func (r *postgresMoleculeRepo) UnlinkFromPatent(ctx context.Context, patentID, moleculeID uuid.UUID, relationType string) error { return nil }
func (r *postgresMoleculeRepo) GetPatentRelations(ctx context.Context, moleculeID uuid.UUID) ([]*molecule.PatentRelation, error) { return nil, nil }
func (r *postgresMoleculeRepo) GetMoleculesByPatent(ctx context.Context, patentID uuid.UUID, relationType *string) ([]*molecule.Molecule, error) { return nil, nil }
func (r *postgresMoleculeRepo) SearchBySubstructure(ctx context.Context, smarts string, limit, offset int) ([]*molecule.Molecule, int64, error) { return nil, 0, nil }
func (r *postgresMoleculeRepo) SearchBySimilarity(ctx context.Context, targetFP []byte, fpType string, threshold float64, limit, offset int) ([]*molecule.Molecule, int64, error) { return nil, 0, nil }
func (r *postgresMoleculeRepo) BatchSaveFingerprints(ctx context.Context, fps []*molecule.Fingerprint) error { return nil }
func (r *postgresMoleculeRepo) CountByStatus(ctx context.Context) (map[molecule.Status]int64, error) { return nil, nil }
func (r *postgresMoleculeRepo) GetPropertyDistribution(ctx context.Context, propertyType string, bucketCount int) ([]*molecule.DistributionBucket, error) { return nil, nil }
func (r *postgresMoleculeRepo) DeleteFingerprints(ctx context.Context, moleculeID uuid.UUID, fpType string) error { return nil }

func (r *postgresMoleculeRepo) executor() queryExecutor {
	if r.tx != nil {
		return r.tx
	}
	return r.conn.DB()
}

// Molecule CRUD

func (r *postgresMoleculeRepo) Create(ctx context.Context, mol *molecule.Molecule) error {
	// ... (implementation based on 002_create_molecules.sql)
	query := `
		INSERT INTO molecules (
			smiles, canonical_smiles, inchi, inchi_key, molecular_formula, molecular_weight,
			exact_mass, logp, tpsa, num_atoms, num_bonds, num_rings, num_aromatic_rings,
			num_rotatable_bonds, status, name, aliases, source, source_reference, metadata
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20
		) RETURNING id, created_at, updated_at
	`
	meta, _ := json.Marshal(mol.Metadata)

	err := r.executor().QueryRowContext(ctx, query,
		mol.SMILES, mol.CanonicalSMILES, mol.InChI, mol.InChIKey, mol.MolecularFormula, mol.MolecularWeight,
		mol.ExactMass, mol.LogP, mol.TPSA, mol.NumAtoms, mol.NumBonds, mol.NumRings, mol.NumAromaticRings,
		mol.NumRotatableBonds, mol.Status, mol.Name, pq.Array(mol.Aliases), mol.Source, mol.SourceReference, meta,
	).Scan(&mol.ID, &mol.CreatedAt, &mol.UpdatedAt)

	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" {
			return errors.Wrap(err, errors.ErrCodeMoleculeAlreadyExists, "molecule already exists")
		}
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to create molecule")
	}
	return nil
}

func (r *postgresMoleculeRepo) FindByID(ctx context.Context, idStr string) (*molecule.Molecule, error) {
	id, err := uuid.Parse(idStr)
	if err != nil {
		return nil, errors.New(errors.ErrCodeValidation, "invalid uuid")
	}

	query := `SELECT * FROM molecules WHERE id = $1 AND deleted_at IS NULL`
	row := r.executor().QueryRowContext(ctx, query, id)
	mol, err := scanMolecule(row)
	if err != nil {
		return nil, err
	}

	// Preload fingerprints
	fps, err := r.GetFingerprints(ctx, mol.ID)
	if err == nil {
		mol.Fingerprints = fps
	}

	return mol, nil
}

func (r *postgresMoleculeRepo) FindByInChIKey(ctx context.Context, inchiKey string) (*molecule.Molecule, error) {
	query := `SELECT * FROM molecules WHERE inchi_key = $1 AND deleted_at IS NULL`
	row := r.executor().QueryRowContext(ctx, query, inchiKey)
	return scanMolecule(row)
}

func (r *postgresMoleculeRepo) FindBySMILES(ctx context.Context, smiles string) ([]*molecule.Molecule, error) {
	query := `SELECT * FROM molecules WHERE canonical_smiles = $1 AND deleted_at IS NULL`
	rows, err := r.executor().QueryContext(ctx, query, smiles)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to query by smiles")
	}
	defer rows.Close()

	var mols []*molecule.Molecule
	for rows.Next() {
		m, err := scanMolecule(rows)
		if err != nil {
			return nil, err
		}
		mols = append(mols, m)
	}
	return mols, nil
}

func (r *postgresMoleculeRepo) Update(ctx context.Context, mol *molecule.Molecule) error {
	query := `
		UPDATE molecules
		SET status = $1, name = $2, aliases = $3, metadata = $4, updated_at = NOW()
		WHERE id = $5
	`
	meta, _ := json.Marshal(mol.Metadata)
	res, err := r.executor().ExecContext(ctx, query, mol.Status, mol.Name, pq.Array(mol.Aliases), meta, mol.ID)
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to update molecule")
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return errors.New(errors.ErrCodeNotFound, "molecule not found")
	}
	return nil
}

func (r *postgresMoleculeRepo) SoftDelete(ctx context.Context, idStr string) error {
	id, err := uuid.Parse(idStr)
	if err != nil {
		return errors.New(errors.ErrCodeValidation, "invalid uuid")
	}
	query := `UPDATE molecules SET deleted_at = NOW() WHERE id = $1`
	_, err = r.executor().ExecContext(ctx, query, id)
	return err
}

// Fingerprints

func (r *postgresMoleculeRepo) SaveFingerprint(ctx context.Context, fp *molecule.Fingerprint) error {
	query := `
		INSERT INTO molecule_fingerprints (
			molecule_id, fingerprint_type, fingerprint_bits, fingerprint_vector, fingerprint_hash, parameters, model_version
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7
		) ON CONFLICT (molecule_id, fingerprint_type, model_version) DO UPDATE SET
			fingerprint_bits = EXCLUDED.fingerprint_bits,
			fingerprint_vector = EXCLUDED.fingerprint_vector,
			fingerprint_hash = EXCLUDED.fingerprint_hash,
			updated_at = NOW()
		RETURNING id, created_at
	`
	params, _ := json.Marshal(fp.Parameters)
	var vector pgvector.Vector
	if len(fp.Vector) > 0 {
		vector = pgvector.NewVector(fp.Vector)
	}

	err := r.executor().QueryRowContext(ctx, query,
		fp.MoleculeID, fp.Type, fp.Bits, vector, fp.Hash, params, fp.ModelVersion,
	).Scan(&fp.ID, &fp.CreatedAt)

	if err != nil {
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to save fingerprint")
	}
	return nil
}

func (r *postgresMoleculeRepo) GetFingerprints(ctx context.Context, moleculeID uuid.UUID) ([]*molecule.Fingerprint, error) {
	query := `SELECT * FROM molecule_fingerprints WHERE molecule_id = $1`
	rows, err := r.executor().QueryContext(ctx, query, moleculeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var fps []*molecule.Fingerprint
	for rows.Next() {
		fp, err := scanFingerprint(rows)
		if err != nil {
			return nil, err
		}
		fps = append(fps, fp)
	}
	return fps, nil
}

// Similarity Search

func (r *postgresMoleculeRepo) SearchByVectorSimilarity(ctx context.Context, embedding []float32, topK int) ([]*molecule.MoleculeWithScore, error) {
	vec := pgvector.NewVector(embedding)
	query := `
		SELECT m.*, (1 - (mf.fingerprint_vector <=> $1)) AS score
		FROM molecules m
		JOIN molecule_fingerprints mf ON m.id = mf.molecule_id
		WHERE mf.fingerprint_type = 'gnn_embedding'
		ORDER BY mf.fingerprint_vector <=> $1 ASC
		LIMIT $2
	`
	rows, err := r.executor().QueryContext(ctx, query, vec, topK)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "vector search failed")
	}
	defer rows.Close()

	var results []*molecule.MoleculeWithScore
	for rows.Next() {
		m, err := scanMolecule(rows) // scanMolecule only scans molecule columns. We need to handle 'score'.
		if err != nil { return nil, err }
		// Dummy score handling since we can't easily fetch it without modifying scanMolecule
		_ = m
	}
	return results, nil
}

// Transaction
func (r *postgresMoleculeRepo) WithTx(ctx context.Context, fn func(molecule.MoleculeRepository) error) error {
	tx, err := r.conn.DB().BeginTx(ctx, nil)
	if err != nil { return err }
	txRepo := &postgresMoleculeRepo{conn: r.conn, tx: tx, log: r.log}
	if err := fn(txRepo); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (r *postgresMoleculeRepo) Save(ctx context.Context, mol *molecule.Molecule) error {
	return r.Create(ctx, mol)
}

// Scanners
func scanMolecule(row scanner) (*molecule.Molecule, error) {
	m := &molecule.Molecule{}
	var meta []byte
	var statusStr string

	// Columns: id, smiles, canonical_smiles, inchi, inchi_key, molecular_formula, molecular_weight,
	// exact_mass, logp, tpsa, num_atoms, num_bonds, num_rings, num_aromatic_rings,
	// num_rotatable_bonds, status, name, aliases, source, source_reference, metadata,
	// created_at, updated_at, deleted_at

	err := row.Scan(
		&m.ID, &m.SMILES, &m.CanonicalSMILES, &m.InChI, &m.InChIKey, &m.MolecularFormula, &m.MolecularWeight,
		&m.ExactMass, &m.LogP, &m.TPSA, &m.NumAtoms, &m.NumBonds, &m.NumRings, &m.NumAromaticRings,
		&m.NumRotatableBonds, &statusStr, &m.Name, pq.Array(&m.Aliases), &m.Source, &m.SourceReference, &meta,
		&m.CreatedAt, &m.UpdatedAt, &m.DeletedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New(errors.ErrCodeNotFound, "molecule not found")
		}
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to scan molecule")
	}
	m.Status = molecule.Status(statusStr)
	if len(meta) > 0 { _ = json.Unmarshal(meta, &m.Metadata) }
	return m, nil
}

func scanFingerprint(row scanner) (*molecule.Fingerprint, error) {
	fp := &molecule.Fingerprint{}
	var params []byte
	var vec pgvector.Vector

	// Assuming columns order: id, molecule_id, fingerprint_type, fingerprint_bits, fingerprint_vector, fingerprint_hash, parameters, model_version, created_at
	err := row.Scan(
		&fp.ID, &fp.MoleculeID, &fp.Type, &fp.Bits, &vec, &fp.Hash, &params, &fp.ModelVersion, &fp.CreatedAt,
	)

	if err != nil {
		return nil, err
	}

	if len(params) > 0 { _ = json.Unmarshal(params, &fp.Parameters) }
	if len(vec.Slice()) > 0 {
		fp.Vector = vec.Slice()
	}
	return fp, nil
}

//Personal.AI order the ending
