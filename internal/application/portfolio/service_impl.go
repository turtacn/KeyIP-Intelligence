package portfolio

import (
	"context"
	"time"

	"github.com/google/uuid"
	domain "github.com/turtacn/KeyIP-Intelligence/internal/domain/portfolio"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

type serviceImpl struct {
	repo   domain.PortfolioRepository
	logger logging.Logger
}

func NewService(repo domain.PortfolioRepository, logger logging.Logger) Service {
	return &serviceImpl{repo: repo, logger: logger}
}

func (s *serviceImpl) Create(ctx context.Context, input *CreateInput) (*Portfolio, error) {
	if input.Name == "" {
		return nil, errors.NewValidationError("name", "name is required")
	}
	p := &domain.Portfolio{
		ID:              uuid.New().String(),
		Name:            input.Name,
		Description:     input.Description,
		OwnerID:         input.UserID,
		TechDomains:     input.Tags,
		Status:          domain.StatusActive,
		TargetJurisdictions: []string{},
	}
	if err := s.repo.Create(ctx, p); err != nil {
		return nil, err
	}
	if len(input.PatentIDs) > 0 {
		_ = s.repo.BatchAddPatents(ctx, p.ID, input.PatentIDs, "member", input.UserID)
	}
	return domainToDTO(p), nil
}

func (s *serviceImpl) GetByID(ctx context.Context, id string) (*Portfolio, error) {
	p, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return domainToDTO(p), nil
}

func (s *serviceImpl) List(ctx context.Context, input *ListInput) (*ListResult, error) {
	if input.Page <= 0 { input.Page = 1 }
	if input.PageSize <= 0 { input.PageSize = 20 }

	ps, total, err := s.repo.List(ctx, input.UserID, domain.WithLimit(input.PageSize), domain.WithOffset((input.Page-1)*input.PageSize))
	if err != nil {
		return nil, err
	}
	dtos := make([]*Portfolio, 0, len(ps))
	for _, p := range ps {
		dtos = append(dtos, domainToDTO(p))
	}
	return &ListResult{Portfolios: dtos, Total: total, Page: input.Page, PageSize: input.PageSize}, nil
}

func (s *serviceImpl) Update(ctx context.Context, input *UpdateInput) (*Portfolio, error) {
	p, err := s.repo.GetByID(ctx, input.ID)
	if err != nil {
		return nil, err
	}
	if input.Name != nil { p.Name = *input.Name }
	if input.Description != nil { p.Description = *input.Description }
	if input.Tags != nil { p.TechDomains = input.Tags }
	if err := s.repo.Update(ctx, p); err != nil {
		return nil, err
	}
	return domainToDTO(p), nil
}

func (s *serviceImpl) Delete(ctx context.Context, id string, userID string) error {
	return s.repo.SoftDelete(ctx, id)
}

func (s *serviceImpl) AddPatents(ctx context.Context, id string, patentIDs []string, userID string) error {
	return s.repo.BatchAddPatents(ctx, id, patentIDs, "member", userID)
}

func (s *serviceImpl) RemovePatents(ctx context.Context, id string, patentIDs []string, userID string) error {
	for _, pid := range patentIDs {
		if err := s.repo.RemovePatent(ctx, id, pid); err != nil {
			return err
		}
	}
	return nil
}

func (s *serviceImpl) GetAnalysis(ctx context.Context, id string) (*PortfolioAnalysis, error) {
	p, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	ps, _, err := s.repo.GetPatents(ctx, id, nil, 0, 100)
	if err != nil {
		ps = nil
	}
	analysis := &PortfolioAnalysis{
		PortfolioID:     id,
		TotalPatents:    len(ps),
		ByJurisdiction:  map[string]int{},
		ByStatus:        map[string]int{},
		ByYear:          map[string]int{},
		TopIPCCodes:     []IPCCount{},
		Recommendations: []string{},
	}
	for _, pat := range ps {
		analysis.ByJurisdiction[pat.Jurisdiction]++
		analysis.ByStatus[pat.Status.String()]++
		if pat.FilingDate != nil {
			analysis.ByYear[pat.FilingDate.Format("2006")]++
		}
	}
	if p.Metadata != nil {
		if v, ok := p.Metadata["total_value"].(float64); ok {
			analysis.TotalValue = v
		}
	}
	if len(ps) < 5 {
		analysis.Recommendations = append(analysis.Recommendations, "Consider expanding portfolio coverage")
	}
	return analysis, nil
}

func domainToDTO(p *domain.Portfolio) *Portfolio {
	return &Portfolio{
		ID:          p.ID,
		Name:        p.Name,
		Description: p.Description,
		Tags:        p.TechDomains,
		PatentCount: p.PatentCount,
		CreatedAt:   p.CreatedAt,
		UpdatedAt:   p.UpdatedAt,
	}
}

var _ Service = (*serviceImpl)(nil)
var _ = time.Now
