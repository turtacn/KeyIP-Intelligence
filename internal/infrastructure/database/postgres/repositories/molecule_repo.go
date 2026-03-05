package repositories

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

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
func (r *postgresMoleculeRepo) FindByIDs(ctx context.Context, ids []string) ([]*molecule.Molecule, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	query := `SELECT * FROM molecules WHERE id = ANY($1) AND deleted_at IS NULL`
	rows, err := r.executor().QueryContext(ctx, query, pq.Array(ids))
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to query by ids")
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

func (r *postgresMoleculeRepo) Exists(ctx context.Context, idStr string) (bool, error) {
	id, err := uuid.Parse(idStr)
	if err != nil {
		return false, errors.New(errors.ErrCodeValidation, "invalid uuid")
	}
	query := `SELECT EXISTS(SELECT 1 FROM molecules WHERE id = $1 AND deleted_at IS NULL)`
	var exists bool
	err = r.executor().QueryRowContext(ctx, query, id).Scan(&exists)
	if err != nil {
		return false, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to check existence")
	}
	return exists, nil
}

func (r *postgresMoleculeRepo) ExistsByInChIKey(ctx context.Context, inchiKey string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM molecules WHERE inchi_key = $1 AND deleted_at IS NULL)`
	var exists bool
	err := r.executor().QueryRowContext(ctx, query, inchiKey).Scan(&exists)
	if err != nil {
		return false, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to check existence by inchi key")
	}
	return exists, nil
}
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

	for i := range query.PropertyFilters {
		sb.WriteString(fmt.Sprintf("JOIN molecule_properties mp%d ON m.id = mp%d.molecule_id ", i, i))
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

	for i, pf := range query.PropertyFilters {
		sb.WriteString(fmt.Sprintf("AND mp%d.property_type = $%d ", i, argIdx))
		args = append(args, pf.Name)
		argIdx++
		if pf.MinValue != nil {
			sb.WriteString(fmt.Sprintf("AND mp%d.value >= $%d ", i, argIdx))
			args = append(args, *pf.MinValue)
			argIdx++
		}
		if pf.MaxValue != nil {
			sb.WriteString(fmt.Sprintf("AND mp%d.value <= $%d ", i, argIdx))
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
func (r *postgresMoleculeRepo) FindBySource(ctx context.Context, source molecule.MoleculeSource, offset, limit int) ([]*molecule.Molecule, error) {
	query := `SELECT * FROM molecules WHERE source = $1 AND deleted_at IS NULL ORDER BY created_at DESC LIMIT $2 OFFSET $3`
	rows, err := r.executor().QueryContext(ctx, query, source, limit, offset)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to query by source")
	}
	defer rows.Close()

	var mols []*molecule.Molecule
	for rows.Next() {
		m, err := scanMolecule(rows)
		if err != nil { return nil, err }
		mols = append(mols, m)
	}
	return mols, nil
}

func (r *postgresMoleculeRepo) FindByStatus(ctx context.Context, status molecule.MoleculeStatus, offset, limit int) ([]*molecule.Molecule, error) {
	query := `SELECT * FROM molecules WHERE status = $1 AND deleted_at IS NULL ORDER BY created_at DESC LIMIT $2 OFFSET $3`
	rows, err := r.executor().QueryContext(ctx, query, status, limit, offset)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to query by status")
	}
	defer rows.Close()

	var mols []*molecule.Molecule
	for rows.Next() {
		m, err := scanMolecule(rows)
		if err != nil { return nil, err }
		mols = append(mols, m)
	}
	return mols, nil
}

func (r *postgresMoleculeRepo) FindByTags(ctx context.Context, tags []string, offset, limit int) ([]*molecule.Molecule, error) {
	if len(tags) == 0 {
		return nil, nil
	}
	query := `SELECT * FROM molecules WHERE tags @> $1 AND deleted_at IS NULL ORDER BY created_at DESC LIMIT $2 OFFSET $3`
	rows, err := r.executor().QueryContext(ctx, query, pq.Array(tags), limit, offset)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to query by tags")
	}
	defer rows.Close()

	var mols []*molecule.Molecule
	for rows.Next() {
		m, err := scanMolecule(rows)
		if err != nil { return nil, err }
		mols = append(mols, m)
	}
	return mols, nil
}

func (r *postgresMoleculeRepo) FindByMolecularWeightRange(ctx context.Context, minWeight, maxWeight float64, offset, limit int) ([]*molecule.Molecule, error) {
	query := `SELECT * FROM molecules WHERE molecular_weight >= $1 AND molecular_weight <= $2 AND deleted_at IS NULL ORDER BY molecular_weight ASC LIMIT $3 OFFSET $4`
	rows, err := r.executor().QueryContext(ctx, query, minWeight, maxWeight, limit, offset)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to query by weight")
	}
	defer rows.Close()

	var mols []*molecule.Molecule
	for rows.Next() {
		m, err := scanMolecule(rows)
		if err != nil { return nil, err }
		mols = append(mols, m)
	}
	return mols, nil
}

func (r *postgresMoleculeRepo) FindWithFingerprint(ctx context.Context, fpType molecule.FingerprintType, offset, limit int) ([]*molecule.Molecule, error) {
	query := `
		SELECT m.* FROM molecules m
		JOIN molecule_fingerprints mf ON m.id = mf.molecule_id
		WHERE mf.fingerprint_type = $1 AND m.deleted_at IS NULL
		ORDER BY m.created_at DESC LIMIT $2 OFFSET $3
	`
	rows, err := r.executor().QueryContext(ctx, query, fpType, limit, offset)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to query with fingerprint")
	}
	defer rows.Close()

	var mols []*molecule.Molecule
	for rows.Next() {
		m, err := scanMolecule(rows)
		if err != nil { return nil, err }
		mols = append(mols, m)
	}
	return mols, nil
}

func (r *postgresMoleculeRepo) FindWithoutFingerprint(ctx context.Context, fpType molecule.FingerprintType, offset, limit int) ([]*molecule.Molecule, error) {
	query := `
		SELECT m.* FROM molecules m
		LEFT JOIN molecule_fingerprints mf ON m.id = mf.molecule_id AND mf.fingerprint_type = $1
		WHERE mf.molecule_id IS NULL AND m.deleted_at IS NULL
		ORDER BY m.created_at DESC LIMIT $2 OFFSET $3
	`
	rows, err := r.executor().QueryContext(ctx, query, fpType, limit, offset)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to query without fingerprint")
	}
	defer rows.Close()

	var mols []*molecule.Molecule
	for rows.Next() {
		m, err := scanMolecule(rows)
		if err != nil { return nil, err }
		mols = append(mols, m)
	}
	return mols, nil
}

func (r *postgresMoleculeRepo) Delete(ctx context.Context, idStr string) error {
	id, err := uuid.Parse(idStr)
	if err != nil {
		return errors.New(errors.ErrCodeValidation, "invalid uuid")
	}
	query := `UPDATE molecules SET deleted_at = NOW() WHERE id = $1`
	_, err = r.executor().ExecContext(ctx, query, id)
	return err
}
func (r *postgresMoleculeRepo) AddProperty(ctx context.Context, prop *molecule.Property) error {
	query := `
		INSERT INTO molecule_properties (
			molecule_id, property_type, value, unit, measurement_conditions, data_source, confidence, source_reference
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8
		) RETURNING id, created_at
	`
	conds, _ := json.Marshal(prop.MeasurementConditions)
	err := r.executor().QueryRowContext(ctx, query,
		prop.MoleculeID, prop.Type, prop.Value, prop.Unit, conds, prop.DataSource, prop.Confidence, prop.SourceReference,
	).Scan(&prop.ID, &prop.CreatedAt)
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to add property")
	}
	return nil
}

func scanProperty(row scanner) (*molecule.Property, error) {
	p := &molecule.Property{}
	var conds []byte
	err := row.Scan(
		&p.ID, &p.MoleculeID, &p.Type, &p.Value, &p.Unit, &conds, &p.DataSource, &p.Confidence, &p.SourceReference, &p.CreatedAt,
	)
	if err != nil { return nil, err }
	if len(conds) > 0 { _ = json.Unmarshal(conds, &p.MeasurementConditions) }
	p.Name = p.Type // alias
	return p, nil
}

func (r *postgresMoleculeRepo) GetProperties(ctx context.Context, moleculeID uuid.UUID, propertyTypes []string) ([]*molecule.Property, error) {
	query := `SELECT id, molecule_id, property_type, value, unit, measurement_conditions, data_source, confidence, source_reference, created_at FROM molecule_properties WHERE molecule_id = $1`
	var args []interface{}
	args = append(args, moleculeID)

	if len(propertyTypes) > 0 {
		query += ` AND property_type = ANY($2)`
		args = append(args, pq.Array(propertyTypes))
	}
	query += ` ORDER BY created_at DESC`

	rows, err := r.executor().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to get properties")
	}
	defer rows.Close()

	var props []*molecule.Property
	for rows.Next() {
		p, err := scanProperty(rows)
		if err != nil { return nil, err }
		props = append(props, p)
	}
	return props, nil
}

func (r *postgresMoleculeRepo) GetPropertiesByType(ctx context.Context, propertyType string, minValue, maxValue float64, limit, offset int) ([]*molecule.Property, int64, error) {
	countQuery := `SELECT COUNT(*) FROM molecule_properties WHERE property_type = $1 AND value >= $2 AND value <= $3`
	var count int64
	err := r.executor().QueryRowContext(ctx, countQuery, propertyType, minValue, maxValue).Scan(&count)
	if err != nil {
		return nil, 0, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to count properties")
	}

	query := `
		SELECT id, molecule_id, property_type, value, unit, measurement_conditions, data_source, confidence, source_reference, created_at
		FROM molecule_properties
		WHERE property_type = $1 AND value >= $2 AND value <= $3
		ORDER BY value ASC
		LIMIT $4 OFFSET $5
	`
	rows, err := r.executor().QueryContext(ctx, query, propertyType, minValue, maxValue, limit, offset)
	if err != nil {
		return nil, 0, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to query properties")
	}
	defer rows.Close()

	var props []*molecule.Property
	for rows.Next() {
		p, err := scanProperty(rows)
		if err != nil { return nil, 0, err }
		props = append(props, p)
	}
	return props, count, nil
}

func (r *postgresMoleculeRepo) LinkToPatent(ctx context.Context, rel *molecule.PatentRelation) error {
	query := `
		INSERT INTO patent_molecule_relations (
			patent_id, molecule_id, relation_type, location_in_patent, page_reference, claim_numbers, extraction_method, confidence
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8
		) ON CONFLICT (patent_id, molecule_id, relation_type) DO UPDATE SET
			location_in_patent = EXCLUDED.location_in_patent,
			page_reference = EXCLUDED.page_reference,
			claim_numbers = EXCLUDED.claim_numbers,
			extraction_method = EXCLUDED.extraction_method,
			confidence = EXCLUDED.confidence
		RETURNING id, created_at
	`
	// Check if claims numbers should be pq.Array mapped, in Go it is usually []int64 mapped via pq.Array
	err := r.executor().QueryRowContext(ctx, query,
		rel.PatentID, rel.MoleculeID, rel.RelationType, rel.LocationInPatent, rel.PageReference, pq.Array(rel.ClaimNumbers), rel.ExtractionMethod, rel.Confidence,
	).Scan(&rel.ID, &rel.CreatedAt)
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to link molecule to patent")
	}
	return nil
}

func (r *postgresMoleculeRepo) UnlinkFromPatent(ctx context.Context, patentID, moleculeID uuid.UUID, relationType string) error {
	query := `DELETE FROM patent_molecule_relations WHERE patent_id = $1 AND molecule_id = $2 AND relation_type = $3`
	_, err := r.executor().ExecContext(ctx, query, patentID, moleculeID, relationType)
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to unlink molecule from patent")
	}
	return nil
}

func scanPatentRelation(row scanner) (*molecule.PatentRelation, error) {
	r := &molecule.PatentRelation{}
	var claims []int64
	err := row.Scan(
		&r.ID, &r.PatentID, &r.MoleculeID, &r.RelationType, &r.LocationInPatent, &r.PageReference, pq.Array(&claims), &r.ExtractionMethod, &r.Confidence, &r.CreatedAt,
	)
	if err != nil { return nil, err }
	r.ClaimNumbers = claims
	return r, nil
}

func (r *postgresMoleculeRepo) GetPatentRelations(ctx context.Context, moleculeID uuid.UUID) ([]*molecule.PatentRelation, error) {
	query := `
		SELECT id, patent_id, molecule_id, relation_type, location_in_patent, page_reference, claim_numbers, extraction_method, confidence, created_at
		FROM patent_molecule_relations
		WHERE molecule_id = $1
		ORDER BY created_at DESC
	`
	rows, err := r.executor().QueryContext(ctx, query, moleculeID)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to get patent relations")
	}
	defer rows.Close()

	var rels []*molecule.PatentRelation
	for rows.Next() {
		rel, err := scanPatentRelation(rows)
		if err != nil { return nil, err }
		rels = append(rels, rel)
	}
	return rels, nil
}

func (r *postgresMoleculeRepo) GetMoleculesByPatent(ctx context.Context, patentID uuid.UUID, relationType *string) ([]*molecule.Molecule, error) {
	var query string
	var args []interface{}
	args = append(args, patentID)

	if relationType != nil {
		query = `
			SELECT m.* FROM molecules m
			JOIN patent_molecule_relations r ON m.id = r.molecule_id
			WHERE r.patent_id = $1 AND r.relation_type = $2 AND m.deleted_at IS NULL
		`
		args = append(args, *relationType)
	} else {
		query = `
			SELECT m.* FROM molecules m
			JOIN patent_molecule_relations r ON m.id = r.molecule_id
			WHERE r.patent_id = $1 AND m.deleted_at IS NULL
		`
	}

	rows, err := r.executor().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to get molecules by patent")
	}
	defer rows.Close()

	var mols []*molecule.Molecule
	for rows.Next() {
		m, err := scanMolecule(rows)
		if err != nil { return nil, err }
		mols = append(mols, m)
	}
	return mols, nil
}
func (r *postgresMoleculeRepo) SearchBySubstructure(ctx context.Context, smarts string, limit, offset int) ([]*molecule.Molecule, int64, error) {
	// Typically requires pgchem or rdkit extension for genuine SMARTS substructure search
	// SELECT m.* FROM molecules m WHERE m.smiles@>$1; (using RDKit cartridge operator)
	// Without explicit extension setup, returning an error pointing to extension needs
	return nil, 0, errors.New(errors.ErrCodeInvalidOperation, "substructure search requires RDKit extension setup, unimplemented in standard pg layer")
}

func (r *postgresMoleculeRepo) SearchBySimilarity(ctx context.Context, targetFP []byte, fpType string, threshold float64, limit, offset int) ([]*molecule.Molecule, int64, error) {
	// Usually requires RDKit cartridge (<% operator for Tanimoto) or pgvector (<=> operator for Cosine)
	// Without explicit extension, returning an error.
	return nil, 0, errors.New(errors.ErrCodeInvalidOperation, "similarity search implemented via pgvector in SearchByVectorSimilarity, or requires RDKit cartridge")
}

func (r *postgresMoleculeRepo) BatchSaveFingerprints(ctx context.Context, fps []*molecule.Fingerprint) error {
	if len(fps) == 0 {
		return nil
	}

	totalAffected := 0
	batchSize := 1000

	for start := 0; start < len(fps); start += batchSize {
		end := start + batchSize
		if end > len(fps) {
			end = len(fps)
		}
		batch := fps[start:end]

		query := `
			INSERT INTO molecule_fingerprints (
				molecule_id, fingerprint_type, fingerprint_bits, fingerprint_vector, fingerprint_hash, parameters, model_version, created_at
			) VALUES
		`

		var values []interface{}
		var placeholders []string

		for i, fp := range batch {
			base := i * 8
			placeholders = append(placeholders, fmt.Sprintf("($%d, $%d, $%d, $%d, $%d, $%d, $%d, $%d)",
				base+1, base+2, base+3, base+4, base+5, base+6, base+7, base+8))

			params, _ := json.Marshal(fp.Parameters)
			var vector interface{}
			if len(fp.Vector) > 0 {
				vector = pgvector.NewVector(fp.Vector)
			} else {
				vector = nil
			}

			values = append(values,
				fp.MoleculeID, fp.Type, fp.Bits, vector, fp.Hash, params, fp.ModelVersion, time.Now(),
			)
		}

		query += strings.Join(placeholders, ",") + ` ON CONFLICT (molecule_id, fingerprint_type, model_version) DO UPDATE SET
			fingerprint_bits = EXCLUDED.fingerprint_bits,
			fingerprint_vector = EXCLUDED.fingerprint_vector,
			fingerprint_hash = EXCLUDED.fingerprint_hash
		`

		res, err := r.executor().ExecContext(ctx, query, values...)
		if err != nil {
			return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to batch save fingerprints")
		}

		rowsAffected, _ := res.RowsAffected()
		totalAffected += int(rowsAffected)
	}

	return nil
}

func (r *postgresMoleculeRepo) CountByStatus(ctx context.Context) (map[molecule.Status]int64, error) {
	query := `SELECT status, COUNT(*) FROM molecules WHERE deleted_at IS NULL GROUP BY status`
	rows, err := r.executor().QueryContext(ctx, query)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to count by status")
	}
	defer rows.Close()

	counts := make(map[molecule.Status]int64)
	for rows.Next() {
		var status string
		var count int64
		if err := rows.Scan(&status, &count); err != nil {
			return nil, err
		}
		counts[molecule.Status(status)] = count
	}
	return counts, nil
}

func (r *postgresMoleculeRepo) GetPropertyDistribution(ctx context.Context, propertyType string, bucketCount int) ([]*molecule.DistributionBucket, error) {
	if bucketCount <= 0 {
		return nil, errors.New(errors.ErrCodeValidation, "bucketCount must be positive")
	}

	query := `
		WITH stats AS (
			SELECT MIN(value) as min_val, MAX(value) as max_val
			FROM molecule_properties
			WHERE property_type = $1
		),
		buckets AS (
			SELECT
				width_bucket(value, stats.min_val, stats.max_val + 0.000001, $2) as bucket,
				COUNT(*) as count,
				MIN(value) as bucket_min,
				MAX(value) as bucket_max
			FROM molecule_properties, stats
			WHERE property_type = $1
			GROUP BY bucket
		)
		SELECT bucket, count, bucket_min, bucket_max
		FROM buckets
		ORDER BY bucket ASC
	`
	rows, err := r.executor().QueryContext(ctx, query, propertyType, bucketCount)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to get property distribution")
	}
	defer rows.Close()

	var buckets []*molecule.DistributionBucket
	for rows.Next() {
		var b int
		var c int64
		var min, max float64
		if err := rows.Scan(&b, &c, &min, &max); err != nil {
			return nil, err
		}
		buckets = append(buckets, &molecule.DistributionBucket{
			Bucket: b,
			Count:  c,
			Min:    min,
			Max:    max,
		})
	}
	return buckets, nil
}

func (r *postgresMoleculeRepo) DeleteFingerprints(ctx context.Context, moleculeID uuid.UUID, fpType string) error {
	query := `DELETE FROM molecule_fingerprints WHERE molecule_id = $1 AND fingerprint_type = $2`
	_, err := r.executor().ExecContext(ctx, query, moleculeID, fpType)
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to delete fingerprints")
	}
	return nil
}

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
