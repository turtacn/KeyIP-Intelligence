package patent

import (
	"context"
	"testing"
	"time"
)

func TestPatentSearchCriteria_Validate(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		c := PatentSearchCriteria{Limit: 20}
		if err := c.Validate(); err != nil {
			t.Errorf("Expected valid, got %v", err)
		}
	})

	t.Run("Limit Too Large", func(t *testing.T) {
		c := PatentSearchCriteria{Limit: 1001}
		if err := c.Validate(); err == nil {
			t.Error("Expected error for limit > 1000")
		}
	})

	t.Run("Limit Negative", func(t *testing.T) {
		c := PatentSearchCriteria{Limit: -1}
		if err := c.Validate(); err == nil {
			t.Error("Expected error for negative limit")
		}
	})

	t.Run("Offset Negative", func(t *testing.T) {
		c := PatentSearchCriteria{Offset: -1}
		if err := c.Validate(); err == nil {
			t.Error("Expected error for negative offset")
		}
	})

	t.Run("Invalid SortBy", func(t *testing.T) {
		c := PatentSearchCriteria{SortBy: "invalid"}
		if err := c.Validate(); err == nil {
			t.Error("Expected error for invalid sort_by")
		}
	})

	t.Run("Invalid SortOrder", func(t *testing.T) {
		c := PatentSearchCriteria{SortOrder: "invalid"}
		if err := c.Validate(); err == nil {
			t.Error("Expected error for invalid sort_order")
		}
	})

	t.Run("Date Range Invalid", func(t *testing.T) {
		from := time.Now().UTC()
		to := from.AddDate(-1, 0, 0)
		c := PatentSearchCriteria{FilingDateFrom: &from, FilingDateTo: &to}
		if err := c.Validate(); err == nil {
			t.Error("Expected error for from > to")
		}
	})
}

func TestPatentSearchCriteria_HasFilters(t *testing.T) {
	t.Run("True", func(t *testing.T) {
		c := PatentSearchCriteria{TitleKeywords: []string{"OLED"}}
		if !c.HasFilters() {
			t.Error("Expected true")
		}
	})

	t.Run("False", func(t *testing.T) {
		c := PatentSearchCriteria{Limit: 20, Offset: 10}
		if c.HasFilters() {
			t.Error("Expected false (pagination doesn't count as filter)")
		}
	})
}

func TestPatentSearchResult_PageCount(t *testing.T) {
	tests := []struct {
		total int64
		limit int
		want  int
	}{
		{100, 20, 5},
		{101, 20, 6},
		{5, 20, 1},
		{0, 20, 0},
		{100, 0, 0},
	}
	for _, tt := range tests {
		r := PatentSearchResult{Total: tt.total, Limit: tt.limit}
		if got := r.PageCount(); got != tt.want {
			t.Errorf("PageCount(%d, %d) = %d, want %d", tt.total, tt.limit, got, tt.want)
		}
	}
}

func TestPatentSearchResult_CurrentPage(t *testing.T) {
	tests := []struct {
		offset int
		limit  int
		want   int
	}{
		{0, 20, 1},
		{20, 20, 2},
		{40, 20, 3},
		{0, 0, 1},
	}
	for _, tt := range tests {
		r := PatentSearchResult{Offset: tt.offset, Limit: tt.limit}
		if got := r.CurrentPage(); got != tt.want {
			t.Errorf("CurrentPage(%d, %d) = %d, want %d", tt.offset, tt.limit, got, tt.want)
		}
	}
}

// Interface compliance checks

type mockPatentRepo struct{}

func (m *mockPatentRepo) Save(ctx context.Context, patent *Patent) error                      { return nil }
func (m *mockPatentRepo) FindByID(ctx context.Context, id string) (*Patent, error)             { return nil, nil }
func (m *mockPatentRepo) FindByPatentNumber(ctx context.Context, patentNumber string) (*Patent, error) { return nil, nil }
func (m *mockPatentRepo) Delete(ctx context.Context, id string) error                        { return nil }
func (m *mockPatentRepo) Exists(ctx context.Context, patentNumber string) (bool, error)        { return false, nil }
func (m *mockPatentRepo) SaveBatch(ctx context.Context, patents []*Patent) error              { return nil }
func (m *mockPatentRepo) FindByIDs(ctx context.Context, ids []string) ([]*Patent, error)       { return nil, nil }
func (m *mockPatentRepo) FindByPatentNumbers(ctx context.Context, numbers []string) ([]*Patent, error) { return nil, nil }
func (m *mockPatentRepo) Search(ctx context.Context, criteria PatentSearchCriteria) (*PatentSearchResult, error) { return nil, nil }
func (m *mockPatentRepo) FindByMoleculeID(ctx context.Context, moleculeID string) ([]*Patent, error) { return nil, nil }
func (m *mockPatentRepo) FindByFamilyID(ctx context.Context, familyID string) ([]*Patent, error) { return nil, nil }
func (m *mockPatentRepo) FindByIPCCode(ctx context.Context, ipcCode string) ([]*Patent, error) { return nil, nil }
func (m *mockPatentRepo) FindByApplicant(ctx context.Context, applicantName string) ([]*Patent, error) { return nil, nil }
func (m *mockPatentRepo) FindCitedBy(ctx context.Context, patentNumber string) ([]*Patent, error) { return nil, nil }
func (m *mockPatentRepo) FindCiting(ctx context.Context, patentNumber string) ([]*Patent, error) { return nil, nil }
func (m *mockPatentRepo) CountByStatus(ctx context.Context) (map[PatentStatus]int64, error)   { return nil, nil }
func (m *mockPatentRepo) CountByOffice(ctx context.Context) (map[PatentOffice]int64, error)   { return nil, nil }
func (m *mockPatentRepo) CountByIPCSection(ctx context.Context) (map[string]int64, error)     { return nil, nil }
func (m *mockPatentRepo) CountByYear(ctx context.Context, field string) (map[int]int64, error) { return nil, nil }
func (m *mockPatentRepo) FindExpiringBefore(ctx context.Context, date time.Time) ([]*Patent, error) { return nil, nil }
func (m *mockPatentRepo) FindActiveByIPCCode(ctx context.Context, ipcCode string) ([]*Patent, error) { return nil, nil }
func (m *mockPatentRepo) FindWithMarkushStructures(ctx context.Context, offset, limit int) ([]*Patent, error) { return nil, nil }

func TestPatentRepository_InterfaceCompile(t *testing.T) {
	var _ PatentRepository = (*mockPatentRepo)(nil)
}

type mockMarkushRepo struct{}

func (m *mockMarkushRepo) Save(ctx context.Context, markush *MarkushStructure) error           { return nil }
func (m *mockMarkushRepo) FindByID(ctx context.Context, id string) (*MarkushStructure, error)  { return nil, nil }
func (m *mockMarkushRepo) FindByPatentID(ctx context.Context, patentID string) ([]*MarkushStructure, error) { return nil, nil }
func (m *mockMarkushRepo) FindByClaimNumber(ctx context.Context, patentID string, claimNumber int) ([]*MarkushStructure, error) { return nil, nil }
func (m *mockMarkushRepo) FindMatchingMolecule(ctx context.Context, smiles string) ([]*MarkushStructure, error) { return nil, nil }
func (m *mockMarkushRepo) Delete(ctx context.Context, id string) error                         { return nil }
func (m *mockMarkushRepo) CountByPatentID(ctx context.Context, patentID string) (int64, error)  { return 0, nil }

func TestMarkushRepository_InterfaceCompile(t *testing.T) {
	var _ MarkushRepository = (*mockMarkushRepo)(nil)
}

type mockPatentEventRepo struct{}

func (m *mockPatentEventRepo) SaveEvents(ctx context.Context, events ...DomainEvent) error      { return nil }
func (m *mockPatentEventRepo) FindByAggregateID(ctx context.Context, aggregateID string) ([]DomainEvent, error) { return nil, nil }
func (m *mockPatentEventRepo) FindByEventType(ctx context.Context, eventType EventType, offset, limit int) ([]DomainEvent, error) { return nil, nil }
func (m *mockPatentEventRepo) FindSince(ctx context.Context, aggregateID string, version int) ([]DomainEvent, error) { return nil, nil }

func TestPatentEventRepository_InterfaceCompile(t *testing.T) {
	var _ PatentEventRepository = (*mockPatentEventRepo)(nil)
}

//Personal.AI order the ending
