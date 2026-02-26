package patent

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// MockPatentRepository
type MockPatentRepository struct {
	mock.Mock
}

func (m *MockPatentRepository) Save(ctx context.Context, patent *Patent) error {
	return m.Called(ctx, patent).Error(0)
}
func (m *MockPatentRepository) FindByID(ctx context.Context, id string) (*Patent, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Patent), args.Error(1)
}
func (m *MockPatentRepository) FindByPatentNumber(ctx context.Context, patentNumber string) (*Patent, error) {
	args := m.Called(ctx, patentNumber)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Patent), args.Error(1)
}
func (m *MockPatentRepository) Delete(ctx context.Context, id string) error {
	return m.Called(ctx, id).Error(0)
}
func (m *MockPatentRepository) Exists(ctx context.Context, patentNumber string) (bool, error) {
	args := m.Called(ctx, patentNumber)
	return args.Bool(0), args.Error(1)
}
func (m *MockPatentRepository) SaveBatch(ctx context.Context, patents []*Patent) error {
	return m.Called(ctx, patents).Error(0)
}
func (m *MockPatentRepository) FindByIDs(ctx context.Context, ids []string) ([]*Patent, error) {
	args := m.Called(ctx, ids)
	return args.Get(0).([]*Patent), args.Error(1)
}
func (m *MockPatentRepository) FindByPatentNumbers(ctx context.Context, numbers []string) ([]*Patent, error) {
	args := m.Called(ctx, numbers)
	return args.Get(0).([]*Patent), args.Error(1)
}
func (m *MockPatentRepository) Search(ctx context.Context, criteria PatentSearchCriteria) (*PatentSearchResult, error) {
	args := m.Called(ctx, criteria)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*PatentSearchResult), args.Error(1)
}
func (m *MockPatentRepository) SearchBySimilarity(ctx context.Context, req *SimilaritySearchRequest) ([]*PatentSearchResultWithSimilarity, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*PatentSearchResultWithSimilarity), args.Error(1)
}
func (m *MockPatentRepository) FindByMoleculeID(ctx context.Context, moleculeID string) ([]*Patent, error) {
	args := m.Called(ctx, moleculeID)
	return args.Get(0).([]*Patent), args.Error(1)
}
func (m *MockPatentRepository) FindByFamilyID(ctx context.Context, familyID string) ([]*Patent, error) {
	args := m.Called(ctx, familyID)
	return args.Get(0).([]*Patent), args.Error(1)
}
func (m *MockPatentRepository) FindByIPCCode(ctx context.Context, ipcCode string) ([]*Patent, error) {
	args := m.Called(ctx, ipcCode)
	return args.Get(0).([]*Patent), args.Error(1)
}
func (m *MockPatentRepository) FindByApplicant(ctx context.Context, applicantName string) ([]*Patent, error) {
	args := m.Called(ctx, applicantName)
	return args.Get(0).([]*Patent), args.Error(1)
}
func (m *MockPatentRepository) FindCitedBy(ctx context.Context, patentNumber string) ([]*Patent, error) {
	args := m.Called(ctx, patentNumber)
	return args.Get(0).([]*Patent), args.Error(1)
}
func (m *MockPatentRepository) FindCiting(ctx context.Context, patentNumber string) ([]*Patent, error) {
	args := m.Called(ctx, patentNumber)
	return args.Get(0).([]*Patent), args.Error(1)
}
func (m *MockPatentRepository) CountByStatus(ctx context.Context) (map[PatentStatus]int64, error) {
	args := m.Called(ctx)
	return args.Get(0).(map[PatentStatus]int64), args.Error(1)
}
func (m *MockPatentRepository) CountByOffice(ctx context.Context) (map[PatentOffice]int64, error) {
	args := m.Called(ctx)
	return args.Get(0).(map[PatentOffice]int64), args.Error(1)
}
func (m *MockPatentRepository) CountByIPCSection(ctx context.Context) (map[string]int64, error) {
	args := m.Called(ctx)
	return args.Get(0).(map[string]int64), args.Error(1)
}
func (m *MockPatentRepository) CountByYear(ctx context.Context, field string) (map[int]int64, error) {
	args := m.Called(ctx, field)
	return args.Get(0).(map[int]int64), args.Error(1)
}
func (m *MockPatentRepository) FindExpiringBefore(ctx context.Context, date time.Time) ([]*Patent, error) {
	args := m.Called(ctx, date)
	return args.Get(0).([]*Patent), args.Error(1)
}
func (m *MockPatentRepository) FindActiveByIPCCode(ctx context.Context, ipcCode string) ([]*Patent, error) {
	args := m.Called(ctx, ipcCode)
	return args.Get(0).([]*Patent), args.Error(1)
}
func (m *MockPatentRepository) FindWithMarkushStructures(ctx context.Context, offset, limit int) ([]*Patent, error) {
	args := m.Called(ctx, offset, limit)
	return args.Get(0).([]*Patent), args.Error(1)
}

// MockMarkushRepository
type MockMarkushRepository struct {
	mock.Mock
}

func (m *MockMarkushRepository) Save(ctx context.Context, markush *MarkushStructure) error {
	return m.Called(ctx, markush).Error(0)
}
func (m *MockMarkushRepository) FindByID(ctx context.Context, id string) (*MarkushStructure, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*MarkushStructure), args.Error(1)
}
func (m *MockMarkushRepository) FindByPatentID(ctx context.Context, patentID string) ([]*MarkushStructure, error) {
	args := m.Called(ctx, patentID)
	return args.Get(0).([]*MarkushStructure), args.Error(1)
}
func (m *MockMarkushRepository) FindByClaimNumber(ctx context.Context, patentID string, claimNumber int) ([]*MarkushStructure, error) {
	args := m.Called(ctx, patentID, claimNumber)
	return args.Get(0).([]*MarkushStructure), args.Error(1)
}
func (m *MockMarkushRepository) FindMatchingMolecule(ctx context.Context, smiles string) ([]*MarkushStructure, error) {
	args := m.Called(ctx, smiles)
	return args.Get(0).([]*MarkushStructure), args.Error(1)
}
func (m *MockMarkushRepository) Delete(ctx context.Context, id string) error {
	return m.Called(ctx, id).Error(0)
}
func (m *MockMarkushRepository) CountByPatentID(ctx context.Context, patentID string) (int64, error) {
	args := m.Called(ctx, patentID)
	return args.Get(0).(int64), args.Error(1)
}

// MockEventBus
type MockEventBus struct {
	mock.Mock
}

func (m *MockEventBus) Publish(ctx context.Context, events ...DomainEvent) error {
	args := m.Called(ctx, events)
	return args.Error(0)
}
func (m *MockEventBus) Subscribe(handler EventHandler) error {
	return m.Called(handler).Error(0)
}
func (m *MockEventBus) Unsubscribe(handler EventHandler) error {
	return m.Called(handler).Error(0)
}

// MockLogger
type MockLogger struct {
	mock.Mock
}

func (m *MockLogger) Debug(msg string, keysAndValues ...interface{}) { m.Called(msg, keysAndValues) }
func (m *MockLogger) Info(msg string, keysAndValues ...interface{})  { m.Called(msg, keysAndValues) }
func (m *MockLogger) Warn(msg string, keysAndValues ...interface{})  { m.Called(msg, keysAndValues) }
func (m *MockLogger) Error(msg string, keysAndValues ...interface{}) { m.Called(msg, keysAndValues) }

func TestNewPatentService_Success(t *testing.T) {
	svc, err := NewPatentService(&MockPatentRepository{}, &MockMarkushRepository{}, nil, nil, nil)
	assert.NoError(t, err)
	assert.NotNil(t, svc)
}

func TestNewPatentService_NilPatentRepo(t *testing.T) {
	_, err := NewPatentService(nil, &MockMarkushRepository{}, nil, nil, nil)
	assert.Error(t, err)
}

func TestPatentService_CreatePatent_Success(t *testing.T) {
	mockRepo := new(MockPatentRepository)
	mockBus := new(MockEventBus)
	svc, _ := NewPatentService(mockRepo, &MockMarkushRepository{}, mockBus, nil, nil)

	ctx := context.Background()
	mockRepo.On("Exists", ctx, "CN123").Return(false, nil)
	mockRepo.On("Save", ctx, mock.AnythingOfType("*patent.Patent")).Return(nil)
	mockBus.On("Publish", ctx, mock.Anything).Return(nil)

	p, err := svc.CreatePatent(ctx, "CN123", "Title", OfficeCNIPA, time.Now())
	assert.NoError(t, err)
	assert.NotNil(t, p)
	assert.Equal(t, "CN123", p.PatentNumber)

	mockRepo.AssertExpectations(t)
	mockBus.AssertExpectations(t)
}

func TestPatentService_CreatePatent_AlreadyExists(t *testing.T) {
	mockRepo := new(MockPatentRepository)
	svc, _ := NewPatentService(mockRepo, &MockMarkushRepository{}, nil, nil, nil)

	ctx := context.Background()
	mockRepo.On("Exists", ctx, "CN123").Return(true, nil)

	_, err := svc.CreatePatent(ctx, "CN123", "Title", OfficeCNIPA, time.Now())
	assert.Error(t, err)
}

func TestPatentService_GetPatent_Success(t *testing.T) {
	mockRepo := new(MockPatentRepository)
	svc, _ := NewPatentService(mockRepo, &MockMarkushRepository{}, nil, nil, nil)

	ctx := context.Background()
	p := &Patent{ID: "ID1", PatentNumber: "CN123"}
	mockRepo.On("FindByID", ctx, "ID1").Return(p, nil)

	res, err := svc.GetPatent(ctx, "ID1")
	assert.NoError(t, err)
	assert.Equal(t, p, res)
}

func TestPatentService_GetPatent_NotFound(t *testing.T) {
	mockRepo := new(MockPatentRepository)
	svc, _ := NewPatentService(mockRepo, &MockMarkushRepository{}, nil, nil, nil)

	ctx := context.Background()
	mockRepo.On("FindByID", ctx, "ID1").Return(nil, nil) // Return nil patent, nil error implies not found in FindByID (or repo returns specific error)
	// Service logic: "if patent == nil { return NotFound }"

	_, err := svc.GetPatent(ctx, "ID1")
	assert.Error(t, err)
	assert.True(t, errors.IsNotFound(err))
}

func TestPatentService_UpdatePatentStatus_Publish(t *testing.T) {
	mockRepo := new(MockPatentRepository)
	mockBus := new(MockEventBus)
	svc, _ := NewPatentService(mockRepo, &MockMarkushRepository{}, mockBus, nil, nil)

	ctx := context.Background()
	p, _ := NewPatent("CN123", "Title", OfficeCNIPA, time.Now()) // Filed
	mockRepo.On("FindByID", ctx, p.ID).Return(p, nil)
	mockRepo.On("Save", ctx, p).Return(nil)
	mockBus.On("Publish", ctx, mock.MatchedBy(func(events []DomainEvent) bool {
		return len(events) == 1 && events[0].EventType() == EventPatentPublished
	})).Return(nil)

	now := time.Now()
	res, err := svc.UpdatePatentStatus(ctx, p.ID, PatentStatusPublished, StatusTransitionParams{PublicationDate: &now})
	assert.NoError(t, err)
	assert.Equal(t, PatentStatusPublished, res.Status)
}

func TestPatentService_SetPatentClaims_Success(t *testing.T) {
	mockRepo := new(MockPatentRepository)
	mockBus := new(MockEventBus)
	svc, _ := NewPatentService(mockRepo, &MockMarkushRepository{}, mockBus, nil, nil)

	ctx := context.Background()
	p, _ := NewPatent("CN123", "Title", OfficeCNIPA, time.Now())
	mockRepo.On("FindByID", ctx, p.ID).Return(p, nil)
	mockRepo.On("Save", ctx, p).Return(nil)
	mockBus.On("Publish", ctx, mock.MatchedBy(func(events []DomainEvent) bool {
		return len(events) == 1 && events[0].EventType() == EventPatentClaimsUpdated
	})).Return(nil)

	c, _ := NewClaim(1, "A valid claim text with sufficient length", ClaimTypeIndependent, ClaimCategoryProduct)
	claims := ClaimSet{*c}
	res, err := svc.SetPatentClaims(ctx, p.ID, claims)
	assert.NoError(t, err)
	assert.Len(t, res.Claims, 1)
}

func TestPatentService_AnalyzeMarkushCoverage_Success(t *testing.T) {
	mockMarkushRepo := new(MockMarkushRepository)
	// We need a mock matcher for MatchesMolecule
	mockMatcher := new(MockMarkushMatcher)

	svc, _ := NewPatentService(&MockPatentRepository{}, mockMarkushRepo, nil, nil, mockMatcher)

	ctx := context.Background()
	ms, _ := NewMarkushStructure("M1", "Core", 1)
	ms.PreferredExamples = []string{"C1"}

	mockMarkushRepo.On("FindByPatentID", ctx, "PID").Return([]*MarkushStructure{ms}, nil)
	// MatchesMolecule uses PreferredExamples, so no matcher calls needed if we pass preferred example

	res, err := svc.AnalyzeMarkushCoverage(ctx, "PID", []string{"C1"})
	assert.NoError(t, err)
	assert.Equal(t, 1, res.MatchedMolecules)
}

//Personal.AI order the ending
