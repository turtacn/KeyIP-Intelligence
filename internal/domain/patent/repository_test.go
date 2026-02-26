package patent

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
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
	from := time.Now()
	to := from.AddDate(0, -1, 0)
	c := PatentSearchCriteria{FilingDateFrom: &from, FilingDateTo: &to}
	assert.Error(t, c.Validate())
}

func TestPatentSearchCriteria_Validate_DefaultLimit(t *testing.T) {
	c := PatentSearchCriteria{Limit: 0}
	assert.NoError(t, c.Validate())
	assert.Equal(t, 20, c.Limit)
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
	r := PatentSearchResult{Offset: 0, Limit: 20}
	assert.Equal(t, 1, r.CurrentPage())
	r.Offset = 20
	assert.Equal(t, 2, r.CurrentPage())
	r.Offset = 40
	assert.Equal(t, 3, r.CurrentPage())
}

func TestPatentSearchResult_CurrentPage_ZeroLimit(t *testing.T) {
	r := PatentSearchResult{Offset: 0, Limit: 0}
	assert.Equal(t, 1, r.CurrentPage())
}

// MockPatentRepo ensures interface is implementable
type MockPatentRepo struct {
	mock.Mock
}

func (m *MockPatentRepo) Save(ctx context.Context, patent *Patent) error { return nil }
func (m *MockPatentRepo) FindByID(ctx context.Context, id string) (*Patent, error) { return nil, nil }
func (m *MockPatentRepo) FindByPatentNumber(ctx context.Context, patentNumber string) (*Patent, error) { return nil, nil }
func (m *MockPatentRepo) Delete(ctx context.Context, id string) error { return nil }
func (m *MockPatentRepo) Exists(ctx context.Context, patentNumber string) (bool, error) { return false, nil }
func (m *MockPatentRepo) SaveBatch(ctx context.Context, patents []*Patent) error { return nil }
func (m *MockPatentRepo) FindByIDs(ctx context.Context, ids []string) ([]*Patent, error) { return nil, nil }
func (m *MockPatentRepo) FindByPatentNumbers(ctx context.Context, numbers []string) ([]*Patent, error) { return nil, nil }
func (m *MockPatentRepo) Search(ctx context.Context, criteria PatentSearchCriteria) (*PatentSearchResult, error) { return nil, nil }
func (m *MockPatentRepo) SearchBySimilarity(ctx context.Context, req *SimilaritySearchRequest) ([]*PatentSearchResultWithSimilarity, error) { return nil, nil }
func (m *MockPatentRepo) FindByMoleculeID(ctx context.Context, moleculeID string) ([]*Patent, error) { return nil, nil }
func (m *MockPatentRepo) FindByFamilyID(ctx context.Context, familyID string) ([]*Patent, error) { return nil, nil }
func (m *MockPatentRepo) FindByIPCCode(ctx context.Context, ipcCode string) ([]*Patent, error) { return nil, nil }
func (m *MockPatentRepo) FindByApplicant(ctx context.Context, applicantName string) ([]*Patent, error) { return nil, nil }
func (m *MockPatentRepo) FindCitedBy(ctx context.Context, patentNumber string) ([]*Patent, error) { return nil, nil }
func (m *MockPatentRepo) FindCiting(ctx context.Context, patentNumber string) ([]*Patent, error) { return nil, nil }
func (m *MockPatentRepo) CountByStatus(ctx context.Context) (map[PatentStatus]int64, error) { return nil, nil }
func (m *MockPatentRepo) CountByOffice(ctx context.Context) (map[PatentOffice]int64, error) { return nil, nil }
func (m *MockPatentRepo) CountByIPCSection(ctx context.Context) (map[string]int64, error) { return nil, nil }
func (m *MockPatentRepo) CountByYear(ctx context.Context, field string) (map[int]int64, error) { return nil, nil }
func (m *MockPatentRepo) FindExpiringBefore(ctx context.Context, date time.Time) ([]*Patent, error) { return nil, nil }
func (m *MockPatentRepo) FindActiveByIPCCode(ctx context.Context, ipcCode string) ([]*Patent, error) { return nil, nil }
func (m *MockPatentRepo) FindWithMarkushStructures(ctx context.Context, offset, limit int) ([]*Patent, error) { return nil, nil }

func TestPatentRepository_InterfaceCompile(t *testing.T) {
	var _ PatentRepository = (*MockPatentRepo)(nil)
}

type MockMarkushRepo struct {
	mock.Mock
}

func (m *MockMarkushRepo) Save(ctx context.Context, markush *MarkushStructure) error { return nil }
func (m *MockMarkushRepo) FindByID(ctx context.Context, id string) (*MarkushStructure, error) { return nil, nil }
func (m *MockMarkushRepo) FindByPatentID(ctx context.Context, patentID string) ([]*MarkushStructure, error) { return nil, nil }
func (m *MockMarkushRepo) FindByClaimNumber(ctx context.Context, patentID string, claimNumber int) ([]*MarkushStructure, error) { return nil, nil }
func (m *MockMarkushRepo) FindMatchingMolecule(ctx context.Context, smiles string) ([]*MarkushStructure, error) { return nil, nil }
func (m *MockMarkushRepo) Delete(ctx context.Context, id string) error { return nil }
func (m *MockMarkushRepo) CountByPatentID(ctx context.Context, patentID string) (int64, error) { return 0, nil }

func TestMarkushRepository_InterfaceCompile(t *testing.T) {
	var _ MarkushRepository = (*MockMarkushRepo)(nil)
}

//Personal.AI order the ending
