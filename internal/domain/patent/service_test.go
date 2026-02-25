package patent

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
)

// MockPatentRepository is a mock implementation of PatentRepository
type MockPatentRepository struct {
	mock.Mock
}

func (m *MockPatentRepository) Create(ctx context.Context, p *Patent) error {
	args := m.Called(ctx, p)
	return args.Error(0)
}

func (m *MockPatentRepository) GetByID(ctx context.Context, id uuid.UUID) (*Patent, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Patent), args.Error(1)
}

func (m *MockPatentRepository) GetByPatentNumber(ctx context.Context, number string) (*Patent, error) {
	args := m.Called(ctx, number)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*Patent), args.Error(1)
}

func (m *MockPatentRepository) Update(ctx context.Context, p *Patent) error {
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

func (m *MockPatentRepository) Search(ctx context.Context, query SearchQuery) (*SearchResult, error) {
	args := m.Called(ctx, query)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*SearchResult), args.Error(1)
}

func (m *MockPatentRepository) ListByPortfolio(ctx context.Context, portfolioID string) ([]*Patent, error) {
	args := m.Called(ctx, portfolioID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*Patent), args.Error(1)
}

func (m *MockPatentRepository) GetByFamilyID(ctx context.Context, familyID string) ([]*Patent, error) {
	args := m.Called(ctx, familyID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*Patent), args.Error(1)
}

func (m *MockPatentRepository) GetByAssignee(ctx context.Context, assigneeID uuid.UUID, limit, offset int) ([]*Patent, int64, error) {
	args := m.Called(ctx, assigneeID, limit, offset)
	if args.Get(0) == nil {
		return nil, 0, args.Error(2)
	}
	return args.Get(0).([]*Patent), args.Get(1).(int64), args.Error(2)
}

func (m *MockPatentRepository) GetByJurisdiction(ctx context.Context, jurisdiction string, limit, offset int) ([]*Patent, int64, error) {
	args := m.Called(ctx, jurisdiction, limit, offset)
	if args.Get(0) == nil {
		return nil, 0, args.Error(2)
	}
	return args.Get(0).([]*Patent), args.Get(1).(int64), args.Error(2)
}

func (m *MockPatentRepository) GetExpiringPatents(ctx context.Context, daysAhead int, limit, offset int) ([]*Patent, int64, error) {
	args := m.Called(ctx, daysAhead, limit, offset)
	if args.Get(0) == nil {
		return nil, 0, args.Error(2)
	}
	return args.Get(0).([]*Patent), args.Get(1).(int64), args.Error(2)
}

func (m *MockPatentRepository) FindDuplicates(ctx context.Context, fullTextHash string) ([]*Patent, error) {
	args := m.Called(ctx, fullTextHash)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*Patent), args.Error(1)
}

func (m *MockPatentRepository) FindByMoleculeID(ctx context.Context, moleculeID string) ([]*Patent, error) {
	args := m.Called(ctx, moleculeID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*Patent), args.Error(1)
}

func (m *MockPatentRepository) AssociateMolecule(ctx context.Context, patentID string, moleculeID string) error {
	args := m.Called(ctx, patentID, moleculeID)
	return args.Error(0)
}

func (m *MockPatentRepository) CreateClaim(ctx context.Context, claim *Claim) error {
	args := m.Called(ctx, claim)
	return args.Error(0)
}

func (m *MockPatentRepository) GetClaimsByPatent(ctx context.Context, patentID uuid.UUID) ([]*Claim, error) {
	args := m.Called(ctx, patentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*Claim), args.Error(1)
}

func (m *MockPatentRepository) UpdateClaim(ctx context.Context, claim *Claim) error {
	args := m.Called(ctx, claim)
	return args.Error(0)
}

func (m *MockPatentRepository) DeleteClaimsByPatent(ctx context.Context, patentID uuid.UUID) error {
	args := m.Called(ctx, patentID)
	return args.Error(0)
}

func (m *MockPatentRepository) BatchCreateClaims(ctx context.Context, claims []*Claim) error {
	args := m.Called(ctx, claims)
	return args.Error(0)
}

func (m *MockPatentRepository) GetIndependentClaims(ctx context.Context, patentID uuid.UUID) ([]*Claim, error) {
	args := m.Called(ctx, patentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*Claim), args.Error(1)
}

func (m *MockPatentRepository) SetInventors(ctx context.Context, patentID uuid.UUID, inventors []*Inventor) error {
	args := m.Called(ctx, patentID, inventors)
	return args.Error(0)
}

func (m *MockPatentRepository) GetInventors(ctx context.Context, patentID uuid.UUID) ([]*Inventor, error) {
	args := m.Called(ctx, patentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*Inventor), args.Error(1)
}

func (m *MockPatentRepository) SearchByInventor(ctx context.Context, inventorName string, limit, offset int) ([]*Patent, int64, error) {
	args := m.Called(ctx, inventorName, limit, offset)
	if args.Get(0) == nil {
		return nil, 0, args.Error(2)
	}
	return args.Get(0).([]*Patent), args.Get(1).(int64), args.Error(2)
}

func (m *MockPatentRepository) SearchByAssigneeName(ctx context.Context, assigneeName string, limit, offset int) ([]*Patent, int64, error) {
	args := m.Called(ctx, assigneeName, limit, offset)
	if args.Get(0) == nil {
		return nil, 0, args.Error(2)
	}
	return args.Get(0).([]*Patent), args.Get(1).(int64), args.Error(2)
}

func (m *MockPatentRepository) SetPriorityClaims(ctx context.Context, patentID uuid.UUID, claims []*PriorityClaim) error {
	args := m.Called(ctx, patentID, claims)
	return args.Error(0)
}

func (m *MockPatentRepository) GetPriorityClaims(ctx context.Context, patentID uuid.UUID) ([]*PriorityClaim, error) {
	args := m.Called(ctx, patentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*PriorityClaim), args.Error(1)
}

func (m *MockPatentRepository) BatchCreate(ctx context.Context, patents []*Patent) (int, error) {
	args := m.Called(ctx, patents)
	return args.Int(0), args.Error(1)
}

func (m *MockPatentRepository) BatchUpdateStatus(ctx context.Context, ids []uuid.UUID, status PatentStatus) (int64, error) {
	args := m.Called(ctx, ids, status)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockPatentRepository) CountByStatus(ctx context.Context) (map[PatentStatus]int64, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(map[PatentStatus]int64), args.Error(1)
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

func (m *MockPatentRepository) WithTx(ctx context.Context, fn func(PatentRepository) error) error {
	args := m.Called(ctx, fn)
	return args.Error(0)
}

// MockMarkushRepository is a mock implementation of MarkushRepository
type MockMarkushRepository struct {
	mock.Mock
}

func (m *MockMarkushRepository) FindByPatentID(ctx context.Context, patentID string) ([]*MarkushStructure, error) {
	args := m.Called(ctx, patentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*MarkushStructure), args.Error(1)
}

// MockEventBus is a mock implementation of EventBus
type MockEventBus struct {
	mock.Mock
}

func (m *MockEventBus) Publish(ctx context.Context, events ...common.DomainEvent) error {
	args := m.Called(ctx, events)
	return args.Error(0)
}

// MockLogger is a mock implementation of logging.Logger
type MockLogger struct {
	mock.Mock
}

func (m *MockLogger) Debug(msg string, fields ...logging.Field) {}
func (m *MockLogger) Info(msg string, fields ...logging.Field)  {}
func (m *MockLogger) Warn(msg string, fields ...logging.Field)  {}
func (m *MockLogger) Error(msg string, fields ...logging.Field) {}
func (m *MockLogger) Fatal(msg string, fields ...logging.Field) {}
func (m *MockLogger) With(fields ...logging.Field) logging.Logger {
	return m
}
func (m *MockLogger) WithContext(ctx context.Context) logging.Logger {
	return m
}
func (m *MockLogger) WithError(err error) logging.Logger {
	return m
}
func (m *MockLogger) Sync() error { return nil }

// Test cases

func TestNewPatentService(t *testing.T) {
	mockRepo := new(MockPatentRepository)
	mockMarkush := new(MockMarkushRepository)
	mockEventBus := new(MockEventBus)
	mockLogger := new(MockLogger)

	service := NewPatentService(mockRepo, mockMarkush, mockEventBus, mockLogger)

	assert.NotNil(t, service)
}

func TestNewPatentService_NilRepo_Panics(t *testing.T) {
	mockMarkush := new(MockMarkushRepository)
	mockEventBus := new(MockEventBus)
	mockLogger := new(MockLogger)

	assert.Panics(t, func() {
		NewPatentService(nil, mockMarkush, mockEventBus, mockLogger)
	})
}

func TestCreatePatent_Success(t *testing.T) {
	mockRepo := new(MockPatentRepository)
	mockMarkush := new(MockMarkushRepository)
	mockEventBus := new(MockEventBus)
	mockLogger := new(MockLogger)

	service := NewPatentService(mockRepo, mockMarkush, mockEventBus, mockLogger)

	filingDate := time.Now().UTC()

	// Patent doesn't exist
	mockRepo.On("GetByPatentNumber", mock.Anything, "US12345678").Return(nil, errors.New("not found"))
	mockRepo.On("Create", mock.Anything, mock.AnythingOfType("*patent.Patent")).Return(nil)
	mockEventBus.On("Publish", mock.Anything, mock.Anything).Return(nil)

	result, err := service.CreatePatent(context.Background(), "US12345678", "Test Patent Title", OfficeUSPTO, filingDate)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "US12345678", result.PatentNumber)
	assert.Equal(t, PatentStatusFiled, result.Status)
	mockRepo.AssertExpectations(t)
}

func TestCreatePatent_AlreadyExists(t *testing.T) {
	mockRepo := new(MockPatentRepository)
	mockMarkush := new(MockMarkushRepository)
	mockEventBus := new(MockEventBus)
	mockLogger := new(MockLogger)

	service := NewPatentService(mockRepo, mockMarkush, mockEventBus, mockLogger)

	filingDate := time.Now().UTC()
	existingPatent := &Patent{
		ID:           uuid.New(),
		PatentNumber: "US12345678",
	}

	mockRepo.On("GetByPatentNumber", mock.Anything, "US12345678").Return(existingPatent, nil)

	result, err := service.CreatePatent(context.Background(), "US12345678", "Test Patent Title", OfficeUSPTO, filingDate)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "already exists")
}

func TestGetPatent_Success(t *testing.T) {
	mockRepo := new(MockPatentRepository)
	mockMarkush := new(MockMarkushRepository)
	mockEventBus := new(MockEventBus)
	mockLogger := new(MockLogger)

	service := NewPatentService(mockRepo, mockMarkush, mockEventBus, mockLogger)

	patentID := uuid.New()
	expectedPatent := &Patent{
		ID:           patentID,
		PatentNumber: "US12345678",
		Status:       PatentStatusFiled,
	}

	mockRepo.On("GetByID", mock.Anything, patentID).Return(expectedPatent, nil)

	result, err := service.GetPatent(context.Background(), patentID.String())

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, patentID, result.ID)
	mockRepo.AssertExpectations(t)
}

func TestGetPatent_InvalidID(t *testing.T) {
	mockRepo := new(MockPatentRepository)
	mockMarkush := new(MockMarkushRepository)
	mockEventBus := new(MockEventBus)
	mockLogger := new(MockLogger)

	service := NewPatentService(mockRepo, mockMarkush, mockEventBus, mockLogger)

	result, err := service.GetPatent(context.Background(), "invalid-uuid")

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "invalid UUID")
}

func TestGetPatent_NotFound(t *testing.T) {
	mockRepo := new(MockPatentRepository)
	mockMarkush := new(MockMarkushRepository)
	mockEventBus := new(MockEventBus)
	mockLogger := new(MockLogger)

	service := NewPatentService(mockRepo, mockMarkush, mockEventBus, mockLogger)

	patentID := uuid.New()
	mockRepo.On("GetByID", mock.Anything, patentID).Return(nil, nil)

	result, err := service.GetPatent(context.Background(), patentID.String())

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "not found")
	mockRepo.AssertExpectations(t)
}

func TestGetPatentByNumber_Success(t *testing.T) {
	mockRepo := new(MockPatentRepository)
	mockMarkush := new(MockMarkushRepository)
	mockEventBus := new(MockEventBus)
	mockLogger := new(MockLogger)

	service := NewPatentService(mockRepo, mockMarkush, mockEventBus, mockLogger)

	expectedPatent := &Patent{
		ID:           uuid.New(),
		PatentNumber: "US12345678",
	}

	mockRepo.On("GetByPatentNumber", mock.Anything, "US12345678").Return(expectedPatent, nil)

	result, err := service.GetPatentByNumber(context.Background(), "US12345678")

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "US12345678", result.PatentNumber)
	mockRepo.AssertExpectations(t)
}

func TestSearchPatents_Success(t *testing.T) {
	mockRepo := new(MockPatentRepository)
	mockMarkush := new(MockMarkushRepository)
	mockEventBus := new(MockEventBus)
	mockLogger := new(MockLogger)

	service := NewPatentService(mockRepo, mockMarkush, mockEventBus, mockLogger)

	criteria := SearchQuery{
		Keyword: "OLED",
		Limit:   10,
	}

	expectedResult := &SearchResult{
		Items:      []*Patent{{PatentNumber: "US123"}},
		TotalCount: 1,
	}

	mockRepo.On("Search", mock.Anything, criteria).Return(expectedResult, nil)

	result, err := service.SearchPatents(context.Background(), criteria)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result.Items, 1)
	mockRepo.AssertExpectations(t)
}

func TestUpdatePatentStatus_Publish(t *testing.T) {
	mockRepo := new(MockPatentRepository)
	mockMarkush := new(MockMarkushRepository)
	mockEventBus := new(MockEventBus)
	mockLogger := new(MockLogger)

	service := NewPatentService(mockRepo, mockMarkush, mockEventBus, mockLogger)

	patentID := uuid.New()
	filingDate := time.Now().UTC()
	patent := &Patent{
		ID:           patentID,
		PatentNumber: "US12345678",
		Status:       PatentStatusFiled,
		Dates:        PatentDate{FilingDate: &filingDate},
	}

	mockRepo.On("GetByID", mock.Anything, patentID).Return(patent, nil)
	mockRepo.On("Update", mock.Anything, mock.AnythingOfType("*patent.Patent")).Return(nil)
	mockEventBus.On("Publish", mock.Anything, mock.Anything).Return(nil)

	pubDate := time.Now().UTC()
	result, err := service.UpdatePatentStatus(context.Background(), patentID.String(), PatentStatusPublished, StatusTransitionParams{
		PublicationDate: &pubDate,
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, PatentStatusPublished, result.Status)
	mockRepo.AssertExpectations(t)
}

func TestUpdatePatentStatus_Publish_MissingDate(t *testing.T) {
	mockRepo := new(MockPatentRepository)
	mockMarkush := new(MockMarkushRepository)
	mockEventBus := new(MockEventBus)
	mockLogger := new(MockLogger)

	service := NewPatentService(mockRepo, mockMarkush, mockEventBus, mockLogger)

	patentID := uuid.New()
	filingDate := time.Now().UTC()
	patent := &Patent{
		ID:           patentID,
		PatentNumber: "US12345678",
		Status:       PatentStatusFiled,
		Dates:        PatentDate{FilingDate: &filingDate},
	}

	mockRepo.On("GetByID", mock.Anything, patentID).Return(patent, nil)

	result, err := service.UpdatePatentStatus(context.Background(), patentID.String(), PatentStatusPublished, StatusTransitionParams{})

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "publication date is required")
}

func TestUpdatePatentStatus_Grant(t *testing.T) {
	mockRepo := new(MockPatentRepository)
	mockMarkush := new(MockMarkushRepository)
	mockEventBus := new(MockEventBus)
	mockLogger := new(MockLogger)

	service := NewPatentService(mockRepo, mockMarkush, mockEventBus, mockLogger)

	patentID := uuid.New()
	filingDate := time.Now().UTC()
	patent := &Patent{
		ID:           patentID,
		PatentNumber: "US12345678",
		Status:       PatentStatusUnderExamination,
		Dates:        PatentDate{FilingDate: &filingDate},
	}

	mockRepo.On("GetByID", mock.Anything, patentID).Return(patent, nil)
	mockRepo.On("Update", mock.Anything, mock.AnythingOfType("*patent.Patent")).Return(nil)
	mockEventBus.On("Publish", mock.Anything, mock.Anything).Return(nil)

	grantDate := time.Now().UTC()
	expiryDate := grantDate.AddDate(20, 0, 0)
	result, err := service.UpdatePatentStatus(context.Background(), patentID.String(), PatentStatusGranted, StatusTransitionParams{
		GrantDate:  &grantDate,
		ExpiryDate: &expiryDate,
	})

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, PatentStatusGranted, result.Status)
	mockRepo.AssertExpectations(t)
}

func TestGetPatentsByMoleculeID_Success(t *testing.T) {
	mockRepo := new(MockPatentRepository)
	mockMarkush := new(MockMarkushRepository)
	mockEventBus := new(MockEventBus)
	mockLogger := new(MockLogger)

	service := NewPatentService(mockRepo, mockMarkush, mockEventBus, mockLogger)

	moleculeID := uuid.New().String()
	expectedPatents := []*Patent{
		{PatentNumber: "US123"},
		{PatentNumber: "US456"},
	}

	mockRepo.On("FindByMoleculeID", mock.Anything, moleculeID).Return(expectedPatents, nil)

	results, err := service.GetPatentsByMoleculeID(context.Background(), moleculeID)

	assert.NoError(t, err)
	assert.Len(t, results, 2)
	mockRepo.AssertExpectations(t)
}

func TestSearchBySimilarity_ReturnsEmptyResults(t *testing.T) {
	mockRepo := new(MockPatentRepository)
	mockMarkush := new(MockMarkushRepository)
	mockEventBus := new(MockEventBus)
	mockLogger := new(MockLogger)

	service := NewPatentService(mockRepo, mockMarkush, mockEventBus, mockLogger)

	req := &SimilaritySearchRequest{
		SMILES:     "CCO",
		Threshold:  0.7,
		MaxResults: 10,
	}

	results, err := service.SearchBySimilarity(context.Background(), req)

	assert.NoError(t, err)
	assert.NotNil(t, results)
	assert.Len(t, results, 0) // Dummy implementation returns empty
}

// Additional entity tests

func TestPatentStatus_AllStatuses(t *testing.T) {
	statuses := []struct {
		status   PatentStatus
		expected string
	}{
		{PatentStatusDraft, "draft"},
		{PatentStatusFiled, "filed"},
		{PatentStatusPublished, "published"},
		{PatentStatusUnderExamination, "under_examination"},
		{PatentStatusGranted, "granted"},
		{PatentStatusRejected, "rejected"},
		{PatentStatusWithdrawn, "withdrawn"},
		{PatentStatusExpired, "expired"},
		{PatentStatusInvalidated, "invalidated"},
		{PatentStatusLapsed, "lapsed"},
		{PatentStatusUnknown, "unknown"},
	}

	for _, tc := range statuses {
		assert.Equal(t, tc.expected, tc.status.String())
	}
}

func TestPatentOffice_IsValid(t *testing.T) {
	validOffices := []PatentOffice{OfficeCNIPA, OfficeUSPTO, OfficeEPO, OfficeJPO, OfficeKIPO, OfficeWIPO}
	for _, office := range validOffices {
		assert.True(t, office.IsValid())
	}

	assert.False(t, PatentOffice("INVALID").IsValid())
}

func TestPatent_AddMolecule(t *testing.T) {
	p := &Patent{ID: uuid.New()}
	err := p.AddMolecule("mol-123")
	assert.NoError(t, err)
	assert.Contains(t, p.MoleculeIDs, "mol-123")
}

func TestPatent_AddCitation(t *testing.T) {
	p := &Patent{ID: uuid.New()}
	err := p.AddCitation("US999")
	assert.NoError(t, err)
	assert.Contains(t, p.Cites, "US999")
}

func TestPatent_SetClaims(t *testing.T) {
	p := &Patent{ID: uuid.New()}
	claims := []*Claim{{Number: 1}, {Number: 2}}
	err := p.SetClaims(claims)
	assert.NoError(t, err)
	assert.Len(t, p.Claims, 2)
}

func TestPatent_ClaimCount(t *testing.T) {
	p := &Patent{
		Claims: []*Claim{{Number: 1}, {Number: 2}, {Number: 3}},
	}
	assert.Equal(t, 3, p.ClaimCount())
}

func TestPatent_GetPrimaryTechDomain(t *testing.T) {
	// With KeyIP tech codes
	p := &Patent{KeyIPTechCodes: []string{"OLED", "Emitter"}}
	assert.Equal(t, "OLED", p.GetPrimaryTechDomain())

	// Without KeyIP but with IPC
	p2 := &Patent{IPCCodes: []string{"C09K11/06"}}
	assert.Equal(t, "C09K11/06", p2.GetPrimaryTechDomain())

	// Neither
	p3 := &Patent{}
	assert.Equal(t, "", p3.GetPrimaryTechDomain())
}

func TestPatent_GetValueScore(t *testing.T) {
	// With metadata score
	p := &Patent{Metadata: map[string]any{"value_score": 85.5}}
	assert.Equal(t, 85.5, p.GetValueScore())

	// Without metadata
	p2 := &Patent{}
	assert.Equal(t, 0.0, p2.GetValueScore())
}

func TestPatent_Getters(t *testing.T) {
	id := uuid.New()
	p := &Patent{
		ID:           id,
		PatentNumber: "US123",
		AssigneeName: "Test Corp",
		Status:       PatentStatusGranted,
	}

	assert.Equal(t, id.String(), p.GetID())
	assert.Equal(t, "US123", p.GetPatentNumber())
	assert.Equal(t, "Test Corp", p.GetAssignee())
	assert.Equal(t, "granted", p.GetLegalStatus())
}

func TestPatentDate_RemainingLifeYears(t *testing.T) {
	// No expiry date
	d := PatentDate{}
	assert.Equal(t, 0.0, d.RemainingLifeYears())

	// Already expired
	past := time.Now().UTC().AddDate(-1, 0, 0)
	d2 := PatentDate{ExpiryDate: &past}
	assert.Equal(t, 0.0, d2.RemainingLifeYears())

	// 10 years remaining
	future := time.Now().UTC().AddDate(10, 0, 0)
	d3 := PatentDate{ExpiryDate: &future}
	remaining := d3.RemainingLifeYears()
	assert.Greater(t, remaining, 9.9)
	assert.Less(t, remaining, 10.1)
}

func TestPatentDate_Validate(t *testing.T) {
	// Valid
	now := time.Now()
	d := PatentDate{FilingDate: &now}
	assert.NoError(t, d.Validate())

	// Missing filing date
	d2 := PatentDate{}
	assert.Error(t, d2.Validate())
}

func TestPatent_Reject(t *testing.T) {
	now := time.Now().UTC()
	p, _ := NewPatent("US123", "Test", OfficeUSPTO, now)
	_ = p.Publish(now)
	_ = p.EnterExamination()

	err := p.Reject()
	assert.NoError(t, err)
	assert.Equal(t, PatentStatusRejected, p.Status)
}

func TestPatent_Withdraw(t *testing.T) {
	now := time.Now().UTC()
	p, _ := NewPatent("US123", "Test", OfficeUSPTO, now)

	err := p.Withdraw()
	assert.NoError(t, err)
	assert.Equal(t, PatentStatusWithdrawn, p.Status)

	// Can't withdraw from Granted
	p2, _ := NewPatent("US456", "Test", OfficeUSPTO, now)
	_ = p2.Publish(now)
	_ = p2.EnterExamination()
	_ = p2.Grant(now, now.AddDate(20, 0, 0))

	err = p2.Withdraw()
	assert.Error(t, err)
}

func TestPatent_Invalidate(t *testing.T) {
	now := time.Now().UTC()
	p, _ := NewPatent("US123", "Test", OfficeUSPTO, now)
	_ = p.Publish(now)
	_ = p.EnterExamination()
	_ = p.Grant(now, now.AddDate(20, 0, 0))

	err := p.Invalidate()
	assert.NoError(t, err)
	assert.Equal(t, PatentStatusInvalidated, p.Status)
}

func TestPatent_Lapse(t *testing.T) {
	now := time.Now().UTC()
	p, _ := NewPatent("US123", "Test", OfficeUSPTO, now)
	_ = p.Publish(now)
	_ = p.EnterExamination()
	_ = p.Grant(now, now.AddDate(20, 0, 0))

	err := p.Lapse()
	assert.NoError(t, err)
	assert.Equal(t, PatentStatusLapsed, p.Status)
}
