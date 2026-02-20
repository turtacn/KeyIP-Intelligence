// Package repositories provides PostgreSQL-backed implementations of all domain
// repository interfaces for the KeyIP-Intelligence platform.
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
)

// ─────────────────────────────────────────────────────────────────────────────
// Domain entity stubs — thin mirrors of internal/domain/patent types
//
// In a real build these would be imported from internal/domain/patent.  We
// define local aliases / structs here so the file compiles independently and
// the SQL mapping logic is fully visible.
// ─────────────────────────────────────────────────────────────────────────────

// Patent is the aggregate root for the patent domain.
type Patent struct {
	ID                common.ID
	TenantID          common.TenantID
	PatentNumber      string
	Title             string
	Abstract          string
	Description       string
	Applicants        []string
	Inventors         []string
	IPCCodes          []string
	Jurisdiction      string
	FilingDate        time.Time
	PublicationDate   time.Time
	GrantDate         *time.Time
	ExpiryDate        *time.Time
	Priority          []string
	FamilyID          string
	Status            string
	LegalStatus       string
	Citations         []string
	Claims            []Claim
	MarkushStructures []MarkushStructure
	Metadata          map[string]interface{}
	CreatedAt         time.Time
	UpdatedAt         time.Time
	CreatedBy         common.UserID
	Version           int
}

// Claim represents a single patent claim.
type Claim struct {
	ID          common.ID
	PatentID    common.ID
	ClaimNumber int
	ClaimType   string
	ParentID    *common.ID
	Text        string
	IsIndependent bool
}

// MarkushStructure represents a Markush chemical structure within a patent.
type MarkushStructure struct {
	ID          common.ID
	PatentID    common.ID
	SMARTS      string
	Description string
	RGroups     map[string][]string
}

// PatentSearchCriteria carries the dynamic filter parameters for Search.
type PatentSearchCriteria struct {
	Keyword      string
	Jurisdiction string
	Applicant    string
	IPCCode      string
	FilingFrom   *time.Time
	FilingTo     *time.Time
	Status       string
	Page         int
	PageSize     int
	SortBy       string
	SortOrder    string
}

// ─────────────────────────────────────────────────────────────────────────────
// PatentRepository
// ─────────────────────────────────────────────────────────────────────────────

// PatentRepository is the PostgreSQL implementation of the patent domain's
// Repository interface.  Every public method accepts a context.Context for
// cancellation / timeout propagation and uses parameterised queries exclusively.
type PatentRepository struct {
	pool   *pgxpool.Pool
	logger Logger
}

// NewPatentRepository constructs a ready-to-use PatentRepository.
func NewPatentRepository(pool *pgxpool.Pool, logger Logger) *PatentRepository {
	return &PatentRepository{pool: pool, logger: logger}
}

// ─────────────────────────────────────────────────────────────────────────────
// Save
// ─────────────────────────────────────────────────────────────────────────────

// Save persists a new Patent aggregate (patent + claims + markush structures)
// inside a single database transaction.
func (r *PatentRepository) Save(ctx context.Context, p *Patent) error {
	r.logger.Debug("PatentRepository.Save", "patent_id", p.ID)

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		r.logger.Error("PatentRepository.Save: begin tx", "error", err)
		return appErrors.Wrap(err, appErrors.CodeDBConnectionError, "failed to begin transaction")
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	// 1. Insert patent main record.
	metaJSON, _ := json.Marshal(p.Metadata)
	_, err = tx.Exec(ctx, `
		INSERT INTO patents (
			id, tenant_id, patent_number, title, abstract, description,
			applicants, inventors, ipc_codes, jurisdiction,
			filing_date, publication_date, grant_date, expiry_date,
			priority, family_id, status, legal_status, citations,
			metadata, created_at, updated_at, created_by, version
		) VALUES (
			$1,$2,$3,$4,$5,$6,
			$7,$8,$9,$10,
			$11,$12,$13,$14,
			$15,$16,$17,$18,$19,
			$20,$21,$22,$23,$24
		)`,
		p.ID, p.TenantID, p.PatentNumber, p.Title, p.Abstract, p.Description,
		p.Applicants, p.Inventors, p.IPCCodes, p.Jurisdiction,
		p.FilingDate, p.PublicationDate, p.GrantDate, p.ExpiryDate,
		p.Priority, p.FamilyID, p.Status, p.LegalStatus, p.Citations,
		metaJSON, p.CreatedAt, p.UpdatedAt, p.CreatedBy, p.Version,
	)
	if err != nil {
		r.logger.Error("PatentRepository.Save: insert patent", "error", err)
		return appErrors.Wrap(err, appErrors.CodeDBQueryError, "failed to insert patent")
	}

	// 2. Batch insert claims.
	if err := r.insertClaims(ctx, tx, p.ID, p.Claims); err != nil {
		return err
	}

	// 3. Batch insert markush structures.
	if err := r.insertMarkushStructures(ctx, tx, p.ID, p.MarkushStructures); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		r.logger.Error("PatentRepository.Save: commit", "error", err)
		return appErrors.Wrap(err, appErrors.CodeDBConnectionError, "failed to commit transaction")
	}
	return nil
}

func (r *PatentRepository) insertClaims(ctx context.Context, tx pgx.Tx, patentID common.ID, claims []Claim) error {
	if len(claims) == 0 {
		return nil
	}
	for _, c := range claims {
		_, err := tx.Exec(ctx, `
			INSERT INTO patent_claims (id, patent_id, claim_number, claim_type, parent_id, text, is_independent)
			VALUES ($1,$2,$3,$4,$5,$6,$7)`,
			c.ID, patentID, c.ClaimNumber, c.ClaimType, c.ParentID, c.Text, c.IsIndependent,
		)
		if err != nil {
			r.logger.Error("PatentRepository.insertClaims", "error", err, "claim_id", c.ID)
			return appErrors.Wrap(err, appErrors.CodeDBQueryError, "failed to insert claim")
		}
	}
	return nil
}

func (r *PatentRepository) insertMarkushStructures(ctx context.Context, tx pgx.Tx, patentID common.ID, ms []MarkushStructure) error {
	if len(ms) == 0 {
		return nil
	}
	for _, m := range ms {
		rgJSON, _ := json.Marshal(m.RGroups)
		_, err := tx.Exec(ctx, `
			INSERT INTO markush_structures (id, patent_id, smarts, description, r_groups)
			VALUES ($1,$2,$3,$4,$5)`,
			m.ID, patentID, m.SMARTS, m.Description, rgJSON,
		)
		if err != nil {
			r.logger.Error("PatentRepository.insertMarkushStructures", "error", err, "markush_id", m.ID)
			return appErrors.Wrap(err, appErrors.CodeDBQueryError, "failed to insert markush structure")
		}
	}
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// FindByID
// ─────────────────────────────────────────────────────────────────────────────

// FindByID loads a complete Patent aggregate by its primary key.
func (r *PatentRepository) FindByID(ctx context.Context, id common.ID) (*Patent, error) {
	r.logger.Debug("PatentRepository.FindByID", "id", id)

	p, err := r.scanPatent(r.pool.QueryRow(ctx, `
		SELECT id, tenant_id, patent_number, title, abstract, description,
		       applicants, inventors, ipc_codes, jurisdiction,
		       filing_date, publication_date, grant_date, expiry_date,
		       priority, family_id, status, legal_status, citations,
		       metadata, created_at, updated_at, created_by, version
		FROM patents WHERE id = $1`, id))
	if err != nil {
		return nil, err
	}

	claims, err := r.findClaimsByPatentID(ctx, id)
	if err != nil {
		return nil, err
	}
	p.Claims = claims

	markush, err := r.findMarkushByPatentID(ctx, id)
	if err != nil {
		return nil, err
	}
	p.MarkushStructures = markush

	return p, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// FindByNumber
// ─────────────────────────────────────────────────────────────────────────────

// FindByNumber locates a patent by its publication / grant number.
func (r *PatentRepository) FindByNumber(ctx context.Context, number string) (*Patent, error) {
	r.logger.Debug("PatentRepository.FindByNumber", "number", number)

	p, err := r.scanPatent(r.pool.QueryRow(ctx, `
		SELECT id, tenant_id, patent_number, title, abstract, description,
		       applicants, inventors, ipc_codes, jurisdiction,
		       filing_date, publication_date, grant_date, expiry_date,
		       priority, family_id, status, legal_status, citations,
		       metadata, created_at, updated_at, created_by, version
		FROM patents WHERE patent_number = $1`, number))
	if err != nil {
		return nil, err
	}

	p.Claims, _ = r.findClaimsByPatentID(ctx, p.ID)
	p.MarkushStructures, _ = r.findMarkushByPatentID(ctx, p.ID)
	return p, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// FindByFamilyID
// ─────────────────────────────────────────────────────────────────────────────

// FindByFamilyID returns all patents belonging to the same patent family.
func (r *PatentRepository) FindByFamilyID(ctx context.Context, familyID string) ([]*Patent, error) {
	r.logger.Debug("PatentRepository.FindByFamilyID", "family_id", familyID)

	rows, err := r.pool.Query(ctx, `
		SELECT id, tenant_id, patent_number, title, abstract, description,
		       applicants, inventors, ipc_codes, jurisdiction,
		       filing_date, publication_date, grant_date, expiry_date,
		       priority, family_id, status, legal_status, citations,
		       metadata, created_at, updated_at, created_by, version
		FROM patents WHERE family_id = $1
		ORDER BY filing_date ASC`, familyID)
	if err != nil {
		r.logger.Error("PatentRepository.FindByFamilyID", "error", err)
		return nil, appErrors.Wrap(err, appErrors.CodeDBQueryError, "failed to query patents by family")
	}
	defer rows.Close()

	return r.scanPatents(rows)
}

// ─────────────────────────────────────────────────────────────────────────────
// Search — dynamic full-text + faceted query
// ─────────────────────────────────────────────────────────────────────────────

// Search builds a dynamic SQL query from the supplied criteria, supporting
// PostgreSQL full-text search (to_tsquery), jurisdiction / applicant / IPC
// facets, date-range filters, and cursor-based pagination.
func (r *PatentRepository) Search(ctx context.Context, criteria PatentSearchCriteria) ([]*Patent, int64, error) {
	r.logger.Debug("PatentRepository.Search", "criteria", criteria)

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

	// Full-text keyword search.
	if criteria.Keyword != "" {
		ph := nextArg(criteria.Keyword)
		conditions = append(conditions,
			fmt.Sprintf("to_tsvector('english', title || ' ' || abstract) @@ plainto_tsquery('english', %s)", ph))
	}

	// Jurisdiction filter.
	if criteria.Jurisdiction != "" {
		ph := nextArg(criteria.Jurisdiction)
		conditions = append(conditions, fmt.Sprintf("jurisdiction = %s", ph))
	}

	// Applicant filter (array contains).
	if criteria.Applicant != "" {
		ph := nextArg(criteria.Applicant)
		conditions = append(conditions, fmt.Sprintf("applicants @> ARRAY[%s]::TEXT[]", ph))
	}

	// IPC code filter (array contains).
	if criteria.IPCCode != "" {
		ph := nextArg(criteria.IPCCode)
		conditions = append(conditions, fmt.Sprintf("ipc_codes @> ARRAY[%s]::TEXT[]", ph))
	}

	// Filing date range.
	if criteria.FilingFrom != nil {
		ph := nextArg(*criteria.FilingFrom)
		conditions = append(conditions, fmt.Sprintf("filing_date >= %s", ph))
	}
	if criteria.FilingTo != nil {
		ph := nextArg(*criteria.FilingTo)
		conditions = append(conditions, fmt.Sprintf("filing_date <= %s", ph))
	}

	// Status filter.
	if criteria.Status != "" {
		ph := nextArg(criteria.Status)
		conditions = append(conditions, fmt.Sprintf("status = %s", ph))
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	// Count total matching rows.
	countSQL := fmt.Sprintf("SELECT COUNT(*) FROM patents %s", whereClause)
	var total int64
	if err := r.pool.QueryRow(ctx, countSQL, args...).Scan(&total); err != nil {
		r.logger.Error("PatentRepository.Search: count", "error", err)
		return nil, 0, appErrors.Wrap(err, appErrors.CodeDBQueryError, "failed to count search results")
	}

	// Sort.
	sortCol := "filing_date"
	if criteria.SortBy != "" {
		sortCol = sanitiseSortColumn(criteria.SortBy)
	}
	sortDir := "DESC"
	if strings.EqualFold(criteria.SortOrder, "asc") {
		sortDir = "ASC"
	}

	// Pagination.
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
		SELECT id, tenant_id, patent_number, title, abstract, description,
		       applicants, inventors, ipc_codes, jurisdiction,
		       filing_date, publication_date, grant_date, expiry_date,
		       priority, family_id, status, legal_status, citations,
		       metadata, created_at, updated_at, created_by, version
		FROM patents %s
		ORDER BY %s %s
		LIMIT %s OFFSET %s`,
		whereClause, sortCol, sortDir, phLimit, phOffset)

	rows, err := r.pool.Query(ctx, dataSQL, args...)
	if err != nil {
		r.logger.Error("PatentRepository.Search: query", "error", err)
		return nil, 0, appErrors.Wrap(err, appErrors.CodeDBQueryError, "failed to execute search query")
	}
	defer rows.Close()

	patents, err := r.scanPatents(rows)
	if err != nil {
		return nil, 0, err
	}
	return patents, total, nil
}

// sanitiseSortColumn maps user-supplied sort field names to safe column names.
func sanitiseSortColumn(col string) string {
	allowed := map[string]string{
		"filing_date":      "filing_date",
		"publication_date": "publication_date",
		"grant_date":       "grant_date",
		"expiry_date":      "expiry_date",
		"title":            "title",
		"patent_number":    "patent_number",
		"created_at":       "created_at",
		"updated_at":       "updated_at",
	}
	if safe, ok := allowed[col]; ok {
		return safe
	}
	return "filing_date"
}

// ─────────────────────────────────────────────────────────────────────────────
// Update — optimistic locking
// ─────────────────────────────────────────────────────────────────────────────

// Update persists mutations to an existing Patent aggregate using optimistic
// locking.  If the version in the database does not match p.Version the update
// is rejected with a CodeConflict error.  Claims are replaced using a
// delete-then-insert strategy within the same transaction.
func (r *PatentRepository) Update(ctx context.Context, p *Patent) error {
	r.logger.Debug("PatentRepository.Update", "patent_id", p.ID, "version", p.Version)

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		r.logger.Error("PatentRepository.Update: begin tx", "error", err)
		return appErrors.Wrap(err, appErrors.CodeDBConnectionError, "failed to begin transaction")
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	metaJSON, _ := json.Marshal(p.Metadata)
	newVersion := p.Version + 1

	tag, err := tx.Exec(ctx, `
		UPDATE patents SET
			title=$1, abstract=$2, description=$3,
			applicants=$4, inventors=$5, ipc_codes=$6, jurisdiction=$7,
			filing_date=$8, publication_date=$9, grant_date=$10, expiry_date=$11,
			priority=$12, family_id=$13, status=$14, legal_status=$15, citations=$16,
			metadata=$17, updated_at=$18, version=$19
		WHERE id=$20 AND version=$21`,
		p.Title, p.Abstract, p.Description,
		p.Applicants, p.Inventors, p.IPCCodes, p.Jurisdiction,
		p.FilingDate, p.PublicationDate, p.GrantDate, p.ExpiryDate,
		p.Priority, p.FamilyID, p.Status, p.LegalStatus, p.Citations,
		metaJSON, time.Now().UTC(), newVersion,
		p.ID, p.Version,
	)
	if err != nil {
		r.logger.Error("PatentRepository.Update: exec", "error", err)
		return appErrors.Wrap(err, appErrors.CodeDBQueryError, "failed to update patent")
	}
	if tag.RowsAffected() == 0 {
		return appErrors.New(appErrors.CodeConflict, "optimistic lock conflict: patent version mismatch").
			WithDetail(fmt.Sprintf("patent_id=%s expected_version=%d", p.ID, p.Version))
	}

	// Replace claims: delete existing, then re-insert.
	if _, err := tx.Exec(ctx, `DELETE FROM patent_claims WHERE patent_id = $1`, p.ID); err != nil {
		r.logger.Error("PatentRepository.Update: delete claims", "error", err)
		return appErrors.Wrap(err, appErrors.CodeDBQueryError, "failed to delete existing claims")
	}
	if err := r.insertClaims(ctx, tx, p.ID, p.Claims); err != nil {
		return err
	}

	// Replace markush structures.
	if _, err := tx.Exec(ctx, `DELETE FROM markush_structures WHERE patent_id = $1`, p.ID); err != nil {
		r.logger.Error("PatentRepository.Update: delete markush", "error", err)
		return appErrors.Wrap(err, appErrors.CodeDBQueryError, "failed to delete existing markush structures")
	}
	if err := r.insertMarkushStructures(ctx, tx, p.ID, p.MarkushStructures); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		r.logger.Error("PatentRepository.Update: commit", "error", err)
		return appErrors.Wrap(err, appErrors.CodeDBConnectionError, "failed to commit update transaction")
	}

	p.Version = newVersion
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Delete
// ─────────────────────────────────────────────────────────────────────────────

// Delete performs a soft delete (sets status to 'deleted') by default.
// When hard is true it CASCADE-deletes the patent and all associated records.
func (r *PatentRepository) Delete(ctx context.Context, id common.ID, hard bool) error {
	r.logger.Debug("PatentRepository.Delete", "id", id, "hard", hard)

	if hard {
		tx, err := r.pool.Begin(ctx)
		if err != nil {
			return appErrors.Wrap(err, appErrors.CodeDBConnectionError, "failed to begin delete transaction")
		}
		defer tx.Rollback(ctx) //nolint:errcheck

		if _, err := tx.Exec(ctx, `DELETE FROM markush_structures WHERE patent_id = $1`, id); err != nil {
			r.logger.Error("PatentRepository.Delete: markush", "error", err)
			return appErrors.Wrap(err, appErrors.CodeDBQueryError, "failed to delete markush structures")
		}
		if _, err := tx.Exec(ctx, `DELETE FROM patent_claims WHERE patent_id = $1`, id); err != nil {
			r.logger.Error("PatentRepository.Delete: claims", "error", err)
			return appErrors.Wrap(err, appErrors.CodeDBQueryError, "failed to delete claims")
		}
		if _, err := tx.Exec(ctx, `DELETE FROM patents WHERE id = $1`, id); err != nil {
			r.logger.Error("PatentRepository.Delete: patent", "error", err)
			return appErrors.Wrap(err, appErrors.CodeDBQueryError, "failed to delete patent")
		}
		return tx.Commit(ctx)
	}

	tag, err := r.pool.Exec(ctx, `UPDATE patents SET status = 'deleted', updated_at = $1 WHERE id = $2`,
		time.Now().UTC(), id)
	if err != nil {
		r.logger.Error("PatentRepository.Delete: soft", "error", err)
		return appErrors.Wrap(err, appErrors.CodeDBQueryError, "failed to soft-delete patent")
	}
	if tag.RowsAffected() == 0 {
		return appErrors.New(appErrors.CodePatentNotFound, "patent not found").WithDetail(fmt.Sprintf("id=%s", id))
	}
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// FindByApplicant
// ─────────────────────────────────────────────────────────────────────────────

func (r *PatentRepository) FindByApplicant(ctx context.Context, applicant string, page, pageSize int) ([]*Patent, int64, error) {
	r.logger.Debug("PatentRepository.FindByApplicant", "applicant", applicant)
	return r.findByArrayContains(ctx, "applicants", applicant, page, pageSize)
}

// ─────────────────────────────────────────────────────────────────────────────
// FindByJurisdiction
// ─────────────────────────────────────────────────────────────────────────────

func (r *PatentRepository) FindByJurisdiction(ctx context.Context, jurisdiction string, page, pageSize int) ([]*Patent, int64, error) {
	r.logger.Debug("PatentRepository.FindByJurisdiction", "jurisdiction", jurisdiction)
	return r.findByScalarColumn(ctx, "jurisdiction", jurisdiction, page, pageSize)
}

// ─────────────────────────────────────────────────────────────────────────────
// FindByIPCCode
// ─────────────────────────────────────────────────────────────────────────────

func (r *PatentRepository) FindByIPCCode(ctx context.Context, ipcCode string, page, pageSize int) ([]*Patent, int64, error) {
	r.logger.Debug("PatentRepository.FindByIPCCode", "ipc_code", ipcCode)
	return r.findByArrayContains(ctx, "ipc_codes", ipcCode, page, pageSize)
}

// ─────────────────────────────────────────────────────────────────────────────
// CountByStatus
// ─────────────────────────────────────────────────────────────────────────────

// CountByStatus returns a map of status → count for all patents.
func (r *PatentRepository) CountByStatus(ctx context.Context) (map[string]int64, error) {
	r.logger.Debug("PatentRepository.CountByStatus")

	rows, err := r.pool.Query(ctx, `SELECT status, COUNT(*) FROM patents GROUP BY status`)
	if err != nil {
		r.logger.Error("PatentRepository.CountByStatus", "error", err)
		return nil, appErrors.Wrap(err, appErrors.CodeDBQueryError, "failed to count patents by status")
	}
	defer rows.Close()

	result := make(map[string]int64)
	for rows.Next() {
		var status string
		var count int64
		if err := rows.Scan(&status, &count); err != nil {
			r.logger.Error("PatentRepository.CountByStatus: scan", "error", err)
			return nil, appErrors.Wrap(err, appErrors.CodeDBQueryError, "failed to scan status count")
		}
		result[status] = count
	}
	return result, rows.Err()
}

// ─────────────────────────────────────────────────────────────────────────────
// FindExpiring
// ─────────────────────────────────────────────────────────────────────────────

// FindExpiring returns patents whose expiry_date is on or before the given
// deadline and whose status is not already terminal.
func (r *PatentRepository) FindExpiring(ctx context.Context, before time.Time, page, pageSize int) ([]*Patent, int64, error) {
	r.logger.Debug("PatentRepository.FindExpiring", "before", before)

	where := `WHERE expiry_date <= $1 AND status NOT IN ('expired','abandoned','revoked','deleted')`

	var total int64
	if err := r.pool.QueryRow(ctx, fmt.Sprintf("SELECT COUNT(*) FROM patents %s", where), before).Scan(&total); err != nil {
		r.logger.Error("PatentRepository.FindExpiring: count", "error", err)
		return nil, 0, appErrors.Wrap(err, appErrors.CodeDBQueryError, "failed to count expiring patents")
	}

	if pageSize <= 0 {
		pageSize = 20
	}
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * pageSize

	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id, tenant_id, patent_number, title, abstract, description,
		       applicants, inventors, ipc_codes, jurisdiction,
		       filing_date, publication_date, grant_date, expiry_date,
		       priority, family_id, status, legal_status, citations,
		       metadata, created_at, updated_at, created_by, version
		FROM patents %s
		ORDER BY expiry_date ASC
		LIMIT $2 
		OFFSET $3`, where), before, pageSize, offset)
	if err != nil {
		r.logger.Error("PatentRepository.FindExpiring: query", "error", err)
		return nil, 0, appErrors.Wrap(err, appErrors.CodeDBQueryError, "failed to query expiring patents")
	}
	defer rows.Close()

	patents, err := r.scanPatents(rows)
	if err != nil {
		return nil, 0, err
	}
	return patents, total, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Internal helpers — reusable paginated finders
// ─────────────────────────────────────────────────────────────────────────────

// findByScalarColumn is a generic helper for paginated queries that filter on
// a single scalar column (e.g., jurisdiction = $1).
func (r *PatentRepository) findByScalarColumn(
	ctx context.Context, column, value string, page, pageSize int,
) ([]*Patent, int64, error) {
	where := fmt.Sprintf("WHERE %s = $1", column)

	var total int64
	if err := r.pool.QueryRow(ctx,
		fmt.Sprintf("SELECT COUNT(*) FROM patents %s", where), value,
	).Scan(&total); err != nil {
		r.logger.Error("PatentRepository.findByScalarColumn: count", "column", column, "error", err)
		return nil, 0, appErrors.Wrap(err, appErrors.CodeDBQueryError, "count failed")
	}

	if pageSize <= 0 {
		pageSize = 20
	}
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * pageSize

	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id, tenant_id, patent_number, title, abstract, description,
		       applicants, inventors, ipc_codes, jurisdiction,
		       filing_date, publication_date, grant_date, expiry_date,
		       priority, family_id, status, legal_status, citations,
		       metadata, created_at, updated_at, created_by, version
		FROM patents %s
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`, where), value, pageSize, offset)
	if err != nil {
		r.logger.Error("PatentRepository.findByScalarColumn: query", "column", column, "error", err)
		return nil, 0, appErrors.Wrap(err, appErrors.CodeDBQueryError, "query failed")
	}
	defer rows.Close()

	patents, err := r.scanPatents(rows)
	return patents, total, err
}

// findByArrayContains is a generic helper for paginated queries that filter on
// a PostgreSQL TEXT[] column using the @> (contains) operator.
func (r *PatentRepository) findByArrayContains(
	ctx context.Context, column, value string, page, pageSize int,
) ([]*Patent, int64, error) {
	where := fmt.Sprintf("WHERE %s @> ARRAY[$1]::TEXT[]", column)

	var total int64
	if err := r.pool.QueryRow(ctx,
		fmt.Sprintf("SELECT COUNT(*) FROM patents %s", where), value,
	).Scan(&total); err != nil {
		r.logger.Error("PatentRepository.findByArrayContains: count", "column", column, "error", err)
		return nil, 0, appErrors.Wrap(err, appErrors.CodeDBQueryError, "count failed")
	}

	if pageSize <= 0 {
		pageSize = 20
	}
	if page < 1 {
		page = 1
	}
	offset := (page - 1) * pageSize

	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id, tenant_id, patent_number, title, abstract, description,
		       applicants, inventors, ipc_codes, jurisdiction,
		       filing_date, publication_date, grant_date, expiry_date,
		       priority, family_id, status, legal_status, citations,
		       metadata, created_at, updated_at, created_by, version
		FROM patents %s
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3`, where), value, pageSize, offset)
	if err != nil {
		r.logger.Error("PatentRepository.findByArrayContains: query", "column", column, "error", err)
		return nil, 0, appErrors.Wrap(err, appErrors.CodeDBQueryError, "query failed")
	}
	defer rows.Close()

	patents, err := r.scanPatents(rows)
	return patents, total, err
}

// ─────────────────────────────────────────────────────────────────────────────
// Internal helpers — row scanners
// ─────────────────────────────────────────────────────────────────────────────

// findClaimsByPatentID loads all claims for a given patent.
func (r *PatentRepository) findClaimsByPatentID(ctx context.Context, patentID common.ID) ([]Claim, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, patent_id, claim_number, claim_type, parent_id, text, is_independent
		FROM patent_claims
		WHERE patent_id = $1
		ORDER BY claim_number ASC`, patentID)
	if err != nil {
		r.logger.Error("findClaimsByPatentID", "error", err)
		return nil, appErrors.Wrap(err, appErrors.CodeDBQueryError, "failed to query claims")
	}
	defer rows.Close()

	var claims []Claim
	for rows.Next() {
		var c Claim
		if err := rows.Scan(&c.ID, &c.PatentID, &c.ClaimNumber, &c.ClaimType,
			&c.ParentID, &c.Text, &c.IsIndependent); err != nil {
			r.logger.Error("findClaimsByPatentID: scan", "error", err)
			return nil, appErrors.Wrap(err, appErrors.CodeDBQueryError, "failed to scan claim")
		}
		claims = append(claims, c)
	}
	return claims, rows.Err()
}

// findMarkushByPatentID loads all Markush structures for a given patent.
func (r *PatentRepository) findMarkushByPatentID(ctx context.Context, patentID common.ID) ([]MarkushStructure, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, patent_id, smarts, description, r_groups
		FROM markush_structures
		WHERE patent_id = $1`, patentID)
	if err != nil {
		r.logger.Error("findMarkushByPatentID", "error", err)
		return nil, appErrors.Wrap(err, appErrors.CodeDBQueryError, "failed to query markush structures")
	}
	defer rows.Close()

	var result []MarkushStructure
	for rows.Next() {
		var m MarkushStructure
		var rgJSON []byte
		if err := rows.Scan(&m.ID, &m.PatentID, &m.SMARTS, &m.Description, &rgJSON); err != nil {
			r.logger.Error("findMarkushByPatentID: scan", "error", err)
			return nil, appErrors.Wrap(err, appErrors.CodeDBQueryError, "failed to scan markush structure")
		}
		if len(rgJSON) > 0 {
			_ = json.Unmarshal(rgJSON, &m.RGroups)
		}
		result = append(result, m)
	}
	return result, rows.Err()
}

// scanPatent scans a single row into a Patent struct.
func (r *PatentRepository) scanPatent(row pgx.Row) (*Patent, error) {
	var p Patent
	var metaJSON []byte

	err := row.Scan(
		&p.ID, &p.TenantID, &p.PatentNumber, &p.Title, &p.Abstract, &p.Description,
		&p.Applicants, &p.Inventors, &p.IPCCodes, &p.Jurisdiction,
		&p.FilingDate, &p.PublicationDate, &p.GrantDate, &p.ExpiryDate,
		&p.Priority, &p.FamilyID, &p.Status, &p.LegalStatus, &p.Citations,
		&metaJSON, &p.CreatedAt, &p.UpdatedAt, &p.CreatedBy, &p.Version,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, appErrors.New(appErrors.CodePatentNotFound, "patent not found")
		}
		r.logger.Error("scanPatent", "error", err)
		return nil, appErrors.Wrap(err, appErrors.CodeDBQueryError, "failed to scan patent row")
	}

	if len(metaJSON) > 0 {
		_ = json.Unmarshal(metaJSON, &p.Metadata)
	}
	return &p, nil
}

// scanPatents scans multiple rows into a Patent slice.
func (r *PatentRepository) scanPatents(rows pgx.Rows) ([]*Patent, error) {
	var patents []*Patent
	for rows.Next() {
		var p Patent
		var metaJSON []byte

		err := rows.Scan(
			&p.ID, &p.TenantID, &p.PatentNumber, &p.Title, &p.Abstract, &p.Description,
			&p.Applicants, &p.Inventors, &p.IPCCodes, &p.Jurisdiction,
			&p.FilingDate, &p.PublicationDate, &p.GrantDate, &p.ExpiryDate,
			&p.Priority, &p.FamilyID, &p.Status, &p.LegalStatus, &p.Citations,
			&metaJSON, &p.CreatedAt, &p.UpdatedAt, &p.CreatedBy, &p.Version,
		)
		if err != nil {
			r.logger.Error("scanPatents", "error", err)
			return nil, appErrors.Wrap(err, appErrors.CodeDBQueryError, "failed to scan patent row")
		}

		if len(metaJSON) > 0 {
			_ = json.Unmarshal(metaJSON, &p.Metadata)
		}
		patents = append(patents, &p)
	}
	if err := rows.Err(); err != nil {
		return nil, appErrors.Wrap(err, appErrors.CodeDBQueryError, "row iteration error")
	}
	return patents, nil
}

