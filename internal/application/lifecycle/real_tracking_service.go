package lifecycle

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	domain "github.com/turtacn/KeyIP-Intelligence/internal/domain/lifecycle"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
)

type realTrackingService struct {
	repo   domain.LifecycleRepository
	logger logging.Logger
}

func NewRealTrackingService(repo domain.LifecycleRepository, logger logging.Logger) TrackingService {
	return &realTrackingService{repo: repo, logger: logger}
}

func (s *realTrackingService) resolvePatentID(ctx context.Context, id string) (string, error) {
	if _, err := uuid.Parse(id); err == nil {
		return id, nil
	}
	status, err := s.repo.GetByPatentID(ctx, id)
	if err != nil {
		return "", err
	}
	if status.PatentID == "" {
		return "", fmt.Errorf("patent not found: %s", id)
	}
	return status.PatentID, nil
}

// GetLifecycle returns lifecycle for a patent (by patent_number or UUID)
func (s *realTrackingService) GetLifecycle(ctx context.Context, patentID string) (*Lifecycle, error) {
	pid, err := s.resolvePatentID(ctx, patentID)
	if err != nil {
		return &Lifecycle{PatentID: patentID, Phase: "unknown", Status: "inactive", CurrentDate: time.Now()}, nil
	}
	events, _, err := s.repo.GetEventsByPatent(ctx, pid, nil, 0, 50)
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
		}
	}
	return &Lifecycle{PatentID: patentID, Phase: phase, Status: status, CurrentDate: time.Now()}, nil
}

func (s *realTrackingService) AdvancePhase(ctx context.Context, input *AdvancePhaseInput) (*Lifecycle, error) {
	pid, err := s.resolvePatentID(ctx, input.PatentID)
	if err != nil {
		return nil, err
	}
	evt := &domain.LifecycleEvent{
		ID:        uuid.New().String(),
		PatentID:  pid,
		EventType: domain.EventType(input.NewPhase),
		EventDate: input.Date,
		Title:     input.NewPhase,
	}
	if err := s.repo.CreateEvent(ctx, evt); err != nil {
		return nil, err
	}
	return &Lifecycle{PatentID: input.PatentID, Phase: input.NewPhase, Status: "active", CurrentDate: time.Now()}, nil
}

func (s *realTrackingService) AddMilestone(ctx context.Context, input *AddMilestoneInput) (*Milestone, error) {
	pid, err := s.resolvePatentID(ctx, input.PatentID)
	if err != nil {
		return nil, err
	}
	deadline := &domain.Deadline{
		ID:           uuid.New().String(),
		PatentID:     pid,
		DeadlineType: input.Type,
		DueDate:      input.Date,
		Description:  input.Notes,
		OriginalDueDate: input.Date,
		Status:       domain.DeadlineStatusActive,
		Priority:     "medium",
	}
	if err := s.repo.CreateDeadline(ctx, deadline); err != nil {
		return nil, err
	}
	return &Milestone{ID: deadline.ID, PatentID: input.PatentID, Type: deadline.DeadlineType, Date: deadline.DueDate, Notes: deadline.Description, CreatedAt: time.Now()}, nil
}

func (s *realTrackingService) ListMilestones(ctx context.Context, patentID string) (*MilestoneList, error) {
	pid, err := s.resolvePatentID(ctx, patentID)
	if err != nil {
		return &MilestoneList{Milestones: []*Milestone{}, Total: 0}, nil
	}
	dls, err := s.repo.GetDeadlinesByPatent(ctx, pid, nil)
	if err != nil {
		return nil, err
	}
	ms := make([]*Milestone, 0, len(dls))
	for _, d := range dls {
		ms = append(ms, &Milestone{ID: d.ID, PatentID: patentID, Type: d.DeadlineType, Date: d.DueDate, Notes: d.Description})
	}
	return &MilestoneList{Milestones: ms, Total: len(ms)}, nil
}

func (s *realTrackingService) RecordFee(ctx context.Context, input *RecordFeeInput) (*Fee, error) {
	pid, err := s.resolvePatentID(ctx, input.PatentID)
	if err != nil {
		return nil, err
	}
	record := &domain.CostRecord{
		ID:           uuid.New().String(),
		PatentID:     pid,
		CostType:     input.Type,
		Amount:       int64(input.Amount * 100),
		Currency:     input.Currency,
		IncurredDate: input.DueDate,
	}
	if err := s.repo.CreateCostRecord(ctx, record); err != nil {
		return nil, err
	}
	return &Fee{ID: record.ID, PatentID: input.PatentID, Type: record.CostType, Amount: float64(record.Amount) / 100, Currency: record.Currency, DueDate: record.IncurredDate, Status: "pending"}, nil
}

func (s *realTrackingService) ListFees(ctx context.Context, patentID string) (*FeeList, error) {
	pid, err := s.resolvePatentID(ctx, patentID)
	if err != nil {
		return &FeeList{Fees: []*Fee{}, Total: 0}, nil
	}
	records, err := s.repo.GetCostsByPatent(ctx, pid)
	if err != nil {
		return nil, err
	}
	fees := make([]*Fee, 0, len(records))
	for _, r := range records {
		fees = append(fees, &Fee{ID: r.ID, PatentID: patentID, Type: r.CostType, Amount: float64(r.Amount) / 100, Currency: r.Currency, DueDate: r.IncurredDate, Status: "pending"})
	}
	return &FeeList{Fees: fees, Total: len(fees)}, nil
}

func (s *realTrackingService) GetTimeline(ctx context.Context, patentID string) (*Timeline, error) {
	pid, err := s.resolvePatentID(ctx, patentID)
	if err != nil {
		return &Timeline{PatentID: patentID, Events: []*TimelineEvent{}}, nil
	}
	events, err := s.repo.GetEventTimeline(ctx, pid)
	if err != nil {
		return nil, err
	}
	te := make([]*TimelineEvent, 0, len(events))
	for _, e := range events {
		te = append(te, &TimelineEvent{Date: e.EventDate, Type: string(e.EventType), Description: e.Description})
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
		infos = append(infos, &DeadlineInfo{PatentID: d.PatentID, Type: d.DeadlineType, DueDate: d.DueDate, Description: d.Description, Priority: d.Priority})
	}
	return infos, nil
}
