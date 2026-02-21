package lifecycle

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/turtacn/KeyIP-Intelligence/internal/domain/portfolio"
)

// Mock repositories
type mockAnnuityRepository struct {
	mock.Mock
}

func (m *mockAnnuityRepository) Save(ctx context.Context, record *AnnuityRecord) error {
	args := m.Called(ctx, record)
	return args.Error(0)
}

func (m *mockAnnuityRepository) SaveBatch(ctx context.Context, records []*AnnuityRecord) error {
	args := m.Called(ctx, records)
	return args.Error(0)
}

func (m *mockAnnuityRepository) FindByID(ctx context.Context, id string) (*AnnuityRecord, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*AnnuityRecord), args.Error(1)
}

func (m *mockAnnuityRepository) FindByPatentID(ctx context.Context, patentID string) ([]*AnnuityRecord, error) {
	args := m.Called(ctx, patentID)
	return args.Get(0).([]*AnnuityRecord), args.Error(1)
}

func (m *mockAnnuityRepository) FindByStatus(ctx context.Context, status AnnuityStatus) ([]*AnnuityRecord, error) {
	args := m.Called(ctx, status)
	return args.Get(0).([]*AnnuityRecord), args.Error(1)
}

func (m *mockAnnuityRepository) FindPending(ctx context.Context, beforeDate time.Time) ([]*AnnuityRecord, error) {
	args := m.Called(ctx, beforeDate)
	return args.Get(0).([]*AnnuityRecord), args.Error(1)
}

func (m *mockAnnuityRepository) FindOverdue(ctx context.Context, asOfDate time.Time) ([]*AnnuityRecord, error) {
	args := m.Called(ctx, asOfDate)
	return args.Get(0).([]*AnnuityRecord), args.Error(1)
}

func (m *mockAnnuityRepository) SumByPortfolio(ctx context.Context, portfolioID string, fromDate, toDate time.Time) (int64, string, error) {
	args := m.Called(ctx, portfolioID, fromDate, toDate)
	return args.Get(0).(int64), args.String(1), args.Error(2)
}

func (m *mockAnnuityRepository) Delete(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

type mockPortfolioRepository struct {
	mock.Mock
}

func (m *mockPortfolioRepository) Save(ctx context.Context, p *portfolio.Portfolio) error {
	args := m.Called(ctx, p)
	return args.Error(0)
}

func (m *mockPortfolioRepository) FindByID(ctx context.Context, id string) (*portfolio.Portfolio, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*portfolio.Portfolio), args.Error(1)
}

func (m *mockPortfolioRepository) FindByOwnerID(ctx context.Context, ownerID string, opts ...portfolio.QueryOption) ([]*portfolio.Portfolio, error) {
	args := m.Called(ctx, ownerID)
	return args.Get(0).([]*portfolio.Portfolio), args.Error(1)
}

func (m *mockPortfolioRepository) FindByStatus(ctx context.Context, status portfolio.PortfolioStatus, opts ...portfolio.QueryOption) ([]*portfolio.Portfolio, error) {
	args := m.Called(ctx, status)
	return args.Get(0).([]*portfolio.Portfolio), args.Error(1)
}

func (m *mockPortfolioRepository) FindByTechDomain(ctx context.Context, techDomain string, opts ...portfolio.QueryOption) ([]*portfolio.Portfolio, error) {
	args := m.Called(ctx, techDomain)
	return args.Get(0).([]*portfolio.Portfolio), args.Error(1)
}

func (m *mockPortfolioRepository) Delete(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *mockPortfolioRepository) Count(ctx context.Context, ownerID string) (int64, error) {
	args := m.Called(ctx, ownerID)
	return args.Get(0).(int64), args.Error(1)
}

func (m *mockPortfolioRepository) ListSummaries(ctx context.Context, ownerID string, opts ...portfolio.QueryOption) ([]*portfolio.PortfolioSummary, error) {
	args := m.Called(ctx, ownerID)
	return args.Get(0).([]*portfolio.PortfolioSummary), args.Error(1)
}

func (m *mockPortfolioRepository) FindContainingPatent(ctx context.Context, patentID string) ([]*portfolio.Portfolio, error) {
	args := m.Called(ctx, patentID)
	return args.Get(0).([]*portfolio.Portfolio), args.Error(1)
}

func TestNewMoney(t *testing.T) {
	m := NewMoney(100, "USD")
	assert.Equal(t, int64(100), m.Amount)
	assert.Equal(t, "USD", m.Currency)
}

func TestMoney_ToFloat64(t *testing.T) {
	m := NewMoney(100, "USD")
	assert.Equal(t, 1.0, m.ToFloat64())
}

func TestMoney_Add_SameCurrency(t *testing.T) {
	m1 := NewMoney(100, "USD")
	m2 := NewMoney(50, "USD")
	res, err := m1.Add(m2)
	assert.NoError(t, err)
	assert.Equal(t, int64(150), res.Amount)
}

func TestMoney_Add_DifferentCurrency(t *testing.T) {
	m1 := NewMoney(100, "USD")
	m2 := NewMoney(50, "CNY")
	_, err := m1.Add(m2)
	assert.Error(t, err)
}

func TestMoney_Validate_Valid(t *testing.T) {
	m := NewMoney(100, "USD")
	assert.NoError(t, m.Validate())
}

func TestMoney_Validate_NegativeAmount(t *testing.T) {
	m := NewMoney(-1, "USD")
	assert.Error(t, m.Validate())
}

func TestMoney_Validate_EmptyCurrency(t *testing.T) {
	m := NewMoney(100, "")
	assert.Error(t, m.Validate())
}

func TestGenerateSchedule_CN(t *testing.T) {
	s := NewAnnuityService(nil, nil)
	filingDate := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	schedule, err := s.GenerateSchedule(context.Background(), "pat1", "CN", filingDate, 20)
	assert.NoError(t, err)
	assert.Equal(t, 18, len(schedule.Records)) // 3 to 20
	assert.Equal(t, "CNY", schedule.Records[0].Currency)
}

func TestGenerateSchedule_US(t *testing.T) {
	s := NewAnnuityService(nil, nil)
	filingDate := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	schedule, err := s.GenerateSchedule(context.Background(), "pat1", "US", filingDate, 20)
	assert.NoError(t, err)
	assert.Equal(t, 3, len(schedule.Records)) // 3.5, 7.5, 11.5
	assert.Equal(t, "USD", schedule.Records[0].Currency)
}

func TestGenerateSchedule_EP(t *testing.T) {
	s := NewAnnuityService(nil, nil)
	filingDate := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	schedule, err := s.GenerateSchedule(context.Background(), "pat1", "EP", filingDate, 20)
	assert.NoError(t, err)
	assert.Equal(t, 18, len(schedule.Records))
}

func TestGenerateSchedule_JP(t *testing.T) {
	s := NewAnnuityService(nil, nil)
	filingDate := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	schedule, err := s.GenerateSchedule(context.Background(), "pat1", "JP", filingDate, 20)
	assert.NoError(t, err)
	assert.Equal(t, 20, len(schedule.Records))
}

func TestGenerateSchedule_InvalidJurisdiction(t *testing.T) {
	s := NewAnnuityService(nil, nil)
	filingDate := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	_, err := s.GenerateSchedule(context.Background(), "pat1", "INVALID", filingDate, 20)
	assert.Error(t, err)
}

func TestCalculateAnnuityFee_CN_Year3(t *testing.T) {
	s := NewAnnuityService(nil, nil)
	fee, err := s.CalculateAnnuityFee(context.Background(), "CN", 3)
	assert.NoError(t, err)
	assert.Equal(t, int64(90000), fee.Amount)
}

func TestCalculateAnnuityFee_CN_Year10(t *testing.T) {
	s := NewAnnuityService(nil, nil)
	fee, err := s.CalculateAnnuityFee(context.Background(), "CN", 10)
	assert.NoError(t, err)
	assert.Equal(t, int64(200000), fee.Amount)
}

func TestCalculateAnnuityFee_CN_Year16(t *testing.T) {
	s := NewAnnuityService(nil, nil)
	fee, err := s.CalculateAnnuityFee(context.Background(), "CN", 16)
	assert.NoError(t, err)
	assert.Equal(t, int64(600000), fee.Amount)
}

func TestCalculateAnnuityFee_US_Year4(t *testing.T) {
	s := NewAnnuityService(nil, nil)
	fee, err := s.CalculateAnnuityFee(context.Background(), "US", 4)
	assert.NoError(t, err)
	assert.Equal(t, int64(160000), fee.Amount)
}

func TestCalculateAnnuityFee_US_Year8(t *testing.T) {
	s := NewAnnuityService(nil, nil)
	fee, err := s.CalculateAnnuityFee(context.Background(), "US", 8)
	assert.NoError(t, err)
	assert.Equal(t, int64(360000), fee.Amount)
}

func TestCalculateAnnuityFee_US_Year12(t *testing.T) {
	s := NewAnnuityService(nil, nil)
	fee, err := s.CalculateAnnuityFee(context.Background(), "US", 12)
	assert.NoError(t, err)
	assert.Equal(t, int64(740000), fee.Amount)
}

func TestCalculateAnnuityFee_EP_Year3(t *testing.T) {
	s := NewAnnuityService(nil, nil)
	fee, err := s.CalculateAnnuityFee(context.Background(), "EP", 3)
	assert.NoError(t, err)
	assert.Equal(t, int64(47000), fee.Amount)
}

func TestCalculateAnnuityFee_InvalidYear(t *testing.T) {
	s := NewAnnuityService(nil, nil)
	_, err := s.CalculateAnnuityFee(context.Background(), "CN", 25)
	assert.Error(t, err)
}

func TestMarkAsPaid_Success(t *testing.T) {
	repo := new(mockAnnuityRepository)
	s := NewAnnuityService(repo, nil)
	record := &AnnuityRecord{ID: "rec1", Status: AnnuityStatusPending}
	repo.On("FindByID", mock.Anything, "rec1").Return(record, nil)
	repo.On("Save", mock.Anything, mock.Anything).Return(nil)

	err := s.MarkAsPaid(context.Background(), "rec1", NewMoney(100, "USD"), time.Now())
	assert.NoError(t, err)
	assert.Equal(t, AnnuityStatusPaid, record.Status)
}

func TestMarkAsPaid_AlreadyPaid(t *testing.T) {
	repo := new(mockAnnuityRepository)
	s := NewAnnuityService(repo, nil)
	record := &AnnuityRecord{ID: "rec1", Status: AnnuityStatusPaid}
	repo.On("FindByID", mock.Anything, "rec1").Return(record, nil)

	err := s.MarkAsPaid(context.Background(), "rec1", NewMoney(100, "USD"), time.Now())
	assert.Error(t, err)
}

func TestMarkAsPaid_Abandoned(t *testing.T) {
	repo := new(mockAnnuityRepository)
	s := NewAnnuityService(repo, nil)
	record := &AnnuityRecord{ID: "rec1", Status: AnnuityStatusAbandoned}
	repo.On("FindByID", mock.Anything, "rec1").Return(record, nil)

	err := s.MarkAsPaid(context.Background(), "rec1", NewMoney(100, "USD"), time.Now())
	assert.Error(t, err)
}

func TestMarkAsAbandoned_Success(t *testing.T) {
	repo := new(mockAnnuityRepository)
	s := NewAnnuityService(repo, nil)
	record := &AnnuityRecord{ID: "rec1", Status: AnnuityStatusPending}
	repo.On("FindByID", mock.Anything, "rec1").Return(record, nil)
	repo.On("Save", mock.Anything, mock.Anything).Return(nil)

	err := s.MarkAsAbandoned(context.Background(), "rec1", "reason")
	assert.NoError(t, err)
	assert.Equal(t, AnnuityStatusAbandoned, record.Status)
}

func TestCheckOverdue_NoOverdue(t *testing.T) {
	repo := new(mockAnnuityRepository)
	s := NewAnnuityService(repo, nil)
	now := time.Now().UTC()
	record := &AnnuityRecord{
		ID:            "rec1",
		Status:        AnnuityStatusPending,
		DueDate:       now.AddDate(0, 0, 10),
		GraceDeadline: now.AddDate(0, 0, 20),
	}
	repo.On("FindPending", mock.Anything, now).Return([]*AnnuityRecord{record}, nil)

	changed, err := s.CheckOverdue(context.Background(), now)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(changed))
}

func TestCheckOverdue_InGracePeriod(t *testing.T) {
	repo := new(mockAnnuityRepository)
	s := NewAnnuityService(repo, nil)
	now := time.Now().UTC()
	record := &AnnuityRecord{
		ID:            "rec1",
		Status:        AnnuityStatusPending,
		DueDate:       now.AddDate(0, 0, -5),
		GraceDeadline: now.AddDate(0, 0, 5),
	}
	repo.On("FindPending", mock.Anything, now).Return([]*AnnuityRecord{record}, nil)
	repo.On("Save", mock.Anything, record).Return(nil)

	changed, err := s.CheckOverdue(context.Background(), now)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(changed))
	assert.Equal(t, AnnuityStatusGracePeriod, record.Status)
}

func TestCheckOverdue_PastGracePeriod(t *testing.T) {
	repo := new(mockAnnuityRepository)
	s := NewAnnuityService(repo, nil)
	now := time.Now().UTC()
	record := &AnnuityRecord{
		ID:            "rec1",
		Status:        AnnuityStatusPending,
		DueDate:       now.AddDate(0, 0, -10),
		GraceDeadline: now.AddDate(0, 0, -5),
	}
	repo.On("FindPending", mock.Anything, now).Return([]*AnnuityRecord{record}, nil)
	repo.On("Save", mock.Anything, record).Return(nil)

	changed, err := s.CheckOverdue(context.Background(), now)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(changed))
	assert.Equal(t, AnnuityStatusOverdue, record.Status)
}

func TestForecastCosts_Success(t *testing.T) {
	repo := new(mockAnnuityRepository)
	pRepo := new(mockPortfolioRepository)
	s := NewAnnuityService(repo, pRepo)

	now := time.Now().UTC()
	port := &portfolio.Portfolio{ID: "port1", PatentIDs: []string{"pat1"}}
	record := &AnnuityRecord{
		PatentID:         "pat1",
		DueDate:          now.AddDate(0, 0, 30),
		Amount:           NewMoney(1000, "USD"),
		Status:           AnnuityStatusPending,
		Currency:         "USD",
		JurisdictionCode: "US",
	}

	pRepo.On("FindByID", mock.Anything, "port1").Return(port, nil)
	repo.On("FindByPatentID", mock.Anything, "pat1").Return([]*AnnuityRecord{record}, nil)

	forecast, err := s.ForecastCosts(context.Background(), "port1", 1)
	assert.NoError(t, err)
	assert.Equal(t, int64(1000), forecast.TotalForecastCost.Amount)
}

func TestForecastCosts_EmptyPortfolio(t *testing.T) {
	repo := new(mockAnnuityRepository)
	pRepo := new(mockPortfolioRepository)
	s := NewAnnuityService(repo, pRepo)

	port := &portfolio.Portfolio{ID: "port1", PatentIDs: []string{}}
	pRepo.On("FindByID", mock.Anything, "port1").Return(port, nil)

	forecast, err := s.ForecastCosts(context.Background(), "port1", 1)
	assert.NoError(t, err)
	assert.Equal(t, int64(0), forecast.TotalForecastCost.Amount)
}

func TestGetUpcomingPayments_WithinDays(t *testing.T) {
	repo := new(mockAnnuityRepository)
	pRepo := new(mockPortfolioRepository)
	s := NewAnnuityService(repo, pRepo)

	now := time.Now().UTC()
	port := &portfolio.Portfolio{ID: "port1", PatentIDs: []string{"pat1"}}
	record := &AnnuityRecord{
		PatentID: "pat1",
		DueDate:  now.AddDate(0, 0, 10),
		Status:   AnnuityStatusPending,
	}

	pRepo.On("FindByID", mock.Anything, "port1").Return(port, nil)
	repo.On("FindByPatentID", mock.Anything, "pat1").Return([]*AnnuityRecord{record}, nil)

	upcoming, err := s.GetUpcomingPayments(context.Background(), "port1", 30)
	assert.NoError(t, err)
	assert.Equal(t, 1, len(upcoming))
}

func TestGetUpcomingPayments_NoneUpcoming(t *testing.T) {
	repo := new(mockAnnuityRepository)
	pRepo := new(mockPortfolioRepository)
	s := NewAnnuityService(repo, pRepo)

	now := time.Now().UTC()
	port := &portfolio.Portfolio{ID: "port1", PatentIDs: []string{"pat1"}}
	record := &AnnuityRecord{
		PatentID: "pat1",
		DueDate:  now.AddDate(0, 0, 40),
		Status:   AnnuityStatusPending,
	}

	pRepo.On("FindByID", mock.Anything, "port1").Return(port, nil)
	repo.On("FindByPatentID", mock.Anything, "pat1").Return([]*AnnuityRecord{record}, nil)

	upcoming, err := s.GetUpcomingPayments(context.Background(), "port1", 30)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(upcoming))
}

//Personal.AI order the ending
