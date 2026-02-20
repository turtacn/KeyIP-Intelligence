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
// Domain entity — local mirror of internal/domain/portfolio
// ─────────────────────────────────────────────────────────────────────────────

// ValuationResult holds the monetary valuation of a portfolio.
type ValuationResult struct {
	TotalValue    float64   `json:"total_value"`
	Currency      string    `json:"currency"`
	Method        string    `json:"method"`
	Confidence    float64   `json:"confidence"`
	CalculatedAt  time.Time `json:"calculated_at"`
	BreakdownByID map[string]float64 `json:"breakdown_by_id,omitempty"`
}

// Portfolio is the aggregate root for the portfolio domain.
type Portfolio struct {
	ID          common.ID
	TenantID    common.TenantID
	Name        string
	Description string
	OwnerID     common.UserID
	PatentIDs   []common.ID
	Tags        []string
	TotalValue  *ValuationResult
	Status      string
	Metadata    map[string]interface{}
	CreatedAt   time.Time
	UpdatedAt   time.Time
	CreatedBy   common.UserID
	Version     int
}

// PortfolioSearchCriteria carries dynamic filter parameters.
type PortfolioSearchCriteria struct {
	Name     string
	OwnerID  string
	Status   string
	Tag      string
	Page     int
	PageSize int
}

// ─────────────────────────────────────────────────────────────────────────────
// PortfolioRepository
// ─────────────────────────────────────────────────────────────────────────────

// PortfolioRepository is the PostgreSQL implementation of the portfolio
// domain's Repository interface.
type PortfolioRepository struct {
	pool   *pgxpool.Pool
	logger Logger
}

// NewPortfolioRepository constructs a ready-to-use PortfolioRepository.
func NewPortfolioRepository(pool *pgxpool.Pool, logger Logger) *PortfolioRepository {
	return &PortfolioRepository{pool: pool, logger: logger}
}

// ─────────────────────────────────────────────────────────────────────────────
// Save
// ─────────────────────────────────────────────────────────────────────────────

func (r *PortfolioRepository) Save(ctx context.Context, p *Portfolio) error {
	r.logger.Debug("PortfolioRepository.Save", "portfolio_id", p.ID)

	valJSON, _ := json.Marshal(p.TotalValue)
	metaJSON, _ := json.Marshal(p.Metadata)

	// Convert []common.ID to []string for PostgreSQL TEXT[] storage.
	patentIDStrs := make([]string, len(p.PatentIDs))
	for i, id := range p.PatentIDs {
		patentIDStrs[i] = string(id)
	}

	_, err := r.pool.Exec(ctx, `
		INSERT INTO portfolios (
			id, tenant_id, name, description, owner_id,
			patent_ids, tags, total_value, status,
			metadata, created_at, updated_at, created_by, version
		) VALUES (
			$1,$2,$3,$4,$5,
			$6,$7,$8,$9,
			$10,$11,$12,$13,$14
		)`,
		p.ID, p.TenantID, p.Name, p.Description, p.OwnerID,
		patentIDStrs, p.Tags, valJSON, p.Status,
		metaJSON, p.CreatedAt, p.UpdatedAt, p.CreatedBy, p.Version,
	)
	if err != nil {
		r.logger.Error("PortfolioRepository.Save", "error", err)
		return appErrors.Wrap(err, appErrors.CodeDBQueryError, "failed to insert portfolio")
	}
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// FindByID
// ─────────────────────────────────────────────────────────────────────────────

func (r *PortfolioRepository) FindByID(ctx context.Context, id common.ID) (*Portfolio, error) {
	r.logger.Debug("PortfolioRepository.FindByID", "id", id)

	return r.scanPortfolio(r.pool.QueryRow(ctx, `
		SELECT id, tenant_id, name, description, owner_id,
		       patent_ids, tags, total_value, status,
		       metadata, created_at, updated_at, created_by, version
		FROM portfolios WHERE id = $1`, id))
}

// ─────────────────────────────────────────────────────────────────────────────
// FindByOwner
// ─────────────────────────────────────────────────────────────────────────────

func (r *PortfolioRepository) FindByOwner(ctx context.Context, ownerID common.UserID, page, pageSize int) ([]*Portfolio, int64, error) {
	r.logger.Debug("PortfolioRepository.FindByOwner", "owner_id", ownerID)

	where := "WHERE owner_id = $1"

	var total int64
	if err := r.pool.QueryRow(ctx,
		fmt.Sprintf("SELECT COUNT(*) FROM portfolios %s", where), ownerID,
	).Scan(&total); err != nil {
		r.logger.Error("PortfolioRepository.FindByOwner: count", "error", err)
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
		SELECT id, tenant_id, name, description, owner_id,
		       patent_ids, tags, total_value, status,
		       metadata, created_at, updated_at, created_by, version
		FROM portfolios %s
		ORDER BY updated_at DESC
		LIMIT $2 OFFSET $3`, where), ownerID, pageSize, offset)
	if err != nil {
		r.logger.Error("PortfolioRepository.FindByOwner: query", "error", err)
		return nil, 0, appErrors.Wrap(err, appErrors.CodeDBQueryError, "query failed")
	}
	defer rows.Close()

	portfolios, err := r.scanPortfolios(rows)
	return portfolios, total, err
}

// ─────────────────────────────────────────────────────────────────────────────
// FindByPatentID — array contains
// ─────────────────────────────────────────────────────────────────────────────

func (r *PortfolioRepository) FindByPatentID(ctx context.Context, patentID common.ID) ([]*Portfolio, error) {
	r.logger.Debug("PortfolioRepository.FindByPatentID", "patent_id", patentID)

	rows, err := r.pool.Query(ctx, `
		SELECT id, tenant_id, name, description, owner_id,
		       patent_ids, tags, total_value, status,
		       metadata, created_at, updated_at, created_by, version
		FROM portfolios
		WHERE patent_ids @> ARRAY[$1]::TEXT[]`, string(patentID))
	if err != nil {
		r.logger.Error("PortfolioRepository.FindByPatentID", "error", err)
		return nil, appErrors.Wrap(err, appErrors.CodeDBQueryError, "query failed")
	}
	defer rows.Close()

	return r.scanPortfolios(rows)
}

// ─────────────────────────────────────────────────────────────────────────────
// FindByTag — array contains
// ─────────────────────────────────────────────────────────────────────────────

func (r *PortfolioRepository) FindByTag(ctx context.Context, tag string, page, pageSize int) ([]*Portfolio, int64, error) {
	r.logger.Debug("PortfolioRepository.FindByTag", "tag", tag)

	where := "WHERE tags @> ARRAY[$1]::TEXT[]"

	var total int64
	if err := r.pool.QueryRow(ctx,
		fmt.Sprintf("SELECT COUNT(*) FROM portfolios %s", where), tag,
	).Scan(&total); err != nil {
		r.logger.Error("PortfolioRepository.FindByTag: count", "error", err)
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
		SELECT id, tenant_id, name, description, owner_id,
		       patent_ids, tags, total_value, status,
		       metadata, created_at, updated_at, created_by, version
		FROM portfolios %s
		ORDER BY updated_at DESC
		LIMIT $2 OFFSET $3`, where), tag, pageSize, offset)
	if err != nil {
		r.logger.Error("PortfolioRepository.FindByTag: query", "error", err)
		return nil, 0, appErrors.Wrap(err, appErrors.CodeDBQueryError, "query failed")
	}
	defer rows.Close()

	portfolios, err := r.scanPortfolios(rows)
	return portfolios, total, err
}

// ─────────────────────────────────────────────────────────────────────────────
// Search
// ─────────────────────────────────────────────────────────────────────────────

func (r *PortfolioRepository) Search(ctx context.Context, criteria PortfolioSearchCriteria) ([]*Portfolio, int64, error) {
	r.logger.Debug("PortfolioRepository.Search", "criteria", criteria)

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
	if criteria.OwnerID != "" {
		ph := nextArg(criteria.OwnerID)
		conditions = append(conditions, fmt.Sprintf("owner_id = %s", ph))
	}
	if criteria.Status != "" {
		ph := nextArg(criteria.Status)
		conditions = append(conditions, fmt.Sprintf("status = %s", ph))
	}
	if criteria.Tag != "" {
		ph := nextArg(criteria.Tag)
		conditions = append(conditions, fmt.Sprintf("tags @> ARRAY[%s]::TEXT[]", ph))
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	var total int64
	if err := r.pool.QueryRow(ctx,
		fmt.Sprintf("SELECT COUNT(*) FROM portfolios %s", whereClause), args...,
	).Scan(&total); err != nil {
		r.logger.Error("PortfolioRepository.Search: count", "error", err)
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

	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT id, tenant_id, name, description, owner_id,
		       patent_ids, tags, total_value, status,
		       metadata, created_at, updated_at, created_by, version
		FROM portfolios %s
		ORDER BY updated_at DESC
		LIMIT %s OFFSET %s`, whereClause, phLimit, phOffset), args...)
	if err != nil {
		r.logger.Error("PortfolioRepository.Search: query", "error", err)
		return nil, 0, appErrors.Wrap(err, appErrors.CodeDBQueryError, "search query failed")
	}
	defer rows.Close()

	portfolios, err := r.scanPortfolios(rows)
	return portfolios, total, err
}

// ─────────────────────────────────────────────────────────────────────────────
// Update — optimistic locking
// ─────────────────────────────────────────────────────────────────────────────

func (r *PortfolioRepository) Update(ctx context.Context, p *Portfolio) error {
	r.logger.Debug("PortfolioRepository.Update", "portfolio_id", p.ID, "version", p.Version)

	valJSON, _ := json.Marshal(p.TotalValue)
	metaJSON, _ := json.Marshal(p.Metadata)
	newVersion := p.Version + 1

	patentIDStrs := make([]string, len(p.PatentIDs))
	for i, id := range p.PatentIDs {
		patentIDStrs[i] = string(id)
	}

	tag, err := r.pool.Exec(ctx, `
		UPDATE portfolios SET
			name=$1, description=$2, owner_id=$3,
			patent_ids=$4, tags=$5, total_value=$6, status=$7,
			metadata=$8, updated_at=$9, version=$10
		WHERE id=$11 AND version=$12`,
		p.Name, p.Description, p.OwnerID,
		patentIDStrs, p.Tags, valJSON, p.Status,
		metaJSON, time.Now().UTC(), newVersion,
		p.ID, p.Version,
	)
	if err != nil {
		r.logger.Error("PortfolioRepository.Update", "error", err)
		return appErrors.Wrap(err, appErrors.CodeDBQueryError, "failed to update portfolio")
	}
	if tag.RowsAffected() == 0 {
		return appErrors.New(appErrors.CodeConflict, "optimistic lock conflict: portfolio version mismatch")
	}
	p.Version = newVersion
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Delete
// ─────────────────────────────────────────────────────────────────────────────

func (r *PortfolioRepository) Delete(ctx context.Context, id common.ID) error {
	r.logger.Debug("PortfolioRepository.Delete", "id", id)

	tag, err := r.pool.Exec(ctx, `DELETE FROM portfolios WHERE id = $1`, id)
	if err != nil {
		r.logger.Error("PortfolioRepository.Delete", "error", err)
		return appErrors.Wrap(err, appErrors.CodeDBQueryError, "failed to delete portfolio")
	}
	if tag.RowsAffected() == 0 {
		return appErrors.New(appErrors.CodeNotFound, "portfolio not found")
	}
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Count
// ─────────────────────────────────────────────────────────────────────────────

func (r *PortfolioRepository) Count(ctx context.Context) (int64, error) {
	r.logger.Debug("PortfolioRepository.Count")

	var count int64
	if err := r.pool.QueryRow(ctx, `SELECT COUNT(*) FROM portfolios`).Scan(&count); err != nil {
		r.logger.Error("PortfolioRepository.Count", "error", err)
		return 0, appErrors.Wrap(err, appErrors.CodeDBQueryError, "failed to count portfolios")
	}
	return count, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Internal scanners
// ─────────────────────────────────────────────────────────────────────────────

func (r *PortfolioRepository) scanPortfolio(row pgx.Row) (*Portfolio, error) {
	var p Portfolio
	var valJSON, metaJSON []byte
	var patentIDStrs []string

	err := row.Scan(
		&p.ID, &p.TenantID, &p.Name, &p.Description, &p.OwnerID,
		&patentIDStrs, &p.Tags, &valJSON, &p.Status,
		&metaJSON, &p.CreatedAt, &p.UpdatedAt, &p.CreatedBy, &p.Version,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, appErrors.New(appErrors.CodeNotFound, "portfolio not found")
		}
		r.logger.Error("scanPortfolio", "error", err)
		return nil, appErrors.Wrap(err, appErrors.CodeDBQueryError, "failed to scan portfolio")
	}

	p.PatentIDs = make([]common.ID, len(patentIDStrs))
	for i, s := range patentIDStrs {
		p.PatentIDs[i] = common.ID(s)
	}

	if len(valJSON) > 0 {
		var v ValuationResult
		if err := json.Unmarshal(valJSON, &v); err == nil {
			p.TotalValue = &v
		}
	}
	if len(metaJSON) > 0 {
		_ = json.Unmarshal(metaJSON, &p.Metadata)
	}
	return &p, nil
}

func (r *PortfolioRepository) scanPortfolios(rows pgx.Rows) ([]*Portfolio, error) {
	var portfolios []*Portfolio
	for rows.Next() {
		var p Portfolio
		var valJSON, metaJSON []byte
		var patentIDStrs []string

		err := rows.Scan(
			&p.ID, &p.TenantID, &p.Name, &p.Description, &p.OwnerID,
			&patentIDStrs, &p.Tags, &valJSON, &p.Status,
			&metaJSON, &p.CreatedAt, &p.UpdatedAt, &p.CreatedBy, &p.Version,
		)
		if err != nil {
			r.logger.Error("scanPortfolios", "error", err)
			return nil, appErrors.Wrap(err, appErrors.CodeDBQueryError, "failed to scan portfolio row")
		}

		p.PatentIDs = make([]common.ID, len(patentIDStrs))
		for i, s := range patentIDStrs {
			p.PatentIDs[i] = common.ID(s)
		}

		if len(valJSON) > 0 {
			var v ValuationResult
			if err := json.Unmarshal(valJSON, &v); err == nil {
				p.TotalValue = &v
			}
		}
		if len(metaJSON) > 0 {
			_ = json.Unmarshal(metaJSON, &p.Metadata)
		}
		portfolios = append(portfolios, &p)
	}
	if err := rows.Err(); err != nil {
		return nil, appErrors.Wrap(err, appErrors.CodeDBQueryError, "row iteration error")
	}
	return portfolios, nil
}

//Personal.AI order the ending
