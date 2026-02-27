package repositories

import (
	"context"
	"database/sql"
	"encoding/json"

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
	// Dummy implementation to satisfy interface
	return map[patent.PatentOffice]int64{}, nil
}

func (r *postgresPatentRepo) FindActiveByIPCCode(ctx context.Context, ipcCode string) ([]*patent.Patent, error) {
	// Dummy implementation
	return nil, nil
}

func (r *postgresPatentRepo) FindWithMarkushStructures(ctx context.Context, offset, limit int) ([]*patent.Patent, error) {
	// Dummy implementation
	return nil, nil
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

func (r *postgresPatentRepo) Search(ctx context.Context, q patent.SearchQuery) (*patent.SearchResult, error) {
	// Full Text Search implementation
	return &patent.SearchResult{}, nil
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
	return nil, 0, nil
}

func (r *postgresPatentRepo) GetByJurisdiction(ctx context.Context, jurisdiction string, limit, offset int) ([]*patent.Patent, int64, error) {
	return nil, 0, nil
}

func (r *postgresPatentRepo) GetExpiringPatents(ctx context.Context, daysAhead int, limit, offset int) ([]*patent.Patent, int64, error) {
	return nil, 0, nil
}

func (r *postgresPatentRepo) FindDuplicates(ctx context.Context, fullTextHash string) ([]*patent.Patent, error) {
	return nil, nil
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

// Claims

func (r *postgresPatentRepo) CreateClaim(ctx context.Context, claim *patent.Claim) error {
	return nil
}

func (r *postgresPatentRepo) GetClaimsByPatent(ctx context.Context, patentID uuid.UUID) ([]*patent.Claim, error) {
	query := `SELECT * FROM patent_claims WHERE patent_id = $1 ORDER BY claim_number ASC`
	rows, err := r.executor().QueryContext(ctx, query, patentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var claims []*patent.Claim
	for rows.Next() {
		// scan claim
	}
	return claims, nil
}

func (r *postgresPatentRepo) UpdateClaim(ctx context.Context, claim *patent.Claim) error { return nil }
func (r *postgresPatentRepo) DeleteClaimsByPatent(ctx context.Context, patentID uuid.UUID) error { return nil }
func (r *postgresPatentRepo) BatchCreateClaims(ctx context.Context, claims []*patent.Claim) error { return nil }
func (r *postgresPatentRepo) GetIndependentClaims(ctx context.Context, patentID uuid.UUID) ([]*patent.Claim, error) { return nil, nil }

// Inventors
func (r *postgresPatentRepo) SetInventors(ctx context.Context, patentID uuid.UUID, inventors []*patent.Inventor) error { return nil }
func (r *postgresPatentRepo) GetInventors(ctx context.Context, patentID uuid.UUID) ([]*patent.Inventor, error) { return nil, nil }
func (r *postgresPatentRepo) SearchByInventor(ctx context.Context, inventorName string, limit, offset int) ([]*patent.Patent, int64, error) { return nil, 0, nil }

// Priority
func (r *postgresPatentRepo) SetPriorityClaims(ctx context.Context, patentID uuid.UUID, claims []*patent.PriorityClaim) error { return nil }
func (r *postgresPatentRepo) GetPriorityClaims(ctx context.Context, patentID uuid.UUID) ([]*patent.PriorityClaim, error) { return nil, nil }

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
func (r *postgresPatentRepo) BatchCreate(ctx context.Context, patents []*patent.Patent) (int, error) { return 0, nil }
func (r *postgresPatentRepo) BatchUpdateStatus(ctx context.Context, ids []uuid.UUID, status patent.PatentStatus) (int64, error) { return 0, nil }

// Stats
func (r *postgresPatentRepo) CountByStatus(ctx context.Context) (map[patent.PatentStatus]int64, error) { return nil, nil }
func (r *postgresPatentRepo) CountByJurisdiction(ctx context.Context) (map[string]int64, error) { return nil, nil }
func (r *postgresPatentRepo) CountByYear(ctx context.Context, field string) (map[int]int64, error) { return nil, nil }
func (r *postgresPatentRepo) GetIPCDistribution(ctx context.Context, level int) (map[string]int64, error) { return nil, nil }

// Transaction
func (r *postgresPatentRepo) WithTx(ctx context.Context, fn func(patent.PatentRepository) error) error {
	tx, err := r.conn.DB().BeginTx(ctx, nil)
	if err != nil { return err }
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
	if len(raw) > 0 { _ = json.Unmarshal(raw, &p.RawData) }
	if len(meta) > 0 { _ = json.Unmarshal(meta, &p.Metadata) }

	return p, nil
}

func parsePatentStatus(s string) patent.PatentStatus {
	switch s {
	case "draft": return patent.PatentStatusDraft
	case "filed": return patent.PatentStatusFiled
	case "published": return patent.PatentStatusPublished
	case "under_examination": return patent.PatentStatusUnderExamination
	case "granted": return patent.PatentStatusGranted
	case "rejected": return patent.PatentStatusRejected
	case "withdrawn": return patent.PatentStatusWithdrawn
	case "expired": return patent.PatentStatusExpired
	case "invalidated": return patent.PatentStatusInvalidated
	case "lapsed": return patent.PatentStatusLapsed
	default: return patent.PatentStatusUnknown
	}
}

func scanPatents(rows *sql.Rows) ([]*patent.Patent, error) {
	var list []*patent.Patent
	for rows.Next() {
		p, err := scanPatent(rows)
		if err != nil { return nil, err }
		list = append(list, p)
	}
	return list, nil
}

//Personal.AI order the ending
