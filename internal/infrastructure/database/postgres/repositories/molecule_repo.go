package repositories

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

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
func (r *postgresMoleculeRepo) BatchSave(ctx context.Context, molecules []*molecule.Molecule) (int, error) {
	if len(molecules) == 0 {
		return 0, nil
	}

	totalAffected := 0
	batchSize := 1000 // 1000 molecules * 23 params = 23000 parameters (< 65535 limit)

	for start := 0; start < len(molecules); start += batchSize {
		end := start + batchSize
		if end > len(molecules) {
			end = len(molecules)
		}
		batch := molecules[start:end]

		query := `
			INSERT INTO molecules (
				id, smiles, canonical_smiles, inchi, inchi_key, molecular_formula, molecular_weight,
				exact_mass, logp, tpsa, num_atoms, num_bonds, num_rings, num_aromatic_rings,
				num_rotatable_bonds, status, name, aliases, source, source_reference, metadata,
				created_at, updated_at
			) VALUES
		`

		var values []interface{}
		var placeholders []string

		for i, mol := range batch {
			base := i * 23
			placeholders = append(placeholders, fmt.Sprintf("($%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d)",
				base+1, base+2, base+3, base+4, base+5, base+6, base+7, base+8, base+9, base+10,
				base+11, base+12, base+13, base+14, base+15, base+16, base+17, base+18, base+19, base+20,
				base+21, base+22, base+23))

			meta, _ := json.Marshal(mol.Metadata)
			values = append(values,
				mol.ID, mol.SMILES, mol.CanonicalSMILES, mol.InChI, mol.InChIKey, mol.MolecularFormula, mol.MolecularWeight,
				mol.ExactMass, mol.LogP, mol.TPSA, mol.NumAtoms, mol.NumBonds, mol.NumRings, mol.NumAromaticRings,
				mol.NumRotatableBonds, mol.Status, mol.Name, pq.Array(mol.Aliases), mol.Source, mol.SourceReference, meta,
				mol.CreatedAt, mol.UpdatedAt,
			)
		}

		query += strings.Join(placeholders, ",") + " ON CONFLICT (id) DO NOTHING"

		res, err := r.executor().ExecContext(ctx, query, values...)
		if err != nil {
			if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" {
				return totalAffected, errors.Wrap(err, errors.ErrCodeMoleculeAlreadyExists, "one or more molecules already exist")
			}
			return totalAffected, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to batch save molecules")
		}

		rowsAffected, _ := res.RowsAffected()
		totalAffected += int(rowsAffected)
	}

	return totalAffected, nil
}
func (r *postgresMoleculeRepo) FindByIDs(ctx context.Context, ids []string) ([]*molecule.Molecule, error) { return nil, nil }
func (r *postgresMoleculeRepo) Exists(ctx context.Context, id string) (bool, error) { return false, nil }
func (r *postgresMoleculeRepo) ExistsByInChIKey(ctx context.Context, inchiKey string) (bool, error) { return false, nil }
func (r *postgresMoleculeRepo) buildSearchQuery(query *molecule.MoleculeQuery, isCount bool) (string, []interface{}) {
	var sb strings.Builder
	var args []interface{}
	argIdx := 1

	if isCount {
		sb.WriteString("SELECT COUNT(DISTINCT m.id) FROM molecules m ")
	} else {
		sb.WriteString("SELECT m.* FROM molecules m ")
	}

	// Joins
	if len(query.HasFingerprintTypes) > 0 {
		sb.WriteString("JOIN molecule_fingerprints mf ON m.id = mf.molecule_id ")
	}
	if len(query.PropertyFilters) > 0 {
		sb.WriteString("JOIN molecule_properties mp ON m.id = mp.molecule_id ")
	}

	sb.WriteString("WHERE m.deleted_at IS NULL ")

	// Filters
	if len(query.IDs) > 0 {
		sb.WriteString(fmt.Sprintf("AND m.id = ANY($%d) ", argIdx))
		args = append(args, pq.Array(query.IDs))
		argIdx++
	}

	if query.SMILES != "" {
		sb.WriteString(fmt.Sprintf("AND m.canonical_smiles = $%d ", argIdx))
		args = append(args, query.SMILES)
		argIdx++
	}

	if query.SMILESPattern != "" {
		sb.WriteString(fmt.Sprintf("AND m.canonical_smiles LIKE $%d ", argIdx))
		args = append(args, "%"+query.SMILESPattern+"%")
		argIdx++
	}

	if len(query.InChIKeys) > 0 {
		sb.WriteString(fmt.Sprintf("AND m.inchi_key = ANY($%d) ", argIdx))
		args = append(args, pq.Array(query.InChIKeys))
		argIdx++
	}

	if query.MinMolecularWeight != nil {
		sb.WriteString(fmt.Sprintf("AND m.molecular_weight >= $%d ", argIdx))
		args = append(args, *query.MinMolecularWeight)
		argIdx++
	}

	if query.MaxMolecularWeight != nil {
		sb.WriteString(fmt.Sprintf("AND m.molecular_weight <= $%d ", argIdx))
		args = append(args, *query.MaxMolecularWeight)
		argIdx++
	}

	if len(query.Statuses) > 0 {
		statusStrs := make([]string, len(query.Statuses))
		for i, s := range query.Statuses { statusStrs[i] = string(s) }
		sb.WriteString(fmt.Sprintf("AND m.status = ANY($%d) ", argIdx))
		args = append(args, pq.Array(statusStrs))
		argIdx++
	}

	if len(query.Sources) > 0 {
		sourceStrs := make([]string, len(query.Sources))
		for i, s := range query.Sources { sourceStrs[i] = string(s) }
		sb.WriteString(fmt.Sprintf("AND m.source = ANY($%d) ", argIdx))
		args = append(args, pq.Array(sourceStrs))
		argIdx++
	}

	if len(query.HasFingerprintTypes) > 0 {
		fpStrs := make([]string, len(query.HasFingerprintTypes))
		for i, f := range query.HasFingerprintTypes { fpStrs[i] = string(f) }
		sb.WriteString(fmt.Sprintf("AND mf.fingerprint_type = ANY($%d) ", argIdx))
		args = append(args, pq.Array(fpStrs))
		argIdx++
	}

	if len(query.PropertyFilters) > 0 {
		// Simplified for now: just checks first property filter if any
		pf := query.PropertyFilters[0]
		sb.WriteString(fmt.Sprintf("AND mp.property_type = $%d ", argIdx))
		args = append(args, pf.Name)
		argIdx++
		if pf.MinValue != nil {
			sb.WriteString(fmt.Sprintf("AND mp.value >= $%d ", argIdx))
			args = append(args, *pf.MinValue)
			argIdx++
		}
		if pf.MaxValue != nil {
			sb.WriteString(fmt.Sprintf("AND mp.value <= $%d ", argIdx))
			args = append(args, *pf.MaxValue)
			argIdx++
		}
	}

	if query.Keyword != "" {
		sb.WriteString(fmt.Sprintf("AND (m.name ILIKE $%d OR m.aliases @> $%d) ", argIdx, argIdx+1))
		args = append(args, "%"+query.Keyword+"%", pq.Array([]string{query.Keyword}))
		argIdx += 2
	}

	if !isCount {
		// Group by for DISTINCT behavior when joining
		if len(query.HasFingerprintTypes) > 0 || len(query.PropertyFilters) > 0 {
			sb.WriteString("GROUP BY m.id ")
		}

		// Sort
		sortBy := "created_at"
		validSortColumns := map[string]bool{
			"created_at":          true,
			"updated_at":          true,
			"molecular_weight":    true,
			"smiles":              true,
			"exact_mass":          true,
			"logp":                true,
			"tpsa":                true,
			"num_atoms":           true,
			"num_bonds":           true,
			"num_rings":           true,
			"num_aromatic_rings":  true,
			"num_rotatable_bonds": true,
		}

		if query.SortBy != "" && validSortColumns[query.SortBy] {
			sortBy = query.SortBy
		}

		sortOrder := "DESC"
		if strings.ToUpper(query.SortOrder) == "ASC" {
			sortOrder = "ASC"
		}
		sb.WriteString(fmt.Sprintf("ORDER BY m.%s %s ", sortBy, sortOrder))

		// Pagination
		limit := query.Limit
		if limit <= 0 {
			limit = 20
		}
		sb.WriteString(fmt.Sprintf("LIMIT $%d OFFSET $%d", argIdx, argIdx+1))
		args = append(args, limit, query.Offset)
	}

	return sb.String(), args
}

func (r *postgresMoleculeRepo) Search(ctx context.Context, query *molecule.MoleculeQuery) (*molecule.MoleculeSearchResult, error) {
	if query == nil {
		return nil, errors.New(errors.ErrCodeValidation, "query cannot be nil")
	}

	total, err := r.Count(ctx, query)
	if err != nil {
		return nil, err
	}

	sqlQuery, args := r.buildSearchQuery(query, false)
	rows, err := r.executor().QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to search molecules")
	}
	defer rows.Close()

	var molecules []*molecule.Molecule
	for rows.Next() {
		m, err := scanMolecule(rows)
		if err != nil {
			return nil, err
		}
		molecules = append(molecules, m)
	}

	limit := query.Limit
	if limit <= 0 { limit = 20 }

	return &molecule.MoleculeSearchResult{
		Molecules: molecules,
		Total:     total,
		Offset:    query.Offset,
		Limit:     limit,
		HasMore:   total > int64(query.Offset+limit),
	}, nil
}

func (r *postgresMoleculeRepo) Count(ctx context.Context, query *molecule.MoleculeQuery) (int64, error) {
	if query == nil {
		return 0, errors.New(errors.ErrCodeValidation, "query cannot be nil")
	}

	sqlQuery, args := r.buildSearchQuery(query, true)
	var count int64
	err := r.executor().QueryRowContext(ctx, sqlQuery, args...).Scan(&count)
	if err != nil {
		return 0, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to count molecules")
	}
	return count, nil
}
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

func scanMoleculeWithScore(row scanner) (*molecule.MoleculeWithScore, error) {
	m := &molecule.Molecule{}
	ms := &molecule.MoleculeWithScore{Molecule: m}
	var meta []byte
	var statusStr string

	// Columns match the SELECT m.*, score
	err := row.Scan(
		&m.ID, &m.SMILES, &m.CanonicalSMILES, &m.InChI, &m.InChIKey, &m.MolecularFormula, &m.MolecularWeight,
		&m.ExactMass, &m.LogP, &m.TPSA, &m.NumAtoms, &m.NumBonds, &m.NumRings, &m.NumAromaticRings,
		&m.NumRotatableBonds, &statusStr, &m.Name, pq.Array(&m.Aliases), &m.Source, &m.SourceReference, &meta,
		&m.CreatedAt, &m.UpdatedAt, &m.DeletedAt, &ms.Score,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New(errors.ErrCodeNotFound, "molecule not found")
		}
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to scan molecule with score")
	}
	m.Status = molecule.MoleculeStatus(statusStr)
	if len(meta) > 0 { _ = json.Unmarshal(meta, &m.Metadata) }
	return ms, nil
}

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
		ms, err := scanMoleculeWithScore(rows)
		if err != nil { return nil, err }
		results = append(results, ms)
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
