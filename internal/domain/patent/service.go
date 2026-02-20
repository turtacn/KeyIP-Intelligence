package patent

import (
	"context"
	"time"

	pkgerrors "github.com/turtacn/KeyIP-Intelligence/pkg/errors"
	common "github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
	ptypes "github.com/turtacn/KeyIP-Intelligence/pkg/types/patent"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
)

// ─────────────────────────────────────────────────────────────────────────────
// Service — patent domain service
// ─────────────────────────────────────────────────────────────────────────────

// Service orchestrates patent domain operations by coordinating the Patent
// aggregate, its value objects, and the Repository port.
//
// Domain logic (state-machine invariants, business rules) lives in the
// aggregate and value objects.  Service methods are intentionally thin:
// they retrieve aggregates, invoke domain logic, and persist the result.
//
// Service is consumed by:
//   - internal/application/patent_mining  (ingestion workflows)
//   - internal/application/infringement   (FTO and infringement analysis)
//   - internal/interfaces/http/handlers   (REST API handlers)
type Service struct {
	repo   Repository
	logger logging.Logger
}

// NewService creates a new patent domain Service with the required dependencies.
//
// Parameters:
//   - repo   : the Repository port (injected by the application bootstrap)
//   - logger : structured logger (use logging.NewNopLogger() in tests)
func NewService(repo Repository, logger logging.Logger) *Service {
	return &Service{
		repo:   repo,
		logger: logger,
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// CreatePatent
// ─────────────────────────────────────────────────────────────────────────────

// CreatePatent creates a new Patent aggregate and persists it via the repository.
// It delegates all structural validation to the NewPatent factory function.
//
// On success the method returns the fully constructed Patent with its assigned
// platform ID.  On failure it returns a typed AppError from pkg/errors.
func (s *Service) CreatePatent(
	ctx context.Context,
	number string,
	title string,
	abstract string,
	jurisdiction ptypes.JurisdictionCode,
	applicants []string,
	inventors []string,
	filingDate time.Time,
) (*Patent, error) {
	s.logger.Info("creating patent",
		logging.String("number", number),
		logging.String("jurisdiction", string(jurisdiction)))

	applicant := ""
	if len(applicants) > 0 {
		applicant = applicants[0]
	}

	p, err := NewPatent(number, title, abstract, applicant, jurisdiction, filingDate)
	if err != nil {
		return nil, pkgerrors.Wrap(err, pkgerrors.CodeInvalidParam, "invalid patent parameters")
	}
	p.Inventors = inventors

	if err = s.repo.Save(ctx, p); err != nil {
		s.logger.Error("failed to save patent",
			logging.Err(err),
			logging.String("number", number))
		return nil, pkgerrors.Wrap(err, pkgerrors.CodeDBConnectionError, "failed to persist patent")
	}

	s.logger.Info("patent created",
		logging.String("id", string(p.BaseEntity.ID)),
		logging.String("number", number))
	return p, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// GetPatent / GetPatentByNumber
// ─────────────────────────────────────────────────────────────────────────────

// GetPatent retrieves a Patent aggregate by its platform-internal UUID.
// Returns CodeNotFound when the patent does not exist.
func (s *Service) GetPatent(ctx context.Context, id common.ID) (*Patent, error) {
	p, err := s.repo.FindByID(ctx, id)
	if err != nil {
		return nil, err // already typed by repository
	}
	return p, nil
}

// GetPatentByNumber retrieves a Patent aggregate by its official publication
// number (e.g., "CN202310001234A").
// Returns CodeNotFound when no matching patent exists.
func (s *Service) GetPatentByNumber(ctx context.Context, number string) (*Patent, error) {
	if number == "" {
		return nil, pkgerrors.InvalidParam("patent number must not be empty")
	}
	p, err := s.repo.FindByNumber(ctx, number)
	if err != nil {
		return nil, err
	}
	return p, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// SearchPatents
// ─────────────────────────────────────────────────────────────────────────────

// SearchPatents delegates a structured search request to the repository and
// returns a paginated response of PatentDTOs.
func (s *Service) SearchPatents(
	ctx context.Context,
	req ptypes.PatentSearchRequest,
) (*ptypes.PatentSearchResponse, error) {
	if err := req.PageRequest.Validate(); err != nil {
		return nil, pkgerrors.InvalidParam("invalid pagination parameters").WithCause(err)
	}

	resp, err := s.repo.Search(ctx, req)
	if err != nil {
		s.logger.Error("patent search failed", logging.Err(err))
		return nil, pkgerrors.Wrap(err, pkgerrors.CodeSearchError, "patent search failed")
	}
	return resp, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// AddClaimToPatent
// ─────────────────────────────────────────────────────────────────────────────

// AddClaimToPatent appends the given Claim value object to the Patent identified
// by patentID and persists the updated aggregate.
//
// The method follows the pattern: Load → Mutate → Save, which respects the
// aggregate boundary and ensures all domain invariants are enforced.
func (s *Service) AddClaimToPatent(
	ctx context.Context,
	patentID common.ID,
	claim Claim,
) error {
	p, err := s.repo.FindByID(ctx, patentID)
	if err != nil {
		return err
	}

	if err = p.AddClaim(claim); err != nil {
		return pkgerrors.Wrap(err, pkgerrors.CodeInvalidParam, "failed to add claim to patent")
	}

	p.Version++
	if err = s.repo.Update(ctx, p); err != nil {
		s.logger.Error("failed to update patent after adding claim",
			logging.String("patentID", string(patentID)),
			logging.Int("claimNumber", claim.Number),
			logging.Err(err))
		return pkgerrors.Wrap(err, pkgerrors.CodeDBConnectionError, "failed to persist claim")
	}

	s.logger.Info("claim added to patent",
		logging.String("patentID", string(patentID)),
		logging.Int("claimNumber", claim.Number))
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// AddMarkushToPatent
// ─────────────────────────────────────────────────────────────────────────────

// AddMarkushToPatent appends the given Markush value object to the Patent
// identified by patentID and persists the updated aggregate.
func (s *Service) AddMarkushToPatent(
	ctx context.Context,
	patentID common.ID,
	markush Markush,
) error {
	p, err := s.repo.FindByID(ctx, patentID)
	if err != nil {
		return err
	}

	if err = p.AddMarkush(markush); err != nil {
		return pkgerrors.Wrap(err, pkgerrors.CodeInvalidParam, "failed to add Markush to patent")
	}

	p.Version++
	if err = s.repo.Update(ctx, p); err != nil {
		s.logger.Error("failed to update patent after adding Markush",
			logging.String("patentID", string(patentID)),
			logging.String("markushID", string(markush.ID)),
			logging.Err(err))
		return pkgerrors.Wrap(err, pkgerrors.CodeDBConnectionError, "failed to persist Markush")
	}

	s.logger.Info("Markush added to patent",
		logging.String("patentID", string(patentID)),
		logging.String("markushID", string(markush.ID)))
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// UpdatePatentStatus
// ─────────────────────────────────────────────────────────────────────────────

// UpdatePatentStatus transitions the Patent identified by patentID to the new
// lifecycle status.  The transition is validated by the Patent aggregate's
// state machine; invalid transitions return CodeInvalidParam.
func (s *Service) UpdatePatentStatus(
	ctx context.Context,
	patentID common.ID,
	status ptypes.PatentStatus,
) error {
	p, err := s.repo.FindByID(ctx, patentID)
	if err != nil {
		return err
	}

	oldStatus := p.Status
	if err = p.UpdateStatus(status); err != nil {
		return pkgerrors.Wrap(err, pkgerrors.CodeInvalidParam, "invalid status transition")
	}

	p.Version++
	if err = s.repo.Update(ctx, p); err != nil {
		s.logger.Error("failed to persist status transition",
			logging.String("patentID", string(patentID)),
			logging.String("oldStatus", string(oldStatus)),
			logging.String("newStatus", string(status)),
			logging.Err(err))
		return pkgerrors.Wrap(err, pkgerrors.CodeDBConnectionError, "failed to persist status change")
	}

	s.logger.Info("patent status updated",
		logging.String("patentID", string(patentID)),
		logging.String("oldStatus", string(oldStatus)),
		logging.String("newStatus", string(status)))
	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// GetPatentFamily
// ─────────────────────────────────────────────────────────────────────────────

// GetPatentFamily returns all patents belonging to the specified family ID.
// Returns an empty slice (no error) if the family contains no patents.
func (s *Service) GetPatentFamily(ctx context.Context, familyID string) ([]*Patent, error) {
	if familyID == "" {
		return nil, pkgerrors.InvalidParam("familyID must not be empty")
	}
	patents, err := s.repo.FindByFamilyID(ctx, familyID)
	if err != nil {
		return nil, pkgerrors.Wrap(err, pkgerrors.CodeDBConnectionError, "failed to query patent family")
	}
	return patents, nil
}

// ─────────────────────────────────────────────────────────────────────────────
// FindExpiringPatents
// ─────────────────────────────────────────────────────────────────────────────

// FindExpiringPatents returns all granted patents whose expiry date falls
// within the next withinDays calendar days.
//
// withinDays must be > 0; returns CodeInvalidParam otherwise.
func (s *Service) FindExpiringPatents(ctx context.Context, withinDays int) ([]*Patent, error) {
	if withinDays <= 0 {
		return nil, pkgerrors.InvalidParam("withinDays must be greater than zero").
			WithDetail("withinDays=" + itoa(withinDays))
	}

	threshold := time.Now().UTC().Add(time.Duration(withinDays) * 24 * time.Hour)
	patents, err := s.repo.FindExpiring(ctx, threshold)
	if err != nil {
		return nil, pkgerrors.Wrap(err, pkgerrors.CodeDBConnectionError, "failed to query expiring patents")
	}

	s.logger.Info("found expiring patents",
		logging.Int("withinDays", withinDays),
		logging.Int("count", len(patents)))
	return patents, nil
}

