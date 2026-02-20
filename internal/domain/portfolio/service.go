// Package portfolio provides the domain service layer for managing patent
// portfolios, including creation, modification, and valuation orchestration.
package portfolio

import (
	"context"
	"fmt"

	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
	"github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
)

// Service orchestrates portfolio domain operations by coordinating the
// repository, valuator, and logging infrastructure.
type Service struct {
	repo     Repository
	valuator Valuator
	logger   logging.Logger
}

// NewService constructs a new portfolio domain service.
func NewService(repo Repository, valuator Valuator, logger logging.Logger) *Service {
	return &Service{
		repo:     repo,
		valuator: valuator,
		logger:   logger,
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// Portfolio CRUD operations
// ─────────────────────────────────────────────────────────────────────────────

// CreatePortfolio constructs a new Portfolio aggregate and persists it.
func (s *Service) CreatePortfolio(
	ctx context.Context,
	name, description string,
	ownerID common.UserID,
) (*Portfolio, error) {
	p, err := NewPortfolio(name, description, ownerID)
	if err != nil {
		return nil, errors.Wrap(err, errors.CodeInvalidParam, "failed to create portfolio entity")
	}

	if err := s.repo.Save(ctx, p); err != nil {
		s.logger.Error("failed to save portfolio",
			logging.Err(err),
			logging.String("portfolio_id", string(p.ID)))
		return nil, errors.Wrap(err, errors.CodeDBConnectionError, "failed to persist portfolio")
	}

	s.logger.Info("portfolio created",
		logging.String("portfolio_id", string(p.ID)),
		logging.String("owner_id", string(ownerID)))
	return p, nil
}

// GetPortfolio retrieves a Portfolio by ID.
func (s *Service) GetPortfolio(ctx context.Context, id common.ID) (*Portfolio, error) {
	p, err := s.repo.FindByID(ctx, id)
	if err != nil {
		s.logger.Warn("portfolio not found", logging.String("portfolio_id", string(id)))
		return nil, errors.Wrap(err, errors.CodePortfolioNotFound,
			fmt.Sprintf("portfolio %s not found", id))
	}
	return p, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Patent management operations
// ─────────────────────────────────────────────────────────────────────────────

// AddPatentToPortfolio appends a patent to a portfolio's member list.
func (s *Service) AddPatentToPortfolio(
	ctx context.Context,
	portfolioID, patentID common.ID,
) error {
	p, err := s.GetPortfolio(ctx, portfolioID)
	if err != nil {
		return err
	}

	if err := p.AddPatent(patentID); err != nil {
		s.logger.Warn("failed to add patent to portfolio",
			logging.String("portfolio_id", string(portfolioID)),
			logging.String("patent_id", string(patentID)),
			logging.Err(err))
		return err
	}

	if err := s.repo.Update(ctx, p); err != nil {
		s.logger.Error("failed to update portfolio after adding patent",
			logging.String("portfolio_id", string(portfolioID)),
			logging.Err(err))
		return errors.Wrap(err, errors.CodeDBConnectionError, "failed to update portfolio")
	}

	s.logger.Info("patent added to portfolio",
		logging.String("portfolio_id", string(portfolioID)),
		logging.String("patent_id", string(patentID)))
	return nil
}

// RemovePatentFromPortfolio removes a patent from a portfolio's member list.
func (s *Service) RemovePatentFromPortfolio(
	ctx context.Context,
	portfolioID, patentID common.ID,
) error {
	p, err := s.GetPortfolio(ctx, portfolioID)
	if err != nil {
		return err
	}

	if err := p.RemovePatent(patentID); err != nil {
		s.logger.Warn("failed to remove patent from portfolio",
			logging.String("portfolio_id", string(portfolioID)),
			logging.String("patent_id", string(patentID)),
			logging.Err(err))
		return err
	}

	if err := s.repo.Update(ctx, p); err != nil {
		s.logger.Error("failed to update portfolio after removing patent",
			logging.String("portfolio_id", string(portfolioID)),
			logging.Err(err))
		return errors.Wrap(err, errors.CodeDBConnectionError, "failed to update portfolio")
	}

	s.logger.Info("patent removed from portfolio",
		logging.String("portfolio_id", string(portfolioID)),
		logging.String("patent_id", string(patentID)))
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Valuation operations
// ─────────────────────────────────────────────────────────────────────────────

// ValuatePortfolio computes a fresh valuation for the given portfolio using
// the provided factors and updates the cached TotalValue field.
func (s *Service) ValuatePortfolio(
	ctx context.Context,
	portfolioID common.ID,
	factors map[common.ID]ValuationFactors,
) (*ValuationResult, error) {
	p, err := s.GetPortfolio(ctx, portfolioID)
	if err != nil {
		return nil, err
	}

	// Verify that factors are provided for all patents in the portfolio.
	if len(factors) != p.Size() {
		return nil, errors.InvalidParam(
			fmt.Sprintf("factor count (%d) does not match portfolio size (%d)",
				len(factors), p.Size()))
	}

	result, err := CalculatePortfolioValuation(ctx, s.valuator, factors)
	if err != nil {
		s.logger.Error("failed to calculate portfolio valuation",
			logging.String("portfolio_id", string(portfolioID)),
			logging.Err(err))
		return nil, errors.Wrap(err, errors.CodeInternal, "valuation computation failed")
	}

	p.SetValuation(*result)

	if err := s.repo.Update(ctx, p); err != nil {
		s.logger.Error("failed to update portfolio with valuation result",
			logging.String("portfolio_id", string(portfolioID)),
			logging.Err(err))
		return nil, errors.Wrap(err, errors.CodeDBConnectionError, "failed to persist valuation")
	}

	s.logger.Info("portfolio valuation completed",
		logging.String("portfolio_id", string(portfolioID)),
		logging.Float64("total_value", result.TotalValue),
		logging.String("method", result.Method))

	return result, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// Query operations
// ─────────────────────────────────────────────────────────────────────────────

// GetUserPortfolios retrieves all portfolios owned by the specified user,
// paginated.
func (s *Service) GetUserPortfolios(
	ctx context.Context,
	ownerID common.UserID,
	page common.PageRequest,
) (*common.PageResponse[*Portfolio], error) {
	if err := page.Validate(); err != nil {
		return nil, errors.Wrap(err, errors.CodeInvalidParam, "invalid pagination parameters")
	}

	resp, err := s.repo.FindByOwner(ctx, ownerID, page)
	if err != nil {
		s.logger.Error("failed to retrieve user portfolios",
			logging.String("owner_id", string(ownerID)),
			logging.Err(err))
		return nil, errors.Wrap(err, errors.CodeDBConnectionError, "failed to query portfolios")
	}

	s.logger.Debug("retrieved user portfolios",
		logging.String("owner_id", string(ownerID)),
		logging.Int("count", len(resp.Items)),
		logging.Int64("total", resp.Total))

	return resp, nil
}

