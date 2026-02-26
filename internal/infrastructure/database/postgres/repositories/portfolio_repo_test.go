package repositories

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/turtacn/KeyIP-Intelligence/internal/domain/portfolio"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/database/postgres"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
)

type PortfolioRepoTestSuite struct {
	suite.Suite
	db   *sql.DB
	mock sqlmock.Sqlmock
	repo portfolio.PortfolioRepository
	log  logging.Logger
}

func (s *PortfolioRepoTestSuite) SetupTest() {
	var err error
	s.db, s.mock, err = sqlmock.New()
	require.NoError(s.T(), err)

	s.log = logging.NewNopLogger()
	conn := postgres.NewConnectionWithDB(s.db, s.log)
	s.repo = NewPostgresPortfolioRepo(conn, s.log)
}

func (s *PortfolioRepoTestSuite) TearDownTest() {
	s.db.Close()
}

func (s *PortfolioRepoTestSuite) TestCreate_Success() {
	p := &portfolio.Portfolio{
		ID:      uuid.New(),
		Name:    "My Portfolio",
		OwnerID: uuid.New(),
		Status:  portfolio.StatusActive,
	}

	s.mock.ExpectQuery("INSERT INTO portfolios").
		WithArgs(
			p.ID, p.Name, p.Description, p.OwnerID, p.Status,
			pq.Array(p.TechDomains), pq.Array(p.TargetJurisdictions), sqlmock.AnyArg(),
		).
		WillReturnRows(sqlmock.NewRows([]string{"created_at", "updated_at"}).
			AddRow(time.Now(), time.Now()))

	err := s.repo.Create(context.Background(), p)
	assert.NoError(s.T(), err)
}

func (s *PortfolioRepoTestSuite) TestGetByID_Success() {
	id := uuid.New()

	s.mock.ExpectQuery("SELECT p.*, .* as patent_count FROM portfolios p").
		WithArgs(id).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "name", "description", "owner_id", "status",
			"tech_domains", "target_jurisdictions", "metadata",
			"created_at", "updated_at", "deleted_at", "patent_count",
		}).AddRow(
			id, "Name", "", uuid.New(), portfolio.StatusActive,
			pq.Array([]string{}), pq.Array([]string{}), nil,
			time.Now(), time.Now(), nil, 5,
		))

	p, err := s.repo.GetByID(context.Background(), id)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), id, p.ID)
	assert.Equal(s.T(), 5, p.PatentCount)
}

func (s *PortfolioRepoTestSuite) TestAddPatent_Success() {
	pid := uuid.New()
	patID := uuid.New()

	s.mock.ExpectExec("INSERT INTO portfolio_patents").
		WithArgs(pid, patID, "core", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err := s.repo.AddPatent(context.Background(), pid, patID, "core", uuid.New())
	assert.NoError(s.T(), err)
}

func (s *PortfolioRepoTestSuite) TestCreateValuation_Success() {
	pid := uuid.New()
	v := &portfolio.Valuation{
		ID:             uuid.New(),
		PatentID:       uuid.New(),
		PortfolioID:    &pid,
		TechnicalScore: 85.5,
		CompositeScore: 80.0,
		Tier:           portfolio.ValuationTierA,
	}

	s.mock.ExpectQuery("INSERT INTO patent_valuations").
		WithArgs(
			v.ID, v.PatentID, v.PortfolioID, v.TechnicalScore, v.LegalScore, v.MarketScore, v.StrategicScore,
			v.CompositeScore, v.Tier, v.MonetaryValueLow, v.MonetaryValueMid, v.MonetaryValueHigh,
			v.Currency, v.ValuationMethod, v.ModelVersion, sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			v.ValidFrom, v.ValidUntil, v.EvaluatedBy,
		).
		WillReturnRows(sqlmock.NewRows([]string{"created_at"}).
			AddRow(time.Now()))

	err := s.repo.CreateValuation(context.Background(), v)
	assert.NoError(s.T(), err)
}

func (s *PortfolioRepoTestSuite) TestCreateHealthScore_Success() {
	hs := &portfolio.HealthScore{
		ID:           uuid.New(),
		PortfolioID:  uuid.New(),
		OverallScore: 90.0,
	}

	s.mock.ExpectQuery("INSERT INTO portfolio_health_scores").
		WithArgs(
			hs.ID, hs.PortfolioID, hs.OverallScore, hs.CoverageScore, hs.DiversityScore,
			hs.FreshnessScore, hs.StrengthScore, hs.RiskScore,
			hs.TotalPatents, hs.ActivePatents, hs.ExpiringWithinYear, hs.ExpiringWithin3Years,
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), hs.ModelVersion, hs.EvaluatedAt,
		).
		WillReturnRows(sqlmock.NewRows([]string{"created_at"}).
			AddRow(time.Now()))

	err := s.repo.CreateHealthScore(context.Background(), hs)
	assert.NoError(s.T(), err)
}

func (s *PortfolioRepoTestSuite) TestGetPortfolioSummary() {
	pid := uuid.New()

	// Total patents
	s.mock.ExpectQuery("SELECT COUNT").WithArgs(pid).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(10))

	// Active patents
	s.mock.ExpectQuery("SELECT COUNT").WithArgs(pid).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(8))

	// Status counts
	s.mock.ExpectQuery("SELECT p.status, COUNT").WithArgs(pid).
		WillReturnRows(sqlmock.NewRows([]string{"status", "count"}).
			AddRow("granted", 5).
			AddRow("filed", 3))

	// Valuation
	s.mock.ExpectQuery("SELECT AVG").WithArgs(pid).
		WillReturnRows(sqlmock.NewRows([]string{"avg", "sum"}).
			AddRow(75.5, 1000000))

	// Health score
	s.mock.ExpectQuery("SELECT .* FROM portfolio_health_scores").
		WithArgs(pid).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "portfolio_id", "overall_score", "coverage_score", "diversity_score",
			"freshness_score", "strength_score", "risk_score",
			"total_patents", "active_patents", "expiring_within_year", "expiring_within_3years",
			"jurisdiction_distribution", "tech_domain_distribution", "tier_distribution", "recommendations",
			"model_version", "evaluated_at", "created_at",
		}).AddRow(
			uuid.New(), pid, 88.8, 0, 0, 0, 0, 0, 0, 0, 0, 0,
			[]byte("{}"), []byte("{}"), []byte("{}"), []byte("[]"), "", time.Now(), time.Now(),
		))

	sum, err := s.repo.GetPortfolioSummary(context.Background(), pid)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), 10, sum.TotalPatents)
	assert.Equal(s.T(), 8, sum.ActivePatents)
	assert.Equal(s.T(), 5, sum.StatusCounts["granted"])
	assert.Equal(s.T(), 88.8, sum.HealthScore)
}

func TestPortfolioRepo(t *testing.T) {
	suite.Run(t, new(PortfolioRepoTestSuite))
}
//Personal.AI order the ending
