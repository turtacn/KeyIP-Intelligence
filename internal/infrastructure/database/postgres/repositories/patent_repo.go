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
	"github.com/turtacn/KeyIP-Intelligence/internal/domain/patent"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/database/postgres"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

type postgresPatentRepo struct {
	conn     *postgres.Connection
	log      logging.Logger
	executor queryExecutor
}

func NewPostgresPatentRepo(conn *postgres.Connection, log logging.Logger) patent.PatentRepository {
	return &postgresPatentRepo{
		conn:     conn,
		log:      log,
		executor: conn.DB(),
	}
}

// WithTx implementation
func (r *postgresPatentRepo) WithTx(ctx context.Context, fn func(patent.PatentRepository) error) error {
	tx, err := r.conn.DB().BeginTx(ctx, nil)
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to begin transaction")
	}

	txRepo := &postgresPatentRepo{
		conn:     r.conn,
		log:      r.log,
		executor: tx,
	}

	if err := fn(txRepo); err != nil {
		tx.Rollback()
		return err
	}

	if err := tx.Commit(); err != nil {
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to commit transaction")
	}
	return nil
}

func (r *postgresPatentRepo) Create(ctx context.Context, p *patent.Patent) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	query := `
		INSERT INTO patents (
			id, patent_number, title, title_en, abstract, abstract_en,
			patent_type, status, filing_date, publication_date, grant_date,
			expiry_date, priority_date, assignee_id, assignee_name, jurisdiction,
			ipc_codes, cpc_codes, keyip_tech_codes, family_id, application_number,
			full_text_hash, source, raw_data, metadata
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, $24, $25)
		RETURNING created_at, updated_at
	`

	rawJSON, _ := json.Marshal(p.RawData)
	metaJSON, _ := json.Marshal(p.Metadata)

	err := r.executor.QueryRowContext(ctx, query,
		p.ID, p.PatentNumber, p.Title, p.TitleEn, p.Abstract, p.AbstractEn,
		p.Type, p.Status, p.FilingDate, p.PublicationDate, p.GrantDate,
		p.ExpiryDate, p.PriorityDate, p.AssigneeID, p.AssigneeName, p.Jurisdiction,
		pq.Array(p.IPCCodes), pq.Array(p.CPCCodes), pq.Array(p.KeyIPTechCodes),
		p.FamilyID, p.ApplicationNumber, p.FullTextHash, p.Source, rawJSON, metaJSON,
	).Scan(&p.CreatedAt, &p.UpdatedAt)

	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" {
			return errors.Wrap(err, errors.ErrCodePatentAlreadyExists, "patent already exists")
		}
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to create patent")
	}
	return nil
}

func (r *postgresPatentRepo) GetByID(ctx context.Context, id uuid.UUID) (*patent.Patent, error) {
	query := `SELECT * FROM patents WHERE id = $1 AND deleted_at IS NULL`
	row := r.executor.QueryRowContext(ctx, query, id)
	p, err := scanPatent(row)
	if err != nil {
		return nil, err
	}
	p.Claims, _ = r.GetClaimsByPatent(ctx, p.ID)
	p.Inventors, _ = r.GetInventors(ctx, p.ID)
	p.PriorityClaims, _ = r.GetPriorityClaims(ctx, p.ID)
	return p, nil
}

func (r *postgresPatentRepo) GetByPatentNumber(ctx context.Context, number string) (*patent.Patent, error) {
	query := `SELECT * FROM patents WHERE patent_number = $1 AND deleted_at IS NULL`
	row := r.executor.QueryRowContext(ctx, query, number)
	p, err := scanPatent(row)
	if err != nil {
		return nil, err
	}
	p.Claims, _ = r.GetClaimsByPatent(ctx, p.ID)
	p.Inventors, _ = r.GetInventors(ctx, p.ID)
	p.PriorityClaims, _ = r.GetPriorityClaims(ctx, p.ID)
	return p, nil
}

func (r *postgresPatentRepo) Update(ctx context.Context, p *patent.Patent) error {
	query := `
		UPDATE patents SET
			patent_number = $2, title = $3, title_en = $4, abstract = $5, abstract_en = $6,
			patent_type = $7, status = $8, filing_date = $9, publication_date = $10, grant_date = $11,
			expiry_date = $12, priority_date = $13, assignee_id = $14, assignee_name = $15, jurisdiction = $16,
			ipc_codes = $17, cpc_codes = $18, keyip_tech_codes = $19, family_id = $20, application_number = $21,
			full_text_hash = $22, source = $23, raw_data = $24, metadata = $25,
			updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
	`
	rawJSON, _ := json.Marshal(p.RawData)
	metaJSON, _ := json.Marshal(p.Metadata)

	res, err := r.executor.ExecContext(ctx, query,
		p.ID, p.PatentNumber, p.Title, p.TitleEn, p.Abstract, p.AbstractEn,
		p.Type, p.Status, p.FilingDate, p.PublicationDate, p.GrantDate,
		p.ExpiryDate, p.PriorityDate, p.AssigneeID, p.AssigneeName, p.Jurisdiction,
		pq.Array(p.IPCCodes), pq.Array(p.CPCCodes), pq.Array(p.KeyIPTechCodes),
		p.FamilyID, p.ApplicationNumber, p.FullTextHash, p.Source, rawJSON, metaJSON,
	)
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to update patent")
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return errors.ErrPatentNotFound(p.ID.String())
	}
	return nil
}

func (r *postgresPatentRepo) SoftDelete(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE patents SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL`
	res, err := r.executor.ExecContext(ctx, query, id)
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to delete patent")
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return errors.ErrPatentNotFound(id.String())
	}
	return nil
}

func (r *postgresPatentRepo) Restore(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE patents SET deleted_at = NULL WHERE id = $1`
	res, err := r.executor.ExecContext(ctx, query, id)
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to restore patent")
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return errors.ErrPatentNotFound(id.String())
	}
	return nil
}

func (r *postgresPatentRepo) HardDelete(ctx context.Context, id uuid.UUID) error {
	query := `DELETE FROM patents WHERE id = $1`
	res, err := r.executor.ExecContext(ctx, query, id)
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to hard delete patent")
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return errors.ErrPatentNotFound(id.String())
	}
	return nil
}

func (r *postgresPatentRepo) Search(ctx context.Context, query patent.SearchQuery) (*patent.SearchResult, error) {
	whereClause, whereArgs := r.buildWhereClause(query)

	// Count
	var total int64
	countQ := `SELECT COUNT(*) FROM patents ` + whereClause
	if err := r.executor.QueryRowContext(ctx, countQ, whereArgs...).Scan(&total); err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to count search results")
	}

	// Data
	selectQ := `SELECT * FROM patents ` + whereClause
	// Order
	orderClause := " ORDER BY created_at DESC"
	if query.SortBy != "" {
		if query.SortBy == "relevance" && query.Keyword != "" {
			// ts_rank
			selectQ = `SELECT *, ts_rank(to_tsvector('english', title || ' ' || COALESCE(abstract, '')), to_tsquery($` + fmt.Sprintf("%d", len(whereArgs)+1) + `)) AS rank FROM patents ` + whereClause
			whereArgs = append(whereArgs, strings.ReplaceAll(query.Keyword, " ", " & "))
			orderClause = " ORDER BY rank DESC"
		} else {
			// default desc if not specified
			orderClause = " ORDER BY " + query.SortBy + " DESC"
		}
	}

	selectQ += orderClause
	// Limit
	selectQ += fmt.Sprintf(" LIMIT $%d OFFSET $%d", len(whereArgs)+1, len(whereArgs)+2)
	whereArgs = append(whereArgs, query.Limit, query.Offset)

	rows, err := r.executor.QueryContext(ctx, selectQ, whereArgs...)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "search failed")
	}
	defer rows.Close()

	patents, err := collectPatents(rows)
	if err != nil {
		return nil, err
	}

	facets := make(map[string]map[string]int64)

	return &patent.SearchResult{
		Items:      patents,
		TotalCount: total,
		Facets:     facets,
	}, nil
}

func (r *postgresPatentRepo) buildWhereClause(q patent.SearchQuery) (string, []interface{}) {
	var clauses []string
	var args []interface{}
	clauses = append(clauses, "deleted_at IS NULL")

	if q.Keyword != "" {
		args = append(args, strings.ReplaceAll(q.Keyword, " ", " & "))
		clauses = append(clauses, fmt.Sprintf("to_tsvector('english', title || ' ' || COALESCE(abstract, '')) @@ to_tsquery($%d)", len(args)))
	}
	if q.Status != nil {
		args = append(args, *q.Status)
		clauses = append(clauses, fmt.Sprintf("status = $%d", len(args)))
	}
	if q.Jurisdiction != "" {
		args = append(args, q.Jurisdiction)
		clauses = append(clauses, fmt.Sprintf("jurisdiction = $%d", len(args)))
	}
	if q.AssigneeID != nil {
		args = append(args, q.AssigneeID)
		clauses = append(clauses, fmt.Sprintf("assignee_id = $%d", len(args)))
	}

	return "WHERE " + strings.Join(clauses, " AND "), args
}

func (r *postgresPatentRepo) GetByFamilyID(ctx context.Context, familyID string) ([]*patent.Patent, error) {
	query := `SELECT * FROM patents WHERE family_id = $1 AND deleted_at IS NULL`
	rows, err := r.executor.QueryContext(ctx, query, familyID)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to get by family id")
	}
	defer rows.Close()
	return collectPatents(rows)
}

func (r *postgresPatentRepo) GetByAssignee(ctx context.Context, assigneeID uuid.UUID, limit, offset int) ([]*patent.Patent, int64, error) {
	countQuery := `SELECT COUNT(*) FROM patents WHERE assignee_id = $1 AND deleted_at IS NULL`
	var total int64
	if err := r.executor.QueryRowContext(ctx, countQuery, assigneeID).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := `SELECT * FROM patents WHERE assignee_id = $1 AND deleted_at IS NULL ORDER BY filing_date DESC LIMIT $2 OFFSET $3`
	rows, err := r.executor.QueryContext(ctx, query, assigneeID, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	patents, err := collectPatents(rows)
	return patents, total, err
}

func (r *postgresPatentRepo) GetByJurisdiction(ctx context.Context, jurisdiction string, limit, offset int) ([]*patent.Patent, int64, error) {
	countQuery := `SELECT COUNT(*) FROM patents WHERE jurisdiction = $1 AND deleted_at IS NULL`
	var total int64
	if err := r.executor.QueryRowContext(ctx, countQuery, jurisdiction).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := `SELECT * FROM patents WHERE jurisdiction = $1 AND deleted_at IS NULL ORDER BY filing_date DESC LIMIT $2 OFFSET $3`
	rows, err := r.executor.QueryContext(ctx, query, jurisdiction, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	patents, err := collectPatents(rows)
	return patents, total, err
}

func (r *postgresPatentRepo) GetExpiringPatents(ctx context.Context, daysAhead int, limit, offset int) ([]*patent.Patent, int64, error) {
	targetDate := time.Now().AddDate(0, 0, daysAhead)
	now := time.Now()

	countQuery := `SELECT COUNT(*) FROM patents WHERE expiry_date BETWEEN $1 AND $2 AND deleted_at IS NULL`
	var total int64
	if err := r.executor.QueryRowContext(ctx, countQuery, now, targetDate).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := `SELECT * FROM patents WHERE expiry_date BETWEEN $1 AND $2 AND deleted_at IS NULL ORDER BY expiry_date ASC LIMIT $3 OFFSET $4`
	rows, err := r.executor.QueryContext(ctx, query, now, targetDate, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	patents, err := collectPatents(rows)
	return patents, total, err
}

func (r *postgresPatentRepo) FindDuplicates(ctx context.Context, fullTextHash string) ([]*patent.Patent, error) {
	query := `SELECT * FROM patents WHERE full_text_hash = $1 AND deleted_at IS NULL`
	rows, err := r.executor.QueryContext(ctx, query, fullTextHash)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return collectPatents(rows)
}

// Missing methods implementation

func (r *postgresPatentRepo) ListByPortfolio(ctx context.Context, portfolioID string) ([]*patent.Patent, error) {
	query := `
		SELECT p.* FROM patents p
		JOIN portfolio_patents pp ON p.id = pp.patent_id
		WHERE pp.portfolio_id = $1 AND p.deleted_at IS NULL
	`
	rows, err := r.executor.QueryContext(ctx, query, portfolioID)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to list by portfolio")
	}
	defer rows.Close()
	return collectPatents(rows)
}

func (r *postgresPatentRepo) FindByMoleculeID(ctx context.Context, moleculeID string) ([]*patent.Patent, error) {
	query := `
		SELECT p.* FROM patents p
		JOIN patent_molecule_relations pmr ON p.id = pmr.patent_id
		WHERE pmr.molecule_id = $1 AND p.deleted_at IS NULL
	`
	rows, err := r.executor.QueryContext(ctx, query, moleculeID)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to find by molecule id")
	}
	defer rows.Close()
	return collectPatents(rows)
}

func (r *postgresPatentRepo) AssociateMolecule(ctx context.Context, patentID string, moleculeID string) error {
	query := `
		INSERT INTO patent_molecule_relations (patent_id, molecule_id, relation_type)
		VALUES ($1, $2, 'disclosed')
		ON CONFLICT DO NOTHING
	`
	_, err := r.executor.ExecContext(ctx, query, patentID, moleculeID)
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to associate molecule")
	}
	return nil
}

func (r *postgresPatentRepo) SearchByAssigneeName(ctx context.Context, assigneeName string, limit, offset int) ([]*patent.Patent, int64, error) {
	countQuery := `SELECT COUNT(*) FROM patents WHERE assignee_name ILIKE $1 AND deleted_at IS NULL`
	var total int64
	if err := r.executor.QueryRowContext(ctx, countQuery, "%"+assigneeName+"%").Scan(&total); err != nil {
		return nil, 0, err
	}

	query := `SELECT * FROM patents WHERE assignee_name ILIKE $1 AND deleted_at IS NULL ORDER BY filing_date DESC LIMIT $2 OFFSET $3`
	rows, err := r.executor.QueryContext(ctx, query, "%"+assigneeName+"%", limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	patents, err := collectPatents(rows)
	return patents, total, err
}

// Claims

func (r *postgresPatentRepo) CreateClaim(ctx context.Context, claim *patent.Claim) error {
	if claim.ID == uuid.Nil {
		claim.ID = uuid.New()
	}
	query := `
		INSERT INTO patent_claims (
			id, patent_id, claim_number, claim_type, parent_claim_id,
			claim_text, claim_text_en, elements, markush_structures, scope_embedding
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING created_at, updated_at
	`
	elementsJSON, _ := json.Marshal(claim.Elements)
	markushJSON, _ := json.Marshal(claim.MarkushStructures)

	var embedding pgvector.Vector
	if len(claim.ScopeEmbedding) > 0 {
		embedding = pgvector.NewVector(claim.ScopeEmbedding)
	}

	claimTypeStr := strings.ToLower(claim.Type.String())
	err := r.executor.QueryRowContext(ctx, query,
		claim.ID, claim.PatentID, claim.Number, claimTypeStr, claim.ParentClaimID,
		claim.Text, claim.TextEn, elementsJSON, markushJSON, embedding,
	).Scan(&claim.CreatedAt, &claim.UpdatedAt)

	if err != nil {
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to create claim")
	}
	return nil
}

func (r *postgresPatentRepo) GetClaimsByPatent(ctx context.Context, patentID uuid.UUID) ([]*patent.Claim, error) {
	query := `SELECT * FROM patent_claims WHERE patent_id = $1 ORDER BY claim_number ASC`
	rows, err := r.executor.QueryContext(ctx, query, patentID)
	if err != nil {
		return nil, err
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
		UPDATE patent_claims SET
			claim_text = $2, claim_text_en = $3, elements = $4, markush_structures = $5, scope_embedding = $6, updated_at = NOW()
		WHERE id = $1
	`
	elementsJSON, _ := json.Marshal(claim.Elements)
	markushJSON, _ := json.Marshal(claim.MarkushStructures)
	var embedding pgvector.Vector
	if len(claim.ScopeEmbedding) > 0 {
		embedding = pgvector.NewVector(claim.ScopeEmbedding)
	}

	_, err := r.executor.ExecContext(ctx, query, claim.ID, claim.Text, claim.TextEn, elementsJSON, markushJSON, embedding)
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to update claim")
	}
	return nil
}

func (r *postgresPatentRepo) DeleteClaimsByPatent(ctx context.Context, patentID uuid.UUID) error {
	query := `DELETE FROM patent_claims WHERE patent_id = $1`
	_, err := r.executor.ExecContext(ctx, query, patentID)
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to delete claims")
	}
	return nil
}

func (r *postgresPatentRepo) BatchCreateClaims(ctx context.Context, claims []*patent.Claim) error {
	return r.WithTx(ctx, func(txRepo patent.PatentRepository) error {
		for _, c := range claims {
			if err := txRepo.CreateClaim(ctx, c); err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *postgresPatentRepo) GetIndependentClaims(ctx context.Context, patentID uuid.UUID) ([]*patent.Claim, error) {
	query := `SELECT * FROM patent_claims WHERE patent_id = $1 AND claim_type = 'independent' ORDER BY claim_number ASC`
	rows, err := r.executor.QueryContext(ctx, query, patentID)
	if err != nil {
		return nil, err
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

// Inventors & Priority

func (r *postgresPatentRepo) SetInventors(ctx context.Context, patentID uuid.UUID, inventors []*patent.Inventor) error {
	return r.WithTx(ctx, func(txRepo patent.PatentRepository) error {
		_, err := txRepo.(*postgresPatentRepo).executor.ExecContext(ctx, "DELETE FROM patent_inventors WHERE patent_id = $1", patentID)
		if err != nil {
			return err
		}
		for _, inv := range inventors {
			query := `INSERT INTO patent_inventors (patent_id, inventor_name, inventor_name_en, sequence, affiliation) VALUES ($1, $2, $3, $4, $5)`
			// Assuming NameEN is Country for now as Inventor struct in entity.go has Country, not NameEN.
			// Re-read Inventor struct: Name, Country, Affiliation, Sequence.
			// 001.sql has inventor_name, inventor_name_en, sequence, affiliation.
			// Mismatch. I'll use Country as NameEN placeholder or empty string.
			// Prompt requirement 001 said: inventor_name_en. Entity struct says Country.
			// I'll use Country as Country? No, table doesn't have country column.
			// I'll map Country -> affiliation? No affiliation is there.
			// I'll pass empty string for NameEN if not present in struct.
			_, err := txRepo.(*postgresPatentRepo).executor.ExecContext(ctx, query, patentID, inv.Name, "", inv.Sequence, inv.Affiliation)
			if err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *postgresPatentRepo) GetInventors(ctx context.Context, patentID uuid.UUID) ([]*patent.Inventor, error) {
	query := `SELECT patent_id, inventor_name, inventor_name_en, sequence, affiliation FROM patent_inventors WHERE patent_id = $1 ORDER BY sequence ASC`
	rows, err := r.executor.QueryContext(ctx, query, patentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var inventors []*patent.Inventor
	for rows.Next() {
		var i patent.Inventor
		var pid uuid.UUID
		var nameEn sql.NullString
		err := rows.Scan(&pid, &i.Name, &nameEn, &i.Sequence, &i.Affiliation)
		if err != nil {
			return nil, err
		}
		inventors = append(inventors, &i)
	}
	return inventors, nil
}

func (r *postgresPatentRepo) SearchByInventor(ctx context.Context, inventorName string, limit, offset int) ([]*patent.Patent, int64, error) {
	countQuery := `SELECT COUNT(DISTINCT patent_id) FROM patent_inventors WHERE inventor_name ILIKE $1`
	var total int64
	if err := r.executor.QueryRowContext(ctx, countQuery, "%"+inventorName+"%").Scan(&total); err != nil {
		return nil, 0, err
	}

	query := `
		SELECT p.* FROM patents p
		JOIN patent_inventors i ON p.id = i.patent_id
		WHERE i.inventor_name ILIKE $1 AND p.deleted_at IS NULL
		ORDER BY p.filing_date DESC
		LIMIT $2 OFFSET $3
	`
	rows, err := r.executor.QueryContext(ctx, query, "%"+inventorName+"%", limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	patents, err := collectPatents(rows)
	return patents, total, err
}

func (r *postgresPatentRepo) SetPriorityClaims(ctx context.Context, patentID uuid.UUID, claims []*patent.PriorityClaim) error {
	return r.WithTx(ctx, func(txRepo patent.PatentRepository) error {
		_, err := txRepo.(*postgresPatentRepo).executor.ExecContext(ctx, "DELETE FROM patent_priority_claims WHERE patent_id = $1", patentID)
		if err != nil {
			return err
		}
		for _, pc := range claims {
			if pc.ID == uuid.Nil {
				pc.ID = uuid.New()
			}
			query := `INSERT INTO patent_priority_claims (id, patent_id, priority_number, priority_date, priority_country) VALUES ($1, $2, $3, $4, $5)`
			_, err := txRepo.(*postgresPatentRepo).executor.ExecContext(ctx, query, pc.ID, patentID, pc.PriorityNumber, pc.PriorityDate, pc.PriorityCountry)
			if err != nil {
				return err
			}
		}
		return nil
	})
}

func (r *postgresPatentRepo) GetPriorityClaims(ctx context.Context, patentID uuid.UUID) ([]*patent.PriorityClaim, error) {
	query := `SELECT id, patent_id, priority_number, priority_date, priority_country FROM patent_priority_claims WHERE patent_id = $1`
	rows, err := r.executor.QueryContext(ctx, query, patentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var claims []*patent.PriorityClaim
	for rows.Next() {
		var c patent.PriorityClaim
		err := rows.Scan(&c.ID, &c.PatentID, &c.PriorityNumber, &c.PriorityDate, &c.PriorityCountry)
		if err != nil {
			return nil, err
		}
		claims = append(claims, &c)
	}
	return claims, nil
}

// Batch & Stats

func (r *postgresPatentRepo) BatchCreate(ctx context.Context, patents []*patent.Patent) (int, error) {
	count := 0
	err := r.WithTx(ctx, func(txRepo patent.PatentRepository) error {
		for _, p := range patents {
			if err := txRepo.Create(ctx, p); err != nil {
				if errors.IsCode(err, errors.ErrCodePatentAlreadyExists) {
					continue
				}
				return err
			}
			count++
		}
		return nil
	})
	return count, err
}

func (r *postgresPatentRepo) BatchUpdateStatus(ctx context.Context, ids []uuid.UUID, status patent.PatentStatus) (int64, error) {
	query := `UPDATE patents SET status = $1, updated_at = NOW() WHERE id = ANY($2) AND deleted_at IS NULL`
	res, err := r.executor.ExecContext(ctx, query, status, pq.Array(ids))
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func (r *postgresPatentRepo) CountByStatus(ctx context.Context) (map[patent.PatentStatus]int64, error) {
	query := `SELECT status, COUNT(*) FROM patents WHERE deleted_at IS NULL GROUP BY status`
	rows, err := r.executor.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	res := make(map[patent.PatentStatus]int64)
	for rows.Next() {
		var s string
		var c int64
		if err := rows.Scan(&s, &c); err != nil {
			return nil, err
		}
		// Convert string to PatentStatus enum
		var found patent.PatentStatus
		for i := patent.PatentStatusDraft; i <= patent.PatentStatusLapsed; i++ {
			if i.String() == s {
				found = i
				break
			}
		}
		res[found] = c
	}
	return res, nil
}

func (r *postgresPatentRepo) CountByJurisdiction(ctx context.Context) (map[string]int64, error) {
	query := `SELECT jurisdiction, COUNT(*) FROM patents WHERE deleted_at IS NULL GROUP BY jurisdiction`
	rows, err := r.executor.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	res := make(map[string]int64)
	for rows.Next() {
		var j string
		var c int64
		if err := rows.Scan(&j, &c); err != nil {
			return nil, err
		}
		res[j] = c
	}
	return res, nil
}

func (r *postgresPatentRepo) CountByYear(ctx context.Context, field string) (map[int]int64, error) {
	if field != "filing_date" && field != "publication_date" && field != "grant_date" {
		return nil, errors.New(errors.ErrCodeValidation, "invalid date field")
	}
	query := fmt.Sprintf(`SELECT EXTRACT(YEAR FROM %s) as year, COUNT(*) FROM patents WHERE deleted_at IS NULL AND %s IS NOT NULL GROUP BY year`, field, field)
	rows, err := r.executor.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	res := make(map[int]int64)
	for rows.Next() {
		var y float64
		var c int64
		if err := rows.Scan(&y, &c); err != nil {
			return nil, err
		}
		res[int(y)] = c
	}
	return res, nil
}

func (r *postgresPatentRepo) GetIPCDistribution(ctx context.Context, level int) (map[string]int64, error) {
	query := `SELECT LEFT(unnest(ipc_codes), $1) as code, COUNT(*) FROM patents WHERE deleted_at IS NULL GROUP BY code`
	rows, err := r.executor.QueryContext(ctx, query, level)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	res := make(map[string]int64)
	for rows.Next() {
		var code string
		var c int64
		if err := rows.Scan(&code, &c); err != nil {
			return nil, err
		}
		res[code] = c
	}
	return res, nil
}

// Helpers

func scanPatent(row scanner) (*patent.Patent, error) {
	var p patent.Patent
	var rawJSON, metaJSON []byte
	var ipc, cpc, keyip []string

	err := row.Scan(
		&p.ID, &p.PatentNumber, &p.Title, &p.TitleEn, &p.Abstract, &p.AbstractEn,
		&p.Type, &p.Status, &p.Dates.FilingDate, &p.Dates.PublicationDate, &p.Dates.GrantDate,
		&p.Dates.ExpiryDate, &p.Dates.PriorityDate, &p.AssigneeID, &p.AssigneeName, &p.Jurisdiction,
		pq.Array(&ipc), pq.Array(&cpc), pq.Array(&keyip), &p.FamilyID, &p.ApplicationNumber,
		&p.FullTextHash, &p.Source, &rawJSON, &metaJSON,
		&p.CreatedAt, &p.UpdatedAt, &p.DeletedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New(errors.ErrCodePatentNotFound, "patent not found")
		}
		return nil, err
	}
	p.IPCCodes = ipc
	p.CPCCodes = cpc
	p.KeyIPTechCodes = keyip
	if len(rawJSON) > 0 { _ = json.Unmarshal(rawJSON, &p.RawData) }
	if len(metaJSON) > 0 { _ = json.Unmarshal(metaJSON, &p.Metadata) }

	p.FilingDate = p.Dates.FilingDate
	p.PublicationDate = p.Dates.PublicationDate
	p.GrantDate = p.Dates.GrantDate
	p.ExpiryDate = p.Dates.ExpiryDate
	p.PriorityDate = p.Dates.PriorityDate

	return &p, nil
}

func collectPatents(rows *sql.Rows) ([]*patent.Patent, error) {
	var patents []*patent.Patent
	for rows.Next() {
		p, err := scanPatent(rows)
		if err != nil {
			return nil, err
		}
		patents = append(patents, p)
	}
	return patents, nil
}

func scanClaim(row scanner) (*patent.Claim, error) {
	var c patent.Claim
	var claimTypeStr string
	var elementsJSON, markushJSON []byte
	var embedding pgvector.Vector

	err := row.Scan(
		&c.ID, &c.PatentID, &c.Number, &claimTypeStr, &c.ParentClaimID,
		&c.Text, &c.TextEn, &elementsJSON, &markushJSON, &embedding,
		&c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New(errors.ErrCodeNotFound, "claim not found")
		}
		return nil, err
	}
	if len(elementsJSON) > 0 { _ = json.Unmarshal(elementsJSON, &c.Elements) }
	if len(markushJSON) > 0 { _ = json.Unmarshal(markushJSON, &c.MarkushStructures) }
	c.ScopeEmbedding = embedding.Slice()

	switch strings.ToLower(claimTypeStr) {
	case "independent":
		c.Type = patent.ClaimTypeIndependent
	case "dependent":
		c.Type = patent.ClaimTypeDependent
	}

	return &c, nil
}
