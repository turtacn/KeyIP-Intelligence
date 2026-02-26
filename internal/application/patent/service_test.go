package patent

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	domainPatent "github.com/turtacn/KeyIP-Intelligence/internal/domain/patent"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
)

// --- Mock implementations ---

type mockPatentRepository struct {
	createFn                func(ctx context.Context, p *domainPatent.Patent) error
	getByIDFn               func(ctx context.Context, id uuid.UUID) (*domainPatent.Patent, error)
	getByPatentNumberFn     func(ctx context.Context, number string) (*domainPatent.Patent, error)
	updateFn                func(ctx context.Context, p *domainPatent.Patent) error
	softDeleteFn            func(ctx context.Context, id uuid.UUID) error
	restoreFn               func(ctx context.Context, id uuid.UUID) error
	hardDeleteFn            func(ctx context.Context, id uuid.UUID) error
	searchFn                func(ctx context.Context, query domainPatent.SearchQuery) (*domainPatent.SearchResult, error)
	listByPortfolioFn       func(ctx context.Context, portfolioID string) ([]*domainPatent.Patent, error)
	getByFamilyIDFn         func(ctx context.Context, familyID string) ([]*domainPatent.Patent, error)
	getByAssigneeFn         func(ctx context.Context, assigneeID uuid.UUID, limit, offset int) ([]*domainPatent.Patent, int64, error)
	getByJurisdictionFn     func(ctx context.Context, jurisdiction string, limit, offset int) ([]*domainPatent.Patent, int64, error)
	getExpiringPatentsFn    func(ctx context.Context, daysAhead int, limit, offset int) ([]*domainPatent.Patent, int64, error)
	findDuplicatesFn        func(ctx context.Context, fullTextHash string) ([]*domainPatent.Patent, error)
	findByMoleculeIDFn      func(ctx context.Context, moleculeID string) ([]*domainPatent.Patent, error)
	associateMoleculeFn     func(ctx context.Context, patentID string, moleculeID string) error
	createClaimFn           func(ctx context.Context, claim *domainPatent.Claim) error
	getClaimsByPatentFn     func(ctx context.Context, patentID uuid.UUID) ([]*domainPatent.Claim, error)
	updateClaimFn           func(ctx context.Context, claim *domainPatent.Claim) error
	deleteClaimsByPatentFn  func(ctx context.Context, patentID uuid.UUID) error
	batchCreateClaimsFn     func(ctx context.Context, claims []*domainPatent.Claim) error
	getIndependentClaimsFn  func(ctx context.Context, patentID uuid.UUID) ([]*domainPatent.Claim, error)
	setInventorsFn          func(ctx context.Context, patentID uuid.UUID, inventors []*domainPatent.Inventor) error
	getInventorsFn          func(ctx context.Context, patentID uuid.UUID) ([]*domainPatent.Inventor, error)
	searchByInventorFn      func(ctx context.Context, inventorName string, limit, offset int) ([]*domainPatent.Patent, int64, error)
	searchByAssigneeNameFn  func(ctx context.Context, assigneeName string, limit, offset int) ([]*domainPatent.Patent, int64, error)
	setPriorityClaimsFn     func(ctx context.Context, patentID uuid.UUID, claims []*domainPatent.PriorityClaim) error
	getPriorityClaimsFn     func(ctx context.Context, patentID uuid.UUID) ([]*domainPatent.PriorityClaim, error)
	batchCreateFn           func(ctx context.Context, patents []*domainPatent.Patent) (int, error)
	batchUpdateStatusFn     func(ctx context.Context, ids []uuid.UUID, status domainPatent.PatentStatus) (int64, error)
	countByStatusFn         func(ctx context.Context) (map[domainPatent.PatentStatus]int64, error)
	countByJurisdictionFn   func(ctx context.Context) (map[string]int64, error)
	countByYearFn           func(ctx context.Context, field string) (map[int]int64, error)
	getIPCDistributionFn    func(ctx context.Context, level int) (map[string]int64, error)
	withTxFn                func(ctx context.Context, fn func(domainPatent.PatentRepository) error) error
}

func (m *mockPatentRepository) Create(ctx context.Context, p *domainPatent.Patent) error {
	if m.createFn != nil {
		return m.createFn(ctx, p)
	}
	return nil
}

func (m *mockPatentRepository) GetByID(ctx context.Context, id uuid.UUID) (*domainPatent.Patent, error) {
	if m.getByIDFn != nil {
		return m.getByIDFn(ctx, id)
	}
	return nil, errors.New("not found")
}

func (m *mockPatentRepository) GetByPatentNumber(ctx context.Context, number string) (*domainPatent.Patent, error) {
	if m.getByPatentNumberFn != nil {
		return m.getByPatentNumberFn(ctx, number)
	}
	return nil, nil
}

func (m *mockPatentRepository) Update(ctx context.Context, p *domainPatent.Patent) error {
	if m.updateFn != nil {
		return m.updateFn(ctx, p)
	}
	return nil
}

func (m *mockPatentRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	if m.softDeleteFn != nil {
		return m.softDeleteFn(ctx, id)
	}
	return nil
}

func (m *mockPatentRepository) Restore(ctx context.Context, id uuid.UUID) error {
	if m.restoreFn != nil {
		return m.restoreFn(ctx, id)
	}
	return nil
}

func (m *mockPatentRepository) HardDelete(ctx context.Context, id uuid.UUID) error {
	if m.hardDeleteFn != nil {
		return m.hardDeleteFn(ctx, id)
	}
	return nil
}

func (m *mockPatentRepository) Search(ctx context.Context, query domainPatent.SearchQuery) (*domainPatent.SearchResult, error) {
	if m.searchFn != nil {
		return m.searchFn(ctx, query)
	}
	return &domainPatent.SearchResult{}, nil
}

func (m *mockPatentRepository) ListByPortfolio(ctx context.Context, portfolioID string) ([]*domainPatent.Patent, error) {
	if m.listByPortfolioFn != nil {
		return m.listByPortfolioFn(ctx, portfolioID)
	}
	return nil, nil
}

func (m *mockPatentRepository) GetByFamilyID(ctx context.Context, familyID string) ([]*domainPatent.Patent, error) {
	if m.getByFamilyIDFn != nil {
		return m.getByFamilyIDFn(ctx, familyID)
	}
	return nil, nil
}

func (m *mockPatentRepository) GetByAssignee(ctx context.Context, assigneeID uuid.UUID, limit, offset int) ([]*domainPatent.Patent, int64, error) {
	if m.getByAssigneeFn != nil {
		return m.getByAssigneeFn(ctx, assigneeID, limit, offset)
	}
	return nil, 0, nil
}

func (m *mockPatentRepository) GetByJurisdiction(ctx context.Context, jurisdiction string, limit, offset int) ([]*domainPatent.Patent, int64, error) {
	if m.getByJurisdictionFn != nil {
		return m.getByJurisdictionFn(ctx, jurisdiction, limit, offset)
	}
	return nil, 0, nil
}

func (m *mockPatentRepository) GetExpiringPatents(ctx context.Context, daysAhead int, limit, offset int) ([]*domainPatent.Patent, int64, error) {
	if m.getExpiringPatentsFn != nil {
		return m.getExpiringPatentsFn(ctx, daysAhead, limit, offset)
	}
	return nil, 0, nil
}

func (m *mockPatentRepository) FindDuplicates(ctx context.Context, fullTextHash string) ([]*domainPatent.Patent, error) {
	if m.findDuplicatesFn != nil {
		return m.findDuplicatesFn(ctx, fullTextHash)
	}
	return nil, nil
}

func (m *mockPatentRepository) FindByMoleculeID(ctx context.Context, moleculeID string) ([]*domainPatent.Patent, error) {
	if m.findByMoleculeIDFn != nil {
		return m.findByMoleculeIDFn(ctx, moleculeID)
	}
	return nil, nil
}

func (m *mockPatentRepository) AssociateMolecule(ctx context.Context, patentID string, moleculeID string) error {
	if m.associateMoleculeFn != nil {
		return m.associateMoleculeFn(ctx, patentID, moleculeID)
	}
	return nil
}

func (m *mockPatentRepository) CreateClaim(ctx context.Context, claim *domainPatent.Claim) error {
	if m.createClaimFn != nil {
		return m.createClaimFn(ctx, claim)
	}
	return nil
}

func (m *mockPatentRepository) GetClaimsByPatent(ctx context.Context, patentID uuid.UUID) ([]*domainPatent.Claim, error) {
	if m.getClaimsByPatentFn != nil {
		return m.getClaimsByPatentFn(ctx, patentID)
	}
	return nil, nil
}

func (m *mockPatentRepository) UpdateClaim(ctx context.Context, claim *domainPatent.Claim) error {
	if m.updateClaimFn != nil {
		return m.updateClaimFn(ctx, claim)
	}
	return nil
}

func (m *mockPatentRepository) DeleteClaimsByPatent(ctx context.Context, patentID uuid.UUID) error {
	if m.deleteClaimsByPatentFn != nil {
		return m.deleteClaimsByPatentFn(ctx, patentID)
	}
	return nil
}

func (m *mockPatentRepository) BatchCreateClaims(ctx context.Context, claims []*domainPatent.Claim) error {
	if m.batchCreateClaimsFn != nil {
		return m.batchCreateClaimsFn(ctx, claims)
	}
	return nil
}

func (m *mockPatentRepository) GetIndependentClaims(ctx context.Context, patentID uuid.UUID) ([]*domainPatent.Claim, error) {
	if m.getIndependentClaimsFn != nil {
		return m.getIndependentClaimsFn(ctx, patentID)
	}
	return nil, nil
}

func (m *mockPatentRepository) SetInventors(ctx context.Context, patentID uuid.UUID, inventors []*domainPatent.Inventor) error {
	if m.setInventorsFn != nil {
		return m.setInventorsFn(ctx, patentID, inventors)
	}
	return nil
}

func (m *mockPatentRepository) GetInventors(ctx context.Context, patentID uuid.UUID) ([]*domainPatent.Inventor, error) {
	if m.getInventorsFn != nil {
		return m.getInventorsFn(ctx, patentID)
	}
	return nil, nil
}

func (m *mockPatentRepository) SearchByInventor(ctx context.Context, inventorName string, limit, offset int) ([]*domainPatent.Patent, int64, error) {
	if m.searchByInventorFn != nil {
		return m.searchByInventorFn(ctx, inventorName, limit, offset)
	}
	return nil, 0, nil
}

func (m *mockPatentRepository) SearchByAssigneeName(ctx context.Context, assigneeName string, limit, offset int) ([]*domainPatent.Patent, int64, error) {
	if m.searchByAssigneeNameFn != nil {
		return m.searchByAssigneeNameFn(ctx, assigneeName, limit, offset)
	}
	return nil, 0, nil
}

func (m *mockPatentRepository) SetPriorityClaims(ctx context.Context, patentID uuid.UUID, claims []*domainPatent.PriorityClaim) error {
	if m.setPriorityClaimsFn != nil {
		return m.setPriorityClaimsFn(ctx, patentID, claims)
	}
	return nil
}

func (m *mockPatentRepository) GetPriorityClaims(ctx context.Context, patentID uuid.UUID) ([]*domainPatent.PriorityClaim, error) {
	if m.getPriorityClaimsFn != nil {
		return m.getPriorityClaimsFn(ctx, patentID)
	}
	return nil, nil
}

func (m *mockPatentRepository) BatchCreate(ctx context.Context, patents []*domainPatent.Patent) (int, error) {
	if m.batchCreateFn != nil {
		return m.batchCreateFn(ctx, patents)
	}
	return 0, nil
}

func (m *mockPatentRepository) BatchUpdateStatus(ctx context.Context, ids []uuid.UUID, status domainPatent.PatentStatus) (int64, error) {
	if m.batchUpdateStatusFn != nil {
		return m.batchUpdateStatusFn(ctx, ids, status)
	}
	return 0, nil
}

func (m *mockPatentRepository) CountByStatus(ctx context.Context) (map[domainPatent.PatentStatus]int64, error) {
	if m.countByStatusFn != nil {
		return m.countByStatusFn(ctx)
	}
	return nil, nil
}

func (m *mockPatentRepository) CountByJurisdiction(ctx context.Context) (map[string]int64, error) {
	if m.countByJurisdictionFn != nil {
		return m.countByJurisdictionFn(ctx)
	}
	return nil, nil
}

func (m *mockPatentRepository) CountByYear(ctx context.Context, field string) (map[int]int64, error) {
	if m.countByYearFn != nil {
		return m.countByYearFn(ctx, field)
	}
	return nil, nil
}

func (m *mockPatentRepository) GetIPCDistribution(ctx context.Context, level int) (map[string]int64, error) {
	if m.getIPCDistributionFn != nil {
		return m.getIPCDistributionFn(ctx, level)
	}
	return nil, nil
}

func (m *mockPatentRepository) WithTx(ctx context.Context, fn func(domainPatent.PatentRepository) error) error {
	if m.withTxFn != nil {
		return m.withTxFn(ctx, fn)
	}
	return fn(m)
}

type mockLogger struct{}

func (m *mockLogger) Debug(msg string, fields ...logging.Field) {}
func (m *mockLogger) Info(msg string, fields ...logging.Field)  {}
func (m *mockLogger) Warn(msg string, fields ...logging.Field)  {}
func (m *mockLogger) Error(msg string, fields ...logging.Field) {}
func (m *mockLogger) Fatal(msg string, fields ...logging.Field) {}
func (m *mockLogger) With(fields ...logging.Field) logging.Logger { return m }
func (m *mockLogger) WithContext(ctx context.Context) logging.Logger { return m }
func (m *mockLogger) WithError(err error) logging.Logger { return m }
func (m *mockLogger) Sync() error { return nil }

func TestService_Create_Success(t *testing.T) {
	repo := &mockPatentRepository{
		createFn: func(ctx context.Context, p *domainPatent.Patent) error {
			return nil
		},
	}
	logger := &mockLogger{}
	svc := NewService(repo, logger)

	input := &CreateInput{
		Title:         "Test Patent",
		ApplicationNo: "CN123456",
		FilingDate:    "2023-01-01",
		Jurisdiction:  "CN",
	}

	patent, err := svc.Create(context.Background(), input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if patent.Title != "Test Patent" {
		t.Errorf("expected title 'Test Patent', got '%s'", patent.Title)
	}
}

func TestService_Create_InvalidInput(t *testing.T) {
	repo := &mockPatentRepository{}
	logger := &mockLogger{}
	svc := NewService(repo, logger)

	input := &CreateInput{
		Title: "Test Patent",
		// Missing ApplicationNo
	}

	_, err := svc.Create(context.Background(), input)
	if err == nil {
		t.Fatal("expected error for missing ApplicationNo")
	}
}

func TestService_GetByID_Success(t *testing.T) {
	uid := uuid.New()
	repo := &mockPatentRepository{
		getByIDFn: func(ctx context.Context, id uuid.UUID) (*domainPatent.Patent, error) {
			p, _ := domainPatent.NewPatent("CN123456", "Test Patent", domainPatent.OfficeCNIPA, time.Now())
			p.ID = uid
			return p, nil
		},
	}
	logger := &mockLogger{}
	svc := NewService(repo, logger)

	p, err := svc.GetByID(context.Background(), uid.String())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if p.ID != uid.String() {
		t.Errorf("expected ID %s, got %s", uid.String(), p.ID)
	}
}

func TestService_GetByID_NotFound(t *testing.T) {
	repo := &mockPatentRepository{
		getByIDFn: func(ctx context.Context, id uuid.UUID) (*domainPatent.Patent, error) {
			return nil, errors.New("not found")
		},
	}
	logger := &mockLogger{}
	svc := NewService(repo, logger)

	_, err := svc.GetByID(context.Background(), uuid.New().String())
	if err == nil {
		t.Fatal("expected error for not found")
	}
}

func TestService_GetByID_InvalidID(t *testing.T) {
	repo := &mockPatentRepository{}
	logger := &mockLogger{}
	svc := NewService(repo, logger)

	_, err := svc.GetByID(context.Background(), "invalid-uuid")
	if err == nil {
		t.Fatal("expected error for invalid uuid")
	}
}

func TestService_List_Success(t *testing.T) {
	repo := &mockPatentRepository{
		searchFn: func(ctx context.Context, query domainPatent.SearchQuery) (*domainPatent.SearchResult, error) {
			p1, _ := domainPatent.NewPatent("CN1", "T1", domainPatent.OfficeCNIPA, time.Now())
			p2, _ := domainPatent.NewPatent("CN2", "T2", domainPatent.OfficeCNIPA, time.Now())
			return &domainPatent.SearchResult{
				Items:      []*domainPatent.Patent{p1, p2},
				TotalCount: 2,
			}, nil
		},
	}
	logger := &mockLogger{}
	svc := NewService(repo, logger)

	res, err := svc.List(context.Background(), &ListInput{PageSize: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.Patents) != 2 {
		t.Errorf("expected 2 patents, got %d", len(res.Patents))
	}
	if res.Total != 2 {
		t.Errorf("expected total 2, got %d", res.Total)
	}
}

func TestService_Update_Success(t *testing.T) {
	uid := uuid.New()
	p, _ := domainPatent.NewPatent("CN1", "Original Title", domainPatent.OfficeCNIPA, time.Now())
	p.ID = uid

	repo := &mockPatentRepository{
		getByIDFn: func(ctx context.Context, id uuid.UUID) (*domainPatent.Patent, error) {
			return p, nil
		},
		updateFn: func(ctx context.Context, updated *domainPatent.Patent) error {
			if updated.Title != "Updated Title" {
				t.Errorf("expected updated title, got %s", updated.Title)
			}
			return nil
		},
	}
	logger := &mockLogger{}
	svc := NewService(repo, logger)

	newTitle := "Updated Title"
	updated, err := svc.Update(context.Background(), &UpdateInput{
		ID:    uid.String(),
		Title: &newTitle,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if updated.Title != "Updated Title" {
		t.Errorf("expected updated title 'Updated Title', got '%s'", updated.Title)
	}
}

func TestService_Delete_Success(t *testing.T) {
	repo := &mockPatentRepository{
		softDeleteFn: func(ctx context.Context, id uuid.UUID) error {
			return nil
		},
	}
	logger := &mockLogger{}
	svc := NewService(repo, logger)

	err := svc.Delete(context.Background(), uuid.New().String(), "user-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestService_Search_Success(t *testing.T) {
	repo := &mockPatentRepository{
		searchFn: func(ctx context.Context, query domainPatent.SearchQuery) (*domainPatent.SearchResult, error) {
			p1, _ := domainPatent.NewPatent("CN1", "T1", domainPatent.OfficeCNIPA, time.Now())
			return &domainPatent.SearchResult{
				Items:      []*domainPatent.Patent{p1},
				TotalCount: 1,
			}, nil
		},
	}
	logger := &mockLogger{}
	svc := NewService(repo, logger)

	res, err := svc.Search(context.Background(), &SearchInput{Query: "test"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(res.Patents) != 1 {
		t.Errorf("expected 1 patent, got %d", len(res.Patents))
	}
}

func TestService_GetStats_Success(t *testing.T) {
	repo := &mockPatentRepository{
		countByJurisdictionFn: func(ctx context.Context) (map[string]int64, error) {
			return map[string]int64{"CN": 10}, nil
		},
		countByStatusFn: func(ctx context.Context) (map[domainPatent.PatentStatus]int64, error) {
			return map[domainPatent.PatentStatus]int64{domainPatent.PatentStatusGranted: 10}, nil
		},
	}
	logger := &mockLogger{}
	svc := NewService(repo, logger)

	stats, err := svc.GetStats(context.Background(), &StatsInput{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if stats.TotalPatents != 10 {
		t.Errorf("expected 10 patents, got %d", stats.TotalPatents)
	}
}

//Personal.AI order the ending
