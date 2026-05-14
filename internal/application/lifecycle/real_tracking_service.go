package lifecycle

import (
	"context"
	"time"

	"github.com/google/uuid"
	domain "github.com/turtacn/KeyIP-Intelligence/internal/domain/lifecycle"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

type realTrackingService struct {
	repo   domain.LifecycleRepository
	logger logging.Logger
}

func NewRealTrackingService(repo domain.LifecycleRepository, logger logging.Logger) TrackingService {
	return &realTrackingService{repo: repo, logger: logger}
}

func (s *realTrackingService) GetLifecycle(ctx context.Context, patentID string) (*Lifecycle, error) {
	events, _, err := s.repo.GetEventsByPatent(ctx, patentID, nil, 0, 50)
	if err != nil {
		return nil, err
	}
	phase := "filed"
	status := "active"
	if len(events) > 0 {
		latest := events[0]
		switch latest.EventType {
		case "grant":
			phase = "granted"
		case "expiry":
			phase = "expired"
			status = "inactive"
		case "revocation":
			phase = "revoked"
			status = "inactive"
		}
	}
	return &Lifecycle{PatentID: patentID, Phase: phase, Status: status, CurrentDate: time.Now()}, nil
}

func (s *realTrackingService) AdvancePhase(ctx context.Context, input *AdvancePhaseInput) (*Lifecycle, error) {
	if input.PatentID == "" {
		return nil, errors.NewValidationError("patent_id", "patent_id is required")
	}
	evt := &domain.LifecycleEvent{
		ID:        uuid.New().String(),
		PatentID:  input.PatentID,
		EventType: domain.EventType(input.NewPhase),
		EventDate: input.Date,
		// Notes stored via metadata
	}
	if err := s.repo.CreateEvent(ctx, evt); err != nil {
		return nil, err
	}
	return &Lifecycle{PatentID: input.PatentID, Phase: input.NewPhase, Status: "active", CurrentDate: time.Now()}, nil
}

func (s *realTrackingService) AddMilestone(ctx context.Context, input *AddMilestoneInput) (*Milestone, error) {
	if input.PatentID == "" {
		return nil, errors.NewValidationError("patent_id", "patent_id is required")
	}
	deadline := &domain.Deadline{
		ID:           uuid.New().String(),
		PatentID:     input.PatentID,
		DeadlineType: input.Type,
		DueDate:      input.Date,
		Description:  input.Notes,
		Status:       domain.DeadlineStatusActive,
	}
	if err := s.repo.CreateDeadline(ctx, deadline); err != nil {
		return nil, err
	}
	return &Milestone{
		ID:       deadline.ID,
		PatentID: deadline.PatentID,
		Type:     deadline.DeadlineType,
		Date:     deadline.DueDate,
		Notes:    deadline.Description,
		CreatedAt: time.Now(),
	}, nil
}

func (s *realTrackingService) ListMilestones(ctx context.Context, patentID string) (*MilestoneList, error) {
	dls, err := s.repo.GetDeadlinesByPatent(ctx, patentID, nil)
	if err != nil {
		return nil, err
	}
	ms := make([]*Milestone, 0, len(dls))
	for _, d := range dls {
		ms = append(ms, &Milestone{
			ID:       d.ID,
			PatentID: d.PatentID,
			Type:     d.DeadlineType,
			Date:     d.DueDate,
			Notes:    d.Description,
		})
	}
	return &MilestoneList{Milestones: ms, Total: len(ms)}, nil
}

func (s *realTrackingService) RecordFee(ctx context.Context, input *RecordFeeInput) (*Fee, error) {
	if input.PatentID == "" {
		return nil, errors.NewValidationError("patent_id", "patent_id is required")
	}
	record := &domain.CostRecord{
		ID:           uuid.New().String(),
		PatentID:     input.PatentID,
		CostType:     input.Type,
		Amount:       int64(input.Amount * 100), // Convert to minor units (cents)
		Currency:     input.Currency,
		IncurredDate: input.DueDate,
	}
	if err := s.repo.CreateCostRecord(ctx, record); err != nil {
		return nil, err
	}
	return &Fee{
		ID:       record.ID,
		PatentID: record.PatentID,
		Type:     record.CostType,
		Amount:   float64(record.Amount) / 100,
		Currency: record.Currency,
		DueDate:  record.IncurredDate,
		Status:   "pending",
	}, nil
}

func (s *realTrackingService) ListFees(ctx context.Context, patentID string) (*FeeList, error) {
	records, err := s.repo.GetCostsByPatent(ctx, patentID)
	if err != nil {
		return nil, err
	}
	fees := make([]*Fee, 0, len(records))
	for _, r := range records {
		fees = append(fees, &Fee{
			ID:       r.ID,
			PatentID: r.PatentID,
			Type:     r.CostType,
			Amount:   float64(r.Amount) / 100, // Convert from minor units
			Currency: r.Currency,
			DueDate:  r.IncurredDate,
			Status:   "pending",
		})
	}
	return &FeeList{Fees: fees, Total: len(fees)}, nil
}

func (s *realTrackingService) GetTimeline(ctx context.Context, patentID string) (*Timeline, error) {
	events, err := s.repo.GetEventTimeline(ctx, patentID)
	if err != nil {
		return nil, err
	}
	te := make([]*TimelineEvent, 0, len(events))
	for _, e := range events {
		te = append(te, &TimelineEvent{
			Date: e.EventDate,
			Type: string(e.EventType),
			Description: e.Description,
		})
	}
	return &Timeline{PatentID: patentID, Events: te}, nil
}

func (s *realTrackingService) GetUpcomingDeadlines(ctx context.Context, input *UpcomingDeadlinesInput) ([]*DeadlineInfo, error) {
	days := input.Days
	if days <= 0 {
		days = 30
	}
	dls, _, err := s.repo.GetActiveDeadlines(ctx, nil, days, 0, 50)
	if err != nil {
		return nil, err
	}
	infos := make([]*DeadlineInfo, 0, len(dls))
	for _, d := range dls {
		infos = append(infos, &DeadlineInfo{
			PatentID:    d.PatentID,
			Type:        d.DeadlineType,
			DueDate:     d.DueDate,
			Description: d.Description,
			Priority:    "normal",
		})
	}
	return infos, nil
}
