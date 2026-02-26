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
	conn     *postgres.Connection
	log      logging.Logger
	executor queryExecutor
}

func NewPostgresPortfolioRepo(conn *postgres.Connection, log logging.Logger) portfolio.PortfolioRepository {
	return &postgresPortfolioRepo{
		conn:     conn,
		log:      log,
		executor: conn.DB(),
	}
}

// WithTx implementation
func (r *postgresPortfolioRepo) WithTx(ctx context.Context, fn func(portfolio.PortfolioRepository) error) error {
	tx, err := r.conn.DB().BeginTx(ctx, nil)
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to begin transaction")
	}

	txRepo := &postgresPortfolioRepo{
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

func (r *postgresPortfolioRepo) Create(ctx context.Context, p *portfolio.Portfolio) error {
	if p.ID == uuid.Nil {
		p.ID = uuid.New()
	}
	query := `
		INSERT INTO portfolios (
			id, name, description, owner_id, status, tech_domains, target_jurisdictions, metadata
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING created_at, updated_at
	`
	metaJSON, _ := json.Marshal(p.Metadata)

	err := r.executor.QueryRowContext(ctx, query,
		p.ID, p.Name, p.Description, p.OwnerID, p.Status,
		pq.Array(p.TechDomains), pq.Array(p.TargetJurisdictions), metaJSON,
	).Scan(&p.CreatedAt, &p.UpdatedAt)

	if err != nil {
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to create portfolio")
	}
	return nil
}

func (r *postgresPortfolioRepo) GetByID(ctx context.Context, id uuid.UUID) (*portfolio.Portfolio, error) {
	query := `
		SELECT p.*, (SELECT COUNT(*) FROM portfolio_patents WHERE portfolio_id = p.id) as patent_count
		FROM portfolios p
		WHERE p.id = $1 AND p.deleted_at IS NULL
	`
	row := r.executor.QueryRowContext(ctx, query, id)
	return scanPortfolio(row)
}

func (r *postgresPortfolioRepo) Update(ctx context.Context, p *portfolio.Portfolio) error {
	query := `
		UPDATE portfolios SET
			name = $2, description = $3, status = $4, tech_domains = $5, target_jurisdictions = $6, metadata = $7,
			updated_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL
	`
	metaJSON, _ := json.Marshal(p.Metadata)

	res, err := r.executor.ExecContext(ctx, query,
		p.ID, p.Name, p.Description, p.Status,
		pq.Array(p.TechDomains), pq.Array(p.TargetJurisdictions), metaJSON,
	)
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to update portfolio")
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return errors.New(errors.CodePortfolioNotFound, "portfolio not found")
	}
	return nil
}

func (r *postgresPortfolioRepo) SoftDelete(ctx context.Context, id uuid.UUID) error {
	query := `UPDATE portfolios SET deleted_at = NOW() WHERE id = $1 AND deleted_at IS NULL`
	res, err := r.executor.ExecContext(ctx, query, id)
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to delete portfolio")
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return errors.New(errors.CodePortfolioNotFound, "portfolio not found")
	}
	return nil
}

func (r *postgresPortfolioRepo) List(ctx context.Context, ownerID uuid.UUID, status *portfolio.Status, limit, offset int) ([]*portfolio.Portfolio, int64, error) {
	where := "WHERE owner_id = $1 AND deleted_at IS NULL"
	args := []interface{}{ownerID}

	if status != nil {
		where += " AND status = $2"
		args = append(args, *status)
	}

	countQuery := "SELECT COUNT(*) FROM portfolios " + where
	var total int64
	if err := r.executor.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// Add subquery for patent count
	query := `SELECT p.*, (SELECT COUNT(*) FROM portfolio_patents WHERE portfolio_id = p.id) as patent_count FROM portfolios p ` + where + ` ORDER BY created_at DESC LIMIT $%d OFFSET $%d`
	query = fmt.Sprintf(query, len(args)+1, len(args)+2)
	args = append(args, limit, offset)

	rows, err := r.executor.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var portfolios []*portfolio.Portfolio
	for rows.Next() {
		p, err := scanPortfolio(rows)
		if err != nil {
			return nil, 0, err
		}
		portfolios = append(portfolios, p)
	}
	return portfolios, total, nil
}

func (r *postgresPortfolioRepo) GetByOwner(ctx context.Context, ownerID uuid.UUID) ([]*portfolio.Portfolio, error) {
	res, _, err := r.List(ctx, ownerID, nil, 1000, 0)
	return res, err
}

// Portfolio Patents

func (r *postgresPortfolioRepo) AddPatent(ctx context.Context, portfolioID, patentID uuid.UUID, role string, addedBy uuid.UUID) error {
	query := `
		INSERT INTO portfolio_patents (portfolio_id, patent_id, role_in_portfolio, added_by)
		VALUES ($1, $2, $3, $4)
	`
	_, err := r.executor.ExecContext(ctx, query, portfolioID, patentID, role, addedBy)
	if err != nil {
		if pqErr, ok := err.(*pq.Error); ok && pqErr.Code == "23505" {
			return errors.Wrap(err, errors.ErrCodeConflict, "patent already in portfolio")
		}
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to add patent")
	}
	return nil
}

func (r *postgresPortfolioRepo) RemovePatent(ctx context.Context, portfolioID, patentID uuid.UUID) error {
	query := `DELETE FROM portfolio_patents WHERE portfolio_id = $1 AND patent_id = $2`
	_, err := r.executor.ExecContext(ctx, query, portfolioID, patentID)
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to remove patent")
	}
	return nil
}

func (r *postgresPortfolioRepo) GetPatents(ctx context.Context, portfolioID uuid.UUID, role *string, limit, offset int) ([]*patent.Patent, int64, error) {
	where := "WHERE pp.portfolio_id = $1"
	args := []interface{}{portfolioID}

	if role != nil {
		where += " AND pp.role_in_portfolio = $2"
		args = append(args, *role)
	}

	countQuery := `SELECT COUNT(*) FROM portfolio_patents pp ` + where
	var total int64
	if err := r.executor.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := `
		SELECT p.*
		FROM patents p
		JOIN portfolio_patents pp ON p.id = pp.patent_id
		` + where + ` AND p.deleted_at IS NULL
		ORDER BY pp.added_at DESC
		LIMIT $%d OFFSET $%d
	`
	query = fmt.Sprintf(query, len(args)+1, len(args)+2)
	args = append(args, limit, offset)

	rows, err := r.executor.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	patents, err := collectPatents(rows)
	return patents, total, err
}

func (r *postgresPortfolioRepo) IsPatentInPortfolio(ctx context.Context, portfolioID, patentID uuid.UUID) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM portfolio_patents WHERE portfolio_id = $1 AND patent_id = $2)`
	err := r.executor.QueryRowContext(ctx, query, portfolioID, patentID).Scan(&exists)
	return exists, err
}

func (r *postgresPortfolioRepo) BatchAddPatents(ctx context.Context, portfolioID uuid.UUID, patentIDs []uuid.UUID, role string, addedBy uuid.UUID) error {
	return r.WithTx(ctx, func(txRepo portfolio.PortfolioRepository) error {
		for _, pid := range patentIDs {
			if err := txRepo.AddPatent(ctx, portfolioID, pid, role, addedBy); err != nil {
				// Continue on duplicate? Maybe
				if errors.IsConflict(err) {
					continue
				}
				return err
			}
		}
		return nil
	})
}

func (r *postgresPortfolioRepo) GetPortfoliosByPatent(ctx context.Context, patentID uuid.UUID) ([]*portfolio.Portfolio, error) {
	query := `
		SELECT p.*, (SELECT COUNT(*) FROM portfolio_patents WHERE portfolio_id = p.id) as patent_count
		FROM portfolios p
		JOIN portfolio_patents pp ON p.id = pp.portfolio_id
		WHERE pp.patent_id = $1 AND p.deleted_at IS NULL
	`
	rows, err := r.executor.QueryContext(ctx, query, patentID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var portfolios []*portfolio.Portfolio
	for rows.Next() {
		p, err := scanPortfolio(rows)
		if err != nil {
			return nil, err
		}
		portfolios = append(portfolios, p)
	}
	return portfolios, nil
}

// Valuation

func (r *postgresPortfolioRepo) CreateValuation(ctx context.Context, v *portfolio.Valuation) error {
	if v.ID == uuid.Nil {
		v.ID = uuid.New()
	}
	query := `
		INSERT INTO patent_valuations (
			id, patent_id, portfolio_id, technical_score, legal_score, market_score, strategic_score,
			composite_score, tier, monetary_value_low, monetary_value_mid, monetary_value_high,
			currency, valuation_method, model_version, scoring_details, comparable_patents, assumptions,
			valid_from, valid_until, evaluated_by
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21)
		RETURNING created_at
	`
	scoringJSON, _ := json.Marshal(v.ScoringDetails)
	comparableJSON, _ := json.Marshal(v.ComparablePatents)
	assumptionsJSON, _ := json.Marshal(v.Assumptions)

	err := r.executor.QueryRowContext(ctx, query,
		v.ID, v.PatentID, v.PortfolioID, v.TechnicalScore, v.LegalScore, v.MarketScore, v.StrategicScore,
		v.CompositeScore, v.Tier, v.MonetaryValueLow, v.MonetaryValueMid, v.MonetaryValueHigh,
		v.Currency, v.ValuationMethod, v.ModelVersion, scoringJSON, comparableJSON, assumptionsJSON,
		v.ValidFrom, v.ValidUntil, v.EvaluatedBy,
	).Scan(&v.CreatedAt)

	if err != nil {
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to create valuation")
	}
	return nil
}

func (r *postgresPortfolioRepo) GetLatestValuation(ctx context.Context, patentID uuid.UUID) (*portfolio.Valuation, error) {
	query := `SELECT * FROM patent_valuations WHERE patent_id = $1 ORDER BY created_at DESC LIMIT 1`
	row := r.executor.QueryRowContext(ctx, query, patentID)
	return scanValuation(row)
}

func (r *postgresPortfolioRepo) GetValuationHistory(ctx context.Context, patentID uuid.UUID, limit int) ([]*portfolio.Valuation, error) {
	query := `SELECT * FROM patent_valuations WHERE patent_id = $1 ORDER BY created_at DESC LIMIT $2`
	rows, err := r.executor.QueryContext(ctx, query, patentID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var valuations []*portfolio.Valuation
	for rows.Next() {
		v, err := scanValuation(rows)
		if err != nil {
			return nil, err
		}
		valuations = append(valuations, v)
	}
	return valuations, nil
}

func (r *postgresPortfolioRepo) GetValuationsByPortfolio(ctx context.Context, portfolioID uuid.UUID) ([]*portfolio.Valuation, error) {
	query := `
		SELECT DISTINCT ON (v.patent_id) v.*
		FROM patent_valuations v
		JOIN portfolio_patents pp ON v.patent_id = pp.patent_id
		WHERE pp.portfolio_id = $1
		ORDER BY v.patent_id, v.created_at DESC
	`
	rows, err := r.executor.QueryContext(ctx, query, portfolioID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var valuations []*portfolio.Valuation
	for rows.Next() {
		v, err := scanValuation(rows)
		if err != nil {
			return nil, err
		}
		valuations = append(valuations, v)
	}
	return valuations, nil
}

func (r *postgresPortfolioRepo) GetValuationDistribution(ctx context.Context, portfolioID uuid.UUID) (map[portfolio.ValuationTier]int64, error) {
	query := `
		SELECT v.tier, COUNT(*)
		FROM (
			SELECT DISTINCT ON (patent_id) tier
			FROM patent_valuations
			WHERE portfolio_id = $1
			ORDER BY patent_id, created_at DESC
		) v
		GROUP BY v.tier
	`
	rows, err := r.executor.QueryContext(ctx, query, portfolioID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	res := make(map[portfolio.ValuationTier]int64)
	for rows.Next() {
		var t string
		var c int64
		if err := rows.Scan(&t, &c); err != nil {
			return nil, err
		}
		res[portfolio.ValuationTier(t)] = c
	}
	return res, nil
}

func (r *postgresPortfolioRepo) BatchCreateValuations(ctx context.Context, valuations []*portfolio.Valuation) error {
	return r.WithTx(ctx, func(txRepo portfolio.PortfolioRepository) error {
		for _, v := range valuations {
			if err := txRepo.CreateValuation(ctx, v); err != nil {
				return err
			}
		}
		return nil
	})
}

// Health Score & Suggestions

func (r *postgresPortfolioRepo) CreateHealthScore(ctx context.Context, score *portfolio.HealthScore) error {
	if score.ID == uuid.Nil {
		score.ID = uuid.New()
	}
	query := `
		INSERT INTO portfolio_health_scores (
			id, portfolio_id, overall_score, coverage_score, diversity_score, freshness_score, strength_score, risk_score,
			total_patents, active_patents, expiring_within_year, expiring_within_3years,
			jurisdiction_distribution, tech_domain_distribution, tier_distribution, recommendations, model_version, evaluated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18)
		RETURNING created_at
	`
	jurisJSON, _ := json.Marshal(score.JurisdictionDistribution)
	techJSON, _ := json.Marshal(score.TechDomainDistribution)
	tierJSON, _ := json.Marshal(score.TierDistribution)
	recsJSON, _ := json.Marshal(score.Recommendations)

	err := r.executor.QueryRowContext(ctx, query,
		score.ID, score.PortfolioID, score.OverallScore, score.CoverageScore, score.DiversityScore,
		score.FreshnessScore, score.StrengthScore, score.RiskScore,
		score.TotalPatents, score.ActivePatents, score.ExpiringWithinYear, score.ExpiringWithin3Years,
		jurisJSON, techJSON, tierJSON, recsJSON, score.ModelVersion, score.EvaluatedAt,
	).Scan(&score.CreatedAt)

	if err != nil {
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to create health score")
	}
	return nil
}

func (r *postgresPortfolioRepo) GetLatestHealthScore(ctx context.Context, portfolioID uuid.UUID) (*portfolio.HealthScore, error) {
	query := `SELECT * FROM portfolio_health_scores WHERE portfolio_id = $1 ORDER BY evaluated_at DESC LIMIT 1`
	row := r.executor.QueryRowContext(ctx, query, portfolioID)
	return scanHealthScore(row)
}

func (r *postgresPortfolioRepo) GetHealthScoreHistory(ctx context.Context, portfolioID uuid.UUID, limit int) ([]*portfolio.HealthScore, error) {
	query := `SELECT * FROM portfolio_health_scores WHERE portfolio_id = $1 ORDER BY evaluated_at DESC LIMIT $2`
	rows, err := r.executor.QueryContext(ctx, query, portfolioID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var scores []*portfolio.HealthScore
	for rows.Next() {
		s, err := scanHealthScore(rows)
		if err != nil {
			return nil, err
		}
		scores = append(scores, s)
	}
	return scores, nil
}

func (r *postgresPortfolioRepo) GetHealthScoreTrend(ctx context.Context, portfolioID uuid.UUID, startDate, endDate time.Time) ([]*portfolio.HealthScore, error) {
	query := `SELECT * FROM portfolio_health_scores WHERE portfolio_id = $1 AND evaluated_at BETWEEN $2 AND $3 ORDER BY evaluated_at ASC`
	rows, err := r.executor.QueryContext(ctx, query, portfolioID, startDate, endDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var scores []*portfolio.HealthScore
	for rows.Next() {
		s, err := scanHealthScore(rows)
		if err != nil {
			return nil, err
		}
		scores = append(scores, s)
	}
	return scores, nil
}

func (r *postgresPortfolioRepo) CreateSuggestion(ctx context.Context, s *portfolio.OptimizationSuggestion) error {
	if s.ID == uuid.Nil {
		s.ID = uuid.New()
	}
	query := `
		INSERT INTO portfolio_optimization_suggestions (
			id, portfolio_id, health_score_id, suggestion_type, priority, title, description,
			target_patent_id, target_tech_domain, target_jurisdiction, estimated_impact, estimated_cost,
			rationale, status
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		RETURNING created_at, updated_at
	`
	rationaleJSON, _ := json.Marshal(s.Rationale)

	err := r.executor.QueryRowContext(ctx, query,
		s.ID, s.PortfolioID, s.HealthScoreID, s.SuggestionType, s.Priority, s.Title, s.Description,
		s.TargetPatentID, s.TargetTechDomain, s.TargetJurisdiction, s.EstimatedImpact, s.EstimatedCost,
		rationaleJSON, s.Status,
	).Scan(&s.CreatedAt, &s.UpdatedAt)

	if err != nil {
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to create suggestion")
	}
	return nil
}

func (r *postgresPortfolioRepo) GetSuggestions(ctx context.Context, portfolioID uuid.UUID, status *string, limit, offset int) ([]*portfolio.OptimizationSuggestion, int64, error) {
	where := "WHERE portfolio_id = $1"
	args := []interface{}{portfolioID}

	if status != nil {
		where += " AND status = $2"
		args = append(args, *status)
	}

	countQuery := "SELECT COUNT(*) FROM portfolio_optimization_suggestions " + where
	var total int64
	if err := r.executor.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := `SELECT * FROM portfolio_optimization_suggestions ` + where + ` ORDER BY priority DESC, created_at DESC LIMIT $%d OFFSET $%d`
	query = fmt.Sprintf(query, len(args)+1, len(args)+2)
	args = append(args, limit, offset)

	rows, err := r.executor.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var suggestions []*portfolio.OptimizationSuggestion
	for rows.Next() {
		s, err := scanSuggestion(rows)
		if err != nil {
			return nil, 0, err
		}
		suggestions = append(suggestions, s)
	}
	return suggestions, total, nil
}

func (r *postgresPortfolioRepo) UpdateSuggestionStatus(ctx context.Context, id uuid.UUID, status string, resolvedBy uuid.UUID) error {
	query := `UPDATE portfolio_optimization_suggestions SET status = $1, resolved_by = $2, resolved_at = NOW(), updated_at = NOW() WHERE id = $3`
	res, err := r.executor.ExecContext(ctx, query, status, resolvedBy, id)
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to update suggestion")
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return errors.New(errors.ErrCodeNotFound, "suggestion not found")
	}
	return nil
}

func (r *postgresPortfolioRepo) GetPendingSuggestionCount(ctx context.Context, portfolioID uuid.UUID) (int64, error) {
	query := `SELECT COUNT(*) FROM portfolio_optimization_suggestions WHERE portfolio_id = $1 AND status = 'pending'`
	var count int64
	err := r.executor.QueryRowContext(ctx, query, portfolioID).Scan(&count)
	return count, err
}

// Analysis

func (r *postgresPortfolioRepo) GetPortfolioSummary(ctx context.Context, portfolioID uuid.UUID) (*portfolio.Summary, error) {
	summary := &portfolio.Summary{
		StatusCounts: make(map[string]int),
	}

	// Total patents
	query := `SELECT COUNT(*) FROM portfolio_patents WHERE portfolio_id = $1`
	var total int
	r.executor.QueryRowContext(ctx, query, portfolioID).Scan(&total)
	summary.TotalPatents = total

	// Active patents (via join)
	query = `
		SELECT COUNT(*) FROM portfolio_patents pp
		JOIN patents p ON pp.patent_id = p.id
		WHERE pp.portfolio_id = $1 AND p.status IN ('granted', 'filed', 'published', 'under_examination')
	`
	var active int
	r.executor.QueryRowContext(ctx, query, portfolioID).Scan(&active)
	summary.ActivePatents = active

	// Status counts
	query = `
		SELECT p.status, COUNT(*)
		FROM portfolio_patents pp
		JOIN patents p ON pp.patent_id = p.id
		WHERE pp.portfolio_id = $1
		GROUP BY p.status
	`
	rows, err := r.executor.QueryContext(ctx, query, portfolioID)
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var s string
			var c int
			rows.Scan(&s, &c)
			summary.StatusCounts[s] = c
		}
	}

	// Valuation
	// Simplified: average of latest composite scores
	query = `
		SELECT AVG(v.composite_score), SUM(v.monetary_value_mid)
		FROM (
			SELECT DISTINCT ON (patent_id) composite_score, monetary_value_mid
			FROM patent_valuations
			WHERE portfolio_id = $1
			ORDER BY patent_id, created_at DESC
		) v
	`
	var avgScore *float64
	var totalVal *int64
	r.executor.QueryRowContext(ctx, query, portfolioID).Scan(&avgScore, &totalVal)
	if avgScore != nil { summary.AverageScore = *avgScore }
	if totalVal != nil { summary.TotalValuation = *totalVal }

	// Health score
	hs, _ := r.GetLatestHealthScore(ctx, portfolioID)
	if hs != nil {
		summary.HealthScore = hs.OverallScore
	}

	return summary, nil
}

func (r *postgresPortfolioRepo) GetJurisdictionCoverage(ctx context.Context, portfolioID uuid.UUID) (map[string]int64, error) {
	query := `
		SELECT p.jurisdiction, COUNT(*)
		FROM portfolio_patents pp
		JOIN patents p ON pp.patent_id = p.id
		WHERE pp.portfolio_id = $1
		GROUP BY p.jurisdiction
	`
	rows, err := r.executor.QueryContext(ctx, query, portfolioID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	res := make(map[string]int64)
	for rows.Next() {
		var j string
		var c int64
		rows.Scan(&j, &c)
		res[j] = c
	}
	return res, nil
}

func (r *postgresPortfolioRepo) GetTechDomainCoverage(ctx context.Context, portfolioID uuid.UUID) (map[string]int64, error) {
	// Assuming tech codes map to domains roughly or using IPC top level
	query := `
		SELECT LEFT(unnest(p.ipc_codes), 4) as domain, COUNT(*)
		FROM portfolio_patents pp
		JOIN patents p ON pp.patent_id = p.id
		WHERE pp.portfolio_id = $1
		GROUP BY domain
	`
	rows, err := r.executor.QueryContext(ctx, query, portfolioID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	res := make(map[string]int64)
	for rows.Next() {
		var d string
		var c int64
		rows.Scan(&d, &c)
		res[d] = c
	}
	return res, nil
}

func (r *postgresPortfolioRepo) GetExpiryTimeline(ctx context.Context, portfolioID uuid.UUID) ([]*portfolio.ExpiryTimelineEntry, error) {
	query := `
		SELECT EXTRACT(YEAR FROM p.expiry_date) as year, COUNT(*)
		FROM portfolio_patents pp
		JOIN patents p ON pp.patent_id = p.id
		WHERE pp.portfolio_id = $1 AND p.expiry_date IS NOT NULL
		GROUP BY year
		ORDER BY year ASC
	`
	rows, err := r.executor.QueryContext(ctx, query, portfolioID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var timeline []*portfolio.ExpiryTimelineEntry
	for rows.Next() {
		var y float64
		var c int
		rows.Scan(&y, &c)
		timeline = append(timeline, &portfolio.ExpiryTimelineEntry{Year: int(y), Count: c})
	}
	return timeline, nil
}

func (r *postgresPortfolioRepo) ComparePortfolios(ctx context.Context, portfolioIDs []uuid.UUID) ([]*portfolio.ComparisonResult, error) {
	// Placeholder: returning comparison of patent counts and average valuation
	var results []*portfolio.ComparisonResult

	for _, pid := range portfolioIDs {
		sum, err := r.GetPortfolioSummary(ctx, pid)
		if err == nil {
			results = append(results, &portfolio.ComparisonResult{PortfolioID: pid, Metric: "total_patents", Value: float64(sum.TotalPatents)})
			results = append(results, &portfolio.ComparisonResult{PortfolioID: pid, Metric: "health_score", Value: sum.HealthScore})
			results = append(results, &portfolio.ComparisonResult{PortfolioID: pid, Metric: "valuation", Value: float64(sum.TotalValuation)})
		}
	}
	return results, nil
}

// Helpers

func scanPortfolio(row scanner) (*portfolio.Portfolio, error) {
	var p portfolio.Portfolio
	var metaJSON []byte
	var tech, target []string

	err := row.Scan(
		&p.ID, &p.Name, &p.Description, &p.OwnerID, &p.Status,
		pq.Array(&tech), pq.Array(&target), &metaJSON,
		&p.CreatedAt, &p.UpdatedAt, &p.DeletedAt, &p.PatentCount,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New(errors.CodePortfolioNotFound, "portfolio not found")
		}
		return nil, err
	}
	p.TechDomains = tech
	p.TargetJurisdictions = target
	if len(metaJSON) > 0 { _ = json.Unmarshal(metaJSON, &p.Metadata) }
	return &p, nil
}

func scanValuation(row scanner) (*portfolio.Valuation, error) {
	var v portfolio.Valuation
	var scoringJSON, comparableJSON, assumptionsJSON []byte

	err := row.Scan(
		&v.ID, &v.PatentID, &v.PortfolioID, &v.TechnicalScore, &v.LegalScore, &v.MarketScore, &v.StrategicScore,
		&v.CompositeScore, &v.Tier, &v.MonetaryValueLow, &v.MonetaryValueMid, &v.MonetaryValueHigh,
		&v.Currency, &v.ValuationMethod, &v.ModelVersion, &scoringJSON, &comparableJSON, &assumptionsJSON,
		&v.ValidFrom, &v.ValidUntil, &v.EvaluatedBy, &v.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New(errors.ErrCodeNotFound, "valuation not found")
		}
		return nil, err
	}
	if len(scoringJSON) > 0 { _ = json.Unmarshal(scoringJSON, &v.ScoringDetails) }
	if len(comparableJSON) > 0 { _ = json.Unmarshal(comparableJSON, &v.ComparablePatents) }
	if len(assumptionsJSON) > 0 { _ = json.Unmarshal(assumptionsJSON, &v.Assumptions) }
	return &v, nil
}

func scanHealthScore(row scanner) (*portfolio.HealthScore, error) {
	var s portfolio.HealthScore
	var jurisJSON, techJSON, tierJSON, recsJSON []byte

	err := row.Scan(
		&s.ID, &s.PortfolioID, &s.OverallScore, &s.CoverageScore, &s.DiversityScore,
		&s.FreshnessScore, &s.StrengthScore, &s.RiskScore,
		&s.TotalPatents, &s.ActivePatents, &s.ExpiringWithinYear, &s.ExpiringWithin3Years,
		&jurisJSON, &techJSON, &tierJSON, &recsJSON, &s.ModelVersion, &s.EvaluatedAt, &s.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	if len(jurisJSON) > 0 { _ = json.Unmarshal(jurisJSON, &s.JurisdictionDistribution) }
	if len(techJSON) > 0 { _ = json.Unmarshal(techJSON, &s.TechDomainDistribution) }
	if len(tierJSON) > 0 { _ = json.Unmarshal(tierJSON, &s.TierDistribution) }
	if len(recsJSON) > 0 { _ = json.Unmarshal(recsJSON, &s.Recommendations) }
	return &s, nil
}

func scanSuggestion(row scanner) (*portfolio.OptimizationSuggestion, error) {
	var s portfolio.OptimizationSuggestion
	var rationaleJSON []byte

	err := row.Scan(
		&s.ID, &s.PortfolioID, &s.HealthScoreID, &s.SuggestionType, &s.Priority, &s.Title, &s.Description,
		&s.TargetPatentID, &s.TargetTechDomain, &s.TargetJurisdiction, &s.EstimatedImpact, &s.EstimatedCost,
		&rationaleJSON, &s.Status, &s.ResolvedBy, &s.ResolvedAt, &s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	if len(rationaleJSON) > 0 { _ = json.Unmarshal(rationaleJSON, &s.Rationale) }
	return &s, nil
}
