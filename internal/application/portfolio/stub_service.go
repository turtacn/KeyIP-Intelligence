package portfolio

import "context"

// StubService provides a minimal no-op implementation of Service for development.
type StubService struct{}

func NewStubService() *StubService { return &StubService{} }

func (s *StubService) Create(_ context.Context, _ *CreateInput) (*Portfolio, error) {
	return nil, nil
}
func (s *StubService) GetByID(_ context.Context, _ string) (*Portfolio, error) {
	return nil, nil
}
func (s *StubService) List(_ context.Context, _ *ListInput) (*ListResult, error) {
	return &ListResult{Portfolios: []*Portfolio{}, Total: 0, Page: 1, PageSize: 20}, nil
}
func (s *StubService) Update(_ context.Context, _ *UpdateInput) (*Portfolio, error) {
	return nil, nil
}
func (s *StubService) Delete(_ context.Context, _ string, _ string) error {
	return nil
}
func (s *StubService) AddPatents(_ context.Context, _ string, _ []string, _ string) error {
	return nil
}
func (s *StubService) RemovePatents(_ context.Context, _ string, _ []string, _ string) error {
	return nil
}
func (s *StubService) GetAnalysis(_ context.Context, _ string) (*PortfolioAnalysis, error) {
	return &PortfolioAnalysis{
		TotalPatents:    0,
		ByJurisdiction:  map[string]int{},
		ByStatus:        map[string]int{},
		ByYear:          map[string]int{},
		TopIPCCodes:     []IPCCount{},
		TotalValue:      0,
		Recommendations: []string{},
	}, nil
}

var _ Service = (*StubService)(nil)
