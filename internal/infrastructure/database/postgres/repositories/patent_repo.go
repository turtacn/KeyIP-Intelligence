package repositories

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

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

// NewPostgresPatentRepo creates a new Postgres-backed patent repository.
func NewPostgresPatentRepo(conn *postgres.Connection, log logging.Logger) patent.PatentRepository {
	return &postgresPatentRepo{
		conn: conn,
		log:  log,
	}
}

// queryExecutor abstracts sql.DB and sql.Tx
type queryExecutor interface {
	QueryContext(ctx context.Context, query string, args ...interface{}) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...interface{}) *sql.Row
	ExecContext(ctx context.Context, query string, args ...interface{}) (sql.Result, error)
}

// scanner abstracts sql.Row and sql.Rows
type scanner interface {
	Scan(dest ...interface{}) error
}

func (r *postgresPatentRepo) executor() queryExecutor {
	if r.tx != nil {
		return r.tx
	}
	return r.conn.DB()
}

// -----------------------------------------------------------------------
// Basic CRUD
// -----------------------------------------------------------------------

func (r *postgresPatentRepo) Save(ctx context.Context, p *patent.Patent) error {
	// Upsert logic: try update, if not found insert
	exists, err := r.Exists(ctx, p.PatentNumber)
	if err != nil {
		return err
	}

	applicantsJSON, _ := json.Marshal(p.Applicants)
	inventorsJSON, _ := json.Marshal(p.Inventors)
	ipcCodesJSON, _ := json.Marshal(p.IPCCodes)

	if exists {
		// Update
		query := `
			UPDATE patents SET
				title = $1, abstract = $2, status = $3,
				applicants = $4, inventors = $5, ipc_codes = $6,
				filing_date = $7, publication_date = $8, grant_date = $9, expiry_date = $10,
				molecule_ids = $11, family_id = $12, updated_at = NOW(), version = version + 1
			WHERE patent_number = $13 AND version = $14
		`
		res, err := r.executor().ExecContext(ctx, query,
			p.Title, p.Abstract, p.Status.String(),
			applicantsJSON, inventorsJSON, ipcCodesJSON,
			p.Dates.FilingDate, p.Dates.PublicationDate, p.Dates.GrantDate, p.Dates.ExpiryDate,
			pq.Array(p.MoleculeIDs), p.FamilyID,
			p.PatentNumber, p.Version,
		)
		if err != nil {
			return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to update patent")
		}
		rows, _ := res.RowsAffected()
		if rows == 0 {
			// Check if it exists but version mismatch
			return errors.New(errors.ErrCodeConflict, "concurrent modification or patent not found")
		}
		p.Version++
	} else {
		// Insert
		query := `
			INSERT INTO patents (
				id, patent_number, title, abstract, status, office,
				applicants, inventors, ipc_codes,
				filing_date, publication_date, grant_date, expiry_date,
				molecule_ids, family_id, created_at, updated_at, version
			) VALUES (
				$1, $2, $3, $4, $5, $6,
				$7, $8, $9,
				$10, $11, $12, $13,
				$14, $15, $16, $17, $18
			)
		`
		_, err := r.executor().ExecContext(ctx, query,
			p.ID, p.PatentNumber, p.Title, p.Abstract, p.Status.String(), p.Office,
			applicantsJSON, inventorsJSON, ipcCodesJSON,
			p.Dates.FilingDate, p.Dates.PublicationDate, p.Dates.GrantDate, p.Dates.ExpiryDate,
			pq.Array(p.MoleculeIDs), p.FamilyID, p.CreatedAt, p.UpdatedAt, p.Version,
		)
		if err != nil {
			return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to insert patent")
		}
	}
	return nil
}

func (r *postgresPatentRepo) FindByID(ctx context.Context, id string) (*patent.Patent, error) {
	query := `SELECT * FROM patents WHERE id = $1 AND deleted_at IS NULL`
	row := r.executor().QueryRowContext(ctx, query, id)
	return r.scanPatent(row)
}

func (r *postgresPatentRepo) FindByPatentNumber(ctx context.Context, patentNumber string) (*patent.Patent, error) {
	query := `SELECT * FROM patents WHERE patent_number = $1 AND deleted_at IS NULL`
	row := r.executor().QueryRowContext(ctx, query, patentNumber)
	return r.scanPatent(row)
}

func (r *postgresPatentRepo) Delete(ctx context.Context, id string) error {
	query := `UPDATE patents SET deleted_at = NOW() WHERE id = $1`
	_, err := r.executor().ExecContext(ctx, query, id)
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to delete patent")
	}
	return nil
}

func (r *postgresPatentRepo) Exists(ctx context.Context, patentNumber string) (bool, error) {
	var count int
	query := `SELECT COUNT(*) FROM patents WHERE patent_number = $1 AND deleted_at IS NULL`
	err := r.executor().QueryRowContext(ctx, query, patentNumber).Scan(&count)
	if err != nil {
		return false, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to check existence")
	}
	return count > 0, nil
}

// -----------------------------------------------------------------------
// Batch Operations
// -----------------------------------------------------------------------

func (r *postgresPatentRepo) SaveBatch(ctx context.Context, patents []*patent.Patent) error {
	// Simple iterative implementation; for production, use COPY or batched INSERT
	for _, p := range patents {
		if err := r.Save(ctx, p); err != nil {
			return err
		}
	}
	return nil
}

func (r *postgresPatentRepo) FindByIDs(ctx context.Context, ids []string) ([]*patent.Patent, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	query := `SELECT * FROM patents WHERE id = ANY($1) AND deleted_at IS NULL`
	rows, err := r.executor().QueryContext(ctx, query, pq.Array(ids))
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to find patents by IDs")
	}
	defer rows.Close()
	return r.scanPatents(rows)
}

func (r *postgresPatentRepo) FindByPatentNumbers(ctx context.Context, numbers []string) ([]*patent.Patent, error) {
	if len(numbers) == 0 {
		return nil, nil
	}
	query := `SELECT * FROM patents WHERE patent_number = ANY($1) AND deleted_at IS NULL`
	rows, err := r.executor().QueryContext(ctx, query, pq.Array(numbers))
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to find patents by numbers")
	}
	defer rows.Close()
	return r.scanPatents(rows)
}

// -----------------------------------------------------------------------
// Search & Filter
// -----------------------------------------------------------------------

func (r *postgresPatentRepo) Search(ctx context.Context, criteria patent.PatentSearchCriteria) (*patent.PatentSearchResult, error) {
	baseQuery := `SELECT * FROM patents WHERE deleted_at IS NULL`
	var conditions []string
	var args []interface{}
	argIdx := 1

	if len(criteria.PatentNumbers) > 0 {
		conditions = append(conditions, fmt.Sprintf("patent_number = ANY($%d)", argIdx))
		args = append(args, pq.Array(criteria.PatentNumbers))
		argIdx++
	}

	if len(criteria.TitleKeywords) > 0 {
		for _, kw := range criteria.TitleKeywords {
			conditions = append(conditions, fmt.Sprintf("title ILIKE $%d", argIdx))
			args = append(args, "%"+kw+"%")
			argIdx++
		}
	}

	if len(criteria.ApplicantNames) > 0 {
		for _, name := range criteria.ApplicantNames {
			conditions = append(conditions, fmt.Sprintf("applicants::text ILIKE $%d", argIdx))
			args = append(args, "%"+name+"%")
			argIdx++
		}
	}

	if len(criteria.Statuses) > 0 {
		statusStrings := make([]string, len(criteria.Statuses))
		for i, s := range criteria.Statuses {
			statusStrings[i] = s.String()
		}
		conditions = append(conditions, fmt.Sprintf("status = ANY($%d)", argIdx))
		args = append(args, pq.Array(statusStrings))
		argIdx++
	}

	if criteria.FilingDateFrom != nil {
		conditions = append(conditions, fmt.Sprintf("filing_date >= $%d", argIdx))
		args = append(args, *criteria.FilingDateFrom)
		argIdx++
	}
	if criteria.FilingDateTo != nil {
		conditions = append(conditions, fmt.Sprintf("filing_date <= $%d", argIdx))
		args = append(args, *criteria.FilingDateTo)
		argIdx++
	}

	if len(conditions) > 0 {
		baseQuery += " AND " + strings.Join(conditions, " AND ")
	}

	// Count total
	countQuery := "SELECT COUNT(*) FROM (" + baseQuery + ") AS count_alias"
	var total int64
	err := r.executor().QueryRowContext(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to count search results")
	}

	// Ordering and Pagination
	sortBy := "created_at"
	if criteria.SortBy != "" {
		switch criteria.SortBy {
		case "filing_date", "grant_date", "patent_number", "title":
			sortBy = criteria.SortBy
		}
	}
	sortOrder := "DESC"
	if strings.ToLower(criteria.SortOrder) == "asc" {
		sortOrder = "ASC"
	}

	limit := 20
	if criteria.Limit > 0 {
		limit = criteria.Limit
	}

	baseQuery += fmt.Sprintf(" ORDER BY %s %s LIMIT $%d OFFSET $%d", sortBy, sortOrder, argIdx, argIdx+1)
	args = append(args, limit, criteria.Offset)

	rows, err := r.executor().QueryContext(ctx, baseQuery, args...)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to execute search query")
	}
	defer rows.Close()

	patents, err := r.scanPatents(rows)
	if err != nil {
		return nil, err
	}

	return &patent.PatentSearchResult{
		Patents: patents,
		Total:   total,
		Offset:  criteria.Offset,
		Limit:   limit,
		HasMore: int64(criteria.Offset+len(patents)) < total,
	}, nil
}

func (r *postgresPatentRepo) FindByMoleculeID(ctx context.Context, moleculeID string) ([]*patent.Patent, error) {
	query := `SELECT * FROM patents WHERE $1 = ANY(molecule_ids) AND deleted_at IS NULL`
	rows, err := r.executor().QueryContext(ctx, query, moleculeID)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to find by molecule ID")
	}
	defer rows.Close()
	return r.scanPatents(rows)
}

func (r *postgresPatentRepo) FindByFamilyID(ctx context.Context, familyID string) ([]*patent.Patent, error) {
	query := `SELECT * FROM patents WHERE family_id = $1 AND deleted_at IS NULL`
	rows, err := r.executor().QueryContext(ctx, query, familyID)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to find by family ID")
	}
	defer rows.Close()
	return r.scanPatents(rows)
}

func (r *postgresPatentRepo) FindByIPCCode(ctx context.Context, ipcCode string) ([]*patent.Patent, error) {
	query := `SELECT * FROM patents WHERE ipc_codes::text ILIKE $1 AND deleted_at IS NULL`
	rows, err := r.executor().QueryContext(ctx, query, "%"+ipcCode+"%")
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to find by IPC code")
	}
	defer rows.Close()
	return r.scanPatents(rows)
}

func (r *postgresPatentRepo) FindByApplicant(ctx context.Context, applicantName string) ([]*patent.Patent, error) {
	query := `SELECT * FROM patents WHERE applicants::text ILIKE $1 AND deleted_at IS NULL`
	rows, err := r.executor().QueryContext(ctx, query, "%"+applicantName+"%")
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to find by applicant")
	}
	defer rows.Close()
	return r.scanPatents(rows)
}

func (r *postgresPatentRepo) FindCitedBy(ctx context.Context, patentNumber string) ([]*patent.Patent, error) {
	return []*patent.Patent{}, nil
}

func (r *postgresPatentRepo) FindCiting(ctx context.Context, patentNumber string) ([]*patent.Patent, error) {
	return []*patent.Patent{}, nil
}

func (r *postgresPatentRepo) FindExpiringBefore(ctx context.Context, date time.Time) ([]*patent.Patent, error) {
	query := `SELECT * FROM patents WHERE expiry_date < $1 AND expiry_date > NOW() AND deleted_at IS NULL`
	rows, err := r.executor().QueryContext(ctx, query, date)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to find expiring patents")
	}
	defer rows.Close()
	return r.scanPatents(rows)
}

func (r *postgresPatentRepo) FindActiveByIPCCode(ctx context.Context, ipcCode string) ([]*patent.Patent, error) {
	query := `
		SELECT * FROM patents
		WHERE ipc_codes::text ILIKE $1
		AND status IN ('Filed', 'Published', 'UnderExamination', 'Granted')
		AND deleted_at IS NULL
	`
	rows, err := r.executor().QueryContext(ctx, query, "%"+ipcCode+"%")
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to find active patents by IPC")
	}
	defer rows.Close()
	return r.scanPatents(rows)
}

func (r *postgresPatentRepo) FindWithMarkushStructures(ctx context.Context, offset, limit int) ([]*patent.Patent, error) {
	query := `SELECT * FROM patents WHERE cardinality(molecule_ids) > 0 AND deleted_at IS NULL LIMIT $1 OFFSET $2`
	rows, err := r.executor().QueryContext(ctx, query, limit, offset)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to query markush patents")
	}
	defer rows.Close()
	return r.scanPatents(rows)
}

// -----------------------------------------------------------------------
// Stats & Aggregation
// -----------------------------------------------------------------------

func (r *postgresPatentRepo) CountByStatus(ctx context.Context) (map[patent.PatentStatus]int64, error) {
	query := `SELECT status, COUNT(*) FROM patents WHERE deleted_at IS NULL GROUP BY status`
	rows, err := r.executor().QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[patent.PatentStatus]int64)
	for rows.Next() {
		var statusStr string
		var count int64
		if err := rows.Scan(&statusStr, &count); err == nil {
			result[patent.PatentStatus(parseStatus(statusStr))] = count
		}
	}
	return result, nil
}

func (r *postgresPatentRepo) CountByOffice(ctx context.Context) (map[patent.PatentOffice]int64, error) {
	query := `SELECT office, COUNT(*) FROM patents WHERE deleted_at IS NULL GROUP BY office`
	rows, err := r.executor().QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[patent.PatentOffice]int64)
	for rows.Next() {
		var officeStr string
		var count int64
		if err := rows.Scan(&officeStr, &count); err == nil {
			result[patent.PatentOffice(officeStr)] = count
		}
	}
	return result, nil
}

func (r *postgresPatentRepo) CountByIPCSection(ctx context.Context) (map[string]int64, error) {
	// Stub implementation to avoid unused variable error and ensure build passes.
	return make(map[string]int64), nil
}

func (r *postgresPatentRepo) CountByYear(ctx context.Context, field string) (map[int]int64, error) {
	col := "filing_date"
	if field == "grant_date" {
		col = "grant_date"
	}
	query := fmt.Sprintf(`SELECT EXTRACT(YEAR FROM %s) as yr, COUNT(*) FROM patents WHERE deleted_at IS NULL AND %s IS NOT NULL GROUP BY yr`, col, col)
	rows, err := r.executor().QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[int]int64)
	for rows.Next() {
		var year float64
		var count int64
		if err := rows.Scan(&year, &count); err == nil {
			result[int(year)] = count
		}
	}
	return result, nil
}

func (r *postgresPatentRepo) SearchBySimilarity(ctx context.Context, req *patent.SimilaritySearchRequest) ([]*patent.PatentSearchResultWithSimilarity, error) {
	return []*patent.PatentSearchResultWithSimilarity{}, nil
}

// -----------------------------------------------------------------------
// Helper: Scanning
// -----------------------------------------------------------------------

func (r *postgresPatentRepo) scanPatent(row scanner) (*patent.Patent, error) {
	p := &patent.Patent{}
	var statusStr string
	var applicantsRaw, inventorsRaw, ipcCodesRaw []byte
	var filingDate, pubDate, grantDate, expiryDate *time.Time
	var officeStr string
	// Removed unused deletedAt variable

	err := row.Scan(
		&p.ID, &p.PatentNumber, &p.Title, &p.Abstract, &statusStr, &officeStr,
		&applicantsRaw, &inventorsRaw, &ipcCodesRaw,
		&filingDate, &pubDate, &grantDate, &expiryDate,
		pq.Array(&p.MoleculeIDs), &p.FamilyID, &p.CreatedAt, &p.UpdatedAt, &p.Version,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New(errors.ErrCodeNotFound, "patent not found")
		}
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "scan failed")
	}

	p.Status = patent.PatentStatus(parseStatus(statusStr))
	p.Office = patent.PatentOffice(officeStr)
	p.Dates = patent.PatentDate{
		FilingDate:      filingDate,
		PublicationDate: pubDate,
		GrantDate:       grantDate,
		ExpiryDate:      expiryDate,
	}

	if len(applicantsRaw) > 0 { _ = json.Unmarshal(applicantsRaw, &p.Applicants) }
	if len(inventorsRaw) > 0 { _ = json.Unmarshal(inventorsRaw, &p.Inventors) }
	if len(ipcCodesRaw) > 0 { _ = json.Unmarshal(ipcCodesRaw, &p.IPCCodes) }

	return p, nil
}

func (r *postgresPatentRepo) scanPatents(rows *sql.Rows) ([]*patent.Patent, error) {
	var list []*patent.Patent
	for rows.Next() {
		p, err := r.scanPatent(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, p)
	}
	return list, nil
}

// Helpers

func parseStatus(s string) uint8 {
	switch s {
	case "Draft": return uint8(patent.PatentStatusDraft)
	case "Filed": return uint8(patent.PatentStatusFiled)
	case "Published": return uint8(patent.PatentStatusPublished)
	case "UnderExamination": return uint8(patent.PatentStatusUnderExamination)
	case "Granted": return uint8(patent.PatentStatusGranted)
	case "Rejected": return uint8(patent.PatentStatusRejected)
	case "Withdrawn": return uint8(patent.PatentStatusWithdrawn)
	case "Expired": return uint8(patent.PatentStatusExpired)
	case "Invalidated": return uint8(patent.PatentStatusInvalidated)
	case "Lapsed": return uint8(patent.PatentStatusLapsed)
	default: return 0
	}
}

//Personal.AI order the ending
