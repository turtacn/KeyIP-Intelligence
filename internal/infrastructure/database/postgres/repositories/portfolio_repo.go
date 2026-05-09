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

func (r *postgresPortfolioRepo) GetByID(ctx context.Context, id string) (*portfolio.Portfolio, error) {
	query := `SELECT * FROM portfolios WHERE id = $1 AND deleted_at IS NULL`
	row := r.executor().QueryRowContext(ctx, query, id)
	p, err := scanPortfolio(row)
	if err != nil {
		return nil, err
	}

	// Preload patent count
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
	if rows, _ := res.RowsAffected(); rows == 0 {
		return errors.New(errors.ErrCodeConflict, "portfolio updated by another transaction or not found")
	}
	return nil
}

func (r *postgresPortfolioRepo) SoftDelete(ctx context.Context, id string) error {
	query := `UPDATE portfolios SET deleted_at = NOW() WHERE id = $1`
	_, err := r.executor().ExecContext(ctx, query, id)
	return err
}

func (r *postgresPortfolioRepo) List(ctx context.Context, ownerID string, opts ...portfolio.PortfolioQueryOption) ([]*portfolio.Portfolio, int64, error) {
	// Apply options
	options := portfolio.ApplyPortfolioOptions(opts...)

	baseQuery := `FROM portfolios WHERE owner_id = $1 AND deleted_at IS NULL`
	args := []interface{}{ownerID}

	if options.Status != nil {
		baseQuery += ` AND status = $2`
		args = append(args, *options.Status)
	}

	var total int64
	err := r.executor().QueryRowContext(ctx, "SELECT COUNT(*) "+baseQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	dataQuery := fmt.Sprintf("SELECT * %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d", baseQuery, len(args)+1, len(args)+2)
	args = append(args, options.Limit, options.Offset)

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

func (r *postgresPortfolioRepo) GetByOwner(ctx context.Context, ownerID string) ([]*portfolio.Portfolio, error) {
	list, _, err := r.List(ctx, ownerID)
	return list, err
}

// Patents Association

func (r *postgresPortfolioRepo) AddPatent(ctx context.Context, portfolioID, patentID string, role string, addedBy string) error {
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

func (r *postgresPortfolioRepo) RemovePatent(ctx context.Context, portfolioID, patentID string) error {
	query := `DELETE FROM portfolio_patents WHERE portfolio_id = $1 AND patent_id = $2`
	_, err := r.executor().ExecContext(ctx, query, portfolioID, patentID)
	return err
}

func (r *postgresPortfolioRepo) GetPatents(ctx context.Context, portfolioID string, role *string, limit, offset int) ([]*patent.Patent, int64, error) {
	// JOIN with patents table
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
		p, err := scanPortfolioPatent(rows) // Requires scanPatent from patent_repo.go logic (duplicated here or extracted)
		// scanPatent is not exported from patent_repo.go.
		// I must reimplement it here or extract to a shared location.
		// I'll reimplement simplified version for now.
		if err != nil { return nil, 0, err }
		patents = append(patents, p)
	}
	return patents, total, nil
}

func (r *postgresPortfolioRepo) IsPatentInPortfolio(ctx context.Context, portfolioID, patentID string) (bool, error) {
	var exists bool
	query := `SELECT EXISTS(SELECT 1 FROM portfolio_patents WHERE portfolio_id = $1 AND patent_id = $2)`
	err := r.executor().QueryRowContext(ctx, query, portfolioID, patentID).Scan(&exists)
	return exists, err
}

func (r *postgresPortfolioRepo) BatchAddPatents(ctx context.Context, portfolioID string, patentIDs []string, role string, addedBy string) error {
	if len(patentIDs) == 0 {
		return nil
	}
	query := `
		INSERT INTO portfolio_patents (portfolio_id, patent_id, role_in_portfolio, added_by, added_at)
		SELECT $1, unnest($2::uuid[]), $3, $4, NOW()
		ON CONFLICT (portfolio_id, patent_id) DO NOTHING
	`
	_, err := r.executor().ExecContext(ctx, query, portfolioID, pq.Array(patentIDs), role, addedBy)
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to batch add patents")
	}
	return nil
}

func (r *postgresPortfolioRepo) GetPortfoliosByPatent(ctx context.Context, patentID string) ([]*portfolio.Portfolio, error) {
	query := `
		SELECT p.* FROM portfolios p
		JOIN portfolio_patents pp ON p.id = pp.portfolio_id
		WHERE pp.patent_id = $1 AND p.deleted_at IS NULL
		ORDER BY p.created_at DESC
	`
	rows, err := r.executor().QueryContext(ctx, query, patentID)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to get portfolios by patent")
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
func scanValuation(row scanner) (*portfolio.Valuation, error) {
	v := &portfolio.Valuation{}
	var portfolioID, evaluatedBy uuid.NullUUID
	var monetaryLow, monetaryMid, monetaryHigh sql.NullInt64
	var validUntil sql.NullTime
	var scoringDetails, assumptions, comparablePatents []byte

	err := row.Scan(
		&v.ID, &v.PatentID, &portfolioID,
		&v.TechnicalScore, &v.LegalScore, &v.MarketScore, &v.StrategicScore, &v.CompositeScore,
		&v.Tier,
		&monetaryLow, &monetaryMid, &monetaryHigh,
		&v.Currency, &v.ValuationMethod, &v.ModelVersion,
		&scoringDetails, &comparablePatents, &assumptions,
		&v.ValidFrom, &validUntil, &evaluatedBy,
		&v.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New(errors.ErrCodeNotFound, "valuation not found")
		}
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to scan valuation")
	}
	if portfolioID.Valid {
		s := portfolioID.UUID.String()
		v.PortfolioID = &s
	}
	if evaluatedBy.Valid {
		s := evaluatedBy.UUID.String()
		v.EvaluatedBy = &s
	}
	if monetaryLow.Valid { v.MonetaryValueLow = &monetaryLow.Int64 }
	if monetaryMid.Valid { v.MonetaryValueMid = &monetaryMid.Int64 }
	if monetaryHigh.Valid { v.MonetaryValueHigh = &monetaryHigh.Int64 }
	if validUntil.Valid { v.ValidUntil = &validUntil.Time }
	if len(scoringDetails) > 0 { _ = json.Unmarshal(scoringDetails, &v.ScoringDetails) }
	if len(assumptions) > 0 { _ = json.Unmarshal(assumptions, &v.Assumptions) }
	if len(comparablePatents) > 0 { _ = json.Unmarshal(comparablePatents, &v.ComparablePatents) }
	return v, nil
}

func (r *postgresPortfolioRepo) CreateValuation(ctx context.Context, v *portfolio.Valuation) error {
	query := `
		INSERT INTO patent_valuations (
			patent_id, portfolio_id, technical_score, legal_score, market_score, strategic_score, composite_score,
			tier, monetary_value_low, monetary_value_mid, monetary_value_high, currency, valuation_method, model_version,
			scoring_details, comparable_patents, assumptions, valid_from, valid_until, evaluated_by
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20
		) RETURNING id, created_at
	`
	scoringDetails, _ := json.Marshal(v.ScoringDetails)
	comparablePatents, _ := json.Marshal(v.ComparablePatents)
	assumptions, _ := json.Marshal(v.Assumptions)

	var portfolioID, evaluatedBy interface{}
	if v.PortfolioID != nil { portfolioID = *v.PortfolioID }
	if v.EvaluatedBy != nil { evaluatedBy = *v.EvaluatedBy }

	err := r.executor().QueryRowContext(ctx, query,
		v.PatentID, portfolioID,
		v.TechnicalScore, v.LegalScore, v.MarketScore, v.StrategicScore, v.CompositeScore,
		v.Tier, v.MonetaryValueLow, v.MonetaryValueMid, v.MonetaryValueHigh,
		v.Currency, v.ValuationMethod, v.ModelVersion,
		scoringDetails, comparablePatents, assumptions,
		v.ValidFrom, v.ValidUntil, evaluatedBy,
	).Scan(&v.ID, &v.CreatedAt)

	if err != nil {
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to create valuation")
	}
	return nil
}

func (r *postgresPortfolioRepo) GetLatestValuation(ctx context.Context, patentID string) (*portfolio.Valuation, error) {
	query := `
		SELECT * FROM patent_valuations
		WHERE patent_id = $1
		ORDER BY created_at DESC LIMIT 1
	`
	row := r.executor().QueryRowContext(ctx, query, patentID)
	return scanValuation(row)
}

func (r *postgresPortfolioRepo) GetValuationHistory(ctx context.Context, patentID string, limit int) ([]*portfolio.Valuation, error) {
	query := `
		SELECT * FROM patent_valuations
		WHERE patent_id = $1
		ORDER BY created_at DESC LIMIT $2
	`
	rows, err := r.executor().QueryContext(ctx, query, patentID, limit)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to get valuation history")
	}
	defer rows.Close()

	var valuations []*portfolio.Valuation
	for rows.Next() {
		v, err := scanValuation(rows)
		if err != nil { return nil, err }
		valuations = append(valuations, v)
	}
	return valuations, nil
}

func (r *postgresPortfolioRepo) GetValuationsByPortfolio(ctx context.Context, portfolioID string) ([]*portfolio.Valuation, error) {
	query := `SELECT * FROM patent_valuations WHERE portfolio_id = $1 ORDER BY created_at DESC`
	rows, err := r.executor().QueryContext(ctx, query, portfolioID)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to get valuations by portfolio")
	}
	defer rows.Close()

	var valuations []*portfolio.Valuation
	for rows.Next() {
		v, err := scanValuation(rows)
		if err != nil { return nil, err }
		valuations = append(valuations, v)
	}
	return valuations, nil
}

func (r *postgresPortfolioRepo) GetValuationDistribution(ctx context.Context, portfolioID string) (map[portfolio.ValuationTier]int64, error) {
	query := `
		SELECT tier, COUNT(*) FROM patent_valuations
		WHERE portfolio_id = $1
		GROUP BY tier
	`
	rows, err := r.executor().QueryContext(ctx, query, portfolioID)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to get valuation distribution")
	}
	defer rows.Close()

	dist := make(map[portfolio.ValuationTier]int64)
	for rows.Next() {
		var tier string
		var count int64
		if err := rows.Scan(&tier, &count); err != nil {
			return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to scan distribution")
		}
		dist[portfolio.ValuationTier(tier)] = count
	}
	return dist, nil
}

func (r *postgresPortfolioRepo) BatchCreateValuations(ctx context.Context, valuations []*portfolio.Valuation) error {
	if len(valuations) == 0 {
		return nil
	}
	query := `
		INSERT INTO patent_valuations (
			patent_id, portfolio_id, technical_score, legal_score, market_score, strategic_score, composite_score,
			tier, monetary_value_low, monetary_value_mid, monetary_value_high, currency, valuation_method, model_version,
			scoring_details, comparable_patents, assumptions, valid_from, valid_until, evaluated_by
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20
		)
	`
	stmt, err := r.executor().(interface{ PrepareContext(context.Context, string) (*sql.Stmt, error) }).PrepareContext(ctx, query)
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to prepare batch valuation insert")
	}
	defer stmt.Close()

	for _, v := range valuations {
		scoringDetails, _ := json.Marshal(v.ScoringDetails)
		comparablePatents, _ := json.Marshal(v.ComparablePatents)
		assumptions, _ := json.Marshal(v.Assumptions)

		var portfolioID, evaluatedBy interface{}
		if v.PortfolioID != nil { portfolioID = *v.PortfolioID }
		if v.EvaluatedBy != nil { evaluatedBy = *v.EvaluatedBy }

		_, err := stmt.ExecContext(ctx,
			v.PatentID, portfolioID,
			v.TechnicalScore, v.LegalScore, v.MarketScore, v.StrategicScore, v.CompositeScore,
			v.Tier, v.MonetaryValueLow, v.MonetaryValueMid, v.MonetaryValueHigh,
			v.Currency, v.ValuationMethod, v.ModelVersion,
			scoringDetails, comparablePatents, assumptions,
			v.ValidFrom, v.ValidUntil, evaluatedBy,
		)
		if err != nil {
			return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to batch create valuation")
		}
	}
	return nil
}

// HealthScore
func scanHealthScore(row scanner) (*portfolio.HealthScore, error) {
	h := &portfolio.HealthScore{}
	var jurisdictionDist, techDomainDist, tierDist, recommendations []byte

	err := row.Scan(
		&h.ID, &h.PortfolioID,
		&h.OverallScore, &h.CoverageScore, &h.DiversityScore, &h.FreshnessScore, &h.StrengthScore, &h.RiskScore,
		&h.TotalPatents, &h.ActivePatents, &h.ExpiringWithinYear, &h.ExpiringWithin3Years,
		&jurisdictionDist, &techDomainDist, &tierDist,
		&recommendations, &h.ModelVersion,
		&h.EvaluatedAt, &h.CreatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New(errors.ErrCodeNotFound, "health score not found")
		}
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to scan health score")
	}
	if len(jurisdictionDist) > 0 { _ = json.Unmarshal(jurisdictionDist, &h.JurisdictionDistribution) }
	if len(techDomainDist) > 0 { _ = json.Unmarshal(techDomainDist, &h.TechDomainDistribution) }
	if len(tierDist) > 0 { _ = json.Unmarshal(tierDist, &h.TierDistribution) }
	if len(recommendations) > 0 { _ = json.Unmarshal(recommendations, &h.Recommendations) }
	return h, nil
}

func (r *postgresPortfolioRepo) CreateHealthScore(ctx context.Context, score *portfolio.HealthScore) error {
	query := `
		INSERT INTO portfolio_health_scores (
			portfolio_id, overall_score, coverage_score, diversity_score, freshness_score, strength_score, risk_score,
			total_patents, active_patents, expiring_within_year, expiring_within_3years,
			jurisdiction_distribution, tech_domain_distribution, tier_distribution,
			recommendations, model_version, evaluated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17
		) RETURNING id, created_at
	`
	jurisdictionDist, _ := json.Marshal(score.JurisdictionDistribution)
	techDomainDist, _ := json.Marshal(score.TechDomainDistribution)
	tierDist, _ := json.Marshal(score.TierDistribution)
	recommendations, _ := json.Marshal(score.Recommendations)

	err := r.executor().QueryRowContext(ctx, query,
		score.PortfolioID,
		score.OverallScore, score.CoverageScore, score.DiversityScore, score.FreshnessScore, score.StrengthScore, score.RiskScore,
		score.TotalPatents, score.ActivePatents, score.ExpiringWithinYear, score.ExpiringWithin3Years,
		jurisdictionDist, techDomainDist, tierDist,
		recommendations, score.ModelVersion, score.EvaluatedAt,
	).Scan(&score.ID, &score.CreatedAt)

	if err != nil {
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to create health score")
	}
	return nil
}

func (r *postgresPortfolioRepo) GetLatestHealthScore(ctx context.Context, portfolioID string) (*portfolio.HealthScore, error) {
	query := `SELECT * FROM portfolio_health_scores WHERE portfolio_id = $1 ORDER BY evaluated_at DESC LIMIT 1`
	row := r.executor().QueryRowContext(ctx, query, portfolioID)
	return scanHealthScore(row)
}

func (r *postgresPortfolioRepo) GetHealthScoreHistory(ctx context.Context, portfolioID string, limit int) ([]*portfolio.HealthScore, error) {
	query := `SELECT * FROM portfolio_health_scores WHERE portfolio_id = $1 ORDER BY evaluated_at DESC LIMIT $2`
	rows, err := r.executor().QueryContext(ctx, query, portfolioID, limit)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to get health score history")
	}
	defer rows.Close()

	var scores []*portfolio.HealthScore
	for rows.Next() {
		s, err := scanHealthScore(rows)
		if err != nil { return nil, err }
		scores = append(scores, s)
	}
	return scores, nil
}

func (r *postgresPortfolioRepo) GetHealthScoreTrend(ctx context.Context, portfolioID string, startDate, endDate time.Time) ([]*portfolio.HealthScore, error) {
	query := `
		SELECT * FROM portfolio_health_scores
		WHERE portfolio_id = $1 AND evaluated_at >= $2 AND evaluated_at <= $3
		ORDER BY evaluated_at ASC
	`
	rows, err := r.executor().QueryContext(ctx, query, portfolioID, startDate, endDate)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to get health score trend")
	}
	defer rows.Close()

	var scores []*portfolio.HealthScore
	for rows.Next() {
		s, err := scanHealthScore(rows)
		if err != nil { return nil, err }
		scores = append(scores, s)
	}
	return scores, nil
}

// Suggestions
func scanOptimizationSuggestion(row scanner) (*portfolio.OptimizationSuggestion, error) {
	s := &portfolio.OptimizationSuggestion{}
	var healthScoreID, targetPatentID, resolvedBy uuid.NullUUID
	var resolvedAt sql.NullTime
	var estimatedImpact sql.NullFloat64
	var estimatedCost sql.NullInt64
	var rationale []byte

	err := row.Scan(
		&s.ID, &s.PortfolioID, &healthScoreID,
		&s.SuggestionType, &s.Priority,
		&s.Title, &s.Description,
		&targetPatentID, &s.TargetTechDomain, &s.TargetJurisdiction,
		&estimatedImpact, &estimatedCost,
		&rationale,
		&s.Status,
		&resolvedBy, &resolvedAt,
		&s.CreatedAt, &s.UpdatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New(errors.ErrCodeNotFound, "suggestion not found")
		}
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to scan suggestion")
	}
	if healthScoreID.Valid { h := healthScoreID.UUID.String(); s.HealthScoreID = &h }
	if targetPatentID.Valid { t := targetPatentID.UUID.String(); s.TargetPatentID = &t }
	if resolvedBy.Valid { r := resolvedBy.UUID.String(); s.ResolvedBy = &r }
	if resolvedAt.Valid { s.ResolvedAt = &resolvedAt.Time }
	if estimatedImpact.Valid { s.EstimatedImpact = &estimatedImpact.Float64 }
	if estimatedCost.Valid { s.EstimatedCost = &estimatedCost.Int64 }
	if len(rationale) > 0 { _ = json.Unmarshal(rationale, &s.Rationale) }
	return s, nil
}

func (r *postgresPortfolioRepo) CreateSuggestion(ctx context.Context, s *portfolio.OptimizationSuggestion) error {
	query := `
		INSERT INTO portfolio_optimization_suggestions (
			portfolio_id, health_score_id, suggestion_type, priority, title, description,
			target_patent_id, target_tech_domain, target_jurisdiction,
			estimated_impact, estimated_cost, rationale, status
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13
		) RETURNING id, created_at, updated_at
	`
	rationale, _ := json.Marshal(s.Rationale)

	var healthScoreID, targetPatentID interface{}
	if s.HealthScoreID != nil { healthScoreID = *s.HealthScoreID }
	if s.TargetPatentID != nil { targetPatentID = *s.TargetPatentID }

	err := r.executor().QueryRowContext(ctx, query,
		s.PortfolioID, healthScoreID, s.SuggestionType, s.Priority, s.Title, s.Description,
		targetPatentID, s.TargetTechDomain, s.TargetJurisdiction,
		s.EstimatedImpact, s.EstimatedCost, rationale, s.Status,
	).Scan(&s.ID, &s.CreatedAt, &s.UpdatedAt)

	if err != nil {
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to create suggestion")
	}
	return nil
}

func (r *postgresPortfolioRepo) GetSuggestions(ctx context.Context, portfolioID string, status *string, limit, offset int) ([]*portfolio.OptimizationSuggestion, int64, error) {
	baseQuery := `FROM portfolio_optimization_suggestions WHERE portfolio_id = $1`
	args := []interface{}{portfolioID}
	argIdx := 2

	if status != nil {
		baseQuery += fmt.Sprintf(` AND status = $%d`, argIdx)
		args = append(args, *status)
		argIdx++
	}

	var total int64
	err := r.executor().QueryRowContext(ctx, "SELECT COUNT(*) "+baseQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to count suggestions")
	}

	dataQuery := fmt.Sprintf("SELECT * %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d", baseQuery, argIdx, argIdx+1)
	args = append(args, limit, offset)

	rows, err := r.executor().QueryContext(ctx, dataQuery, args...)
	if err != nil {
		return nil, 0, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to get suggestions")
	}
	defer rows.Close()

	var suggestions []*portfolio.OptimizationSuggestion
	for rows.Next() {
		s, err := scanOptimizationSuggestion(rows)
		if err != nil { return nil, 0, err }
		suggestions = append(suggestions, s)
	}
	return suggestions, total, nil
}

func (r *postgresPortfolioRepo) UpdateSuggestionStatus(ctx context.Context, id string, status string, resolvedBy string) error {
	query := `
		UPDATE portfolio_optimization_suggestions
		SET status = $1, resolved_by = $2, resolved_at = CASE WHEN $1 IN ('accepted', 'rejected', 'implemented') THEN NOW() ELSE resolved_at END,
		    updated_at = NOW()
		WHERE id = $3
	`
	var resolvedByID interface{}
	if resolvedBy != "" {
		resolvedByID = resolvedBy
	}
	res, err := r.executor().ExecContext(ctx, query, status, resolvedByID, id)
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to update suggestion status")
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return errors.New(errors.ErrCodeNotFound, "suggestion not found")
	}
	return nil
}

func (r *postgresPortfolioRepo) GetPendingSuggestionCount(ctx context.Context, portfolioID string) (int64, error) {
	query := `SELECT COUNT(*) FROM portfolio_optimization_suggestions WHERE portfolio_id = $1 AND status = 'pending'`
	var count int64
	err := r.executor().QueryRowContext(ctx, query, portfolioID).Scan(&count)
	if err != nil {
		return 0, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to count pending suggestions")
	}
	return count, nil
}

// Analytics
func (r *postgresPortfolioRepo) GetPortfolioSummary(ctx context.Context, portfolioID string) (*portfolio.Summary, error) {
	query := `
		SELECT
			COUNT(DISTINCT pp.patent_id) AS total_patents,
			COUNT(DISTINCT pp.patent_id) FILTER (WHERE p.status IN ('granted', 'published', 'under_examination', 'filed')) AS active_patents,
			COALESCE(AVG(pv.composite_score), 0) AS average_score,
			COALESCE(SUM(pv.monetary_value_mid), 0) AS total_valuation
		FROM portfolio_patents pp
		LEFT JOIN patents p ON pp.patent_id = p.id AND p.deleted_at IS NULL
		LEFT JOIN LATERAL (
			SELECT composite_score, monetary_value_mid FROM patent_valuations
			WHERE patent_id = pp.patent_id
			ORDER BY created_at DESC LIMIT 1
		) pv ON TRUE
		WHERE pp.portfolio_id = $1
	`
	s := &portfolio.Summary{
		StatusCounts: make(map[string]int),
	}
	err := r.executor().QueryRowContext(ctx, query, portfolioID).Scan(
		&s.TotalPatents, &s.ActivePatents, &s.AverageScore, &s.TotalValuation,
	)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to get portfolio summary")
	}

	// Get health score
	healthQuery := `SELECT overall_score FROM portfolio_health_scores WHERE portfolio_id = $1 ORDER BY evaluated_at DESC LIMIT 1`
	var healthScore sql.NullFloat64
	if err := r.executor().QueryRowContext(ctx, healthQuery, portfolioID).Scan(&healthScore); err == nil && healthScore.Valid {
		s.HealthScore = healthScore.Float64
	}

	// Get status counts
	statusQuery := `
		SELECT p.status, COUNT(*) AS count
		FROM portfolio_patents pp
		JOIN patents p ON pp.patent_id = p.id AND p.deleted_at IS NULL
		WHERE pp.portfolio_id = $1
		GROUP BY p.status
	`
	statusRows, err := r.executor().QueryContext(ctx, statusQuery, portfolioID)
	if err == nil {
		defer statusRows.Close()
		for statusRows.Next() {
			var status string
			var count int
			if err := statusRows.Scan(&status, &count); err == nil {
				s.StatusCounts[status] = count
			}
		}
	}

	return s, nil
}

func (r *postgresPortfolioRepo) GetJurisdictionCoverage(ctx context.Context, portfolioID string) (map[string]int64, error) {
	query := `
		SELECT p.jurisdiction, COUNT(*) AS count
		FROM portfolio_patents pp
		JOIN patents p ON pp.patent_id = p.id AND p.deleted_at IS NULL
		WHERE pp.portfolio_id = $1
		GROUP BY p.jurisdiction
		ORDER BY count DESC
	`
	rows, err := r.executor().QueryContext(ctx, query, portfolioID)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to get jurisdiction coverage")
	}
	defer rows.Close()

	coverage := make(map[string]int64)
	for rows.Next() {
		var jurisdiction string
		var count int64
		if err := rows.Scan(&jurisdiction, &count); err != nil {
			return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to scan jurisdiction coverage")
		}
		coverage[jurisdiction] = count
	}
	return coverage, nil
}

func (r *postgresPortfolioRepo) GetTechDomainCoverage(ctx context.Context, portfolioID string) (map[string]int64, error) {
	query := `
		SELECT unnest(p.keyip_tech_codes) AS tech_code, COUNT(*) AS count
		FROM portfolio_patents pp
		JOIN patents p ON pp.patent_id = p.id AND p.deleted_at IS NULL
		WHERE pp.portfolio_id = $1 AND p.keyip_tech_codes IS NOT NULL
		GROUP BY tech_code
		ORDER BY count DESC
	`
	rows, err := r.executor().QueryContext(ctx, query, portfolioID)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to get tech domain coverage")
	}
	defer rows.Close()

	coverage := make(map[string]int64)
	for rows.Next() {
		var techCode string
		var count int64
		if err := rows.Scan(&techCode, &count); err != nil {
			return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to scan tech domain coverage")
		}
		coverage[techCode] = count
	}
	return coverage, nil
}

func (r *postgresPortfolioRepo) GetExpiryTimeline(ctx context.Context, portfolioID string) ([]*portfolio.ExpiryTimelineEntry, error) {
	query := `
		SELECT EXTRACT(YEAR FROM p.expiry_date)::int AS year, COUNT(*) AS count
		FROM portfolio_patents pp
		JOIN patents p ON pp.patent_id = p.id AND p.deleted_at IS NULL
		WHERE pp.portfolio_id = $1 AND p.expiry_date IS NOT NULL
		GROUP BY year
		ORDER BY year ASC
	`
	rows, err := r.executor().QueryContext(ctx, query, portfolioID)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to get expiry timeline")
	}
	defer rows.Close()

	var entries []*portfolio.ExpiryTimelineEntry
	for rows.Next() {
		var year int
		var count int
		if err := rows.Scan(&year, &count); err != nil {
			return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to scan expiry timeline")
		}
		entries = append(entries, &portfolio.ExpiryTimelineEntry{Year: year, Count: count})
	}
	return entries, nil
}

func (r *postgresPortfolioRepo) ComparePortfolios(ctx context.Context, portfolioIDs []string) ([]*portfolio.ComparisonResult, error) {
	if len(portfolioIDs) == 0 {
		return nil, nil
	}
	query := `
		SELECT
			pp.portfolio_id,
			COUNT(DISTINCT pp.patent_id) AS total_patents,
			COUNT(DISTINCT pp.patent_id) FILTER (WHERE p.status IN ('granted', 'published')) AS granted_published,
			COUNT(DISTINCT p.jurisdiction) AS jurisdiction_count,
			COALESCE(AVG(pv.composite_score), 0) AS avg_composite_score,
			COALESCE(SUM(pv.monetary_value_mid), 0) AS total_valuation,
			COUNT(DISTINCT CASE WHEN p.expiry_date IS NOT NULL AND p.expiry_date < NOW() + INTERVAL '1 year' THEN pp.patent_id END) AS expiring_soon
		FROM portfolio_patents pp
		LEFT JOIN patents p ON pp.patent_id = p.id AND p.deleted_at IS NULL
		LEFT JOIN LATERAL (
			SELECT composite_score, monetary_value_mid FROM patent_valuations
			WHERE patent_id = pp.patent_id
			ORDER BY created_at DESC LIMIT 1
		) pv ON TRUE
		WHERE pp.portfolio_id = ANY($1)
		GROUP BY pp.portfolio_id
	`
	rows, err := r.executor().QueryContext(ctx, query, pq.Array(portfolioIDs))
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to compare portfolios")
	}
	defer rows.Close()

	var results []*portfolio.ComparisonResult
	for rows.Next() {
		var portfolioID string
		var totalPatents, grantedPublished, jurisdictionCount, expiringSoon int
		var avgCompositeScore, totalValuation float64
		if err := rows.Scan(&portfolioID, &totalPatents, &grantedPublished, &jurisdictionCount, &avgCompositeScore, &totalValuation, &expiringSoon); err != nil {
			return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to scan comparison result")
		}
		results = append(results,
			&portfolio.ComparisonResult{PortfolioID: portfolioID, Metric: "total_patents", Value: float64(totalPatents)},
			&portfolio.ComparisonResult{PortfolioID: portfolioID, Metric: "granted_published", Value: float64(grantedPublished)},
			&portfolio.ComparisonResult{PortfolioID: portfolioID, Metric: "jurisdiction_count", Value: float64(jurisdictionCount)},
			&portfolio.ComparisonResult{PortfolioID: portfolioID, Metric: "avg_composite_score", Value: avgCompositeScore},
			&portfolio.ComparisonResult{PortfolioID: portfolioID, Metric: "total_valuation", Value: totalValuation},
			&portfolio.ComparisonResult{PortfolioID: portfolioID, Metric: "expiring_soon", Value: float64(expiringSoon)},
		)
	}
	return results, nil
}

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
	// id, name, description, owner_id, status, tech_domains, target_jurisdictions, metadata, created_at, updated_at, deleted_at
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
	var raw, meta []byte
	var statusStr string
	err := row.Scan(
		&p.ID, &p.PatentNumber, &p.Title, &p.TitleEn, &p.Abstract, &p.AbstractEn, &p.Type, &statusStr,
		&p.FilingDate, &p.PublicationDate, &p.GrantDate, &p.ExpiryDate, &p.PriorityDate,
		&p.AssigneeID, &p.AssigneeName, &p.Jurisdiction,
		pq.Array(&p.IPCCodes), pq.Array(&p.CPCCodes), pq.Array(&p.KeyIPTechCodes),
		&p.FamilyID, &p.ApplicationNumber, &p.FullTextHash, &p.Source,
		&raw, &meta, &p.CreatedAt, &p.UpdatedAt, &p.DeletedAt,
	)
	if err != nil { return nil, err }

	p.Status = parsePatentStatus(statusStr)
	if len(raw) > 0 { _ = json.Unmarshal(raw, &p.RawData) }
	if len(meta) > 0 { _ = json.Unmarshal(meta, &p.Metadata) }

	return p, nil
}

//Personal.AI order the ending
