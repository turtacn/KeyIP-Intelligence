package patent

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
	"github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
)

// Interfaces
type MarkushRepository interface {
	FindByPatentID(ctx context.Context, patentID string) ([]*MarkushStructure, error)
}

type EventBus interface {
	Publish(ctx context.Context, events ...common.DomainEvent) error
}

// SimilaritySearchRequest defines criteria for finding patents by molecular similarity.
type SimilaritySearchRequest struct {
	SMILES          string
	Threshold       float64
	MaxResults      int
	PatentOffices   []string
	Assignees       []string
	TechDomains     []string
	DateFrom        *time.Time
	DateTo          *time.Time
	ExcludePatents  []string
}

// SimilaritySearchResult represents a patent found via similarity search.
type SimilaritySearchResult struct {
	PatentNumber       string
	Title              string
	Assignee           string
	FilingDate         time.Time
	LegalStatus        string
	IPCCodes           []string
	MorganSimilarity   float64
	RDKitSimilarity    float64
	AtomPairSimilarity float64
}

// PatentDomainService defines the interface for patent domain operations.
type PatentDomainService interface {
	SearchBySimilarity(ctx context.Context, req *SimilaritySearchRequest) ([]*SimilaritySearchResult, error)
	GetPatentsByMoleculeID(ctx context.Context, moleculeID string) ([]*Patent, error)
	GetPatentByNumber(ctx context.Context, patentNumber string) (*Patent, error)
}

// PatentService provides domain services for patent management.
type PatentService struct {
	patentRepo  PatentRepository
	markushRepo MarkushRepository
	eventBus    EventBus
	logger      logging.Logger
}

func NewPatentService(
	patentRepo PatentRepository,
	markushRepo MarkushRepository,
	eventBus EventBus,
	logger logging.Logger,
) *PatentService {
	if patentRepo == nil {
		panic("patentRepo is required")
	}
	// Optional dependencies for now to allow partial testing
	return &PatentService{
		patentRepo:  patentRepo,
		markushRepo: markushRepo,
		eventBus:    eventBus,
		logger:      logger,
	}
}

func (s *PatentService) CreatePatent(
	ctx context.Context,
	patentNumber, title string,
	office PatentOffice,
	filingDate time.Time,
) (*Patent, error) {
	exists, err := s.patentRepo.GetByPatentNumber(ctx, patentNumber)
	if err == nil && exists != nil {
		return nil, errors.New(errors.ErrCodePatentAlreadyExists, fmt.Sprintf("patent %s already exists", patentNumber))
	}

	p, err := NewPatent(patentNumber, title, office, filingDate)
	if err != nil {
		return nil, err
	}

	if err := s.patentRepo.Create(ctx, p); err != nil {
		return nil, err
	}

	s.publishEvents(ctx, p, NewPatentFiledEvent(p))
	return p, nil
}

func (s *PatentService) GetPatent(ctx context.Context, id string) (*Patent, error) {
	uid, err := uuid.Parse(id)
	if err != nil {
		return nil, errors.New(errors.ErrCodeValidation, "invalid UUID")
	}
	p, err := s.patentRepo.GetByID(ctx, uid)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, errors.New(errors.ErrCodePatentNotFound, fmt.Sprintf("patent %s not found", id))
	}
	return p, nil
}

func (s *PatentService) GetPatentByNumber(ctx context.Context, patentNumber string) (*Patent, error) {
	return s.patentRepo.GetByPatentNumber(ctx, patentNumber)
}

func (s *PatentService) SearchPatents(ctx context.Context, criteria SearchQuery) (*SearchResult, error) {
	return s.patentRepo.Search(ctx, criteria)
}

// StatusTransitionParams carries parameters for status changes.
type StatusTransitionParams struct {
	PublicationDate *time.Time
	GrantDate       *time.Time
	ExpiryDate      *time.Time
	Reason          string
}

func (s *PatentService) UpdatePatentStatus(
	ctx context.Context,
	id string,
	targetStatus PatentStatus,
	params StatusTransitionParams,
) (*Patent, error) {
	p, err := s.GetPatent(ctx, id)
	if err != nil {
		return nil, err
	}

	var event common.DomainEvent
	switch targetStatus {
	case PatentStatusPublished:
		if params.PublicationDate == nil {
			return nil, errors.New(errors.ErrCodeValidation, "publication date is required")
		}
		if err := p.Publish(*params.PublicationDate); err != nil {
			return nil, err
		}
		event = NewPatentPublishedEvent(p)
	case PatentStatusUnderExamination:
		if err := p.EnterExamination(); err != nil {
			return nil, err
		}
		// event = NewPatentExaminationStartedEvent(p) // Not defined yet
	case PatentStatusGranted:
		if params.GrantDate == nil || params.ExpiryDate == nil {
			return nil, errors.New(errors.ErrCodeValidation, "grant and expiry dates are required")
		}
		if err := p.Grant(*params.GrantDate, *params.ExpiryDate); err != nil {
			return nil, err
		}
		event = NewPatentGrantedEvent(p)
	// ... other statuses
	}

	if err := s.patentRepo.Update(ctx, p); err != nil {
		return nil, err
	}

	if event != nil {
		s.publishEvents(ctx, p, event)
	}
	return p, nil
}

func (s *PatentService) publishEvents(ctx context.Context, p *Patent, events ...common.DomainEvent) {
	if s.eventBus == nil {
		return
	}
	if err := s.eventBus.Publish(ctx, events...); err != nil {
		s.logger.Error("failed to publish events", logging.Err(err))
	}
}

// SearchBySimilarity finds patents containing molecules similar to the query.
func (s *PatentService) SearchBySimilarity(ctx context.Context, req *SimilaritySearchRequest) ([]*SimilaritySearchResult, error) {
	// Dummy implementation to satisfy the interface.
	// In a real implementation, this would query a specialized index or cross-reference molecule similarity.
	return []*SimilaritySearchResult{}, nil
}

// GetPatentsByMoleculeID retrieves patents associated with a molecule ID.
func (s *PatentService) GetPatentsByMoleculeID(ctx context.Context, moleculeID string) ([]*Patent, error) {
	return s.patentRepo.FindByMoleculeID(ctx, moleculeID)
}

// Type aliases for backward compatibility

// Service is an alias for PatentService for backward compatibility with apiserver.
type Service = PatentService

// NewService creates a new patent service. Alias for NewPatentService.
func NewService(repo PatentRepository, logger logging.Logger) *PatentService {
	return NewPatentService(repo, nil, nil, logger)
}

//Personal.AI order the ending
