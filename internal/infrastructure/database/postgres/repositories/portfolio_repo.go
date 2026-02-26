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
	"github.com/turtacn/KeyIP-Intelligence/internal/domain/portfolio"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/database/postgres"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

type postgresPortfolioRepo struct {
	conn *postgres.Connection
	tx   *sql.Tx
	log  logging.Logger
}

func NewPostgresPortfolioRepo(conn *postgres.Connection, log logging.Logger) portfolio.PortfolioRepository {
	return &postgresPortfolioRepo{
		conn: conn,
		log:  log,
	}
}

func (r *postgresPortfolioRepo) executor() queryExecutor {
	if r.tx != nil {
		return r.tx
	}
	return r.conn.DB()
}

// Portfolio CRUD

func (r *postgresPortfolioRepo) Create(ctx context.Context, p *portfolio.Portfolio) error {
	query := `
		INSERT INTO portfolios (
			name, description, owner_id, status, tech_domains, target_jurisdictions, metadata
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7
		) RETURNING id, created_at, updated_at
	`
	meta, _ := json.Marshal(p.Metadata)
	err := r.executor().QueryRowContext(ctx, query,
		p.Name, p.Description, p.OwnerID, p.Status, pq.Array(p.TechDomains), pq.Array(p.TargetJurisdictions), meta,
	).Scan(&p.ID, &p.CreatedAt, &p.UpdatedAt)

	if err != nil {
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to create portfolio")
	}
	return nil
}

func (r *postgresPortfolioRepo) GetByID(ctx context.Context, id uuid.UUID) (*portfolio.Portfolio, error) {
	query := `SELECT * FROM portfolios WHERE id = $1 AND deleted_at IS NULL`
	row := r.executor().QueryRowContext(ctx, query, id)
	p, err := scanPortfolio(row)
	if err != nil {
		return nil, err
	}

	countQuery := `SELECT COUNT(*) FROM portfolio_patents WHERE portfolio_id = $1`
	var count int
	if err := r.executor().QueryRowContext(ctx, countQuery, id).Scan(&count); err == nil {
		p.PatentCount = count
	}

	return p, nil
}

func (r *postgresPortfolioRepo) Update(ctx context.Context, p *portfolio.Portfolio) error {
	query := `
		UPDATE portfolios
		SET name = $1, description = $2, status = $3, tech_domains = $4, target_jurisdictions = $5, metadata = $6, updated_at = NOW()
		WHERE id = $7 AND updated_at = $8
	`
	meta, _ := json.Marshal(p.Metadata)
	res, err := r.executor().ExecContext(ctx, query,
		p.Name, p.Description, p.Status, pq.Array(p.TechDomains), pq.Array(p.TargetJurisdictions), meta, p.ID, p.UpdatedAt,
	)
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to update portfolio")
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return errors.New(errors.ErrCodeConflict, "portfolio updated by another transaction or not found")
	}
	return nil
}

func (r *postgresPortfolioRepo) SoftDelete(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE portfolios SET deleted_at = NOW() WHERE id = $1`
	_, err := r.executor().ExecContext(ctx, query, id)
	return err
}

func (r *postgresPortfolioRepo) List(ctx context.Context, ownerID uuid.UUID, status *portfolio.Status, limit, offset int) ([]*portfolio.Portfolio, int64, error) {
	baseQuery := `FROM portfolios WHERE owner_id = $1 AND deleted_at IS NULL`
	args := []interface{}{ownerID}

	if status != nil {
		baseQuery += ` AND status = $2`
		args = append(args, *status)
	}

	var total int64
	err := r.executor().QueryRowContext(ctx, "SELECT COUNT(*) "+baseQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	dataQuery := fmt.Sprintf("SELECT * %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d", baseQuery, len(args)+1, len(args)+2)
	args = append(args, limit, offset)

	rows, err := r.executor().QueryContext(ctx, dataQuery, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var portfolios []*portfolio.Portfolio
	for rows.Next() {
		p, err := scanPortfolio(rows)
		if err != nil { return nil, 0, err }
		portfolios = append(portfolios, p)
	}
	return portfolios, total, nil
}

func (r *postgresPortfolioRepo) GetByOwner(ctx context.Context, ownerID uuid.UUID) ([]*portfolio.Portfolio, error) {
	list, _, err := r.List(ctx, ownerID, nil, 1000, 0)
	return list, err
}

// Patents Association

func (r *postgresPortfolioRepo) AddPatent(ctx context.Context, portfolioID, patentID uuid.UUID, role string, addedBy uuid.UUID) error {
	query := `
		INSERT INTO portfolio_patents (portfolio_id, patent_id, role_in_portfolio, added_by, added_at)
		VALUES ($1, $2, $3, $4, NOW())
	`
	_, err := r.executor().ExecContext(ctx, query, portfolioID, patentID, role, addedBy)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" {
			return errors.New(errors.ErrCodeConflict, "patent already in portfolio")
		}
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to add patent")
	}
	return nil
}

func (r *postgresPortfolioRepo) RemovePatent(ctx context.Context, portfolioID, patentID uuid.UUID) error {
	query := `DELETE FROM portfolio_patents WHERE portfolio_id = $1 AND patent_id = $2`
	_, err := r.executor().ExecContext(ctx, query, portfolioID, patentID)
	return err
}

func (r *postgresPortfolioRepo) GetPatents(ctx context.Context, portfolioID uuid.UUID, role *string, limit, offset int) ([]*patent.Patent, int64, error) {
	baseQuery := `
		FROM patents p
		JOIN portfolio_patents pp ON p.id = pp.patent_id
		WHERE pp.portfolio_id = $1 AND p.deleted_at IS NULL
	`
	args := []interface{}{portfolioID}

	if role != nil {
		baseQuery += ` AND pp.role_in_portfolio = $2`
		args = append(args, *role)
	}

	var total int64
	err := r.executor().QueryRowContext(ctx, "SELECT COUNT(*) "+baseQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	dataQuery := fmt.Sprintf("SELECT p.* %s ORDER BY pp.added_at DESC LIMIT $%d OFFSET $%d", baseQuery, len(args)+1, len(args)+2)
	args = append(args, limit, offset)

	rows, err := r.executor().QueryContext(ctx, dataQuery, args...)
	if err != nil { return nil, 0, err }
	defer rows.Close()

	var patents []*patent.Patent
	for rows.Next() {
		p, err := scanPortfolioPatent(rows)
		if err != nil { return nil, 0, err }
		patents = append(patents, p)
	}
	return patents, total, nil
}

func (r *postgresPortfolioRepo) IsPatentInPortfolio(ctx context.Context, portfolioID, patentID uuid.UUID) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM portfolio_patents WHERE portfolio_id = $1 AND patent_id = $2)`
	err := r.executor().QueryRowContext(ctx, query, portfolioID, patentID).Scan(&exists)
	return exists, err
}

func (r *postgresPortfolioRepo) BatchAddPatents(ctx context.Context, portfolioID uuid.UUID, patentIDs []uuid.UUID, role string, addedBy uuid.UUID) error {
	return nil
}

func (r *postgresPortfolioRepo) GetPortfoliosByPatent(ctx context.Context, patentID uuid.UUID) ([]*portfolio.Portfolio, error) {
	return nil, nil
}

// Valuation Stubs
func (r *postgresPortfolioRepo) CreateValuation(ctx context.Context, v *portfolio.Valuation) error { return nil }
func (r *postgresPortfolioRepo) GetLatestValuation(ctx context.Context, patentID uuid.UUID) (*portfolio.Valuation, error) { return nil, nil }
func (r *postgresPortfolioRepo) GetValuationHistory(ctx context.Context, patentID uuid.UUID, limit int) ([]*portfolio.Valuation, error) { return nil, nil }
func (r *postgresPortfolioRepo) GetValuationsByPortfolio(ctx context.Context, portfolioID uuid.UUID) ([]*portfolio.Valuation, error) { return nil, nil }
func (r *postgresPortfolioRepo) GetValuationDistribution(ctx context.Context, portfolioID uuid.UUID) (map[portfolio.ValuationTier]int64, error) { return nil, nil }
func (r *postgresPortfolioRepo) BatchCreateValuations(ctx context.Context, valuations []*portfolio.Valuation) error { return nil }

// HealthScore Stubs
func (r *postgresPortfolioRepo) CreateHealthScore(ctx context.Context, score *portfolio.HealthScore) error { return nil }
func (r *postgresPortfolioRepo) GetLatestHealthScore(ctx context.Context, portfolioID uuid.UUID) (*portfolio.HealthScore, error) { return nil, nil }
func (r *postgresPortfolioRepo) GetHealthScoreHistory(ctx context.Context, portfolioID uuid.UUID, limit int) ([]*portfolio.HealthScore, error) { return nil, nil }
func (r *postgresPortfolioRepo) GetHealthScoreTrend(ctx context.Context, portfolioID uuid.UUID, startDate, endDate time.Time) ([]*portfolio.HealthScore, error) { return nil, nil }

// Suggestions Stubs
func (r *postgresPortfolioRepo) CreateSuggestion(ctx context.Context, s *portfolio.OptimizationSuggestion) error { return nil }
func (r *postgresPortfolioRepo) GetSuggestions(ctx context.Context, portfolioID uuid.UUID, status *string, limit, offset int) ([]*portfolio.OptimizationSuggestion, int64, error) { return nil, 0, nil }
func (r *postgresPortfolioRepo) UpdateSuggestionStatus(ctx context.Context, id uuid.UUID, status string, resolvedBy uuid.UUID) error { return nil }
func (r *postgresPortfolioRepo) GetPendingSuggestionCount(ctx context.Context, portfolioID uuid.UUID) (int64, error) { return 0, nil }

// Analytics Stubs
func (r *postgresPortfolioRepo) GetPortfolioSummary(ctx context.Context, portfolioID uuid.UUID) (*portfolio.Summary, error) { return nil, nil }
func (r *postgresPortfolioRepo) GetJurisdictionCoverage(ctx context.Context, portfolioID uuid.UUID) (map[string]int64, error) { return nil, nil }
func (r *postgresPortfolioRepo) GetTechDomainCoverage(ctx context.Context, portfolioID uuid.UUID) (map[string]int64, error) { return nil, nil }
func (r *postgresPortfolioRepo) GetExpiryTimeline(ctx context.Context, portfolioID uuid.UUID) ([]*portfolio.ExpiryTimelineEntry, error) { return nil, nil }
func (r *postgresPortfolioRepo) ComparePortfolios(ctx context.Context, portfolioIDs []uuid.UUID) ([]*portfolio.ComparisonResult, error) { return nil, nil }

// Transaction
func (r *postgresPortfolioRepo) WithTx(ctx context.Context, fn func(portfolio.PortfolioRepository) error) error {
	tx, err := r.conn.DB().BeginTx(ctx, nil)
	if err != nil { return err }
	txRepo := &postgresPortfolioRepo{conn: r.conn, tx: tx, log: r.log}
	if err := fn(txRepo); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

// Scanners
func scanPortfolio(row scanner) (*portfolio.Portfolio, error) {
	p := &portfolio.Portfolio{}
	var meta []byte
	err := row.Scan(
		&p.ID, &p.Name, &p.Description, &p.OwnerID, &p.Status,
		pq.Array(&p.TechDomains), pq.Array(&p.TargetJurisdictions), &meta,
		&p.CreatedAt, &p.UpdatedAt, &p.DeletedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New(errors.ErrCodeNotFound, "portfolio not found")
		}
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to scan portfolio")
	}
	if len(meta) > 0 { _ = json.Unmarshal(meta, &p.Metadata) }
	return p, nil
}

func scanPortfolioPatent(row scanner) (*patent.Patent, error) {
	p := &patent.Patent{}
	var statusStr string
	var applicantsRaw, inventorsRaw, ipcCodesRaw []byte
	var filingDate, pubDate, grantDate, expiryDate *time.Time
	var officeStr string

	err := row.Scan(
		&p.ID, &p.PatentNumber, &p.Title, &p.Abstract, &statusStr, &officeStr,
		&applicantsRaw, &inventorsRaw, &ipcCodesRaw,
		&filingDate, &pubDate, &grantDate, &expiryDate,
		pq.Array(&p.MoleculeIDs), &p.FamilyID, &p.CreatedAt, &p.UpdatedAt, &p.Version,
	)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to scan portfolio patent")
	}

	p.Status = parsePatentStatus(statusStr)
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

func parsePatentStatus(s string) patent.PatentStatus {
	switch s {
	case "Draft": return patent.PatentStatusDraft
	case "Filed": return patent.PatentStatusFiled
	case "Published": return patent.PatentStatusPublished
	case "UnderExamination": return patent.PatentStatusUnderExamination
	case "Granted": return patent.PatentStatusGranted
	case "Rejected": return patent.PatentStatusRejected
	case "Withdrawn": return patent.PatentStatusWithdrawn
	case "Expired": return patent.PatentStatusExpired
	case "Invalidated": return patent.PatentStatusInvalidated
	case "Lapsed": return patent.PatentStatusLapsed
	default: return 0 // Unknown
	}
}

//Personal.AI order the ending
