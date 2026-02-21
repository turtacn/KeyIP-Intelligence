package patent

import (
	"context"
	"fmt"
	"time"

	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
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
	return s.patentRepo.FindByPatentNumber(ctx, patentNumber)
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
	p, err := s.patentRepo.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, errors.ErrPatentNotFound(id)
	}

	var event DomainEvent
	switch targetStatus {
	case PatentStatusPublished:
		if params.PublicationDate == nil {
			return nil, errors.InvalidParam("publication date is required")
		}
		if err := p.Publish(*params.PublicationDate); err != nil {
			return nil, err
		}
		event = NewPatentPublishedEvent(p)
	case PatentStatusUnderExamination:
		if err := p.EnterExamination(); err != nil {
			return nil, err
		}
		event = NewPatentExaminationStartedEvent(p)
	case PatentStatusGranted:
		if params.GrantDate == nil || params.ExpiryDate == nil {
			return nil, errors.InvalidParam("grant and expiry dates are required")
		}
		if err := p.Grant(*params.GrantDate, *params.ExpiryDate); err != nil {
			return nil, err
		}
		event = NewPatentGrantedEvent(p)
	case PatentStatusRejected:
		if err := p.Reject(); err != nil {
			return nil, err
		}
		event = NewPatentRejectedEvent(p, params.Reason)
	case PatentStatusWithdrawn:
		prev := p.Status
		if err := p.Withdraw(); err != nil {
			return nil, err
		}
		event = NewPatentWithdrawnEvent(p, prev)
	case PatentStatusExpired:
		if err := p.Expire(); err != nil {
			return nil, err
		}
		event = NewPatentExpiredEvent(p)
	case PatentStatusInvalidated:
		if err := p.Invalidate(); err != nil {
			return nil, err
		}
		event = NewPatentInvalidatedEvent(p, params.Reason)
	case PatentStatusLapsed:
		if err := p.Lapse(); err != nil {
			return nil, err
		}
		event = NewPatentLapsedEvent(p)
	default:
		return nil, errors.InvalidParam("unsupported target status")
	}

	if err := s.patentRepo.Save(ctx, p); err != nil {
		return nil, err
	}

	s.publishEvents(ctx, p, event)
	return p, nil
}

func (s *PatentService) SetPatentClaims(ctx context.Context, patentID string, claims ClaimSet) (*Patent, error) {
	p, err := s.patentRepo.FindByID(ctx, patentID)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, errors.ErrPatentNotFound(patentID)
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
	p, err := s.patentRepo.FindByID(ctx, patentID)
	if err != nil {
		return err
	}
	if p == nil {
		return errors.ErrPatentNotFound(patentID)
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
	p, err := s.patentRepo.FindByID(ctx, patentID)
	if err != nil {
		return err
	}
	if p == nil {
		return errors.ErrPatentNotFound(patentID)
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
	p, err := s.patentRepo.FindByID(ctx, patentID)
	if err != nil {
		return err
	}
	if p == nil {
		return errors.ErrPatentNotFound(patentID)
	}

	if direction == "forward" {
		if err := p.AddCitation(citedPatentNumber); err != nil {
			return err
		}
	} else if direction == "backward" {
		if err := p.AddCitedBy(citedPatentNumber); err != nil {
			return err
		}
	} else {
		return errors.InvalidParam("invalid direction: must be forward or backward")
	}

	if err := s.patentRepo.Save(ctx, p); err != nil {
		return err
	}

	s.publishEvents(ctx, p, NewPatentCitationAddedEvent(p, citedPatentNumber, direction))
	return nil
}

func (s *PatentService) AnalyzeMarkushCoverage(ctx context.Context, patentID string, moleculeSMILES []string) (*MarkushCoverageAnalysis, error) {
	markushes, err := s.markushRepo.FindByPatentID(ctx, patentID)
	if err != nil {
		return nil, err
	}

	analysis := &MarkushCoverageAnalysis{
		MarkushID:        patentID, // Using patentID as analysis root
		SampledMolecules: len(moleculeSMILES),
		AnalyzedAt:       time.Now().UTC(),
		PositionDiversity: make(map[string]int),
	}

	matchedMap := make(map[string]bool)
	for _, smiles := range moleculeSMILES {
		for _, m := range markushes {
			match, _, _ := m.MatchesMolecule(smiles)
			if match {
				matchedMap[smiles] = true
				analysis.MatchedMolecules++
				break
			}
		}
	}

	if len(moleculeSMILES) > 0 {
		analysis.CoverageRate = float64(analysis.MatchedMolecules) / float64(len(moleculeSMILES))
	}

	return analysis, nil
}

func (s *PatentService) FindRelatedPatents(ctx context.Context, patentID string) ([]*Patent, error) {
	p, err := s.patentRepo.FindByID(ctx, patentID)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, errors.ErrPatentNotFound(patentID)
	}

	relatedMap := make(map[string]*Patent)

	// Same family
	if p.FamilyID != "" {
		family, _ := s.patentRepo.FindByFamilyID(ctx, p.FamilyID)
		for _, fp := range family {
			if fp.ID != p.ID {
				relatedMap[fp.ID] = fp
			}
		}
	}

	// Citations
	for _, cn := range p.Cites {
		cp, _ := s.patentRepo.FindByPatentNumber(ctx, cn)
		if cp != nil {
			relatedMap[cp.ID] = cp
		}
	}
	for _, cn := range p.CitedBy {
		cp, _ := s.patentRepo.FindByPatentNumber(ctx, cn)
		if cp != nil {
			relatedMap[cp.ID] = cp
		}
	}

	// Same molecules
	for _, molID := range p.MoleculeIDs {
		molPatents, _ := s.patentRepo.FindByMoleculeID(ctx, molID)
		for _, mp := range molPatents {
			if mp.ID != p.ID {
				relatedMap[mp.ID] = mp
			}
		}
	}

	result := make([]*Patent, 0, len(relatedMap))
	for _, rp := range relatedMap {
		result = append(result, rp)
	}
	return result, nil
}

// PatentStatistics aggregates platform-wide patent metrics.
type PatentStatistics struct {
	TotalCount          int64                    `json:"total_count"`
	ByStatus            map[PatentStatus]int64   `json:"by_status"`
	ByOffice            map[PatentOffice]int64   `json:"by_office"`
	ByIPCSection        map[string]int64         `json:"by_ipc_section"`
	ByFilingYear        map[int]int64            `json:"by_filing_year"`
	ByGrantYear         map[int]int64            `json:"by_grant_year"`
	ActiveCount         int64                    `json:"active_count"`
	ExpiringWithinYear  int64                    `json:"expiring_within_year"`
	WithMarkushCount    int64                    `json:"with_markush_count"`
	GeneratedAt         time.Time                `json:"generated_at"`
}

func (s *PatentService) GetPatentStatistics(ctx context.Context) (*PatentStatistics, error) {
	stats := &PatentStatistics{
		GeneratedAt: time.Now().UTC(),
	}

	var err error
	stats.ByStatus, _ = s.patentRepo.CountByStatus(ctx)
	stats.ByOffice, _ = s.patentRepo.CountByOffice(ctx)
	stats.ByIPCSection, _ = s.patentRepo.CountByIPCSection(ctx)
	stats.ByFilingYear, _ = s.patentRepo.CountByYear(ctx, "filing_date")
	stats.ByGrantYear, _ = s.patentRepo.CountByYear(ctx, "grant_date")

	for status, count := range stats.ByStatus {
		stats.TotalCount += count
		if status.IsActive() {
			stats.ActiveCount += count
		}
	}

	expiring, _ := s.patentRepo.FindExpiringBefore(ctx, time.Now().UTC().AddDate(1, 0, 0))
	stats.ExpiringWithinYear = int64(len(expiring))

	markushPatents, _ := s.patentRepo.FindWithMarkushStructures(ctx, 0, 0)
	stats.WithMarkushCount = int64(len(markushPatents))

	return stats, err
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
	}

	var toSave []*Patent
	for i, p := range patents {
		exists, _ := s.patentRepo.Exists(ctx, p.PatentNumber)
		if exists {
			result.SkippedCount++
			continue
		}

		if err := p.Validate(); err != nil {
			result.FailedCount++
			result.Errors = append(result.Errors, BatchImportError{
				Index:        i,
				PatentNumber: p.PatentNumber,
				Error:        err.Error(),
			})
			continue
		}

		toSave = append(toSave, p)
	}

	if len(toSave) > 0 {
		if err := s.patentRepo.SaveBatch(ctx, toSave); err != nil {
			return nil, err
		}
		result.SuccessCount = len(toSave)
		for _, p := range toSave {
			s.publishEvents(ctx, p, NewPatentCreatedEvent(p))
		}
	}

	return result, nil
}

func (s *PatentService) publishEvents(ctx context.Context, p *Patent, events ...DomainEvent) {
	// Add provided events to aggregate
	for _, e := range events {
		if e != nil {
			p.addEvent(e)
		}
	}

	if s.eventBus == nil {
		p.ClearEvents()
		return
	}

	evs := p.DomainEvents()
	if len(evs) > 0 {
		if err := s.eventBus.Publish(ctx, evs...); err != nil {
			s.logger.Error("failed to publish events", logging.Err(err))
		}
		p.ClearEvents()
	}
}

// itoa helper (referenced in some requirements but can use fmt.Sprintf)
func itoa(i int) string {
	return fmt.Sprintf("%d", i)
}

//Personal.AI order the ending
