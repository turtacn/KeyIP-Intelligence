package patent

import (
	"context"
	"fmt"
	"time"

	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// PatentDomainService alias for backward compatibility or interface usage
type PatentDomainService = *PatentService

// Logger interface for domain logging.
type Logger interface {
	Debug(msg string, keysAndValues ...interface{})
	Info(msg string, keysAndValues ...interface{})
	Warn(msg string, keysAndValues ...interface{})
	Error(msg string, keysAndValues ...interface{})
}

// PatentService provides patent domain logic.
type PatentService struct {
	patentRepo  PatentRepository
	markushRepo MarkushRepository
	eventBus    EventBus
	logger      Logger
	matcher     MarkushMatcher
}

// NewPatentService creates a new PatentService.
func NewPatentService(
	patentRepo PatentRepository,
	markushRepo MarkushRepository,
	eventBus EventBus,
	logger Logger,
	matcher MarkushMatcher,
) (*PatentService, error) {
	if patentRepo == nil {
		return nil, errors.NewValidation("patent repository cannot be nil")
	}
	if markushRepo == nil {
		return nil, errors.NewValidation("markush repository cannot be nil")
	}

	return &PatentService{
		patentRepo:  patentRepo,
		markushRepo: markushRepo,
		eventBus:    eventBus,
		logger:      logger,
		matcher:     matcher,
	}, nil
}

func (s *PatentService) logInfo(msg string, keysAndValues ...interface{}) {
	if s.logger != nil {
		s.logger.Info(msg, keysAndValues...)
	}
}

func (s *PatentService) logError(msg string, keysAndValues ...interface{}) {
	if s.logger != nil {
		s.logger.Error(msg, keysAndValues...)
	}
}

func (s *PatentService) CreatePatent(ctx context.Context, patentNumber, title string, office PatentOffice, filingDate time.Time) (*Patent, error) {
	exists, err := s.patentRepo.Exists(ctx, patentNumber)
	if err != nil {
		return nil, errors.NewInternal("failed to check patent existence: %v", err)
	}
	if exists {
		return nil, errors.NewValidation(fmt.Sprintf("patent already exists: %s", patentNumber))
	}

	patent, err := NewPatent(patentNumber, title, office, filingDate)
	if err != nil {
		return nil, err
	}

	patent.addEvent(NewPatentCreatedEvent(patent))

	if err := s.patentRepo.Save(ctx, patent); err != nil {
		return nil, errors.NewInternal("failed to save patent: %v", err)
	}

	s.publishEvents(ctx, patent)
	s.logInfo("patent created", "patent_number", patentNumber)

	return patent, nil
}

func (s *PatentService) GetPatent(ctx context.Context, id string) (*Patent, error) {
	patent, err := s.patentRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if patent == nil {
		return nil, errors.NewNotFound("patent not found: %s", id)
	}
	return patent, nil
}

func (s *PatentService) GetPatentByNumber(ctx context.Context, patentNumber string) (*Patent, error) {
	patent, err := s.patentRepo.FindByPatentNumber(ctx, patentNumber)
	if err != nil {
		return nil, err
	}
	if patent == nil {
		return nil, errors.NewNotFound("patent not found: %s", patentNumber)
	}
	return patent, nil
}

func (s *PatentService) SearchPatents(ctx context.Context, criteria PatentSearchCriteria) (*PatentSearchResult, error) {
	if err := criteria.Validate(); err != nil {
		return nil, err
	}
	return s.patentRepo.Search(ctx, criteria)
}

func (s *PatentService) SearchBySimilarity(ctx context.Context, req *SimilaritySearchRequest) ([]*PatentSearchResultWithSimilarity, error) {
	// Delegate to repository which should support vector search
	return s.patentRepo.SearchBySimilarity(ctx, req)
}

func (s *PatentService) GetPatentsByMoleculeID(ctx context.Context, moleculeID string) ([]*Patent, error) {
	return s.patentRepo.FindByMoleculeID(ctx, moleculeID)
}

// StatusTransitionParams holds parameters for status transitions.
type StatusTransitionParams struct {
	PublicationDate *time.Time
	GrantDate       *time.Time
	ExpiryDate      *time.Time
	Reason          string
}

func (s *PatentService) UpdatePatentStatus(ctx context.Context, id string, targetStatus PatentStatus, params StatusTransitionParams) (*Patent, error) {
	patent, err := s.GetPatent(ctx, id)
	if err != nil {
		return nil, err
	}

	switch targetStatus {
	case PatentStatusPublished:
		if params.PublicationDate == nil {
			return nil, errors.NewValidation("publication date required")
		}
		if err := patent.Publish(*params.PublicationDate); err != nil {
			return nil, err
		}
	case PatentStatusUnderExamination:
		if err := patent.EnterExamination(); err != nil {
			return nil, err
		}
	case PatentStatusGranted:
		if params.GrantDate == nil || params.ExpiryDate == nil {
			return nil, errors.NewValidation("grant date and expiry date required")
		}
		if err := patent.Grant(*params.GrantDate, *params.ExpiryDate); err != nil {
			return nil, err
		}
	case PatentStatusRejected:
		if err := patent.Reject(); err != nil {
			return nil, err
		}
	case PatentStatusWithdrawn:
		if err := patent.Withdraw(); err != nil {
			return nil, err
		}
	case PatentStatusExpired:
		if err := patent.Expire(); err != nil {
			return nil, err
		}
	case PatentStatusInvalidated:
		if err := patent.Invalidate(); err != nil {
			return nil, err
		}
	case PatentStatusLapsed:
		if err := patent.Lapse(); err != nil {
			return nil, err
		}
	default:
		return nil, errors.NewValidation(fmt.Sprintf("unsupported status transition to %s", targetStatus))
	}

	if err := s.patentRepo.Save(ctx, patent); err != nil {
		return nil, errors.NewInternal("failed to update patent status: %v", err)
	}

	s.publishEvents(ctx, patent)
	s.logInfo("patent status updated", "patent_id", id, "status", targetStatus)

	return patent, nil
}

func (s *PatentService) SetPatentClaims(ctx context.Context, patentID string, claims ClaimSet) (*Patent, error) {
	patent, err := s.GetPatent(ctx, patentID)
	if err != nil {
		return nil, err
	}

	if err := patent.SetClaims(claims); err != nil {
		return nil, err
	}

	if err := s.patentRepo.Save(ctx, patent); err != nil {
		return nil, errors.NewInternal("failed to save patent claims: %v", err)
	}

	s.publishEvents(ctx, patent)
	return patent, nil
}

func (s *PatentService) LinkMolecule(ctx context.Context, patentID, moleculeID string) error {
	patent, err := s.GetPatent(ctx, patentID)
	if err != nil {
		return err
	}

	if err := patent.AddMolecule(moleculeID); err != nil {
		return err
	}

	if err := s.patentRepo.Save(ctx, patent); err != nil {
		return errors.NewInternal("failed to save patent: %v", err)
	}

	s.publishEvents(ctx, patent)
	return nil
}

func (s *PatentService) UnlinkMolecule(ctx context.Context, patentID, moleculeID string) error {
	patent, err := s.GetPatent(ctx, patentID)
	if err != nil {
		return err
	}

	if err := patent.RemoveMolecule(moleculeID); err != nil {
		return err
	}

	if err := s.patentRepo.Save(ctx, patent); err != nil {
		return errors.NewInternal("failed to save patent: %v", err)
	}

	s.publishEvents(ctx, patent)
	return nil
}

func (s *PatentService) AddCitation(ctx context.Context, patentID, citedPatentNumber, direction string) error {
	patent, err := s.GetPatent(ctx, patentID)
	if err != nil {
		return err
	}

	if direction == "forward" {
		if err := patent.AddCitation(citedPatentNumber); err != nil {
			return err
		}
	} else if direction == "backward" {
		if err := patent.AddCitedBy(citedPatentNumber); err != nil {
			return err
		}
	} else {
		return errors.NewValidation("invalid direction, must be forward or backward")
	}

	if err := s.patentRepo.Save(ctx, patent); err != nil {
		return errors.NewInternal("failed to save patent: %v", err)
	}

	s.publishEvents(ctx, patent)
	return nil
}

func (s *PatentService) AnalyzeMarkushCoverage(ctx context.Context, patentID string, moleculeSMILES []string) (*MarkushCoverageAnalysis, error) {
	markushList, err := s.markushRepo.FindByPatentID(ctx, patentID)
	if err != nil {
		return nil, errors.NewInternal("failed to load markush structures: %v", err)
	}

	if len(markushList) == 0 {
		return &MarkushCoverageAnalysis{
			MarkushID:         "",
			TotalCombinations: 0,
			SampledMolecules:  len(moleculeSMILES),
			MatchedMolecules:  0,
			CoverageRate:      0,
			AnalyzedAt:        time.Now().UTC(),
		}, nil
	}

	ms := markushList[0]

	matchedCount := 0
	positionDiversity := make(map[string]int)

	for _, smiles := range moleculeSMILES {
		matched, _, err := ms.MatchesMolecule(smiles, s.matcher)
		if err != nil {
			s.logError("markush matching failed", "smiles", smiles, "error", err)
			continue
		}
		if matched {
			matchedCount++
		}
	}

	return &MarkushCoverageAnalysis{
		MarkushID:         ms.ID,
		TotalCombinations: ms.TotalCombinations,
		SampledMolecules:  len(moleculeSMILES),
		MatchedMolecules:  matchedCount,
		CoverageRate:      float64(matchedCount) / float64(len(moleculeSMILES)),
		PositionDiversity: positionDiversity,
		AnalyzedAt:        time.Now().UTC(),
	}, nil
}

func (s *PatentService) FindRelatedPatents(ctx context.Context, patentID string) ([]*Patent, error) {
	patent, err := s.GetPatent(ctx, patentID)
	if err != nil {
		return nil, err
	}

	relatedMap := make(map[string]*Patent)

	if patent.FamilyID != "" {
		family, err := s.patentRepo.FindByFamilyID(ctx, patent.FamilyID)
		if err == nil {
			for _, p := range family {
				if p.ID != patent.ID {
					relatedMap[p.ID] = p
				}
			}
		}
	}

	if len(patent.Cites) > 0 {
		cites, err := s.patentRepo.FindByPatentNumbers(ctx, patent.Cites)
		if err == nil {
			for _, p := range cites {
				relatedMap[p.ID] = p
			}
		}
	}
	if len(patent.CitedBy) > 0 {
		citedBy, err := s.patentRepo.FindByPatentNumbers(ctx, patent.CitedBy)
		if err == nil {
			for _, p := range citedBy {
				relatedMap[p.ID] = p
			}
		}
	}

	for _, molID := range patent.MoleculeIDs {
		mols, err := s.patentRepo.FindByMoleculeID(ctx, molID)
		if err == nil {
			for _, p := range mols {
				if p.ID != patent.ID {
					relatedMap[p.ID] = p
				}
			}
		}
	}

	result := make([]*Patent, 0, len(relatedMap))
	for _, p := range relatedMap {
		result = append(result, p)
	}

	return result, nil
}

// PatentStatistics aggregates stats.
type PatentStatistics struct {
	TotalCount         int64
	ByStatus           map[PatentStatus]int64
	ByOffice           map[PatentOffice]int64
	ByIPCSection       map[string]int64
	ByFilingYear       map[int]int64
	ByGrantYear        map[int]int64
	ActiveCount        int64
	ExpiringWithinYear int64
	WithMarkushCount   int64
	GeneratedAt        time.Time
}

func (s *PatentService) GetPatentStatistics(ctx context.Context) (*PatentStatistics, error) {
	stats := &PatentStatistics{
		GeneratedAt: time.Now().UTC(),
	}

	var err error
	stats.ByStatus, err = s.patentRepo.CountByStatus(ctx)
	if err != nil {
		return nil, err
	}

	stats.TotalCount = 0
	for _, count := range stats.ByStatus {
		stats.TotalCount += count
	}

	stats.ByOffice, err = s.patentRepo.CountByOffice(ctx)
	if err != nil {
		return nil, err
	}

	stats.ByIPCSection, err = s.patentRepo.CountByIPCSection(ctx)
	if err != nil {
		return nil, err
	}

	stats.ActiveCount = stats.ByStatus[PatentStatusFiled] +
						stats.ByStatus[PatentStatusPublished] +
						stats.ByStatus[PatentStatusUnderExamination] +
						stats.ByStatus[PatentStatusGranted]

	nextYear := time.Now().AddDate(1, 0, 0)
	expiring, err := s.patentRepo.FindExpiringBefore(ctx, nextYear)
	if err == nil {
		stats.ExpiringWithinYear = int64(len(expiring))
	}

	return stats, nil
}

type BatchImportError struct {
	Index        int    `json:"index"`
	PatentNumber string `json:"patent_number"`
	Error        string `json:"error"`
}

type BatchImportResult struct {
	TotalCount   int                `json:"total_count"`
	SuccessCount int                `json:"success_count"`
	FailedCount  int                `json:"failed_count"`
	SkippedCount int                `json:"skipped_count"`
	Errors       []BatchImportError `json:"errors"`
}

func (s *PatentService) BatchImportPatents(ctx context.Context, patents []*Patent) (*BatchImportResult, error) {
	result := &BatchImportResult{
		TotalCount: len(patents),
		Errors:     make([]BatchImportError, 0),
	}

	var toSave []*Patent

	for i, p := range patents {
		if p.PatentNumber == "" {
			result.FailedCount++
			result.Errors = append(result.Errors, BatchImportError{Index: i, Error: "patent number empty"})
			continue
		}

		exists, err := s.patentRepo.Exists(ctx, p.PatentNumber)
		if err != nil {
			result.FailedCount++
			result.Errors = append(result.Errors, BatchImportError{Index: i, PatentNumber: p.PatentNumber, Error: "failed to check existence"})
			continue
		}
		if exists {
			result.SkippedCount++
			continue
		}

		if err := p.Validate(); err != nil {
			result.FailedCount++
			result.Errors = append(result.Errors, BatchImportError{Index: i, PatentNumber: p.PatentNumber, Error: err.Error()})
			continue
		}

		toSave = append(toSave, p)
	}

	if len(toSave) > 0 {
		if err := s.patentRepo.SaveBatch(ctx, toSave); err != nil {
			return nil, errors.NewInternal("batch save failed: %v", err)
		}
		result.SuccessCount = len(toSave)
		for _, p := range toSave {
			s.publishEvents(ctx, p)
		}
	}

	return result, nil
}

func (s *PatentService) publishEvents(ctx context.Context, p *Patent) {
	if s.eventBus == nil {
		p.ClearEvents()
		return
	}

	events := p.DomainEvents()
	if len(events) == 0 {
		return
	}

	if err := s.eventBus.Publish(ctx, events...); err != nil {
		s.logError("failed to publish events", "error", err, "count", len(events))
	}
	p.ClearEvents()
}

//Personal.AI order the ending
