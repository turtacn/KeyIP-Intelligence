package patent

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	domainPatent "github.com/turtacn/KeyIP-Intelligence/internal/domain/patent"
	"github.com/turtacn/KeyIP-Intelligence/internal/testutil"
)

// MockPatentRepository is a mock implementation of domainPatent.PatentRepository
type MockPatentRepository struct {
	mock.Mock
}

func (m *MockPatentRepository) Create(ctx context.Context, p *domainPatent.Patent) error {
	args := m.Called(ctx, p)
	return args.Error(0)
}

func (m *MockPatentRepository) GetByID(ctx context.Context, id uuid.UUID) (*domainPatent.Patent, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domainPatent.Patent), args.Error(1)
}

func (m *MockPatentRepository) GetByPatentNumber(ctx context.Context, number string) (*domainPatent.Patent, error) {
	args := m.Called(ctx, number)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domainPatent.Patent), args.Error(1)
}

func (m *MockPatentRepository) Update(ctx context.Context, p *domainPatent.Patent) error {
	args := m.Called(ctx, p)
	return args.Error(0)
}

func (m *MockPatentRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockPatentRepository) Restore(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockPatentRepository) HardDelete(ctx context.Context, id uuid.UUID) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockPatentRepository) Search(ctx context.Context, query domainPatent.SearchQuery) (*domainPatent.SearchResult, error) {
	args := m.Called(ctx, query)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*domainPatent.SearchResult), args.Error(1)
}

func (m *MockPatentRepository) ListByPortfolio(ctx context.Context, portfolioID string) ([]*domainPatent.Patent, error) {
	args := m.Called(ctx, portfolioID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domainPatent.Patent), args.Error(1)
}

func (m *MockPatentRepository) GetByFamilyID(ctx context.Context, familyID string) ([]*domainPatent.Patent, error) {
	args := m.Called(ctx, familyID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domainPatent.Patent), args.Error(1)
}

func (m *MockPatentRepository) GetByAssignee(ctx context.Context, assigneeID uuid.UUID, limit, offset int) ([]*domainPatent.Patent, int64, error) {
	args := m.Called(ctx, assigneeID, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Get(1).(int64), args.Error(2)
	}
	return args.Get(0).([]*domainPatent.Patent), args.Get(1).(int64), args.Error(2)
}

func (m *MockPatentRepository) GetByJurisdiction(ctx context.Context, jurisdiction string, limit, offset int) ([]*domainPatent.Patent, int64, error) {
	args := m.Called(ctx, jurisdiction, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Get(1).(int64), args.Error(2)
	}
	return args.Get(0).([]*domainPatent.Patent), args.Get(1).(int64), args.Error(2)
}

func (m *MockPatentRepository) GetExpiringPatents(ctx context.Context, daysAhead int, limit, offset int) ([]*domainPatent.Patent, int64, error) {
	args := m.Called(ctx, daysAhead, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Get(1).(int64), args.Error(2)
	}
	return args.Get(0).([]*domainPatent.Patent), args.Get(1).(int64), args.Error(2)
}

func (m *MockPatentRepository) FindDuplicates(ctx context.Context, fullTextHash string) ([]*domainPatent.Patent, error) {
	args := m.Called(ctx, fullTextHash)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domainPatent.Patent), args.Error(1)
}

func (m *MockPatentRepository) FindByMoleculeID(ctx context.Context, moleculeID string) ([]*domainPatent.Patent, error) {
	args := m.Called(ctx, moleculeID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domainPatent.Patent), args.Error(1)
}

func (m *MockPatentRepository) AssociateMolecule(ctx context.Context, patentID string, moleculeID string) error {
	args := m.Called(ctx, patentID, moleculeID)
	return args.Error(0)
}

func (m *MockPatentRepository) CreateClaim(ctx context.Context, claim *domainPatent.Claim) error {
	args := m.Called(ctx, claim)
	return args.Error(0)
}

func (m *MockPatentRepository) GetClaimsByPatent(ctx context.Context, patentID uuid.UUID) ([]*domainPatent.Claim, error) {
	args := m.Called(ctx, patentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domainPatent.Claim), args.Error(1)
}

func (m *MockPatentRepository) UpdateClaim(ctx context.Context, claim *domainPatent.Claim) error {
	args := m.Called(ctx, claim)
	return args.Error(0)
}

func (m *MockPatentRepository) DeleteClaimsByPatent(ctx context.Context, patentID uuid.UUID) error {
	args := m.Called(ctx, patentID)
	return args.Error(0)
}

func (m *MockPatentRepository) BatchCreateClaims(ctx context.Context, claims []*domainPatent.Claim) error {
	args := m.Called(ctx, claims)
	return args.Error(0)
}

func (m *MockPatentRepository) GetIndependentClaims(ctx context.Context, patentID uuid.UUID) ([]*domainPatent.Claim, error) {
	args := m.Called(ctx, patentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domainPatent.Claim), args.Error(1)
}

func (m *MockPatentRepository) SetInventors(ctx context.Context, patentID uuid.UUID, inventors []*domainPatent.Inventor) error {
	args := m.Called(ctx, patentID, inventors)
	return args.Error(0)
}

func (m *MockPatentRepository) GetInventors(ctx context.Context, patentID uuid.UUID) ([]*domainPatent.Inventor, error) {
	args := m.Called(ctx, patentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domainPatent.Inventor), args.Error(1)
}

func (m *MockPatentRepository) SearchByInventor(ctx context.Context, inventorName string, limit, offset int) ([]*domainPatent.Patent, int64, error) {
	args := m.Called(ctx, inventorName, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Get(1).(int64), args.Error(2)
	}
	return args.Get(0).([]*domainPatent.Patent), args.Get(1).(int64), args.Error(2)
}

func (m *MockPatentRepository) SearchByAssigneeName(ctx context.Context, assigneeName string, limit, offset int) ([]*domainPatent.Patent, int64, error) {
	args := m.Called(ctx, assigneeName, limit, offset)
	if args.Get(0) == nil {
		return nil, args.Get(1).(int64), args.Error(2)
	}
	return args.Get(0).([]*domainPatent.Patent), args.Get(1).(int64), args.Error(2)
}

func (m *MockPatentRepository) SetPriorityClaims(ctx context.Context, patentID uuid.UUID, claims []*domainPatent.PriorityClaim) error {
	args := m.Called(ctx, patentID, claims)
	return args.Error(0)
}

func (m *MockPatentRepository) GetPriorityClaims(ctx context.Context, patentID uuid.UUID) ([]*domainPatent.PriorityClaim, error) {
	args := m.Called(ctx, patentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*domainPatent.PriorityClaim), args.Error(1)
}

func (m *MockPatentRepository) BatchCreate(ctx context.Context, patents []*domainPatent.Patent) (int, error) {
	args := m.Called(ctx, patents)
	return args.Int(0), args.Error(1)
}

func (m *MockPatentRepository) BatchUpdateStatus(ctx context.Context, ids []uuid.UUID, status domainPatent.PatentStatus) (int64, error) {
	args := m.Called(ctx, ids, status)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockPatentRepository) CountByStatus(ctx context.Context) (map[domainPatent.PatentStatus]int64, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[domainPatent.PatentStatus]int64), args.Error(1)
}

func (m *MockPatentRepository) CountByJurisdiction(ctx context.Context) (map[string]int64, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]int64), args.Error(1)
}

func (m *MockPatentRepository) CountByYear(ctx context.Context, field string) (map[int]int64, error) {
	args := m.Called(ctx, field)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[int]int64), args.Error(1)
}

func (m *MockPatentRepository) GetIPCDistribution(ctx context.Context, level int) (map[string]int64, error) {
	args := m.Called(ctx, level)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[string]int64), args.Error(1)
}

func (m *MockPatentRepository) WithTx(ctx context.Context, fn func(domainPatent.PatentRepository) error) error {
	args := m.Called(ctx, fn)
	return args.Error(0)
}

func TestCreate(t *testing.T) {
	mockRepo := new(MockPatentRepository)
	mockLogger := testutil.NewMockLogger()
	service := NewService(mockRepo, mockLogger)

	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		input := &CreateInput{
			Title:         "OLED Display",
			ApplicationNo: "US12345678",
			Jurisdiction:  "US",
			Applicant:     "Tech Corp",
			Inventors:     []string{"Alice", "Bob"},
		}

		mockRepo.On("Create", ctx, mock.AnythingOfType("*patent.Patent")).Return(nil)

		p, err := service.Create(ctx, input)
		assert.NoError(t, err)
		assert.NotNil(t, p)
		assert.Equal(t, input.Title, p.Title)
		assert.Equal(t, input.ApplicationNo, p.ApplicationNo)
		assert.Equal(t, "US", p.Jurisdiction)
		assert.Equal(t, input.Applicant, p.Applicant)
		assert.Len(t, p.Inventors, 2)

		mockRepo.AssertExpectations(t)
	})

	t.Run("validation error", func(t *testing.T) {
		input := &CreateInput{
			Title: "", // Missing title
		}

		p, err := service.Create(ctx, input)
		assert.Error(t, err)
		assert.Nil(t, p)
	})
}

func TestGetByID(t *testing.T) {
	mockRepo := new(MockPatentRepository)
	mockLogger := testutil.NewMockLogger()
	service := NewService(mockRepo, mockLogger)

	ctx := context.Background()
	patentID := uuid.New()

	t.Run("success", func(t *testing.T) {
		domainP, _ := domainPatent.NewPatent("US123", "Title", domainPatent.OfficeUSPTO, time.Now())
		domainP.ID = patentID

		mockRepo.On("GetByID", ctx, patentID).Return(domainP, nil)

		p, err := service.GetByID(ctx, patentID.String())
		assert.NoError(t, err)
		assert.NotNil(t, p)
		assert.Equal(t, domainP.ID.String(), p.ID)

		mockRepo.AssertExpectations(t)
	})
}

func TestList(t *testing.T) {
	mockRepo := new(MockPatentRepository)
	mockLogger := testutil.NewMockLogger()
	service := NewService(mockRepo, mockLogger)

	ctx := context.Background()

	t.Run("success", func(t *testing.T) {
		input := &ListInput{
			Page:     1,
			PageSize: 10,
		}

		patents := []*domainPatent.Patent{
			{Title: "P1"},
			{Title: "P2"},
		}
		searchResult := &domainPatent.SearchResult{
			Items:      patents,
			TotalCount: 2,
		}

		mockRepo.On("Search", ctx, mock.AnythingOfType("patent.SearchQuery")).Return(searchResult, nil)

		result, err := service.List(ctx, input)
		assert.NoError(t, err)
		assert.NotNil(t, result)
		assert.Equal(t, int64(2), result.Total)
		assert.Len(t, result.Patents, 2)

		mockRepo.AssertExpectations(t)
	})
}
