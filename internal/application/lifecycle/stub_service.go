package lifecycle

import "context"

// StubTrackingService provides a minimal no-op implementation of TrackingService for development.
type StubTrackingService struct{}

func NewStubTrackingService() *StubTrackingService { return &StubTrackingService{} }

func (s *StubTrackingService) GetLifecycle(_ context.Context, patentID string) (*Lifecycle, error) {
	return &Lifecycle{PatentID: patentID, Phase: "filed", Status: "active"}, nil
}
func (s *StubTrackingService) AdvancePhase(_ context.Context, _ *AdvancePhaseInput) (*Lifecycle, error) {
	return nil, nil
}
func (s *StubTrackingService) AddMilestone(_ context.Context, _ *AddMilestoneInput) (*Milestone, error) {
	return nil, nil
}
func (s *StubTrackingService) ListMilestones(_ context.Context, _ string) (*MilestoneList, error) {
	return &MilestoneList{Milestones: []*Milestone{}, Total: 0}, nil
}
func (s *StubTrackingService) RecordFee(_ context.Context, _ *RecordFeeInput) (*Fee, error) {
	return nil, nil
}
func (s *StubTrackingService) ListFees(_ context.Context, _ string) (*FeeList, error) {
	return &FeeList{Fees: []*Fee{}, Total: 0}, nil
}
func (s *StubTrackingService) GetTimeline(_ context.Context, patentID string) (*Timeline, error) {
	return &Timeline{PatentID: patentID, Events: []*TimelineEvent{}}, nil
}
func (s *StubTrackingService) GetUpcomingDeadlines(_ context.Context, _ *UpcomingDeadlinesInput) ([]*DeadlineInfo, error) {
	return []*DeadlineInfo{}, nil
}

var _ TrackingService = (*StubTrackingService)(nil)
