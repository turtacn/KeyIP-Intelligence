package patent

import (
	"context"
	"fmt"
	"time"

	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
	"github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
)

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
	if markushRepo == nil {
		panic("markushRepo is required")
	}
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
	exists, err := s.patentRepo.Exists(ctx, patentNumber)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, errors.ErrPatentAlreadyExists(patentNumber)
	}

	p, err := NewPatent(patentNumber, title, office, filingDate)
	if err != nil {
		return nil, err
	}

	if err := s.patentRepo.Save(ctx, p); err != nil {
		return nil, err
	}

	s.publishEvents(ctx, p, NewPatentCreatedEvent(p))
	return p, nil
}

func (s *PatentService) GetPatent(ctx context.Context, id string) (*Patent, error) {
	p, err := s.patentRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, errors.ErrPatentNotFound(id)
	}
	return p, nil
}

func (s *PatentService) GetPatentByNumber(ctx context.Context, patentNumber string) (*Patent, error) {
	p, err := s.patentRepo.FindByPatentNumber(ctx, patentNumber)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, errors.ErrPatentNotFound(patentNumber)
	}
	return p, nil
}

func (s *PatentService) SearchPatents(ctx context.Context, criteria PatentSearchCriteria) (*PatentSearchResult, error) {
	if err := criteria.Validate(); err != nil {
		return nil, err
	}
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
	var updateErr error

	switch targetStatus {
	case PatentStatusPublished:
		if params.PublicationDate == nil {
			return nil, errors.InvalidParam("publication date is required")
		}
		updateErr = p.Publish(*params.PublicationDate)
		event = NewPatentPublishedEvent(p)
	case PatentStatusUnderExamination:
		updateErr = p.EnterExamination()
		// event = NewPatentExaminationStartedEvent(p)
	case PatentStatusGranted:
		if params.GrantDate == nil || params.ExpiryDate == nil {
			return nil, errors.InvalidParam("grant and expiry dates are required")
		}
		updateErr = p.Grant(*params.GrantDate, *params.ExpiryDate)
		event = NewPatentGrantedEvent(p)
	case PatentStatusRejected:
		updateErr = p.Reject()
		event = NewPatentRejectedEvent(p, params.Reason)
	case PatentStatusWithdrawn:
		prevStatus := p.Status
		updateErr = p.Withdraw()
		event = NewPatentWithdrawnEvent(p, prevStatus)
	case PatentStatusExpired:
		updateErr = p.Expire()
		event = NewPatentExpiredEvent(p)
	case PatentStatusInvalidated:
		updateErr = p.Invalidate()
		event = NewPatentInvalidatedEvent(p, params.Reason)
	case PatentStatusLapsed:
		updateErr = p.Lapse()
		event = NewPatentLapsedEvent(p)
	default:
		return nil, errors.InvalidParam(fmt.Sprintf("unsupported target status: %s", targetStatus))
	}

	if updateErr != nil {
		return nil, updateErr
	}

	if err := s.patentRepo.Save(ctx, p); err != nil {
		return nil, err
	}

	if event != nil {
		s.publishEvents(ctx, p, event)
	}
	return p, nil
}

func (s *PatentService) SetPatentClaims(ctx context.Context, patentID string, claims ClaimSet) (*Patent, error) {
	p, err := s.GetPatent(ctx, patentID)
	if err != nil {
		return nil, err
	}

	if err := claims.Validate(); err != nil {
		return nil, err
	}

	if err := p.SetClaims(claims); err != nil {
		return nil, err
	}

	if err := s.patentRepo.Save(ctx, p); err != nil {
		return nil, err
	}

	s.publishEvents(ctx, p, NewPatentClaimsUpdatedEvent(p))
	return p, nil
}

func (s *PatentService) LinkMolecule(ctx context.Context, patentID, moleculeID string) error {
	p, err := s.GetPatent(ctx, patentID)
	if err != nil {
		return err
	}

	if p.HasMolecule(moleculeID) {
		return errors.ErrConflict("patent", "molecule_link_exists")
	}

	if err := p.AddMolecule(moleculeID); err != nil {
		return err
	}

	if err := s.patentRepo.Save(ctx, p); err != nil {
		return err
	}

	s.publishEvents(ctx, p, NewPatentMoleculeLinkedEvent(p, moleculeID))
	return nil
}

func (s *PatentService) UnlinkMolecule(ctx context.Context, patentID, moleculeID string) error {
	p, err := s.GetPatent(ctx, patentID)
	if err != nil {
		return err
	}

	if !p.HasMolecule(moleculeID) {
		return errors.ErrConflict("patent", "molecule_link_not_found")
	}

	if err := p.RemoveMolecule(moleculeID); err != nil {
		return err
	}

	if err := s.patentRepo.Save(ctx, p); err != nil {
		return err
	}

	s.publishEvents(ctx, p, NewPatentMoleculeUnlinkedEvent(p, moleculeID))
	return nil
}

func (s *PatentService) AddCitation(ctx context.Context, patentID, citedPatentNumber, direction string) error {
	if direction != "forward" && direction != "backward" {
		return errors.InvalidParam("direction must be 'forward' or 'backward'")
	}

	p, err := s.GetPatent(ctx, patentID)
	if err != nil {
		return err
	}

	if direction == "forward" {
		// Forward citation: Current patent CITES the other patent
		if err := p.AddCitation(citedPatentNumber); err != nil {
			return err
		}
	} else {
		// Backward citation: Current patent IS CITED BY the other patent
		if err := p.AddCitedBy(citedPatentNumber); err != nil {
			return err
		}
	}

	if err := s.patentRepo.Save(ctx, p); err != nil {
		return err
	}

	s.publishEvents(ctx, p, NewPatentCitationAddedEvent(p, citedPatentNumber, direction))
	return nil
}

func (s *PatentService) AnalyzeMarkushCoverage(ctx context.Context, patentID string, moleculeSMILES []string) (*MarkushCoverageAnalysis, error) {
	structures, err := s.markushRepo.FindByPatentID(ctx, patentID)
	if err != nil {
		return nil, err
	}

	if len(structures) == 0 {
		return &MarkushCoverageAnalysis{
			MarkushID:    "",
			CoverageRate: 0,
		}, nil
	}

	// Iterate through all Markush structures to find matches.
	// A molecule is considered covered if it matches ANY of the Markush structures in the patent.
	matchedCount := 0

	for _, smiles := range moleculeSMILES {
		isCovered := false
		for _, ms := range structures {
			match, _, _ := ms.MatchesMolecule(smiles)
			if match {
				isCovered = true
				break
			}
		}
		if isCovered {
			matchedCount++
		}
	}

	rate := 0.0
	if len(moleculeSMILES) > 0 {
		rate = float64(matchedCount) / float64(len(moleculeSMILES))
	}

	// Use the first structure's ID as reference, or a composite ID if needed.
	// For analysis purposes, we report the aggregate coverage.
	primaryID := ""
	var totalCombinations int64 = 0
	if len(structures) > 0 {
		primaryID = structures[0].ID
		for _, ms := range structures {
			totalCombinations += ms.TotalCombinations
		}
	}

	return &MarkushCoverageAnalysis{
		MarkushID:         primaryID,
		TotalCombinations: totalCombinations,
		SampledMolecules:  len(moleculeSMILES),
		MatchedMolecules:  matchedCount,
		CoverageRate:      rate,
		AnalyzedAt:        time.Now().UTC(),
	}, nil
}

func (s *PatentService) FindRelatedPatents(ctx context.Context, patentID string) ([]*Patent, error) {
	p, err := s.GetPatent(ctx, patentID)
	if err != nil {
		return nil, err
	}

	relatedMap := make(map[string]*Patent)

	// By Family
	if p.FamilyID != "" {
		familyPatents, err := s.patentRepo.FindByFamilyID(ctx, p.FamilyID)
		if err == nil {
			for _, fp := range familyPatents {
				if fp.ID.String() != patentID {
					relatedMap[fp.ID.String()] = fp
				}
			}
		}
	}

	// By Citations (simplified: just taking cited/citing if we had method to get full objects)
	// Current repository interface methods FindCitedBy/FindCiting return []*Patent
	citing, err := s.patentRepo.FindCiting(ctx, p.PatentNumber)
	if err == nil {
		for _, cp := range citing {
			if cp.ID.String() != patentID {
				relatedMap[cp.ID.String()] = cp
			}
		}
	}
	citedBy, err := s.patentRepo.FindCitedBy(ctx, p.PatentNumber)
	if err == nil {
		for _, cp := range citedBy {
			if cp.ID.String() != patentID {
				relatedMap[cp.ID.String()] = cp
			}
		}
	}

	// By Molecules
	for _, molID := range p.MoleculeIDs {
		molPatents, err := s.patentRepo.FindByMoleculeID(ctx, molID)
		if err == nil {
			for _, mp := range molPatents {
				if mp.ID.String() != patentID {
					relatedMap[mp.ID.String()] = mp
				}
			}
		}
	}

	related := make([]*Patent, 0, len(relatedMap))
	for _, rp := range relatedMap {
		related = append(related, rp)
	}

	return related, nil
}

type PatentStatistics struct {
	TotalCount         int64
	ByStatus           map[PatentStatus]int64
	ByOffice           map[PatentOffice]int64
	ByIPCSection       map[string]int64
	ByFilingYear       map[int]int64
	ByGrantYear        map[int]int64
	ActiveCount        int64
	ExpiringWithinYear int64
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

	stats.ByOffice, err = s.patentRepo.CountByOffice(ctx)
	if err != nil {
		return nil, err
	}

	stats.ByIPCSection, err = s.patentRepo.CountByIPCSection(ctx)
	if err != nil {
		return nil, err
	}

	stats.ByFilingYear, err = s.patentRepo.CountByYear(ctx, "filing_date")
	if err != nil {
		return nil, err
	}

	stats.ByGrantYear, err = s.patentRepo.CountByYear(ctx, "grant_date")
	if err != nil {
		return nil, err
	}

	for status, count := range stats.ByStatus {
		stats.TotalCount += count
		if status.IsActive() {
			stats.ActiveCount += count
		}
	}

	expiring, err := s.patentRepo.FindExpiringBefore(ctx, time.Now().AddDate(1, 0, 0))
	if err == nil {
		stats.ExpiringWithinYear = int64(len(expiring))
	}

	return stats, nil
}

type BatchImportError struct {
	Index        int
	PatentNumber string
	Error        string
}

type BatchImportResult struct {
	TotalCount   int
	SuccessCount int
	FailedCount  int
	SkippedCount int
	Errors       []BatchImportError
}

func (s *PatentService) BatchImportPatents(ctx context.Context, patents []*Patent) (*BatchImportResult, error) {
	result := &BatchImportResult{
		TotalCount: len(patents),
	}

	toSave := make([]*Patent, 0, len(patents))

	for i, p := range patents {
		exists, err := s.patentRepo.Exists(ctx, p.PatentNumber)
		if err != nil {
			result.FailedCount++
			result.Errors = append(result.Errors, BatchImportError{Index: i, PatentNumber: p.PatentNumber, Error: err.Error()})
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
			// If batch save fails, we assume all pending failed for simplicity, or handle partials if repo supports it
			// Assuming SaveBatch is all-or-nothing or returns error
			return nil, err
		}
		result.SuccessCount = len(toSave)

		for _, p := range toSave {
			s.publishEvents(ctx, p, NewPatentCreatedEvent(p))
		}
	}

	return result, nil
}

func (s *PatentService) publishEvents(ctx context.Context, p *Patent, events ...common.DomainEvent) {
	if s.eventBus == nil {
		return
	}
	// Append any internally collected events
	allEvents := append(events, p.DomainEvents()...)

	if len(allEvents) > 0 {
		if err := s.eventBus.Publish(ctx, allEvents...); err != nil {
			if s.logger != nil {
				s.logger.Error("failed to publish events", logging.Err(err))
			}
		}
		p.ClearEvents()
	}
}

// SimilaritySearchRequest defines parameters for patent similarity search.
type SimilaritySearchRequest struct {
	MoleculeID     string
	SMILES         string
	Threshold      float64
	Limit          int
	Offset         int
	MaxResults     int
	PatentOffices  []string
	Assignees      []string
	TechDomains    []string
	DateFrom       *time.Time
	DateTo         *time.Time
	ExcludePatents []string
}

// SimilaritySearchResult defines the output of a similarity search.
type SimilaritySearchResult struct {
	Patent             *Patent
	PatentNumber       string
	Title              string
	Assignee           string
	FilingDate         time.Time
	LegalStatus        string
	IPCCodes           []string
	Score              float64
	MorganSimilarity   float64
	RDKitSimilarity    float64
	AtomPairSimilarity float64
}

func (s *PatentService) GetPatentsByMoleculeID(ctx context.Context, moleculeID string) ([]*Patent, error) {
	return s.patentRepo.FindByMoleculeID(ctx, moleculeID)
}

func (s *PatentService) SearchBySimilarity(ctx context.Context, req *SimilaritySearchRequest) ([]*SimilaritySearchResult, error) {
	// Map request to repository search criteria
	criteria := PatentSearchCriteria{
		MoleculeIDs: []string{req.MoleculeID},
		Limit:       req.Limit,
		Offset:      req.Offset,
	}
	if req.MaxResults > 0 {
		criteria.Limit = req.MaxResults
	}
	if criteria.Limit == 0 {
		criteria.Limit = 50
	}

	if len(req.PatentOffices) > 0 {
		for _, o := range req.PatentOffices {
			criteria.Offices = append(criteria.Offices, PatentOffice(o))
		}
	}
	if len(req.Assignees) > 0 {
		criteria.ApplicantNames = req.Assignees
	}
	if len(req.TechDomains) > 0 {
		criteria.IPCCodes = req.TechDomains
	}
	if req.DateFrom != nil {
		criteria.FilingDateFrom = req.DateFrom
	}
	if req.DateTo != nil {
		criteria.FilingDateTo = req.DateTo
	}

	// Delegate to repository
	result, err := s.patentRepo.Search(ctx, criteria)
	if err != nil {
		return nil, err
	}

	// Convert results
	searchResults := make([]*SimilaritySearchResult, len(result.Patents))
	for i, p := range result.Patents {
		var filingDate time.Time
		if p.FilingDate != nil {
			filingDate = *p.FilingDate
		}
		searchResults[i] = &SimilaritySearchResult{
			Patent:             p,
			PatentNumber:       p.PatentNumber,
			Title:              p.Title,
			Assignee:           p.AssigneeName,
			FilingDate:         filingDate,
			LegalStatus:        p.Status.String(),
			IPCCodes:           p.IPCCodes,
			Score:              1.0, // Dummy score as repo search doesn't return similarity yet
			MorganSimilarity:   1.0,
			RDKitSimilarity:    1.0,
			AtomPairSimilarity: 1.0,
		}
	}

	return searchResults, nil
}
