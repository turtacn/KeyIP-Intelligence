package patent

import (
	"context"
	"testing"
	"time"

	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// Mocks

type MockPatentRepository struct {
	SaveFunc                   func(ctx context.Context, patent *Patent) error
	FindByIDFunc               func(ctx context.Context, id string) (*Patent, error)
	FindByPatentNumberFunc     func(ctx context.Context, patentNumber string) (*Patent, error)
	DeleteFunc                 func(ctx context.Context, id string) error
	ExistsFunc                 func(ctx context.Context, patentNumber string) (bool, error)
	SaveBatchFunc              func(ctx context.Context, patents []*Patent) error
	SearchFunc                 func(ctx context.Context, criteria PatentSearchCriteria) (*PatentSearchResult, error)
	FindByMoleculeIDFunc       func(ctx context.Context, moleculeID string) ([]*Patent, error)
	FindByFamilyIDFunc         func(ctx context.Context, familyID string) ([]*Patent, error)
	CountByStatusFunc          func(ctx context.Context) (map[PatentStatus]int64, error)
	CountByOfficeFunc          func(ctx context.Context) (map[PatentOffice]int64, error)
	CountByIPCSectionFunc      func(ctx context.Context) (map[string]int64, error)
	CountByYearFunc            func(ctx context.Context, field string) (map[int]int64, error)
	FindExpiringBeforeFunc     func(ctx context.Context, date time.Time) ([]*Patent, error)
	FindWithMarkushStructuresFunc func(ctx context.Context, offset, limit int) ([]*Patent, error)
}

func (m *MockPatentRepository) Save(ctx context.Context, patent *Patent) error                      { return m.SaveFunc(ctx, patent) }
func (m *MockPatentRepository) FindByID(ctx context.Context, id string) (*Patent, error)             { return m.FindByIDFunc(ctx, id) }
func (m *MockPatentRepository) FindByPatentNumber(ctx context.Context, patentNumber string) (*Patent, error) { return m.FindByPatentNumberFunc(ctx, patentNumber) }
func (m *MockPatentRepository) Delete(ctx context.Context, id string) error                        { return m.DeleteFunc(ctx, id) }
func (m *MockPatentRepository) Exists(ctx context.Context, patentNumber string) (bool, error)        { return m.ExistsFunc(ctx, patentNumber) }
func (m *MockPatentRepository) SaveBatch(ctx context.Context, patents []*Patent) error              { return m.SaveBatchFunc(ctx, patents) }
func (m *MockPatentRepository) FindByIDs(ctx context.Context, ids []string) ([]*Patent, error)       { return nil, nil }
func (m *MockPatentRepository) FindByPatentNumbers(ctx context.Context, numbers []string) ([]*Patent, error) { return nil, nil }
func (m *MockPatentRepository) Search(ctx context.Context, criteria PatentSearchCriteria) (*PatentSearchResult, error) { return m.SearchFunc(ctx, criteria) }
func (m *MockPatentRepository) FindByMoleculeID(ctx context.Context, moleculeID string) ([]*Patent, error) { return m.FindByMoleculeIDFunc(ctx, moleculeID) }
func (m *MockPatentRepository) FindByFamilyID(ctx context.Context, familyID string) ([]*Patent, error) { return m.FindByFamilyIDFunc(ctx, familyID) }
func (m *MockPatentRepository) FindByIPCCode(ctx context.Context, ipcCode string) ([]*Patent, error) { return nil, nil }
func (m *MockPatentRepository) FindByApplicant(ctx context.Context, applicantName string) ([]*Patent, error) { return nil, nil }
func (m *MockPatentRepository) FindCitedBy(ctx context.Context, patentNumber string) ([]*Patent, error) { return nil, nil }
func (m *MockPatentRepository) FindCiting(ctx context.Context, patentNumber string) ([]*Patent, error) { return nil, nil }
func (m *MockPatentRepository) CountByStatus(ctx context.Context) (map[PatentStatus]int64, error)   { return m.CountByStatusFunc(ctx) }
func (m *MockPatentRepository) CountByOffice(ctx context.Context) (map[PatentOffice]int64, error)   { return m.CountByOfficeFunc(ctx) }
func (m *MockPatentRepository) CountByIPCSection(ctx context.Context) (map[string]int64, error)     { return m.CountByIPCSectionFunc(ctx) }
func (m *MockPatentRepository) CountByYear(ctx context.Context, field string) (map[int]int64, error) { return m.CountByYearFunc(ctx, field) }
func (m *MockPatentRepository) FindExpiringBefore(ctx context.Context, date time.Time) ([]*Patent, error) { return m.FindExpiringBeforeFunc(ctx, date) }
func (m *MockPatentRepository) FindActiveByIPCCode(ctx context.Context, ipcCode string) ([]*Patent, error) { return nil, nil }
func (m *MockPatentRepository) FindWithMarkushStructures(ctx context.Context, offset, limit int) ([]*Patent, error) { return m.FindWithMarkushStructuresFunc(ctx, offset, limit) }

type MockMarkushRepository struct {
	FindByPatentIDFunc func(ctx context.Context, patentID string) ([]*MarkushStructure, error)
}

func (m *MockMarkushRepository) Save(ctx context.Context, markush *MarkushStructure) error           { return nil }
func (m *MockMarkushRepository) FindByID(ctx context.Context, id string) (*MarkushStructure, error)  { return nil, nil }
func (m *MockMarkushRepository) FindByPatentID(ctx context.Context, patentID string) ([]*MarkushStructure, error) { return m.FindByPatentIDFunc(ctx, patentID) }
func (m *MockMarkushRepository) FindByClaimNumber(ctx context.Context, patentID string, claimNumber int) ([]*MarkushStructure, error) { return nil, nil }
func (m *MockMarkushRepository) FindMatchingMolecule(ctx context.Context, smiles string) ([]*MarkushStructure, error) { return nil, nil }
func (m *MockMarkushRepository) Delete(ctx context.Context, id string) error                         { return nil }
func (m *MockMarkushRepository) CountByPatentID(ctx context.Context, patentID string) (int64, error)  { return 0, nil }

type MockEventBus struct {
	PublishedEvents []DomainEvent
}

func (m *MockEventBus) Publish(ctx context.Context, events ...DomainEvent) error {
	m.PublishedEvents = append(m.PublishedEvents, events...)
	return nil
}
func (m *MockEventBus) Subscribe(handler EventHandler) error   { return nil }
func (m *MockEventBus) Unsubscribe(handler EventHandler) error { return nil }

// Tests

func TestPatentService_CreatePatent_Success(t *testing.T) {
	mockRepo := &MockPatentRepository{
		ExistsFunc: func(ctx context.Context, patentNumber string) (bool, error) { return false, nil },
		SaveFunc:   func(ctx context.Context, patent *Patent) error { return nil },
	}
	mockMarkush := &MockMarkushRepository{}
	mockBus := &MockEventBus{}
	service := NewPatentService(mockRepo, mockMarkush, mockBus, logging.NewNopLogger())

	p, err := service.CreatePatent(context.Background(), "CN123", "OLED", OfficeCNIPA, time.Now())
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if p.PatentNumber != "CN123" {
		t.Error("PatentNumber mismatch")
	}
	if len(mockBus.PublishedEvents) != 1 {
		t.Error("Expected 1 published event")
	}
}

func TestPatentService_CreatePatent_AlreadyExists(t *testing.T) {
	mockRepo := &MockPatentRepository{
		ExistsFunc: func(ctx context.Context, patentNumber string) (bool, error) { return true, nil },
	}
	mockMarkush := &MockMarkushRepository{}
	service := NewPatentService(mockRepo, mockMarkush, nil, logging.NewNopLogger())

	_, err := service.CreatePatent(context.Background(), "CN123", "OLED", OfficeCNIPA, time.Now())
	if err == nil {
		t.Error("Expected error for existing patent")
	}
}

func TestPatentService_UpdatePatentStatus_Publish_Success(t *testing.T) {
	patent := &Patent{ID: "P1", Status: PatentStatusFiled}
	mockRepo := &MockPatentRepository{
		FindByIDFunc: func(ctx context.Context, id string) (*Patent, error) { return patent, nil },
		SaveFunc:     func(ctx context.Context, patent *Patent) error { return nil },
	}
	mockBus := &MockEventBus{}
	service := NewPatentService(mockRepo, &MockMarkushRepository{}, mockBus, logging.NewNopLogger())

	now := time.Now()
	_, err := service.UpdatePatentStatus(context.Background(), "P1", PatentStatusPublished, StatusTransitionParams{PublicationDate: &now})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if patent.Status != PatentStatusPublished {
		t.Error("Status not updated")
	}
	if len(mockBus.PublishedEvents) != 1 || mockBus.PublishedEvents[0].EventType() != EventPatentPublished {
		t.Error("Wrong event published")
	}
}

func TestPatentService_SetPatentClaims(t *testing.T) {
	p := &Patent{ID: "P1"}
	mockRepo := &MockPatentRepository{
		FindByIDFunc: func(ctx context.Context, id string) (*Patent, error) { return p, nil },
		SaveFunc:     func(ctx context.Context, patent *Patent) error { return nil },
	}
	service := NewPatentService(mockRepo, &MockMarkushRepository{}, &MockEventBus{}, logging.NewNopLogger())

	claims := ClaimSet{
		{Number: 1, Text: "Valid claim text must be long enough.", Type: ClaimTypeIndependent, Category: ClaimCategoryProduct},
	}
	_, err := service.SetPatentClaims(context.Background(), "P1", claims)
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if p.ClaimCount() != 1 {
		t.Error("Claims not updated")
	}
}

func TestPatentService_AnalyzeMarkushCoverage(t *testing.T) {
	mockMarkush := &MockMarkushRepository{
		FindByPatentIDFunc: func(ctx context.Context, patentID string) ([]*MarkushStructure, error) {
			m, _ := NewMarkushStructure("T1", "C[R1]", 1)
			m.PreferredExamples = []string{"CC"}
			return []*MarkushStructure{m}, nil
		},
	}
	service := NewPatentService(&MockPatentRepository{}, mockMarkush, nil, logging.NewNopLogger())

	analysis, err := service.AnalyzeMarkushCoverage(context.Background(), "P1", []string{"CC", "CCC"})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if analysis.MatchedMolecules != 1 {
		t.Errorf("Expected 1 match, got %d", analysis.MatchedMolecules)
	}
	if analysis.CoverageRate != 0.5 {
		t.Errorf("Expected 0.5 coverage, got %f", analysis.CoverageRate)
	}
}

func TestPatentService_GetPatentStatistics(t *testing.T) {
	mockRepo := &MockPatentRepository{
		CountByStatusFunc: func(ctx context.Context) (map[PatentStatus]int64, error) {
			return map[PatentStatus]int64{
				PatentStatusGranted: 10,
				PatentStatusFiled:   20,
			}, nil
		},
		CountByOfficeFunc:             func(ctx context.Context) (map[PatentOffice]int64, error) { return nil, nil },
		CountByIPCSectionFunc:         func(ctx context.Context) (map[string]int64, error) { return nil, nil },
		CountByYearFunc:               func(ctx context.Context, field string) (map[int]int64, error) { return nil, nil },
		FindExpiringBeforeFunc:        func(ctx context.Context, date time.Time) ([]*Patent, error) { return nil, nil },
		FindWithMarkushStructuresFunc: func(ctx context.Context, offset, limit int) ([]*Patent, error) { return nil, nil },
	}
	service := NewPatentService(mockRepo, &MockMarkushRepository{}, nil, logging.NewNopLogger())

	stats, err := service.GetPatentStatistics(context.Background())
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if stats.TotalCount != 30 {
		t.Errorf("Expected 30 total, got %d", stats.TotalCount)
	}
	if stats.ActiveCount != 30 {
		t.Errorf("Expected 30 active, got %d", stats.ActiveCount)
	}
}

func TestPatentService_BatchImportPatents(t *testing.T) {
	mockRepo := &MockPatentRepository{
		ExistsFunc:    func(ctx context.Context, patentNumber string) (bool, error) { return false, nil },
		SaveBatchFunc: func(ctx context.Context, patents []*Patent) error { return nil },
	}
	service := NewPatentService(mockRepo, &MockMarkushRepository{}, &MockEventBus{}, logging.NewNopLogger())

	filingDate := time.Now()
	p1, _ := NewPatent("CN1", "T1", OfficeCNIPA, filingDate)
	p1.Applicants = []Applicant{{Name: "A1", Country: "CN", Type: "company"}}
	p1.Inventors = []Inventor{{Name: "I1", Country: "CN"}}

	p2, _ := NewPatent("CN2", "T2", OfficeCNIPA, filingDate)
	// p2 missing applicants, should fail validation

	result, err := service.BatchImportPatents(context.Background(), []*Patent{p1, p2})
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if result.SuccessCount != 1 {
		t.Errorf("Expected 1 success, got %d", result.SuccessCount)
	}
	if result.FailedCount != 1 {
		t.Errorf("Expected 1 failure, got %d", result.FailedCount)
	}
}

func TestNewPatentService_Panic(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("The code did not panic")
		}
	}()
	NewPatentService(nil, nil, nil, nil)
}

func TestErrors_Patent(t *testing.T) {
	err := errors.ErrPatentNotFound("CN123")
	if !errors.IsCode(err, errors.ErrCodePatentNotFound) {
		t.Error("Wrong error code")
	}
}

//Personal.AI order the ending
