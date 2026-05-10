package repositories

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/turtacn/KeyIP-Intelligence/internal/domain/patent"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/database/postgres"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

type postgresPatentRepo struct {
	conn *postgres.Connection
	tx   *sql.Tx
	log  logging.Logger
}

func NewPostgresPatentRepo(conn *postgres.Connection, log logging.Logger) patent.PatentRepository {
	return &postgresPatentRepo{
		conn: conn,
		log:  log,
	}
}

func (r *postgresPatentRepo) executor() queryExecutor {
	if r.tx != nil {
		return r.tx
	}
	return r.conn.DB()
}

// Patent CRUD

func (r *postgresPatentRepo) Save(ctx context.Context, p *patent.Patent) error {
	// Simple create for now as Create is implemented as Insert.
	// Ideally should check for existence and call Update if exists.
	// But sticking to alias for now.
	return r.Create(ctx, p)
}

func (r *postgresPatentRepo) Create(ctx context.Context, p *patent.Patent) error {
	query := `
		INSERT INTO patents (
			patent_number, title, title_en, abstract, abstract_en, patent_type, status,
			filing_date, publication_date, grant_date, expiry_date, priority_date,
			assignee_id, assignee_name, jurisdiction, ipc_codes, cpc_codes, keyip_tech_codes,
			family_id, application_number, full_text_hash, source, raw_data, metadata
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24
		) RETURNING id, created_at, updated_at
	`
	raw, _ := json.Marshal(p.RawData)
	meta, _ := json.Marshal(p.Metadata)

	err := r.executor().QueryRowContext(ctx, query,
		p.PatentNumber, p.Title, p.TitleEn, p.Abstract, p.AbstractEn, p.Type, p.Status.String(),
		p.FilingDate, p.PublicationDate, p.GrantDate, p.ExpiryDate, p.PriorityDate,
		p.AssigneeID, p.AssigneeName, p.Jurisdiction, pq.Array(p.IPCCodes), pq.Array(p.CPCCodes), pq.Array(p.KeyIPTechCodes),
		p.FamilyID, p.ApplicationNumber, p.FullTextHash, p.Source, raw, meta,
	).Scan(&p.ID, &p.CreatedAt, &p.UpdatedAt)

	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" {
			return errors.Wrap(err, errors.ErrCodePatentAlreadyExists, "patent already exists")
		}
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to create patent")
	}
	return nil
}

func (r *postgresPatentRepo) CountByIPCSection(ctx context.Context) (map[string]int64, error) {
	query := `
		SELECT section, COUNT(*)
		FROM (
			SELECT substring(unnest(ipc_codes) from 1 for 1) as section
			FROM patents
			WHERE deleted_at IS NULL
		) s
		GROUP BY section
	`
	rows, err := r.executor().QueryContext(ctx, query)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to count by IPC section")
	}
	defer rows.Close()

	counts := make(map[string]int64)
	for rows.Next() {
		var section string
		var count int64
		if err := rows.Scan(&section, &count); err != nil {
			return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to scan IPC count")
		}
		counts[section] = count
	}
	return counts, nil
}

func (r *postgresPatentRepo) CountByOffice(ctx context.Context) (map[patent.PatentOffice]int64, error) {
	query := `SELECT jurisdiction, COUNT(*) FROM patents WHERE deleted_at IS NULL GROUP BY jurisdiction`
	rows, err := r.executor().QueryContext(ctx, query)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to count by office")
	}
	defer rows.Close()

	counts := make(map[patent.PatentOffice]int64)
	for rows.Next() {
		var office string
		var count int64
		if err := rows.Scan(&office, &count); err != nil {
			return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to scan office count")
		}
		counts[patent.PatentOffice(office)] = count
	}
	return counts, nil
}

func (r *postgresPatentRepo) FindActiveByIPCCode(ctx context.Context, ipcCode string) ([]*patent.Patent, error) {
	query := `SELECT * FROM patents WHERE $1 = ANY(ipc_codes) AND deleted_at IS NULL AND status IN ('granted', 'published', 'under_examination', 'filed')`
	rows, err := r.executor().QueryContext(ctx, query, ipcCode)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to find active patents by IPC code")
	}
	defer rows.Close()
	return scanPatents(rows)
}

func (r *postgresPatentRepo) FindWithMarkushStructures(ctx context.Context, offset, limit int) ([]*patent.Patent, error) {
	// Markush structures are stored in patent_claims.markush_structures JSONB column.
	// Find patents that have at least one claim with a non-empty markush_structures.
	query := `
		SELECT DISTINCT p.* FROM patents p
		JOIN patent_claims pc ON p.id = pc.patent_id
		WHERE pc.markush_structures IS NOT NULL
		  AND pc.markush_structures != '[]'::jsonb
		  AND pc.markush_structures != 'null'::jsonb
		  AND p.deleted_at IS NULL
		ORDER BY p.filing_date DESC NULLS LAST
		LIMIT $1 OFFSET $2
	`
	rows, err := r.executor().QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to find patents with Markush structures")
	}
	defer rows.Close()
	return scanPatents(rows)
}

func (r *postgresPatentRepo) Exists(ctx context.Context, patentNumber string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM patents WHERE patent_number = $1 AND deleted_at IS NULL)`
	var exists bool
	err := r.executor().QueryRowContext(ctx, query, patentNumber).Scan(&exists)
	return exists, err
}

func (r *postgresPatentRepo) GetByID(ctx context.Context, id uuid.UUID) (*patent.Patent, error) {
	query := `SELECT * FROM patents WHERE id = $1 AND deleted_at IS NULL`
	row := r.executor().QueryRowContext(ctx, query, id)
	p, err := scanPatent(row)
	if err != nil {
		return nil, err
	}

	// Preload Claims
	claims, err := r.GetClaimsByPatent(ctx, id)
	if err == nil {
		claimSet := make(patent.ClaimSet, len(claims))
		for i, c := range claims {
			if c != nil {
				claimSet[i] = *c
			}
		}
		p.Claims = claimSet
	}

	// Preload Inventors
	inventors, err := r.GetInventors(ctx, id)
	if err == nil {
		p.Inventors = inventors
	}

	// Preload PriorityClaims
	pcs, err := r.GetPriorityClaims(ctx, id)
	if err == nil {
		p.PriorityClaims = pcs
	}

	return p, nil
}

func (r *postgresPatentRepo) FindByPatentNumber(ctx context.Context, number string) (*patent.Patent, error) {
	return r.GetByPatentNumber(ctx, number)
}

func (r *postgresPatentRepo) GetByPatentNumber(ctx context.Context, number string) (*patent.Patent, error) {
	query := `SELECT * FROM patents WHERE patent_number = $1 AND deleted_at IS NULL`
	row := r.executor().QueryRowContext(ctx, query, number)
	return scanPatent(row)
}

func (r *postgresPatentRepo) Update(ctx context.Context, p *patent.Patent) error {
	query := `
		UPDATE patents
		SET title = $1, status = $2, metadata = $3, updated_at = NOW()
		WHERE id = $4 AND updated_at = $5
	`
	meta, _ := json.Marshal(p.Metadata)
	// Using optimistic lock via updated_at
	res, err := r.executor().ExecContext(ctx, query, p.Title, p.Status.String(), meta, p.ID, p.UpdatedAt)
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to update patent")
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return errors.New(errors.ErrCodeConflict, "patent updated by another transaction or not found")
	}
	return nil
}

func (r *postgresPatentRepo) Delete(ctx context.Context, id string) error {
	uid, err := uuid.Parse(id)
	if err != nil {
		return errors.NewInvalidInputError("invalid patent ID")
	}
	// Default to soft delete as per requirement
	query := `UPDATE patents SET deleted_at = NOW() WHERE id = $1`
	_, err = r.executor().ExecContext(ctx, query, uid)
	return err
}

func (r *postgresPatentRepo) SoftDelete(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE patents SET deleted_at = NOW() WHERE id = $1`
	_, err := r.executor().ExecContext(ctx, query, id)
	return err
}

func (r *postgresPatentRepo) Restore(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE patents SET deleted_at = NULL WHERE id = $1`
	_, err := r.executor().ExecContext(ctx, query, id)
	return err
}

func (r *postgresPatentRepo) HardDelete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM patents WHERE id = $1`
	_, err := r.executor().ExecContext(ctx, query, id)
	return err
}

// Search

func (r *postgresPatentRepo) Search(ctx context.Context, c patent.PatentSearchCriteria) (*patent.PatentSearchResult, error) {
	query := `FROM patents WHERE deleted_at IS NULL`
	var args []interface{}
	argIdx := 1

	if c.Query != "" {
		// Full text search on title and abstract
		// Assuming plainto_tsquery is available or standard. Using websearch_to_tsquery or plainto_tsquery.
		// Using standard PostgreSQL FTS syntax.
		query += fmt.Sprintf(` AND (to_tsvector('english', title) || to_tsvector('english', abstract)) @@ plainto_tsquery($%d)`, argIdx)
		args = append(args, c.Query)
		argIdx++
	}

	if len(c.Jurisdictions) > 0 {
		query += fmt.Sprintf(` AND jurisdiction = ANY($%d)`, argIdx)
		args = append(args, pq.Array(c.Jurisdictions))
		argIdx++
	}

	if len(c.Status) > 0 {
		statuses := make([]string, len(c.Status))
		for i, s := range c.Status {
			statuses[i] = s.String()
		}
		query += fmt.Sprintf(` AND status = ANY($%d)`, argIdx)
		args = append(args, pq.Array(statuses))
		argIdx++
	}

	if c.FilingDateStart != nil {
		query += fmt.Sprintf(` AND filing_date >= $%d`, argIdx)
		args = append(args, *c.FilingDateStart)
		argIdx++
	}

	if c.FilingDateEnd != nil {
		query += fmt.Sprintf(` AND filing_date <= $%d`, argIdx)
		args = append(args, *c.FilingDateEnd)
		argIdx++
	}

	// Count total
	var total int64
	countQuery := `SELECT COUNT(*) ` + query
	err := r.executor().QueryRowContext(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to count search results")
	}

	// Apply pagination
	if c.Limit <= 0 {
		c.Limit = 20
	}
	if c.Offset < 0 {
		c.Offset = 0
	}

	query += fmt.Sprintf(` ORDER BY filing_date DESC LIMIT $%d OFFSET $%d`, argIdx, argIdx+1)
	args = append(args, c.Limit, c.Offset)

	rows, err := r.executor().QueryContext(ctx, "SELECT * "+query, args...)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to search patents")
	}
	defer rows.Close()

	patents, err := scanPatents(rows)
	if err != nil {
		return nil, err
	}

	return &patent.PatentSearchResult{
		Patents: patents,
		Total:   total,
		Offset:  c.Offset,
		Limit:   c.Limit,
		HasMore: int64(c.Offset+len(patents)) < total,
	}, nil
}

func (r *postgresPatentRepo) FindByID(ctx context.Context, id string) (*patent.Patent, error) {
	uid, err := uuid.Parse(id)
	if err != nil {
		return nil, errors.NewInvalidInputError("invalid patent ID")
	}
	return r.GetByID(ctx, uid)
}

func (r *postgresPatentRepo) FindCitedBy(ctx context.Context, patentNumber string) ([]*patent.Patent, error) {
	// No dedicated citations table exists. Citations tracked via metadata or external.
	// Return empty result until a citation table is added.
	return []*patent.Patent{}, nil
}

func (r *postgresPatentRepo) FindCiting(ctx context.Context, patentNumber string) ([]*patent.Patent, error) {
	return []*patent.Patent{}, nil
}

func (r *postgresPatentRepo) FindByApplicant(ctx context.Context, applicantName string) ([]*patent.Patent, error) {
	pats, _, err := r.SearchByAssigneeName(ctx, applicantName, 50, 0)
	return pats, err
}

func (r *postgresPatentRepo) FindByFamilyID(ctx context.Context, familyID string) ([]*patent.Patent, error) {
	return r.GetByFamilyID(ctx, familyID)
}

func (r *postgresPatentRepo) GetByFamilyID(ctx context.Context, familyID string) ([]*patent.Patent, error) {
	query := `SELECT * FROM patents WHERE family_id = $1 AND deleted_at IS NULL`
	rows, err := r.executor().QueryContext(ctx, query, familyID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPatents(rows)
}

func (r *postgresPatentRepo) ListByPortfolio(ctx context.Context, portfolioID string) ([]*patent.Patent, error) {
	pid, err := uuid.Parse(portfolioID)
	if err != nil {
		return nil, errors.NewInvalidInputError("invalid portfolio ID format")
	}

	query := `
		SELECT p.*
		FROM patents p
		JOIN portfolio_patents pp ON p.id = pp.patent_id
		WHERE pp.portfolio_id = $1 AND p.deleted_at IS NULL
	`
	rows, err := r.executor().QueryContext(ctx, query, pid)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to list patents by portfolio")
	}
	defer rows.Close()

	return scanPatents(rows)
}

func (r *postgresPatentRepo) GetByAssignee(ctx context.Context, assigneeID uuid.UUID, limit, offset int) ([]*patent.Patent, int64, error) {
	if limit <= 0 {
		limit = 20
	}

	baseQuery := `FROM patents WHERE assignee_id = $1 AND deleted_at IS NULL`
	var total int64
	err := r.executor().QueryRowContext(ctx, "SELECT COUNT(*) "+baseQuery, assigneeID).Scan(&total)
	if err != nil {
		return nil, 0, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to count by assignee")
	}

	query := fmt.Sprintf("SELECT * %s ORDER BY filing_date DESC NULLS LAST LIMIT $2 OFFSET $3", baseQuery)
	rows, err := r.executor().QueryContext(ctx, query, assigneeID, limit, offset)
	if err != nil {
		return nil, 0, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to get by assignee")
	}
	defer rows.Close()

	patents, err := scanPatents(rows)
	if err != nil {
		return nil, 0, err
	}
	return patents, total, nil
}

func (r *postgresPatentRepo) GetByJurisdiction(ctx context.Context, jurisdiction string, limit, offset int) ([]*patent.Patent, int64, error) {
	if limit <= 0 {
		limit = 20
	}

	baseQuery := `FROM patents WHERE jurisdiction = $1 AND deleted_at IS NULL`
	var total int64
	err := r.executor().QueryRowContext(ctx, "SELECT COUNT(*) "+baseQuery, jurisdiction).Scan(&total)
	if err != nil {
		return nil, 0, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to count by jurisdiction")
	}

	query := fmt.Sprintf("SELECT * %s ORDER BY filing_date DESC NULLS LAST LIMIT $2 OFFSET $3", baseQuery)
	rows, err := r.executor().QueryContext(ctx, query, jurisdiction, limit, offset)
	if err != nil {
		return nil, 0, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to get by jurisdiction")
	}
	defer rows.Close()

	patents, err := scanPatents(rows)
	if err != nil {
		return nil, 0, err
	}
	return patents, total, nil
}

func (r *postgresPatentRepo) FindExpiringBefore(ctx context.Context, date time.Time) ([]*patent.Patent, error) {
	query := `SELECT * FROM patents WHERE expiry_date IS NOT NULL AND expiry_date < $1 AND deleted_at IS NULL ORDER BY expiry_date ASC`
	rows, err := r.executor().QueryContext(ctx, query, date)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to find expiring patents")
	}
	defer rows.Close()
	return scanPatents(rows)
}

func (r *postgresPatentRepo) GetExpiringPatents(ctx context.Context, daysAhead int, limit, offset int) ([]*patent.Patent, int64, error) {
	if limit <= 0 {
		limit = 20
	}

	baseQuery := `FROM patents WHERE expiry_date IS NOT NULL AND expiry_date <= NOW() + ($1 || ' days')::INTERVAL AND expiry_date >= NOW() AND deleted_at IS NULL`
	// Use a parameterized interval approach
	args := []interface{}{fmt.Sprintf("%d days", daysAhead)}
	argIdx := 2

	var total int64
	err := r.executor().QueryRowContext(ctx, "SELECT COUNT(*) "+baseQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to count expiring patents")
	}

	query := fmt.Sprintf("SELECT * %s ORDER BY expiry_date ASC LIMIT $%d OFFSET $%d", baseQuery, argIdx, argIdx+1)
	args = append(args, limit, offset)

	rows, err := r.executor().QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to get expiring patents")
	}
	defer rows.Close()

	patents, err := scanPatents(rows)
	if err != nil {
		return nil, 0, err
	}
	return patents, total, nil
}

func (r *postgresPatentRepo) FindDuplicates(ctx context.Context, fullTextHash string) ([]*patent.Patent, error) {
	query := `SELECT * FROM patents WHERE full_text_hash = $1 AND deleted_at IS NULL ORDER BY created_at DESC`
	rows, err := r.executor().QueryContext(ctx, query, fullTextHash)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to find duplicates")
	}
	defer rows.Close()
	return scanPatents(rows)
}

func (r *postgresPatentRepo) FindByPatentNumbers(ctx context.Context, numbers []string) ([]*patent.Patent, error) {
	if len(numbers) == 0 {
		return []*patent.Patent{}, nil
	}
	query := `SELECT * FROM patents WHERE patent_number = ANY($1) AND deleted_at IS NULL`
	rows, err := r.executor().QueryContext(ctx, query, pq.Array(numbers))
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to find patents by numbers")
	}
	defer rows.Close()
	return scanPatents(rows)
}

func (r *postgresPatentRepo) FindByIPCCode(ctx context.Context, ipcCode string) ([]*patent.Patent, error) {
	query := `SELECT * FROM patents WHERE $1 = ANY(ipc_codes) AND deleted_at IS NULL`
	rows, err := r.executor().QueryContext(ctx, query, ipcCode)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to find patents by IPC code")
	}
	defer rows.Close()
	return scanPatents(rows)
}

func (r *postgresPatentRepo) FindByIDs(ctx context.Context, ids []string) ([]*patent.Patent, error) {
	if len(ids) == 0 {
		return []*patent.Patent{}, nil
	}
	// Simplified implementation for now, assuming UUIDs
	var uids []uuid.UUID
	for _, id := range ids {
		if uid, err := uuid.Parse(id); err == nil {
			uids = append(uids, uid)
		}
	}
	if len(uids) == 0 {
		return []*patent.Patent{}, nil
	}

	query := `SELECT * FROM patents WHERE id = ANY($1) AND deleted_at IS NULL`
	rows, err := r.executor().QueryContext(ctx, query, pq.Array(uids))
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to find patents by IDs")
	}
	defer rows.Close()
	return scanPatents(rows)
}

func (r *postgresPatentRepo) FindByMoleculeID(ctx context.Context, moleculeID string) ([]*patent.Patent, error) {
	molUUID, err := uuid.Parse(moleculeID)
	if err != nil {
		return nil, errors.NewInvalidInputError("invalid molecule ID format")
	}

	query := `
		SELECT p.*
		FROM patents p
		JOIN patent_molecule_relations pmr ON p.id = pmr.patent_id
		WHERE pmr.molecule_id = $1 AND p.deleted_at IS NULL
	`
	rows, err := r.executor().QueryContext(ctx, query, molUUID)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to find patents by molecule ID")
	}
	defer rows.Close()

	return scanPatents(rows)
}

func (r *postgresPatentRepo) AssociateMolecule(ctx context.Context, patentID string, moleculeID string) error {
	pID, err := uuid.Parse(patentID)
	if err != nil {
		return errors.NewInvalidInputError("invalid patent ID format")
	}
	mID, err := uuid.Parse(moleculeID)
	if err != nil {
		return errors.NewInvalidInputError("invalid molecule ID format")
	}

	// Assuming a join table patent_molecule_relations exists
	query := `
		INSERT INTO patent_molecule_relations (patent_id, molecule_id, created_at, updated_at, relation_type, confidence)
		VALUES ($1, $2, NOW(), NOW(), 'extracted', 1.0)
		ON CONFLICT (patent_id, molecule_id) DO UPDATE SET updated_at = NOW()
	`
	_, err = r.executor().ExecContext(ctx, query, pID, mID)
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to associate molecule with patent")
	}
	return nil
}

func parseClaimType(s string) patent.ClaimType {
	switch s {
	case "independent":
		return patent.ClaimTypeIndependent
	case "dependent":
		return patent.ClaimTypeDependent
	default:
		return patent.ClaimTypeUnknown
	}
}

func scanClaim(row scanner) (*patent.Claim, error) {
	c := &patent.Claim{}
	var claimTypeStr string
	var elementsJSON, markushJSON []byte

	err := row.Scan(
		&c.Number, &claimTypeStr, &c.Text, &elementsJSON, &markushJSON,
	)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to scan claim")
	}
	c.Type = parseClaimType(claimTypeStr)
	if len(elementsJSON) > 0 {
		_ = json.Unmarshal(elementsJSON, &c.Elements)
	}
	if len(markushJSON) > 0 {
		_ = json.Unmarshal(markushJSON, &c.MarkushStructures)
	}
	c.Language = "en"
	return c, nil
}

func (r *postgresPatentRepo) CreateClaim(ctx context.Context, claim *patent.Claim) error {
	// Claim struct does not carry patent_id. This method requires the caller
	// to use a repository with transaction context or the parent patent ID.
	// Consider using GetClaimsByPatent for reads and BatchCreateClaims for bulk writes.
	return errors.New(errors.ErrCodeInvalidOperation, "CreateClaim requires patent_id; use BatchCreateClaims with explicit patent context")
}

func (r *postgresPatentRepo) GetClaimsByPatent(ctx context.Context, patentID uuid.UUID) ([]*patent.Claim, error) {
	query := `SELECT claim_number, claim_type, claim_text, elements, markush_structures FROM patent_claims WHERE patent_id = $1 ORDER BY claim_number ASC`
	rows, err := r.executor().QueryContext(ctx, query, patentID)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to get claims by patent")
	}
	defer rows.Close()

	var claims []*patent.Claim
	for rows.Next() {
		c, err := scanClaim(rows)
		if err != nil {
			return nil, err
		}
		claims = append(claims, c)
	}
	return claims, nil
}

func (r *postgresPatentRepo) UpdateClaim(ctx context.Context, claim *patent.Claim) error {
	query := `
		UPDATE patent_claims
		SET claim_text = $1, elements = $2, markush_structures = $3, updated_at = NOW()
		WHERE patent_id = $4 AND claim_number = $5
	`
	elements, _ := json.Marshal(claim.Elements)
	markushStructures, _ := json.Marshal(claim.MarkushStructures)
	res, err := r.executor().ExecContext(ctx, query, claim.Text, elements, markushStructures, claim.Number, claim.Number)
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to update claim")
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return errors.New(errors.ErrCodeNotFound, "claim not found")
	}
	return nil
}

func (r *postgresPatentRepo) DeleteClaimsByPatent(ctx context.Context, patentID uuid.UUID) error {
	query := `DELETE FROM patent_claims WHERE patent_id = $1`
	_, err := r.executor().ExecContext(ctx, query, patentID)
	return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to delete claims by patent")
}

func (r *postgresPatentRepo) BatchCreateClaims(ctx context.Context, claims []*patent.Claim) error {
	if len(claims) == 0 {
		return nil
	}
	// Note: patent_id must be set externally or provided. Not in Claim struct.
	// This function requires the patentID to be known. For now it returns an error
	// indicating the caller should use an alternative approach.
	return errors.New(errors.ErrCodeInvalidOperation, "BatchCreateClaims requires patent_id not available in Claim struct; use SetClaims on the patent entity instead")
}

func (r *postgresPatentRepo) GetIndependentClaims(ctx context.Context, patentID uuid.UUID) ([]*patent.Claim, error) {
	query := `SELECT claim_number, claim_type, claim_text, elements, markush_structures FROM patent_claims WHERE patent_id = $1 AND claim_type = 'independent' ORDER BY claim_number ASC`
	rows, err := r.executor().QueryContext(ctx, query, patentID)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to get independent claims")
	}
	defer rows.Close()

	var claims []*patent.Claim
	for rows.Next() {
		c, err := scanClaim(rows)
		if err != nil {
			return nil, err
		}
		claims = append(claims, c)
	}
	return claims, nil
}

// Inventors
func (r *postgresPatentRepo) SetInventors(ctx context.Context, patentID uuid.UUID, inventors []*patent.Inventor) error {
	tx, err := r.conn.DB().BeginTx(ctx, nil)
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to begin transaction")
	}
	defer tx.Rollback()

	// Delete existing inventors
	if _, err := tx.ExecContext(ctx, `DELETE FROM patent_inventors WHERE patent_id = $1`, patentID); err != nil {
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to clear inventors")
	}

	// Insert new inventors
	for _, inv := range inventors {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO patent_inventors (patent_id, inventor_name, inventor_name_en, sequence, affiliation)
			VALUES ($1, $2, $3, $4, $5)
		`, patentID, inv.Name, "", inv.Sequence, inv.Affiliation)
		if err != nil {
			return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to insert inventor")
		}
	}

	return tx.Commit()
}

func (r *postgresPatentRepo) GetInventors(ctx context.Context, patentID uuid.UUID) ([]*patent.Inventor, error) {
	query := `SELECT inventor_name, sequence, affiliation FROM patent_inventors WHERE patent_id = $1 ORDER BY sequence ASC`
	rows, err := r.executor().QueryContext(ctx, query, patentID)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to get inventors")
	}
	defer rows.Close()

	var inventors []*patent.Inventor
	for rows.Next() {
		inv := &patent.Inventor{}
		if err := rows.Scan(&inv.Name, &inv.Sequence, &inv.Affiliation); err != nil {
			return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to scan inventor")
		}
		inventors = append(inventors, inv)
	}
	return inventors, nil
}

func (r *postgresPatentRepo) SearchByInventor(ctx context.Context, inventorName string, limit, offset int) ([]*patent.Patent, int64, error) {
	if limit <= 0 {
		limit = 20
	}

	countQuery := `SELECT COUNT(DISTINCT p.id) FROM patents p JOIN patent_inventors pi ON p.id = pi.patent_id WHERE pi.inventor_name ILIKE $1 AND p.deleted_at IS NULL`
	var total int64
	err := r.executor().QueryRowContext(ctx, countQuery, "%"+inventorName+"%").Scan(&total)
	if err != nil {
		return nil, 0, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to count by inventor")
	}

	query := `
		SELECT DISTINCT p.* FROM patents p
		JOIN patent_inventors pi ON p.id = pi.patent_id
		WHERE pi.inventor_name ILIKE $1 AND p.deleted_at IS NULL
		ORDER BY p.filing_date DESC NULLS LAST
		LIMIT $2 OFFSET $3
	`
	rows, err := r.executor().QueryContext(ctx, query, "%"+inventorName+"%", limit, offset)
	if err != nil {
		return nil, 0, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to search by inventor")
	}
	defer rows.Close()

	patents, err := scanPatents(rows)
	if err != nil {
		return nil, 0, err
	}
	return patents, total, nil
}

// Priority
func (r *postgresPatentRepo) SetPriorityClaims(ctx context.Context, patentID uuid.UUID, claims []*patent.PriorityClaim) error {
	tx, err := r.conn.DB().BeginTx(ctx, nil)
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to begin transaction")
	}
	defer tx.Rollback()

	// Delete existing priority claims
	if _, err := tx.ExecContext(ctx, `DELETE FROM patent_priority_claims WHERE patent_id = $1`, patentID); err != nil {
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to clear priority claims")
	}

	// Insert new priority claims
	for _, pc := range claims {
		_, err := tx.ExecContext(ctx, `
			INSERT INTO patent_priority_claims (patent_id, priority_number, priority_date, priority_country)
			VALUES ($1, $2, $3, $4)
		`, patentID, pc.PriorityNumber, pc.PriorityDate, pc.PriorityCountry)
		if err != nil {
			return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to insert priority claim")
		}
	}

	return tx.Commit()
}

func (r *postgresPatentRepo) GetPriorityClaims(ctx context.Context, patentID uuid.UUID) ([]*patent.PriorityClaim, error) {
	query := `SELECT id, patent_id, priority_number, priority_date, priority_country FROM patent_priority_claims WHERE patent_id = $1 ORDER BY priority_date DESC`
	rows, err := r.executor().QueryContext(ctx, query, patentID)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to get priority claims")
	}
	defer rows.Close()

	var claims []*patent.PriorityClaim
	for rows.Next() {
		pc := &patent.PriorityClaim{}
		if err := rows.Scan(&pc.ID, &pc.PatentID, &pc.PriorityNumber, &pc.PriorityDate, &pc.PriorityCountry); err != nil {
			return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to scan priority claim")
		}
		claims = append(claims, pc)
	}
	return claims, nil
}

// Assignee
func (r *postgresPatentRepo) SearchByAssigneeName(ctx context.Context, assigneeName string, limit, offset int) ([]*patent.Patent, int64, error) {
	if limit <= 0 {
		limit = 100
	}
	// Search patents by assignee name (case-insensitive partial match)
	query := `SELECT id, patent_number, title, abstract, status, jurisdiction, filing_date, assignee_name, family_id
		FROM patents WHERE assignee_name ILIKE $1 ORDER BY filing_date DESC LIMIT $2 OFFSET $3`
	rows, err := r.conn.DB().QueryContext(ctx, query, "%"+assigneeName+"%", limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var results []*patent.Patent
	for rows.Next() {
		p := &patent.Patent{}
		if err := rows.Scan(&p.ID, &p.PatentNumber, &p.Title, &p.Abstract, &p.Status, &p.Jurisdiction, &p.FilingDate, &p.AssigneeName, &p.FamilyID); err != nil {
			return nil, 0, err
		}
		results = append(results, p)
	}

	// Count total
	var total int64
	countQuery := `SELECT COUNT(*) FROM patents WHERE assignee_name ILIKE $1`
	r.conn.DB().QueryRowContext(ctx, countQuery, "%"+assigneeName+"%").Scan(&total)

	return results, total, nil
}

// Batch
func (r *postgresPatentRepo) SaveBatch(ctx context.Context, patents []*patent.Patent) error {
	_, err := r.BatchCreate(ctx, patents)
	return err
}

func (r *postgresPatentRepo) BatchCreate(ctx context.Context, patents []*patent.Patent) (int, error) {
	if len(patents) == 0 {
		return 0, nil
	}
	created := 0
	for _, p := range patents {
		err := r.Create(ctx, p)
		if err != nil {
			// Skip duplicates and continue
			if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" {
				continue
			}
			return created, err
		}
		created++
	}
	return created, nil
}

func (r *postgresPatentRepo) BatchUpdateStatus(ctx context.Context, ids []uuid.UUID, status patent.PatentStatus) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	query := `UPDATE patents SET status = $1, updated_at = NOW() WHERE id = ANY($2)`
	res, err := r.executor().ExecContext(ctx, query, status.String(), pq.Array(ids))
	if err != nil {
		return 0, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to batch update status")
	}
	rows, _ := res.RowsAffected()
	return rows, nil
}

// Stats
func (r *postgresPatentRepo) CountByStatus(ctx context.Context) (map[patent.PatentStatus]int64, error) {
	query := `SELECT status, COUNT(*) FROM patents WHERE deleted_at IS NULL GROUP BY status`
	rows, err := r.executor().QueryContext(ctx, query)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to count by status")
	}
	defer rows.Close()

	counts := make(map[patent.PatentStatus]int64)
	for rows.Next() {
		var statusStr string
		var count int64
		if err := rows.Scan(&statusStr, &count); err != nil {
			return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to scan status count")
		}
		status := parsePatentStatus(statusStr)
		counts[status] = count
	}
	return counts, nil
}

func (r *postgresPatentRepo) CountByJurisdiction(ctx context.Context) (map[string]int64, error) {
	query := `
		SELECT jurisdiction, COUNT(*)
		FROM patents
		WHERE deleted_at IS NULL
		GROUP BY jurisdiction
	`
	rows, err := r.executor().QueryContext(ctx, query)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to count by jurisdiction")
	}
	defer rows.Close()

	counts := make(map[string]int64)
	for rows.Next() {
		var jurisdiction string
		var count int64
		if err := rows.Scan(&jurisdiction, &count); err != nil {
			return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to scan jurisdiction count")
		}
		counts[jurisdiction] = count
	}
	return counts, nil
}

func (r *postgresPatentRepo) CountByYear(ctx context.Context, field string) (map[int]int64, error) {
	validFields := map[string]string{
		"filing_date":      "filing_date",
		"publication_date": "publication_date",
		"grant_date":       "grant_date",
		"priority_date":    "priority_date",
		"expiry_date":      "expiry_date",
	}
	dateField, ok := validFields[field]
	if !ok {
		return nil, errors.New(errors.ErrCodeValidation, "invalid date field: "+field)
	}

	query := fmt.Sprintf(`
		SELECT EXTRACT(YEAR FROM %s)::int AS year, COUNT(*)
		FROM patents
		WHERE deleted_at IS NULL AND %s IS NOT NULL
		GROUP BY year
		ORDER BY year ASC
	`, dateField, dateField)

	rows, err := r.executor().QueryContext(ctx, query)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to count by year")
	}
	defer rows.Close()

	counts := make(map[int]int64)
	for rows.Next() {
		var year int
		var count int64
		if err := rows.Scan(&year, &count); err != nil {
			return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to scan year count")
		}
		counts[year] = count
	}
	return counts, nil
}

func (r *postgresPatentRepo) GetIPCDistribution(ctx context.Context, level int) (map[string]int64, error) {
	// IPC code format: "A01B 1/00" where:
	// level 1 = section (1 char): "A"
	// level 2 = class (3 chars): "A01"
	// level 3 = subclass (4 chars): "A01B"
	// level 4 = group: "A01B 1/00"
	var substrLen int
	switch level {
	case 1:
		substrLen = 1 // Section
	case 2:
		substrLen = 3 // Class
	case 3:
		substrLen = 4 // Subclass
	default:
		substrLen = 0 // Full IPC code
	}

	var query string
	if level >= 1 && level <= 3 {
		query = fmt.Sprintf(`
			SELECT SUBSTRING(unnest(ipc_codes) FROM 1 FOR %d) AS code, COUNT(*)
			FROM patents
			WHERE deleted_at IS NULL
			GROUP BY code
			ORDER BY COUNT(*) DESC
		`, substrLen)
	} else {
		query = `
			SELECT unnest(ipc_codes) AS code, COUNT(*)
			FROM patents
			WHERE deleted_at IS NULL
			GROUP BY code
			ORDER BY COUNT(*) DESC
		`
	}

	rows, err := r.executor().QueryContext(ctx, query)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to get IPC distribution")
	}
	defer rows.Close()

	dist := make(map[string]int64)
	for rows.Next() {
		var code string
		var count int64
		if err := rows.Scan(&code, &count); err != nil {
			return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to scan IPC distribution")
		}
		dist[code] = count
	}
	return dist, nil
}

// Transaction
func (r *postgresPatentRepo) WithTx(ctx context.Context, fn func(patent.PatentRepository) error) error {
	tx, err := r.conn.DB().BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	txRepo := &postgresPatentRepo{conn: r.conn, tx: tx, log: r.log}
	if err := fn(txRepo); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

// Scanners
func scanPatent(row scanner) (*patent.Patent, error) {
	p := &patent.Patent{}
	var statusStr string
	var raw, meta []byte

	err := row.Scan(
		&p.ID, &p.PatentNumber, &p.Title, &p.TitleEn, &p.Abstract, &p.AbstractEn, &p.Type, &statusStr,
		&p.FilingDate, &p.PublicationDate, &p.GrantDate, &p.ExpiryDate, &p.PriorityDate,
		&p.AssigneeID, &p.AssigneeName, &p.Jurisdiction, pq.Array(&p.IPCCodes), pq.Array(&p.CPCCodes), pq.Array(&p.KeyIPTechCodes),
		&p.FamilyID, &p.ApplicationNumber, &p.FullTextHash, &p.Source, &raw, &meta,
		&p.CreatedAt, &p.UpdatedAt, &p.DeletedAt,
	)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New(errors.ErrCodeNotFound, "patent not found")
		}
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to scan patent")
	}

	p.Status = parsePatentStatus(statusStr)
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &p.RawData)
	}
	if len(meta) > 0 {
		_ = json.Unmarshal(meta, &p.Metadata)
	}

	return p, nil
}

func parsePatentStatus(s string) patent.PatentStatus {
	switch s {
	case "draft":
		return patent.PatentStatusDraft
	case "filed":
		return patent.PatentStatusFiled
	case "published":
		return patent.PatentStatusPublished
	case "under_examination":
		return patent.PatentStatusUnderExamination
	case "granted":
		return patent.PatentStatusGranted
	case "rejected":
		return patent.PatentStatusRejected
	case "withdrawn":
		return patent.PatentStatusWithdrawn
	case "expired":
		return patent.PatentStatusExpired
	case "invalidated":
		return patent.PatentStatusInvalidated
	case "lapsed":
		return patent.PatentStatusLapsed
	default:
		return patent.PatentStatusUnknown
	}
}

func scanPatents(rows *sql.Rows) ([]*patent.Patent, error) {
	var list []*patent.Patent
	for rows.Next() {
		p, err := scanPatent(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, p)
	}
	return list, nil
}
