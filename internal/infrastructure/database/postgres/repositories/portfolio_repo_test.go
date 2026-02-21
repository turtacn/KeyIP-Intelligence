package repositories

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"
	"github.com/turtacn/KeyIP-Intelligence/internal/domain/portfolio"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/database/postgres"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

type PortfolioRepoTestSuite struct {
	suite.Suite
	mock   sqlmock.Sqlmock
	db     *sql.DB
	repo   portfolio.PortfolioRepository
	logger logging.Logger
}

func (s *PortfolioRepoTestSuite) SetupTest() {
	var err error
	s.db, s.mock, err = sqlmock.New()
	s.NoError(err)

	s.logger = logging.NewNopLogger()
	conn := postgres.NewConnectionWithDB(s.db, s.logger)
	s.repo = NewPostgresPortfolioRepo(conn, s.logger)
}

func (s *PortfolioRepoTestSuite) TearDownTest() {
	s.db.Close()
}

func (s *PortfolioRepoTestSuite) TestCreate_Success() {
	id := uuid.New()
	p := &portfolio.Portfolio{
		Name:    "OLED Tech",
		OwnerID: uuid.New(),
		Status:  "active",
	}
	meta, _ := json.Marshal(p.Metadata)

	s.mock.ExpectQuery("INSERT INTO portfolios").
		WithArgs(p.Name, sqlmock.AnyArg(), p.OwnerID, p.Status, sqlmock.AnyArg(), sqlmock.AnyArg(), meta).
		WillReturnRows(sqlmock.NewRows([]string{"id", "created_at", "updated_at"}).AddRow(id, time.Now(), time.Now()))

	err := s.repo.Create(context.Background(), p)
	s.NoError(err)
	s.Equal(id, p.ID)
}

func (s *PortfolioRepoTestSuite) TestGetByID_Found() {
	id := uuid.New()
	ownerID := uuid.New()

	// Columns: id, name, description, owner_id, status, tech_domains, target_jurisdictions, metadata, created_at, updated_at, deleted_at
	cols := []string{
		"id", "name", "description", "owner_id", "status", "tech_domains", "target_jurisdictions", "metadata",
		"created_at", "updated_at", "deleted_at",
	}
	row := sqlmock.NewRows(cols).AddRow(
		id, "Portfolio 1", "Desc", ownerID, "active", []uint8("{}"), []uint8("{}"), []byte("{}"),
		time.Now(), time.Now(), nil,
	)

	s.mock.ExpectQuery("SELECT \\* FROM portfolios WHERE id = \\$1").
		WithArgs(id).
		WillReturnRows(row)

	// Preload patent count
	s.mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM portfolio_patents WHERE portfolio_id = \\$1").
		WithArgs(id).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(10))

	p, err := s.repo.GetByID(context.Background(), id)
	s.NoError(err)
	s.NotNil(p)
	s.Equal(id, p.ID)
	s.Equal("Portfolio 1", p.Name)
	s.Equal(10, p.PatentCount)
}

func (s *PortfolioRepoTestSuite) TestUpdate_OptimisticLock() {
	id := uuid.New()
	p := &portfolio.Portfolio{
		ID:        id,
		Name:      "Updated",
		UpdatedAt: time.Now(),
	}

	s.mock.ExpectExec("UPDATE portfolios").
		WithArgs(p.Name, sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), p.ID, p.UpdatedAt).
		WillReturnResult(sqlmock.NewResult(0, 1))

	err := s.repo.Update(context.Background(), p)
	s.NoError(err)
}

func (s *PortfolioRepoTestSuite) TestUpdate_Conflict() {
	id := uuid.New()
	p := &portfolio.Portfolio{
		ID:        id,
		UpdatedAt: time.Now(),
	}

	s.mock.ExpectExec("UPDATE portfolios").
		WillReturnResult(sqlmock.NewResult(0, 0))

	err := s.repo.Update(context.Background(), p)
	s.Error(err)
	s.True(errors.IsCode(err, errors.ErrCodeConflict))
}

func TestPortfolioRepoTestSuite(t *testing.T) {
	suite.Run(t, new(PortfolioRepoTestSuite))
}

//Personal.AI order the ending
