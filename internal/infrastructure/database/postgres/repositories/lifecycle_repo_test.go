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
	"github.com/turtacn/KeyIP-Intelligence/internal/domain/lifecycle"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/database/postgres"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

type LifecycleRepoTestSuite struct {
	suite.Suite
	mock   sqlmock.Sqlmock
	db     *sql.DB
	repo   lifecycle.LifecycleRepository
	logger logging.Logger
}

func (s *LifecycleRepoTestSuite) SetupTest() {
	var err error
	s.db, s.mock, err = sqlmock.New()
	s.NoError(err)

	s.logger = logging.NewNopLogger()
	conn := postgres.NewConnectionWithDB(s.db, s.logger)
	s.repo = NewPostgresLifecycleRepo(conn, s.logger)
}

func (s *LifecycleRepoTestSuite) TearDownTest() {
	s.db.Close()
}

func (s *LifecycleRepoTestSuite) TestCreateAnnuity_Success() {
	id := uuid.New().String()
	amount := int64(1000)
	annuity := &lifecycle.Annuity{
		PatentID:   uuid.New().String(),
		YearNumber: 1,
		DueDate:    time.Now().Add(time.Hour * 24 * 365),
		Status:     lifecycle.AnnuityStatusUpcoming,
		Currency:   "USD",
		Amount:     &amount,
		Metadata:   map[string]any{"key": "value"},
	}
	metaJSON, _ := json.Marshal(annuity.Metadata)

	s.mock.ExpectQuery("INSERT INTO patent_annuities").
		WithArgs(annuity.PatentID, annuity.YearNumber, annuity.DueDate, annuity.GraceDeadline, annuity.Status,
			annuity.Amount, annuity.Currency, annuity.PaidAmount, annuity.PaidDate, annuity.PaymentReference,
			annuity.AgentName, annuity.AgentReference, annuity.Notes, annuity.ReminderSentAt, annuity.ReminderCount, metaJSON).
		WillReturnRows(sqlmock.NewRows([]string{"id", "created_at", "updated_at"}).AddRow(id, time.Now(), time.Now()))

	err := s.repo.CreateAnnuity(context.Background(), annuity)
	s.NoError(err)
	s.Equal(id, annuity.ID)
}

func (s *LifecycleRepoTestSuite) TestGetAnnuity_Found() {
	id := uuid.New().String()
	patentID := uuid.New().String()
	dueDate := time.Now()
	meta := map[string]any{"k": "v"}
	metaJSON, _ := json.Marshal(meta)
	amount := int64(500)

	s.mock.ExpectQuery("SELECT \\* FROM patent_annuities WHERE id = \\$1").
		WithArgs(id).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "patent_id", "year_number", "due_date", "grace_deadline", "status",
			"amount", "currency", "paid_amount", "paid_date", "payment_reference",
			"agent_name", "agent_reference", "notes", "reminder_sent_at", "reminder_count", "metadata",
			"created_at", "updated_at",
		}).AddRow(
			id, patentID, 5, dueDate, nil, "upcoming",
			amount, "USD", nil, nil, "",
			"", "", "", nil, 0, metaJSON,
			time.Now(), time.Now(),
		))

	a, err := s.repo.GetAnnuity(context.Background(), id)
	s.NoError(err)
	if a != nil {
		s.Equal(id, a.ID)
		s.Equal(patentID, a.PatentID)
		s.Equal("v", a.Metadata["k"])
	}
}

func (s *LifecycleRepoTestSuite) TestGetAnnuity_NotFound() {
	id := uuid.New().String()
	s.mock.ExpectQuery("SELECT \\* FROM patent_annuities WHERE id = \\$1").
		WithArgs(id).
		WillReturnError(sql.ErrNoRows)

	a, err := s.repo.GetAnnuity(context.Background(), id)
	s.Error(err)
	s.Nil(a)
	s.True(errors.IsCode(err, errors.ErrCodeNotFound))
}

func (s *LifecycleRepoTestSuite) TestGetUpcomingAnnuities() {
	s.mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM patent_annuities a JOIN patents p").
		WithArgs(sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	amount := int64(100)
	s.mock.ExpectQuery("SELECT a\\.\\* FROM patent_annuities a JOIN patents p").
		WithArgs(sqlmock.AnyArg(), 10, 0).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "patent_id", "year_number", "due_date", "grace_deadline", "status",
			"amount", "currency", "paid_amount", "paid_date", "payment_reference",
			"agent_name", "agent_reference", "notes", "reminder_sent_at", "reminder_count", "metadata",
			"created_at", "updated_at",
		}).AddRow(
			uuid.New().String(), uuid.New().String(), 1, time.Now(), nil, "upcoming",
			amount, "USD", nil, nil, "",
			"", "", "", nil, 0, []byte("{}"),
			time.Now(), time.Now(),
		))

	list, count, err := s.repo.GetUpcomingAnnuities(context.Background(), 30, 10, 0)
	s.NoError(err)
	s.Equal(int64(1), count)
	s.Len(list, 1)
}

func (s *LifecycleRepoTestSuite) TestWithTx_Commit() {
	s.mock.ExpectBegin()
	s.mock.ExpectCommit()

	err := s.repo.WithTx(context.Background(), func(r lifecycle.LifecycleRepository) error {
		return nil
	})
	s.NoError(err)
}

func (s *LifecycleRepoTestSuite) TestWithTx_Rollback() {
	s.mock.ExpectBegin()
	s.mock.ExpectRollback()

	err := s.repo.WithTx(context.Background(), func(r lifecycle.LifecycleRepository) error {
		return errors.New(errors.ErrCodeInternal, "fail")
	})
	s.Error(err)
}

func TestLifecycleRepoTestSuite(t *testing.T) {
	suite.Run(t, new(LifecycleRepoTestSuite))
}

//Personal.AI order the ending
