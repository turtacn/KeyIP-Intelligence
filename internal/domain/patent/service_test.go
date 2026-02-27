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

// Mocks

type MockPatentRepository struct {
	mock.Mock
}

func (m *MockPatentRepository) Save(ctx context.Context, patent *Patent) error {
	args := m.Called(ctx, patent)
	return args.Error(0)
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
	args := m.Called(ctx, id)
	return args.Error(0)
}
func (m *MockPatentRepository) Exists(ctx context.Context, patentNumber string) (bool, error) {
	args := m.Called(ctx, patentNumber)
	return args.Bool(0), args.Error(1)
}
func (m *MockPatentRepository) SaveBatch(ctx context.Context, patents []*Patent) error {
	args := m.Called(ctx, patents)
	return args.Error(0)
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
func (m *MockPatentRepository) FindByMoleculeID(ctx context.Context, moleculeID string) ([]*Patent, error) {
	args := m.Called(ctx, moleculeID)
	return args.Get(0).([]*Patent), args.Error(1)
}
func (m *MockPatentRepository) FindByFamilyID(ctx context.Context, familyID string) ([]*Patent, error) {
	args := m.Called(ctx, familyID)
	return args.Get(0).([]*Patent), args.Error(1)
}
func (m *MockPatentRepository) GetByFamilyID(ctx context.Context, familyID string) ([]*Patent, error) {
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
func (m *MockPatentRepository) SearchByAssigneeName(ctx context.Context, assigneeName string, limit, offset int) ([]*Patent, int64, error) {
	args := m.Called(ctx, assigneeName, limit, offset)
	return args.Get(0).([]*Patent), args.Get(1).(int64), args.Error(2)
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
func (m *MockPatentRepository) CountByJurisdiction(ctx context.Context) (map[string]int64, error) {
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
func (m *MockPatentRepository) AssociateMolecule(ctx context.Context, patentID string, moleculeID string) error {
	args := m.Called(ctx, patentID, moleculeID)
	return args.Error(0)
}
func (m *MockPatentRepository) GetByID(ctx context.Context, id uuid.UUID) (*Patent, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(*Patent), args.Error(1)
}
func (m *MockPatentRepository) GetByPatentNumber(ctx context.Context, number string) (*Patent, error) {
	args := m.Called(ctx, number)
	return args.Get(0).(*Patent), args.Error(1)
}
func (m *MockPatentRepository) ListByPortfolio(ctx context.Context, portfolioID string) ([]*Patent, error) {
	args := m.Called(ctx, portfolioID)
	return args.Get(0).([]*Patent), args.Error(1)
}

type MockMarkushRepository struct {
	mock.Mock
}

func (m *MockMarkushRepository) Save(ctx context.Context, markush *MarkushStructure) error {
	args := m.Called(ctx, markush)
	return args.Error(0)
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
	args := m.Called(ctx, id)
	return args.Error(0)
}
func (m *MockMarkushRepository) CountByPatentID(ctx context.Context, patentID string) (int64, error) {
	args := m.Called(ctx, patentID)
	return args.Get(0).(int64), args.Error(1)
}

type MockEventBus struct {
	mock.Mock
}

func (m *MockEventBus) Publish(ctx context.Context, events ...common.DomainEvent) error {
	args := m.Called(ctx, events)
	return args.Error(0)
}
func (m *MockEventBus) Subscribe(handler EventHandler) error {
	args := m.Called(handler)
	return args.Error(0)
}
func (m *MockEventBus) Unsubscribe(handler EventHandler) error {
	args := m.Called(handler)
	return args.Error(0)
}

func TestNewPatentService_Success(t *testing.T) {
	repo := new(MockPatentRepository)
	markushRepo := new(MockMarkushRepository)
	bus := new(MockEventBus)
	logger := logging.NewNopLogger()

	svc := NewPatentService(repo, markushRepo, bus, logger)
	assert.NotNil(t, svc)
}

func TestPatentService_CreatePatent_Success(t *testing.T) {
	repo := new(MockPatentRepository)
	markushRepo := new(MockMarkushRepository)
	bus := new(MockEventBus)
	logger := logging.NewNopLogger()
	svc := NewPatentService(repo, markushRepo, bus, logger)

	ctx := context.Background()

	repo.On("Exists", ctx, "CN123").Return(false, nil)
	repo.On("Save", ctx, mock.AnythingOfType("*patent.Patent")).Return(nil)
	bus.On("Publish", ctx, mock.Anything).Return(nil)

	p, err := svc.CreatePatent(ctx, "CN123", "Title", OfficeCNIPA, time.Now())
	assert.NoError(t, err)
	assert.NotNil(t, p)
	repo.AssertExpectations(t)
	bus.AssertExpectations(t)
}

func TestPatentService_CreatePatent_AlreadyExists(t *testing.T) {
	repo := new(MockPatentRepository)
	markushRepo := new(MockMarkushRepository)
	bus := new(MockEventBus)
	logger := logging.NewNopLogger()
	svc := NewPatentService(repo, markushRepo, bus, logger)

	ctx := context.Background()
	repo.On("Exists", ctx, "CN123").Return(true, nil)

	_, err := svc.CreatePatent(ctx, "CN123", "Title", OfficeCNIPA, time.Now())
	assert.Error(t, err)
}

func TestPatentService_GetPatent_Success(t *testing.T) {
	repo := new(MockPatentRepository)
	markushRepo := new(MockMarkushRepository)
	svc := NewPatentService(repo, markushRepo, nil, logging.NewNopLogger())

	ctx := context.Background()
	existing := &Patent{PatentNumber: "CN123"}
	repo.On("FindByID", ctx, "ID1").Return(existing, nil)

	p, err := svc.GetPatent(ctx, "ID1")
	assert.NoError(t, err)
	assert.Equal(t, "CN123", p.PatentNumber)
}

func TestPatentService_GetPatent_NotFound(t *testing.T) {
	repo := new(MockPatentRepository)
	markushRepo := new(MockMarkushRepository)
	svc := NewPatentService(repo, markushRepo, nil, logging.NewNopLogger())

	ctx := context.Background()
	repo.On("FindByID", ctx, "ID1").Return(nil, nil)

	_, err := svc.GetPatent(ctx, "ID1")
	assert.Error(t, err)
}

func TestPatentService_SearchPatents_Success(t *testing.T) {
	repo := new(MockPatentRepository)
	markushRepo := new(MockMarkushRepository)
	svc := NewPatentService(repo, markushRepo, nil, logging.NewNopLogger())

	ctx := context.Background()
	criteria := PatentSearchCriteria{Limit: 10}
	result := &PatentSearchResult{Total: 1}
	repo.On("Search", ctx, criteria).Return(result, nil)

	res, err := svc.SearchPatents(ctx, criteria)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), res.Total)
}

func TestPatentService_UpdatePatentStatus_Publish_Success(t *testing.T) {
	repo := new(MockPatentRepository)
	markushRepo := new(MockMarkushRepository)
	bus := new(MockEventBus)
	svc := NewPatentService(repo, markushRepo, bus, logging.NewNopLogger())

	ctx := context.Background()
	p, _ := NewPatent("CN123", "Title", OfficeCNIPA, time.Now())
	p.ID.String() // ensure ID generated

	repo.On("FindByID", ctx, p.ID.String()).Return(p, nil)
	repo.On("Save", ctx, p).Return(nil)
	bus.On("Publish", ctx, mock.Anything).Return(nil)

	now := time.Now().UTC()
	updated, err := svc.UpdatePatentStatus(ctx, p.ID.String(), PatentStatusPublished, StatusTransitionParams{PublicationDate: &now})
	assert.NoError(t, err)
	assert.Equal(t, PatentStatusPublished, updated.Status)
}

func TestPatentService_SetPatentClaims_Success(t *testing.T) {
	repo := new(MockPatentRepository)
	markushRepo := new(MockMarkushRepository)
	bus := new(MockEventBus)
	svc := NewPatentService(repo, markushRepo, bus, logging.NewNopLogger())

	ctx := context.Background()
	p, _ := NewPatent("CN123", "Title", OfficeCNIPA, time.Now())

	repo.On("FindByID", ctx, p.ID.String()).Return(p, nil)
	repo.On("Save", ctx, p).Return(nil)
	bus.On("Publish", ctx, mock.Anything).Return(nil)

	c1, err := NewClaim(1, "Claim 1 text must be longer than 10 chars", ClaimTypeIndependent, ClaimCategoryProduct)
	assert.NoError(t, err)
	if c1 == nil {
		t.Fatal("Failed to create claim")
	}
	claims := ClaimSet{*c1}

	updated, err := svc.SetPatentClaims(ctx, p.ID.String(), claims)
	assert.NoError(t, err)
	assert.Len(t, updated.Claims, 1)
}

func TestPatentService_LinkMolecule_Success(t *testing.T) {
	repo := new(MockPatentRepository)
	markushRepo := new(MockMarkushRepository)
	bus := new(MockEventBus)
	svc := NewPatentService(repo, markushRepo, bus, logging.NewNopLogger())

	ctx := context.Background()
	p, _ := NewPatent("CN123", "Title", OfficeCNIPA, time.Now())

	repo.On("FindByID", ctx, p.ID.String()).Return(p, nil)
	repo.On("Save", ctx, p).Return(nil)
	bus.On("Publish", ctx, mock.Anything).Return(nil)

	err := svc.LinkMolecule(ctx, p.ID.String(), "MOL1")
	assert.NoError(t, err)
	assert.Contains(t, p.MoleculeIDs, "MOL1")
}

func TestPatentService_AnalyzeMarkushCoverage_Success(t *testing.T) {
	repo := new(MockPatentRepository)
	markushRepo := new(MockMarkushRepository)
	svc := NewPatentService(repo, markushRepo, nil, logging.NewNopLogger())

	ctx := context.Background()
	ms, _ := NewMarkushStructure("M1", "Core", 1)
	ms.PreferredExamples = []string{"C1"}

	markushRepo.On("FindByPatentID", ctx, "PID").Return([]*MarkushStructure{ms}, nil)

	analysis, err := svc.AnalyzeMarkushCoverage(ctx, "PID", []string{"C1", "C2"})
	assert.NoError(t, err)
	assert.Equal(t, 0.5, analysis.CoverageRate)
	assert.Equal(t, 1, analysis.MatchedMolecules)
}

func TestPatentService_FindRelatedPatents_Success(t *testing.T) {
	repo := new(MockPatentRepository)
	markushRepo := new(MockMarkushRepository)
	svc := NewPatentService(repo, markushRepo, nil, logging.NewNopLogger())

	ctx := context.Background()
	p, _ := NewPatent("CN123", "Title", OfficeCNIPA, time.Now())
	p.FamilyID = "FAM1"

	relatedP, _ := NewPatent("US123", "Related", OfficeUSPTO, time.Now())
	relatedP.FamilyID = "FAM1"

	repo.On("FindByID", ctx, p.ID.String()).Return(p, nil)
	repo.On("FindByFamilyID", ctx, "FAM1").Return([]*Patent{p, relatedP}, nil)
	repo.On("FindCiting", ctx, "CN123").Return([]*Patent{}, nil)
	repo.On("FindCitedBy", ctx, "CN123").Return([]*Patent{}, nil)

	related, err := svc.FindRelatedPatents(ctx, p.ID.String())
	assert.NoError(t, err)
	assert.Len(t, related, 1)
	assert.Equal(t, "US123", related[0].PatentNumber)
}

func TestPatentService_GetPatentStatistics_Success(t *testing.T) {
	repo := new(MockPatentRepository)
	markushRepo := new(MockMarkushRepository)
	svc := NewPatentService(repo, markushRepo, nil, logging.NewNopLogger())

	ctx := context.Background()
	repo.On("CountByStatus", ctx).Return(map[PatentStatus]int64{PatentStatusGranted: 5}, nil)
	repo.On("CountByOffice", ctx).Return(map[PatentOffice]int64{OfficeCNIPA: 5}, nil)
	repo.On("CountByIPCSection", ctx).Return(map[string]int64{"C": 5}, nil)
	repo.On("CountByYear", ctx, "filing_date").Return(map[int]int64{2023: 5}, nil)
	repo.On("CountByYear", ctx, "grant_date").Return(map[int]int64{2024: 5}, nil)
	repo.On("FindExpiringBefore", ctx, mock.Anything).Return([]*Patent{}, nil)

	stats, err := svc.GetPatentStatistics(ctx)
	assert.NoError(t, err)
	assert.Equal(t, int64(5), stats.TotalCount)
	assert.Equal(t, int64(5), stats.ActiveCount)
}

func TestPatentService_BatchImportPatents_AllSuccess(t *testing.T) {
	repo := new(MockPatentRepository)
	markushRepo := new(MockMarkushRepository)
	bus := new(MockEventBus)
	svc := NewPatentService(repo, markushRepo, bus, logging.NewNopLogger())

	ctx := context.Background()
	p1, _ := NewPatent("CN1", "T1", OfficeCNIPA, time.Now())
	p2, _ := NewPatent("CN2", "T2", OfficeCNIPA, time.Now())

	repo.On("Exists", ctx, "CN1").Return(false, nil)
	repo.On("Exists", ctx, "CN2").Return(false, nil)
	repo.On("SaveBatch", ctx, mock.MatchedBy(func(ps []*Patent) bool { return len(ps) == 2 })).Return(nil)
	bus.On("Publish", ctx, mock.Anything).Return(nil) // For p1
	// bus.On("Publish", ctx, mock.Anything).Return(nil) // For p2 - mock might need looser matching or specific call count

	res, err := svc.BatchImportPatents(ctx, []*Patent{p1, p2})
	assert.NoError(t, err)
	assert.Equal(t, 2, res.SuccessCount)
}

func TestPatentService_BatchImportPatents_PartialSkip(t *testing.T) {
	repo := new(MockPatentRepository)
	markushRepo := new(MockMarkushRepository)
	bus := new(MockEventBus)
	svc := NewPatentService(repo, markushRepo, bus, logging.NewNopLogger())

	ctx := context.Background()
	p1, _ := NewPatent("CN1", "T1", OfficeCNIPA, time.Now())
	p2, _ := NewPatent("CN2", "T2", OfficeCNIPA, time.Now())

	repo.On("Exists", ctx, "CN1").Return(false, nil)
	repo.On("Exists", ctx, "CN2").Return(true, nil)
	repo.On("SaveBatch", ctx, mock.MatchedBy(func(ps []*Patent) bool { return len(ps) == 1 })).Return(nil)
	bus.On("Publish", ctx, mock.Anything).Return(nil)

	res, err := svc.BatchImportPatents(ctx, []*Patent{p1, p2})
	assert.NoError(t, err)
	assert.Equal(t, 1, res.SuccessCount)
	assert.Equal(t, 1, res.SkippedCount)
}

func TestPatentService_BatchImportPatents_Error(t *testing.T) {
	repo := new(MockPatentRepository)
	markushRepo := new(MockMarkushRepository)
	svc := NewPatentService(repo, markushRepo, nil, logging.NewNopLogger())

	ctx := context.Background()
	p1, _ := NewPatent("CN1", "T1", OfficeCNIPA, time.Now())

	repo.On("Exists", ctx, "CN1").Return(false, errors.New("db error"))

	res, err := svc.BatchImportPatents(ctx, []*Patent{p1})
	assert.NoError(t, err) // Batch returns result object even on individual failures
	assert.Equal(t, 1, res.FailedCount)
	assert.Equal(t, "db error", res.Errors[0].Error)
}
