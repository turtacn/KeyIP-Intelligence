package repositories

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	appErrors "github.com/turtacn/KeyIP-Intelligence/pkg/errors"
	"github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
	moleculeTypes "github.com/turtacn/KeyIP-Intelligence/pkg/types/molecule"
)

// ─────────────────────────────────────────────────────────────────────────────
// Domain entity — local mirror of internal/domain/molecule
// ─────────────────────────────────────────────────────────────────────────────

// Molecule is the aggregate root for the molecule domain.
type Molecule struct {
	ID               common.ID
	TenantID         common.TenantID
	SMILES           string
	CanonicalSMILES  string
	InChI            string
	InChIKey         string
	MolecularFormula string
	MolecularWeight  float64
	Name             string
	Synonyms         []string
	Type             moleculeTypes.MoleculeType
	Properties       moleculeTypes.MolecularProperties
	Fingerprints     map[moleculeTypes.FingerprintType][]byte
	SourcePatentIDs  []common.ID
	Metadata         map[string]interface{}
	CreatedAt        time.Time
	UpdatedAt        time.Time
	CreatedBy        common.UserID
	Version          int
}

// MoleculeSearchCriteria carries dynamic filter parameters for molecule search.
type MoleculeSearchCriteria struct {
	Name     string
	Type     string
	Page     int
	PageSize int
}

// ─────────────────────────────────────────────────────────────────────────────
// MoleculeRepository
// ─────────────────────────────────────────────────────────────────────────────

// MoleculeRepository is the PostgreSQL implementation of the molecule domain's
// Repository interface.
type MoleculeRepository struct {
	pool   *pgxpool.Pool
	logger Logger
}

// NewMoleculeRepository constructs a ready-to-use MoleculeRepository.
func NewMoleculeRepository(pool *pgxpool.Pool, logger Logger) *MoleculeRepository {
	return &MoleculeRepository{pool: pool, logger: logger}
}

// ─────────────────────────────────────────────────────────────────────────────
// Save
// ─────────────────────────────────────────────────────────────────────────────

// Save persists a single Molecule.  Fingerprints and Properties are serialised
// as JSONB columns.
func (r *MoleculeRepository) Save(ctx context.Context, m *Molecule) error {
	r.logger.Debug("MoleculeRepository.Save", "molecule_id", m.ID)

	propsJSON, _ := json.Marshal(m.Properties)
	fpJSON, _ := json.Marshal(m.Fingerprints)
	metaJSON, _ := json.Marshal(m.Metadata)

	_, err := r.pool.Exec(ctx, `
		INSERT INTO molecules (
			id, tenant_id, smiles, canonical_smiles, inchi, inchi_key,
			molecular_formula, molecular_weight, name, synonyms,
			type, properties, fingerprints, source_patent_ids,
			metadata, created_at, updated_at, created_by, version
		) VALUES (
			$1,$2,$3,$4,$5,$6,
			$7,$8,$9,$10,
			$11,$12,$13,$14,
			$15,$16,$17,$18,$19
		)`,
		m.ID, m.TenantID, m.SMILES, m.CanonicalSMILES, m.InChI, m.InChIKey,
		m.MolecularFormula, m.MolecularWeight, m.Name, m.Synonyms,
		m.Type, propsJSON, fpJSON, m.SourcePatentIDs,
		metaJSON, m.CreatedAt, m.UpdatedAt, m.CreatedBy, m.Version,
	)
	if err != nil {
		r.logger.Error("MoleculeRepository.Save", "error", err)
		return appErrors.Wrap(err, appErrors.CodeDBQueryError, "failed to insert molecule")
	}
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// BatchSave — high-throughput bulk insert via pgx.CopyFrom
// ─────────────────────────────────────────────────────────────────────────────

// BatchSave inserts multiple molecules in a single round-trip using the
// PostgreSQL COPY protocol for maximum throughput.
func (r *MoleculeRepository) BatchSave(ctx context.Context, molecules []*Molecule) error {
	r.logger.Debug("MoleculeRepository.BatchSave", "count", len(molecules))

	if len(molecules) == 0 {
		return nil
	}

	columns := []string{
		"id", "tenant_id", "smiles", "canonical_smiles", "inchi", "inchi_key",
		"molecular_formula", "molecular_weight", "name", "synonyms",
		"type", "properties", "fingerprints", "source_patent_ids",
		"metadata", "created_at", "updated_at", "created_by", "version",
	}

	rows := make([][]interface{}, 0, len(molecules))
	for _, m := range molecules {
		propsJSON, _ := json.Marshal(m.Properties)
		fpJSON, _ := json.Marshal(m.Fingerprints)
		metaJSON, _ := json.Marshal(m.Metadata)

		rows = append(rows, []interface{}{
			m.ID, m.TenantID, m.SMILES, m.CanonicalSMILES, m.InChI, m.InChIKey,
			m.MolecularFormula, m.MolecularWeight, m.Name, m.Synonyms,
			string(m.Type), propsJSON, fpJSON, m.SourcePatentIDs,
			metaJSON, m.CreatedAt, m.UpdatedAt, m.CreatedBy, m.Version,
		})
	}

	copyCount, err := r.pool.CopyFrom(
		ctx,
		pgx.Identifier{"molecules"},
		columns,
		pgx.CopyFromRows(rows),
	)
	if err != nil {
		r.logger.Error("MoleculeRepository.BatchSave", "error", err)
		return appErrors.Wrap(err, appErrors.CodeDBQueryError, "failed to batch insert molecules")
	}

	r.logger.Debug("MoleculeRepository.BatchSave: done", "inserted", copyCount)
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// FindByID
// ─────────────────────────────────────────────────────────────────────────────

func (r *MoleculeRepository) FindByID(ctx context.Context, id common.ID) (*Molecule, error) {
	r.logger.Debug("MoleculeRepository.FindByID", "id", id)

	return r.scanMolecule(r.pool.QueryRow(ctx, `
		SELECT id, tenant_id, smiles, canonical_smiles, inchi, inchi_key,
		       molecular_formula, molecular_weight, name, synonyms,
		       type, properties, fingerprints, source_patent_ids,
		       metadata, created_at, updated_at, created_by, version
		FROM molecules WHERE id = $1`, id))
}

// ─────────────────────────────────────────────────────────────────────────────
// FindBySMILES
// ─────────────────────────────────────────────────────────────────────────────

func (r *MoleculeRepository) FindBySMILES(ctx context.Context, canonicalSMILES string) (*Molecule, error) {
	r.logger.Debug("MoleculeRepository.FindBySMILES", "smiles", canonicalSMILES)

	return r.scanMolecule(r.pool.QueryRow(ctx, `
		SELECT id, tenant_id, smiles, canonical_smiles, inchi, inchi_key,
		       molecular_formula, molecular_weight, name, synonyms,
		       type, properties, fingerprints, source_patent_ids,
		       metadata, created_at, updated_at, created_by, version
		FROM molecules WHERE canonical_smiles = $1`, canonicalSMILES))
}

// ─────────────────────────────────────────────────────────────────────────────
// FindByInChIKey
// ─────────────────────────────────────────────────────────────────────────────

func (r *MoleculeRepository) FindByInChIKey(ctx context.Context, inchiKey string) (*Molecule, error) {
	r.logger.Debug("MoleculeRepository.FindByInChIKey", "inchi_key", inchiKey)

	return r.scanMolecule(r.pool.QueryRow(ctx, `
		SELECT id, tenant_id, smiles, canonical_smiles, inchi, inchi_key,
		       molecular_formula, molecular_weight, name, synonyms,
		       type, properties, fingerprints, source_patent_ids,
		       metadata, created_at, updated_at, created_by, version
		FROM molecules WHERE inchi_key = $1`, inchiKey))
}

// ─────────────────────────────────────────────────────────────────────────────
// Search — name fuzzy + type filter
// ─────────────────────────────────────────────────────────────────────────────

func (r *MoleculeRepository) Search(ctx context.Context, criteria MoleculeSearchCriteria) ([]*Molecule, int64, error) {
	r.logger.Debug("MoleculeRepository.Search", "criteria", criteria)

	var (
		conditions []string
		args       []interface{}
		argIdx     int
	)

	nextArg := func(v interface{}) string {
		argIdx++
		args = append(args, v)
		return fmt.Sprintf("$%d", argIdx)
	}

	if criteria.Name != "" {
		ph := nextArg("%" + strings.ToLower(criteria.Name) + "%")
		conditions = append(conditions, fmt.Sprintf("LOWER(name) LIKE %s", ph))
	}

	if criteria.Type != "" {
		ph := nextArg(criteria.Type)
		conditions = append(conditions, fmt.Sprintf("type = %s", ph))
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	var total int64
	if err := r.pool.QueryRow(ctx,
		fmt.Sprintf("SELECT COUNT(*) FROM molecules %s", whereClause), args...,
	).Scan(&total); err != nil {
		r.logger.Error("MoleculeRepository.Search: count", "error", err)
		return nil, 0, appErrors.Wrap(err, appErrors.CodeDBQueryError, "count failed")
	}

	pageSize := criteria.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	page := criteria.Page
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * pageSize

	phLimit := nextArg(pageSize)
	phOffset := nextArg(offset)

	dataSQL := fmt.Sprintf(`
		SELECT id, tenant_id, smiles, canonical_smiles, inchi, inchi_key,
		       molecular_formula, molecular_weight, name, synonyms,
		       type, properties, fingerprints, source_patent_ids,
		       metadata, created_at, updated_at, created_by, version
		FROM molecules %s
		ORDER BY created_at DESC
		LIMIT %s OFFSET %s`, whereClause, phLimit, phOffset)

	rows, err := r.pool.Query(ctx, dataSQL, args...)
	if err != nil {
		r.logger.Error("MoleculeRepository.Search: query", "error", err)
		return nil, 0, appErrors.Wrap(err, appErrors.CodeDBQueryError, "search query failed")
	}
	defer rows.Close()

	molecules, err := r.scanMolecules(rows)
	return molecules, total, err
}

// ─────────────────────────────────────────────────────────────────────────────
// FindSimilar — PostgreSQL fallback (degraded mode when Milvus unavailable)
// ─────────────────────────────────────────────────────────────────────────────

// FindSimilar performs a similarity search.  In production the heavy lifting is
// done by Milvus; this PostgreSQL implementation serves as a degraded fallback.
// It pre-filters candidates by type, loads them into memory, and computes
// Tanimoto similarity on Morgan fingerprints.
func (r *MoleculeRepository) FindSimilar(
	ctx context.Context,
	targetFingerprint []byte,
	threshold float64,
	moleculeType string,
	limit int,
) ([]*Molecule, error) {
	r.logger.Debug("MoleculeRepository.FindSimilar",
		"threshold", threshold, "type", moleculeType, "limit", limit)

	var (
		whereClause string
		args        []interface{}
	)
	if moleculeType != "" {
		whereClause = "WHERE type = $1"
		args = append(args, moleculeType)
	}

	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id, tenant_id, smiles, canonical_smiles, inchi, inchi_key,
		       molecular_formula, molecular_weight, name, synonyms,
		       type, properties, fingerprints, source_patent_ids,
		       metadata, created_at, updated_at, created_by, version
		FROM molecules %s`, whereClause), args...)
	if err != nil {
		r.logger.Error("MoleculeRepository.FindSimilar: query", "error", err)
		return nil, appErrors.Wrap(err, appErrors.CodeDBQueryError, "failed to load candidates")
	}
	defer rows.Close()

	candidates, err := r.scanMolecules(rows)
	if err != nil {
		return nil, err
	}

	// In-memory Tanimoto similarity on raw fingerprint bytes.
	type scored struct {
		mol   *Molecule
		score float64
	}
	var hits []scored
	for _, c := range candidates {
		for _, fp := range c.Fingerprints {
			sim := tanimotoSimilarity(targetFingerprint, fp)
			if sim >= threshold {
				hits = append(hits, scored{mol: c, score: sim})
				break
			}
		}
	}

	// Sort descending by score.
	for i := 0; i < len(hits); i++ {
		for j := i + 1; j < len(hits); j++ {
			if hits[j].score > hits[i].score {
				hits[i], hits[j] = hits[j], hits[i]
			}
		}
	}

	if limit > 0 && len(hits) > limit {
		hits = hits[:limit]
	}

	result := make([]*Molecule, len(hits))
	for i, h := range hits {
		result[i] = h.mol
	}
	return result, nil
}

// tanimotoSimilarity computes the Tanimoto coefficient between two binary
// fingerprint byte slices.  Returns 0.0 if either is empty.
func tanimotoSimilarity(a, b []byte) float64 {
	if len(a) == 0 || len(b) == 0 {
		return 0.0
	}
	minLen := len(a)
	if len(b) < minLen {
		minLen = len(b)
	}

	var andBits, orBits int
	for i := 0; i < minLen; i++ {
		andBits += popcount(a[i] & b[i])
		orBits += popcount(a[i] | b[i])
	}
	// Remaining bytes in the longer slice contribute only to OR.
	for i := minLen; i < len(a); i++ {
		orBits += popcount(a[i])
	}
	for i := minLen; i < len(b); i++ {
		orBits += popcount(b[i])
	}

	if orBits == 0 {
		return 0.0
	}
	return float64(andBits) / float64(orBits)
}

// popcount returns the number of set bits in a byte.
func popcount(b byte) int {
	count := 0
	for b != 0 {
		count += int(b & 1)
		b >>= 1
	}
	return count
}

// ─────────────────────────────────────────────────────────────────────────────
// SubstructureSearch — simplified in-memory SMARTS matching
// ─────────────────────────────────────────────────────────────────────────────

// SubstructureSearch performs a simplified substructure search.  In production
// this should use the RDKit PostgreSQL cartridge; this implementation loads
// candidates and does a naive SMARTS substring check as a placeholder.
func (r *MoleculeRepository) SubstructureSearch(
	ctx context.Context,
	smartsPattern string,
	page, pageSize int,
) ([]*Molecule, int64, error) {
	r.logger.Debug("MoleculeRepository.SubstructureSearch", "smarts", smartsPattern)

	rows, err := r.pool.Query(ctx, `
		SELECT id, tenant_id, smiles, canonical_smiles, inchi, inchi_key,
		       molecular_formula, molecular_weight, name, synonyms,
		       type, properties, fingerprints, source_patent_ids,
		       metadata, created_at, updated_at, created_by, version
		FROM molecules`)
	if err != nil {
		r.logger.Error("MoleculeRepository.SubstructureSearch: query", "error", err)
		return nil, 0, appErrors.Wrap(err, appErrors.CodeDBQueryError, "failed to load molecules")
	}
	defer rows.Close()

	all, err := r.scanMolecules(rows)
	if err != nil {
		return nil, 0, err
	}

	// Naive substring match on canonical SMILES as a placeholder for real
	// SMARTS matching (production should use RDKit cartridge).
	var matched []*Molecule
	for _, m := range all {
		if strings.Contains(m.CanonicalSMILES, smartsPattern) ||
			strings.Contains(m.SMILES, smartsPattern) {
			matched = append(matched, m)
		}
	}

	total := int64(len(matched))

	if pageSize <= 0 {
		pageSize = 20
	}
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * pageSize
	end := offset + pageSize
	if offset > len(matched) {
		return []*Molecule{}, total, nil
	}
	if end > len(matched) {
		end = len(matched)
	}

	return matched[offset:end], total, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// FindByPatentID
// ─────────────────────────────────────────────────────────────────────────────

// FindByPatentID returns all molecules extracted from a given patent, using
// the PostgreSQL array-contains operator on source_patent_ids.
func (r *MoleculeRepository) FindByPatentID(ctx context.Context, patentID common.ID) ([]*Molecule, error) {
	r.logger.Debug("MoleculeRepository.FindByPatentID", "patent_id", patentID)

	rows, err := r.pool.Query(ctx, `
		SELECT id, tenant_id, smiles, canonical_smiles, inchi, inchi_key,
		       molecular_formula, molecular_weight, name, synonyms,
		       type, properties, fingerprints, source_patent_ids,
		       metadata, created_at, updated_at, created_by, version
		FROM molecules
		WHERE source_patent_ids @> ARRAY[$1]::TEXT[]`, string(patentID))
	if err != nil {
		r.logger.Error("MoleculeRepository.FindByPatentID", "error", err)
		return nil, appErrors.Wrap(err, appErrors.CodeDBQueryError, "failed to query molecules by patent")
	}
	defer rows.Close()

	return r.scanMolecules(rows)
}

// ─────────────────────────────────────────────────────────────────────────────
// Count
// ─────────────────────────────────────────────────────────────────────────────

func (r *MoleculeRepository) Count(ctx context.Context) (int64, error) {
	r.logger.Debug("MoleculeRepository.Count")

	var count int64
	if err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM molecules`).Scan(&count); err != nil {
		r.logger.Error("MoleculeRepository.Count", "error", err)
		return 0, appErrors.Wrap(err, appErrors.CodeDBQueryError, "failed to count molecules")
	}
	return count, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Update
// ─────────────────────────────────────────────────────────────────────────────

func (r *MoleculeRepository) Update(ctx context.Context, m *Molecule) error {
	r.logger.Debug("MoleculeRepository.Update", "molecule_id", m.ID, "version", m.Version)

	propsJSON, _ := json.Marshal(m.Properties)
	fpJSON, _ := json.Marshal(m.Fingerprints)
	metaJSON, _ := json.Marshal(m.Metadata)
	newVersion := m.Version + 1

	tag, err := r.pool.Exec(ctx, `
		UPDATE molecules SET
			smiles=$1, canonical_smiles=$2, inchi=$3, inchi_key=$4,
			molecular_formula=$5, molecular_weight=$6, name=$7, synonyms=$8,
			type=$9, properties=$10, fingerprints=$11, source_patent_ids=$12,
			metadata=$13, updated_at=$14, version=$15
		WHERE id=$16 AND version=$17`,
		m.SMILES, m.CanonicalSMILES, m.InChI, m.InChIKey,
		m.MolecularFormula, m.MolecularWeight, m.Name, m.Synonyms,
		m.Type, propsJSON, fpJSON, m.SourcePatentIDs,
		metaJSON, time.Now().UTC(), newVersion,
		m.ID, m.Version,
	)
	if err != nil {
		r.logger.Error("MoleculeRepository.Update", "error", err)
		return appErrors.Wrap(err, appErrors.CodeDBQueryError, "failed to update molecule")
	}
	if tag.RowsAffected() == 0 {
		return appErrors.New(appErrors.CodeConflict, "optimistic lock conflict: molecule version mismatch")
	}
	m.Version = newVersion
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Delete
// ─────────────────────────────────────────────────────────────────────────────

func (r *MoleculeRepository) Delete(ctx context.Context, id common.ID) error {
	r.logger.Debug("MoleculeRepository.Delete", "id", id)

	tag, err := r.pool.Exec(ctx, `DELETE FROM molecules WHERE id = $1`, id)
	if err != nil {
		r.logger.Error("MoleculeRepository.Delete", "error", err)
		return appErrors.Wrap(err, appErrors.CodeDBQueryError, "failed to delete molecule")
	}
	if tag.RowsAffected() == 0 {
		return appErrors.New(appErrors.CodeNotFound, "molecule not found")
	}
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Internal scanners
// ─────────────────────────────────────────────────────────────────────────────

func (r *MoleculeRepository) scanMolecule(row pgx.Row) (*Molecule, error) {
	var m Molecule
	var propsJSON, fpJSON, metaJSON []byte

	err := row.Scan(
		&m.ID, &m.TenantID, &m.SMILES, &m.CanonicalSMILES, &m.InChI, &m.InChIKey,
		&m.MolecularFormula, &m.MolecularWeight, &m.Name, &m.Synonyms,
		&m.Type, &propsJSON, &fpJSON, &m.SourcePatentIDs,
		&metaJSON, &m.CreatedAt, &m.UpdatedAt, &m.CreatedBy, &m.Version,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, appErrors.New(appErrors.CodeNotFound, "molecule not found")
		}
		r.logger.Error("scanMolecule", "error", err)
		return nil, appErrors.Wrap(err, appErrors.CodeDBQueryError, "failed to scan molecule")
	}

	if len(propsJSON) > 0 {
		_ = json.Unmarshal(propsJSON, &m.Properties)
	}
	if len(fpJSON) > 0 {
		_ = json.Unmarshal(fpJSON, &m.Fingerprints)
	}
	if len(metaJSON) > 0 {
		_ = json.Unmarshal(metaJSON, &m.Metadata)
	}
	return &m, nil
}

func (r *MoleculeRepository) scanMolecules(rows pgx.Rows) ([]*Molecule, error) {
	var molecules []*Molecule
	for rows.Next() {
		var m Molecule
		var propsJSON, fpJSON, metaJSON []byte

		err := rows.Scan(
			&m.ID, &m.TenantID, &m.SMILES, &m.CanonicalSMILES, &m.InChI, &m.InChIKey,
			&m.MolecularFormula, &m.MolecularWeight, &m.Name, &m.Synonyms,
			&m.Type, &propsJSON, &fpJSON, &m.SourcePatentIDs,
			&metaJSON, &m.CreatedAt, &m.UpdatedAt, &m.CreatedBy, &m.Version,
		)
		if err != nil {
			r.logger.Error("scanMolecules", "error", err)
			return nil, appErrors.Wrap(err, appErrors.CodeDBQueryError, "failed to scan molecule row")
		}

		if len(propsJSON) > 0 {
			_ = json.Unmarshal(propsJSON, &m.Properties)
		}
		if len(fpJSON) > 0 {
			_ = json.Unmarshal(fpJSON, &m.Fingerprints)
		}
		if len(metaJSON) > 0 {
			_ = json.Unmarshal(metaJSON, &m.Metadata)
		}
		molecules = append(molecules, &m)
	}
	if err := rows.Err(); err != nil {
		return nil, appErrors.Wrap(err, appErrors.CodeDBQueryError, "row iteration error")
	}
	return molecules, nil
}

//Personal.AI order the ending
