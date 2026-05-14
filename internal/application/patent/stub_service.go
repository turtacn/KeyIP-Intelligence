package patent

import "context"

// StubService provides a minimal no-op implementation of Service for development.
type StubService struct{}

func NewStubService() *StubService { return &StubService{} }

func (s *StubService) Create(_ context.Context, _ *CreateInput) (*Patent, error) {
	return nil, nil
}
func (s *StubService) GetByID(_ context.Context, _ string) (*Patent, error) {
	return nil, nil
}
func (s *StubService) List(_ context.Context, _ *ListInput) (*ListResult, error) {
	return &ListResult{Patents: []*Patent{}, Total: 0, Page: 1, PageSize: 20, TotalPages: 0}, nil
}
func (s *StubService) Update(_ context.Context, _ *UpdateInput) (*Patent, error) {
	return nil, nil
}
func (s *StubService) Delete(_ context.Context, _ string, _ string) error {
	return nil
}
func (s *StubService) Search(_ context.Context, _ *SearchInput) (*SearchResult, error) {
	return &SearchResult{Patents: []*Patent{}, Total: 0, Page: 1, PageSize: 20, TotalPages: 0}, nil
}
func (s *StubService) AdvancedSearch(_ context.Context, _ *AdvancedSearchInput) (*SearchResult, error) {
	return &SearchResult{Patents: []*Patent{}, Total: 0, Page: 1, PageSize: 20, TotalPages: 0}, nil
}
func (s *StubService) GetStats(_ context.Context, _ *StatsInput) (*Stats, error) {
	return &Stats{
		ByJurisdiction: map[string]int64{},
		ByYear:         map[string]int64{},
		TopApplicants:  []ApplicantStat{},
		TopIPCCodes:    []IPCStat{},
	}, nil
}

var _ Service = (*StubService)(nil)
