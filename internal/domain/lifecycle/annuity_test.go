package lifecycle

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/turtacn/KeyIP-Intelligence/internal/domain/portfolio"
)

// Mocks

type MockAnnuityRepository struct {
	mock.Mock
}

func (m *MockAnnuityRepository) Save(ctx context.Context, record *AnnuityRecord) error {
	args := m.Called(ctx, record)
	return args.Error(0)
}

func (m *MockAnnuityRepository) SaveBatch(ctx context.Context, records []*AnnuityRecord) error {
	args := m.Called(ctx, records)
	return args.Error(0)
}

func (m *MockAnnuityRepository) FindByID(ctx context.Context, id string) (*AnnuityRecord, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*AnnuityRecord), args.Error(1)
}

func (m *MockAnnuityRepository) FindByPatentID(ctx context.Context, patentID string) ([]*AnnuityRecord, error) {
	args := m.Called(ctx, patentID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*AnnuityRecord), args.Error(1)
}

func (m *MockAnnuityRepository) FindByStatus(ctx context.Context, status AnnuityStatus) ([]*AnnuityRecord, error) {
	args := m.Called(ctx, status)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*AnnuityRecord), args.Error(1)
}

func (m *MockAnnuityRepository) FindPending(ctx context.Context, beforeDate time.Time) ([]*AnnuityRecord, error) {
	args := m.Called(ctx, beforeDate)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*AnnuityRecord), args.Error(1)
}

func (m *MockAnnuityRepository) FindOverdue(ctx context.Context, asOfDate time.Time) ([]*AnnuityRecord, error) {
	args := m.Called(ctx, asOfDate)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*AnnuityRecord), args.Error(1)
}

func (m *MockAnnuityRepository) SumByPortfolio(ctx context.Context, portfolioID string, fromDate, toDate time.Time) (int64, string, error) {
	args := m.Called(ctx, portfolioID, fromDate, toDate)
	return args.Get(0).(int64), args.String(1), args.Error(2)
}

func (m *MockAnnuityRepository) Delete(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

type MockPortfolioRepository struct {
	mock.Mock
}

func (m *MockPortfolioRepository) Save(ctx context.Context, p *portfolio.Portfolio) error {
	args := m.Called(ctx, p)
	return args.Error(0)
}

func (m *MockPortfolioRepository) FindByID(ctx context.Context, id string) (*portfolio.Portfolio, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*portfolio.Portfolio), args.Error(1)
}
// Implement other methods as needed or define empty stubs?
// Go interfaces must be fully implemented.
// I need full implementation for compilation.
// I'll define empty methods for unused ones.

func (m *MockPortfolioRepository) FindByOwnerID(ctx context.Context, ownerID string, opts ...portfolio.QueryOption) ([]*portfolio.Portfolio, error) {
	return nil, nil
}
func (m *MockPortfolioRepository) FindByStatus(ctx context.Context, status portfolio.PortfolioStatus, opts ...portfolio.QueryOption) ([]*portfolio.Portfolio, error) {
	return nil, nil
}
func (m *MockPortfolioRepository) FindByTechDomain(ctx context.Context, techDomain string, opts ...portfolio.QueryOption) ([]*portfolio.Portfolio, error) {
	return nil, nil
}
func (m *MockPortfolioRepository) Delete(ctx context.Context, id string) error {
	return nil
}
func (m *MockPortfolioRepository) Count(ctx context.Context, ownerID string) (int64, error) {
	return 0, nil
}
func (m *MockPortfolioRepository) ListSummaries(ctx context.Context, ownerID string, opts ...portfolio.QueryOption) ([]*portfolio.PortfolioSummary, error) {
	return nil, nil
}
func (m *MockPortfolioRepository) FindContainingPatent(ctx context.Context, patentID string) ([]*portfolio.Portfolio, error) {
	return nil, nil
}

// Tests

func TestMoney_Operations(t *testing.T) {
	m1 := NewMoney(100, "USD")
	m2 := NewMoney(200, "USD")

	sum, err := m1.Add(m2)
	assert.NoError(t, err)
	assert.Equal(t, int64(300), sum.Amount)
	assert.Equal(t, "USD", sum.Currency)

	m3 := NewMoney(100, "CNY")
	_, err = m1.Add(m3)
	assert.Error(t, err)

	assert.Equal(t, 1.0, m1.ToFloat64())

	err = m1.Validate()
	assert.NoError(t, err)

	mInv := NewMoney(-1, "USD")
	err = mInv.Validate()
	assert.Error(t, err)
}

func TestGenerateSchedule_CN(t *testing.T) {
	reg := NewJurisdictionRegistry()
	repo := new(MockAnnuityRepository)
	pRepo := new(MockPortfolioRepository)
	svc := NewAnnuityService(repo, pRepo, reg)

	filingDate := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	sched, err := svc.GenerateSchedule(context.Background(), "p1", "CN", filingDate, 20)
	assert.NoError(t, err)
	assert.NotNil(t, sched)
	// CN starts year 3. 2020+3 = 2023.
	// 20 years max -> up to year 20.
	// Records should be for year 3 to 20 = 18 records.
	assert.Equal(t, 18, len(sched.Records))
	assert.Equal(t, 3, sched.Records[0].YearNumber)
}

func TestGenerateSchedule_US(t *testing.T) {
	reg := NewJurisdictionRegistry()
	repo := new(MockAnnuityRepository)
	pRepo := new(MockPortfolioRepository)
	svc := NewAnnuityService(repo, pRepo, reg)

	filingDate := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	sched, err := svc.GenerateSchedule(context.Background(), "p1", "US", filingDate, 20)
	assert.NoError(t, err)
	assert.NotNil(t, sched)
	// US has 3 payments: 3.5, 7.5, 11.5
	assert.Equal(t, 3, len(sched.Records))
	assert.Equal(t, 3, sched.Records[0].YearNumber) // 3.5 stored as 3
}

func TestCalculateAnnuityFee(t *testing.T) {
	reg := NewJurisdictionRegistry()
	repo := new(MockAnnuityRepository)
	pRepo := new(MockPortfolioRepository)
	svc := NewAnnuityService(repo, pRepo, reg)

	// CN Year 3 -> 900
	fee, err := svc.CalculateAnnuityFee(context.Background(), "CN", 3)
	assert.NoError(t, err)
	assert.Equal(t, int64(90000), fee.Amount)

	// US Year 3 -> 1600
	fee, err = svc.CalculateAnnuityFee(context.Background(), "US", 3)
	assert.NoError(t, err)
	assert.Equal(t, int64(160000), fee.Amount)
}

func TestMarkAsPaid_Success(t *testing.T) {
	reg := NewJurisdictionRegistry()
	repo := new(MockAnnuityRepository)
	pRepo := new(MockPortfolioRepository)
	svc := NewAnnuityService(repo, pRepo, reg)

	rec := &AnnuityRecord{ID: "r1", Status: AnnuityStatusPending}
	repo.On("FindByID", mock.Anything, "r1").Return(rec, nil)
	repo.On("Save", mock.Anything, rec).Return(nil)

	err := svc.MarkAsPaid(context.Background(), "r1", NewMoney(100, "USD"), time.Now())
	assert.NoError(t, err)
	assert.Equal(t, AnnuityStatusPaid, rec.Status)
}

func TestForecastCosts_Success(t *testing.T) {
	reg := NewJurisdictionRegistry()
	repo := new(MockAnnuityRepository)
	pRepo := new(MockPortfolioRepository)
	svc := NewAnnuityService(repo, pRepo, reg)

	p := &portfolio.Portfolio{ID: "port1", PatentIDs: []string{"p1"}}
	pRepo.On("FindByID", mock.Anything, "port1").Return(p, nil)

	recs := []*AnnuityRecord{
		{
			ID: "r1", PatentID: "p1", JurisdictionCode: "US", Amount: NewMoney(10000, "USD"),
			DueDate: time.Now().AddDate(1, 0, 0), Status: AnnuityStatusPending, Currency: "USD",
		},
	}
	repo.On("FindByPatentID", mock.Anything, "p1").Return(recs, nil)

	fc, err := svc.ForecastCosts(context.Background(), "port1", 5)
	assert.NoError(t, err)
	assert.NotNil(t, fc)
	assert.Equal(t, int64(10000), fc.TotalForecastCost.Amount)
}

func TestGetUpcomingPayments(t *testing.T) {
	reg := NewJurisdictionRegistry()
	repo := new(MockAnnuityRepository)
	pRepo := new(MockPortfolioRepository)
	svc := NewAnnuityService(repo, pRepo, reg)

	p := &portfolio.Portfolio{ID: "port1", PatentIDs: []string{"p1"}}
	pRepo.On("FindByID", mock.Anything, "port1").Return(p, nil)

	recs := []*AnnuityRecord{
		{
			ID: "r1", PatentID: "p1", JurisdictionCode: "US", Amount: NewMoney(10000, "USD"),
			DueDate: time.Now().AddDate(0, 0, 10), Status: AnnuityStatusPending,
		},
		{
			ID: "r2", PatentID: "p1", JurisdictionCode: "US", Amount: NewMoney(10000, "USD"),
			DueDate: time.Now().AddDate(0, 0, 100), Status: AnnuityStatusPending,
		},
	}
	repo.On("FindByPatentID", mock.Anything, "p1").Return(recs, nil)

	upcoming, err := svc.GetUpcomingPayments(context.Background(), "port1", 30)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(upcoming))
	assert.Equal(t, "r1", upcoming[0].ID)
}

//Personal.AI order the ending
