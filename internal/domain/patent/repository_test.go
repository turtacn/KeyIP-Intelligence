package patent

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
)

func TestPatentSearchCriteria_Validate_Success(t *testing.T) {
	c := PatentSearchCriteria{
		Limit:  20,
		Offset: 0,
		SortBy: "filing_date",
	}
	assert.NoError(t, c.Validate())
}

func TestPatentSearchCriteria_Validate_LimitTooLarge(t *testing.T) {
	c := PatentSearchCriteria{Limit: 1001}
	assert.Error(t, c.Validate())
}

func TestPatentSearchCriteria_Validate_LimitNegative(t *testing.T) {
	c := PatentSearchCriteria{Limit: -1}
	assert.Error(t, c.Validate())
}

func TestPatentSearchCriteria_Validate_OffsetNegative(t *testing.T) {
	c := PatentSearchCriteria{Offset: -1}
	assert.Error(t, c.Validate())
}

func TestPatentSearchCriteria_Validate_InvalidSortBy(t *testing.T) {
	c := PatentSearchCriteria{SortBy: "invalid"}
	assert.Error(t, c.Validate())
}

func TestPatentSearchCriteria_Validate_InvalidSortOrder(t *testing.T) {
	c := PatentSearchCriteria{SortOrder: "invalid"}
	assert.Error(t, c.Validate())
}

func TestPatentSearchCriteria_Validate_DateRangeInvalid(t *testing.T) {
	now := time.Now()
	later := now.Add(time.Hour)
	c := PatentSearchCriteria{
		FilingDateStart: &later,
		FilingDateEnd:   &now,
	}
	assert.Error(t, c.Validate())
}

func TestPatentSearchCriteria_Validate_DefaultLimit(t *testing.T) {
	c := PatentSearchCriteria{Limit: 0}
	assert.NoError(t, c.Validate())
}

func TestPatentSearchCriteria_HasFilters_True(t *testing.T) {
	c := PatentSearchCriteria{PatentNumbers: []string{"CN123"}}
	assert.True(t, c.HasFilters())
}

func TestPatentSearchCriteria_HasFilters_False(t *testing.T) {
	c := PatentSearchCriteria{Limit: 20}
	assert.False(t, c.HasFilters())
}

func TestPatentSearchResult_PageCount_ExactDivision(t *testing.T) {
	r := PatentSearchResult{Total: 100, Limit: 20}
	assert.Equal(t, 5, r.PageCount())
}

func TestPatentSearchResult_PageCount_WithRemainder(t *testing.T) {
	r := PatentSearchResult{Total: 101, Limit: 20}
	assert.Equal(t, 6, r.PageCount())
}

func TestPatentSearchResult_PageCount_SinglePage(t *testing.T) {
	r := PatentSearchResult{Total: 5, Limit: 20}
	assert.Equal(t, 1, r.PageCount())
}

func TestPatentSearchResult_PageCount_Empty(t *testing.T) {
	r := PatentSearchResult{Total: 0, Limit: 20}
	assert.Equal(t, 0, r.PageCount())
}

func TestPatentSearchResult_PageCount_ZeroLimit(t *testing.T) {
	r := PatentSearchResult{Total: 100, Limit: 0}
	assert.Equal(t, 0, r.PageCount())
}

func TestPatentSearchResult_CurrentPage(t *testing.T) {
	assert.Equal(t, 1, PatentSearchResult{Offset: 0, Limit: 20}.CurrentPage())
	assert.Equal(t, 2, PatentSearchResult{Offset: 20, Limit: 20}.CurrentPage())
	assert.Equal(t, 3, PatentSearchResult{Offset: 40, Limit: 20}.CurrentPage())
	assert.Equal(t, 1, PatentSearchResult{Offset: 0, Limit: 0}.CurrentPage())
}

// Mocks to verify interface definition compilation

type mockPatentRepo struct{}

func (m *mockPatentRepo) Save(ctx context.Context, patent *Patent) error { return nil }
func (m *mockPatentRepo) FindByID(ctx context.Context, id string) (*Patent, error) { return nil, nil }
func (m *mockPatentRepo) FindByPatentNumber(ctx context.Context, patentNumber string) (*Patent, error) { return nil, nil }
func (m *mockPatentRepo) Delete(ctx context.Context, id string) error { return nil }
func (m *mockPatentRepo) Exists(ctx context.Context, patentNumber string) (bool, error) { return false, nil }
func (m *mockPatentRepo) SaveBatch(ctx context.Context, patents []*Patent) error { return nil }
func (m *mockPatentRepo) FindByIDs(ctx context.Context, ids []string) ([]*Patent, error) { return nil, nil }
func (m *mockPatentRepo) FindByPatentNumbers(ctx context.Context, numbers []string) ([]*Patent, error) { return nil, nil }
func (m *mockPatentRepo) Search(ctx context.Context, criteria PatentSearchCriteria) (*PatentSearchResult, error) { return nil, nil }
func (m *mockPatentRepo) FindByMoleculeID(ctx context.Context, moleculeID string) ([]*Patent, error) { return nil, nil }
func (m *mockPatentRepo) FindByFamilyID(ctx context.Context, familyID string) ([]*Patent, error) { return nil, nil }
func (m *mockPatentRepo) FindByIPCCode(ctx context.Context, ipcCode string) ([]*Patent, error) { return nil, nil }
func (m *mockPatentRepo) FindByApplicant(ctx context.Context, applicantName string) ([]*Patent, error) { return nil, nil }
func (m *mockPatentRepo) FindCitedBy(ctx context.Context, patentNumber string) ([]*Patent, error) { return nil, nil }
func (m *mockPatentRepo) FindCiting(ctx context.Context, patentNumber string) ([]*Patent, error) { return nil, nil }
func (m *mockPatentRepo) CountByStatus(ctx context.Context) (map[PatentStatus]int64, error) { return nil, nil }
func (m *mockPatentRepo) CountByOffice(ctx context.Context) (map[PatentOffice]int64, error) { return nil, nil }
func (m *mockPatentRepo) CountByIPCSection(ctx context.Context) (map[string]int64, error) { return nil, nil }
func (m *mockPatentRepo) CountByYear(ctx context.Context, field string) (map[int]int64, error) { return nil, nil }
func (m *mockPatentRepo) CountByJurisdiction(ctx context.Context) (map[string]int64, error) { return nil, nil }
func (m *mockPatentRepo) GetByFamilyID(ctx context.Context, familyID string) ([]*Patent, error) { return nil, nil }
func (m *mockPatentRepo) GetByPatentNumber(ctx context.Context, number string) (*Patent, error) { return nil, nil }
func (m *mockPatentRepo) GetByID(ctx context.Context, id uuid.UUID) (*Patent, error) { return nil, nil }
func (m *mockPatentRepo) ListByPortfolio(ctx context.Context, portfolioID string) ([]*Patent, error) { return nil, nil }
func (m *mockPatentRepo) SearchByAssigneeName(ctx context.Context, assigneeName string, limit, offset int) ([]*Patent, int64, error) { return nil, 0, nil }
func (m *mockPatentRepo) FindExpiringBefore(ctx context.Context, date time.Time) ([]*Patent, error) { return nil, nil }
func (m *mockPatentRepo) FindActiveByIPCCode(ctx context.Context, ipcCode string) ([]*Patent, error) { return nil, nil }
func (m *mockPatentRepo) FindWithMarkushStructures(ctx context.Context, offset, limit int) ([]*Patent, error) { return nil, nil }
func (m *mockPatentRepo) AssociateMolecule(ctx context.Context, patentID string, moleculeID string) error { return nil }

func TestPatentRepository_InterfaceCompile(t *testing.T) {
	var _ PatentRepository = (*mockPatentRepo)(nil)
}

type mockMarkushRepo struct{}

func (m *mockMarkushRepo) Save(ctx context.Context, markush *MarkushStructure) error { return nil }
func (m *mockMarkushRepo) FindByID(ctx context.Context, id string) (*MarkushStructure, error) { return nil, nil }
func (m *mockMarkushRepo) FindByPatentID(ctx context.Context, patentID string) ([]*MarkushStructure, error) { return nil, nil }
func (m *mockMarkushRepo) FindByClaimNumber(ctx context.Context, patentID string, claimNumber int) ([]*MarkushStructure, error) { return nil, nil }
func (m *mockMarkushRepo) FindMatchingMolecule(ctx context.Context, smiles string) ([]*MarkushStructure, error) { return nil, nil }
func (m *mockMarkushRepo) Delete(ctx context.Context, id string) error { return nil }
func (m *mockMarkushRepo) CountByPatentID(ctx context.Context, patentID string) (int64, error) { return 0, nil }

func TestMarkushRepository_InterfaceCompile(t *testing.T) {
	var _ MarkushRepository = (*mockMarkushRepo)(nil)
}

type mockPatentEventRepo struct{}

func (m *mockPatentEventRepo) SaveEvents(ctx context.Context, events ...common.DomainEvent) error { return nil }
func (m *mockPatentEventRepo) FindByAggregateID(ctx context.Context, aggregateID string) ([]common.DomainEvent, error) { return nil, nil }
func (m *mockPatentEventRepo) FindByEventType(ctx context.Context, eventType common.EventType, offset, limit int) ([]common.DomainEvent, error) { return nil, nil }
func (m *mockPatentEventRepo) FindSince(ctx context.Context, aggregateID string, version int) ([]common.DomainEvent, error) { return nil, nil }

func TestPatentEventRepository_InterfaceCompile(t *testing.T) {
	var _ PatentEventRepository = (*mockPatentEventRepo)(nil)
}
