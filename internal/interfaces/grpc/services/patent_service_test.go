// Phase 11 - File 257: internal/interfaces/grpc/services/patent_service_test.go
package services

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	pb "github.com/turtacn/KeyIP-Intelligence/api/proto/v1"
	"github.com/turtacn/KeyIP-Intelligence/internal/domain/patent"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// MockPatentRepo mocks the PatentRepository interface
type MockPatentRepo struct {
	mock.Mock
}

func (m *MockPatentRepo) GetByPatentNumber(ctx context.Context, patentNumber string) (*patent.Patent, error) {
	args := m.Called(ctx, patentNumber)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*patent.Patent), args.Error(1)
}

func (m *MockPatentRepo) Search(ctx context.Context, criteria patent.PatentSearchCriteria) (*patent.PatentSearchResult, error) {
	args := m.Called(ctx, criteria)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*patent.PatentSearchResult), args.Error(1)
}

func (m *MockPatentRepo) Create(ctx context.Context, p *patent.Patent) error {
	args := m.Called(ctx, p)
	return args.Error(0)
}

func (m *MockPatentRepo) Update(ctx context.Context, p *patent.Patent) error {
	args := m.Called(ctx, p)
	return args.Error(0)
}

func (m *MockPatentRepo) Delete(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

// FindByID implements the string version
func (m *MockPatentRepo) FindByID(ctx context.Context, id string) (*patent.Patent, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*patent.Patent), args.Error(1)
}

// GetByID implements the uuid.UUID version (Legacy alias in repo interface)
func (m *MockPatentRepo) GetByID(ctx context.Context, id uuid.UUID) (*patent.Patent, error) {
	return m.FindByID(ctx, id.String())
}

func (m *MockPatentRepo) GetByFamilyID(ctx context.Context, familyID string) ([]*patent.Patent, error) {
	args := m.Called(ctx, familyID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*patent.Patent), args.Error(1)
}

func (m *MockPatentRepo) BatchCreate(ctx context.Context, patents []*patent.Patent) error {
	args := m.Called(ctx, patents)
	return args.Error(0)
}

// AssociateMolecule implementation
func (m *MockPatentRepo) AssociateMolecule(ctx context.Context, patentID string, moleculeID string) error {
	args := m.Called(ctx, patentID, moleculeID)
	return args.Error(0)
}

// Missing methods implementation stubs
func (m *MockPatentRepo) Save(ctx context.Context, p *patent.Patent) error { return m.Create(ctx, p) }
func (m *MockPatentRepo) FindByPatentNumber(ctx context.Context, patentNumber string) (*patent.Patent, error) { return m.GetByPatentNumber(ctx, patentNumber) }
func (m *MockPatentRepo) Exists(ctx context.Context, patentNumber string) (bool, error) { return false, nil }
func (m *MockPatentRepo) SaveBatch(ctx context.Context, patents []*patent.Patent) error { return nil }
func (m *MockPatentRepo) FindByIDs(ctx context.Context, ids []string) ([]*patent.Patent, error) { return nil, nil }
func (m *MockPatentRepo) FindByPatentNumbers(ctx context.Context, numbers []string) ([]*patent.Patent, error) { return nil, nil }
func (m *MockPatentRepo) FindByMoleculeID(ctx context.Context, moleculeID string) ([]*patent.Patent, error) { return nil, nil }
func (m *MockPatentRepo) FindByFamilyID(ctx context.Context, familyID string) ([]*patent.Patent, error) { return nil, nil }
func (m *MockPatentRepo) FindByIPCCode(ctx context.Context, ipcCode string) ([]*patent.Patent, error) { return nil, nil }
func (m *MockPatentRepo) FindByApplicant(ctx context.Context, applicantName string) ([]*patent.Patent, error) { return nil, nil }
func (m *MockPatentRepo) SearchByAssigneeName(ctx context.Context, assigneeName string, limit, offset int) ([]*patent.Patent, int64, error) { return nil, 0, nil }
func (m *MockPatentRepo) FindCitedBy(ctx context.Context, patentNumber string) ([]*patent.Patent, error) { return nil, nil }
func (m *MockPatentRepo) FindCiting(ctx context.Context, patentNumber string) ([]*patent.Patent, error) { return nil, nil }
func (m *MockPatentRepo) CountByStatus(ctx context.Context) (map[patent.PatentStatus]int64, error) { return nil, nil }
func (m *MockPatentRepo) CountByOffice(ctx context.Context) (map[patent.PatentOffice]int64, error) { return nil, nil }
func (m *MockPatentRepo) CountByIPCSection(ctx context.Context) (map[string]int64, error) { return nil, nil }
func (m *MockPatentRepo) CountByJurisdiction(ctx context.Context) (map[string]int64, error) { return nil, nil }
func (m *MockPatentRepo) CountByYear(ctx context.Context, field string) (map[int]int64, error) { return nil, nil }
func (m *MockPatentRepo) FindExpiringBefore(ctx context.Context, date time.Time) ([]*patent.Patent, error) { return nil, nil }
func (m *MockPatentRepo) FindActiveByIPCCode(ctx context.Context, ipcCode string) ([]*patent.Patent, error) { return nil, nil }
func (m *MockPatentRepo) FindWithMarkushStructures(ctx context.Context, offset, limit int) ([]*patent.Patent, error) { return nil, nil }
func (m *MockPatentRepo) ListByPortfolio(ctx context.Context, portfolioID string) ([]*patent.Patent, error) { return nil, nil }

// Removed GetStats as it causes compilation errors due to undefined types

func TestGetPatent(t *testing.T) {
	mockRepo := new(MockPatentRepo)
	mockLogger := new(MockLogger) // Reusing MockLogger from molecule_service_test.go
	service := NewPatentServiceServer(mockRepo, nil, mockLogger)

	ctx := context.Background()
	pn := "CN123456789A"

	// Set up expected patent data
	now := time.Now()
	expectedPatent := &patent.Patent{
		PatentNumber: pn,
		Title:        "Test Patent",
		Abstract:     "Test Abstract",
		AssigneeName: "Test Assignee",
		Status:       patent.PatentStatusGranted,
		FilingDate:   &now,
	}

	t.Run("Success", func(t *testing.T) {
		mockRepo.On("GetByPatentNumber", ctx, pn).Return(expectedPatent, nil)

		resp, err := service.GetPatent(ctx, &pb.GetPatentRequest{PatentNumber: pn})
		assert.NoError(t, err)
		assert.NotNil(t, resp.Patent)
		assert.Equal(t, pn, resp.Patent.PatentNumber)
		assert.Equal(t, "Test Patent", resp.Patent.Title)
		mockRepo.AssertExpectations(t)
	})

	t.Run("NotFound", func(t *testing.T) {
		// Updated to use format string for NewNotFound
		mockRepo.On("GetByPatentNumber", ctx, "CN999999999A").Return(nil, errors.NewNotFound("%s not found", "patent"))
		mockLogger.On("Error", mock.Anything, mock.Anything).Return()

		resp, err := service.GetPatent(ctx, &pb.GetPatentRequest{PatentNumber: "CN999999999A"})
		assert.Error(t, err)
		assert.Nil(t, resp)
		assert.Equal(t, codes.NotFound, status.Code(err))
	})

	t.Run("InvalidFormat", func(t *testing.T) {
		resp, err := service.GetPatent(ctx, &pb.GetPatentRequest{PatentNumber: "INVALID"})
		assert.Error(t, err)
		assert.Nil(t, resp)
		assert.Equal(t, codes.InvalidArgument, status.Code(err))
	})
}

func TestSearchPatents(t *testing.T) {
	mockRepo := new(MockPatentRepo)
	mockLogger := new(MockLogger)
	service := NewPatentServiceServer(mockRepo, nil, mockLogger)

	ctx := context.Background()

	t.Run("Success", func(t *testing.T) {
		req := &pb.SearchPatentsRequest{
			Query:    "OLED",
			PageSize: 10,
		}

		result := &patent.PatentSearchResult{
			Patents: []*patent.Patent{
				{PatentNumber: "CN123456789A", Title: "OLED Device"},
			},
			Total: 1,
		}

		mockRepo.On("Search", ctx, mock.AnythingOfType("patent.PatentSearchCriteria")).Return(result, nil)

		resp, err := service.SearchPatents(ctx, req)
		assert.NoError(t, err)
		assert.NotNil(t, resp)
		assert.Equal(t, int64(1), resp.TotalCount)
		assert.Len(t, resp.Patents, 1)
		mockRepo.AssertExpectations(t)
	})
}

//Personal.AI order the ending
