package repositories

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/turtacn/KeyIP-Intelligence/internal/domain/lifecycle"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/database/postgres"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

type LifecycleRepoTestSuite struct {
	suite.Suite
	db   *sql.DB
	mock sqlmock.Sqlmock
	repo lifecycle.LifecycleRepository
	log  logging.Logger
}

func (s *LifecycleRepoTestSuite) SetupTest() {
	var err error
	s.db, s.mock, err = sqlmock.New()
	require.NoError(s.T(), err)

	s.log = logging.NewNopLogger()
	conn := postgres.NewConnectionWithDB(s.db, s.log)
	s.repo = NewPostgresLifecycleRepo(conn, s.log)
}

func (s *LifecycleRepoTestSuite) TearDownTest() {
	s.db.Close()
}

func (s *LifecycleRepoTestSuite) TestCreateAnnuity_Success() {
	annuity := &lifecycle.Annuity{
		PatentID:   uuid.New(),
		YearNumber: 1,
		DueDate:    time.Now(),
		Status:     lifecycle.AnnuityStatusUpcoming,
		Amount:     new(int64),
	}
	*annuity.Amount = 1000

	s.mock.ExpectQuery("INSERT INTO patent_annuities").
		WithArgs(
			annuity.PatentID, annuity.YearNumber, annuity.DueDate, annuity.GraceDeadline, annuity.Status,
			annuity.Amount, annuity.Currency, annuity.PaidAmount, annuity.PaidDate,
			annuity.PaymentReference, annuity.AgentName, annuity.AgentReference,
			annuity.Notes, annuity.ReminderSentAt, annuity.ReminderCount, sqlmock.AnyArg(),
		).
		WillReturnRows(sqlmock.NewRows([]string{"id", "created_at", "updated_at"}).
			AddRow(uuid.New(), time.Now(), time.Now()))

	err := s.repo.CreateAnnuity(context.Background(), annuity)
	assert.NoError(s.T(), err)
	assert.NotEqual(s.T(), uuid.Nil, annuity.ID)
}

func (s *LifecycleRepoTestSuite) TestGetAnnuity_Found() {
	id := uuid.New()
	s.mock.ExpectQuery("SELECT .* FROM patent_annuities WHERE id =").
		WithArgs(id).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "patent_id", "year_number", "due_date", "grace_deadline",
			"status", "amount", "currency", "paid_amount", "paid_date",
			"payment_reference", "agent_name", "agent_reference", "notes",
			"reminder_sent_at", "reminder_count", "metadata",
			"created_at", "updated_at",
		}).AddRow(
			id, uuid.New(), 1, time.Now(), nil,
			lifecycle.AnnuityStatusUpcoming, 1000, "USD", nil, nil,
			"", "", "", "",
			nil, 0, nil,
			time.Now(), time.Now(),
		))

	a, err := s.repo.GetAnnuity(context.Background(), id)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), id, a.ID)
}

func (s *LifecycleRepoTestSuite) TestGetAnnuity_NotFound() {
	id := uuid.New()
	s.mock.ExpectQuery("SELECT .* FROM patent_annuities WHERE id =").
		WithArgs(id).
		WillReturnError(sql.ErrNoRows)

	a, err := s.repo.GetAnnuity(context.Background(), id)
	assert.Error(s.T(), err)
	assert.Nil(s.T(), a)
	assert.True(s.T(), errors.IsCode(err, errors.ErrCodeNotFound))
}

func (s *LifecycleRepoTestSuite) TestGetUpcomingAnnuities() {
	s.mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM patent_annuities").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	s.mock.ExpectQuery("SELECT .* FROM patent_annuities").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "patent_id", "year_number", "due_date", "grace_deadline",
			"status", "amount", "currency", "paid_amount", "paid_date",
			"payment_reference", "agent_name", "agent_reference", "notes",
			"reminder_sent_at", "reminder_count", "metadata",
			"created_at", "updated_at",
		}).AddRow(
			uuid.New(), uuid.New(), 1, time.Now(), nil,
			lifecycle.AnnuityStatusUpcoming, 1000, "USD", nil, nil,
			"", "", "", "",
			nil, 0, nil,
			time.Now(), time.Now(),
		))

	anns, total, err := s.repo.GetUpcomingAnnuities(context.Background(), 30, 10, 0)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), int64(1), total)
	assert.Len(s.T(), anns, 1)
}

func (s *LifecycleRepoTestSuite) TestCreateDeadline_Success() {
	dl := &lifecycle.Deadline{
		PatentID:        uuid.New(),
		DeadlineType:    "office_action",
		Title:           "Respond to OA",
		DueDate:         time.Now(),
		OriginalDueDate: time.Now(),
		Status:          lifecycle.DeadlineStatusActive,
		Priority:        "high",
	}

	s.mock.ExpectQuery("INSERT INTO patent_deadlines").
		WithArgs(
			dl.PatentID, dl.DeadlineType, dl.Title, dl.Description,
			dl.DueDate, dl.OriginalDueDate, dl.Status, dl.Priority,
			dl.AssigneeID, sqlmock.AnyArg(), sqlmock.AnyArg(),
		).
		WillReturnRows(sqlmock.NewRows([]string{"id", "created_at", "updated_at"}).
			AddRow(uuid.New(), time.Now(), time.Now()))

	err := s.repo.CreateDeadline(context.Background(), dl)
	assert.NoError(s.T(), err)
	assert.NotEqual(s.T(), uuid.Nil, dl.ID)
}

func (s *LifecycleRepoTestSuite) TestExtendDeadline_Success() {
	id := uuid.New()
	newDate := time.Now().AddDate(0, 1, 0)

	// First GetDeadline is called
	s.mock.ExpectQuery("SELECT .* FROM patent_deadlines WHERE id =").
		WithArgs(id).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "patent_id", "deadline_type", "title", "description",
			"due_date", "original_due_date", "status", "priority",
			"assignee_id", "completed_at", "completed_by",
			"extension_count", "extension_history", "reminder_config", "last_reminder_at",
			"metadata", "created_at", "updated_at",
		}).AddRow(
			id, uuid.New(), "type", "title", "",
			time.Now(), time.Now(), lifecycle.DeadlineStatusActive, "medium",
			nil, nil, nil,
			0, []byte("[]"), []byte("{}"), nil,
			nil, time.Now(), time.Now(),
		))

	s.mock.ExpectExec("UPDATE patent_deadlines").
		WithArgs(newDate, sqlmock.AnyArg(), id).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err := s.repo.ExtendDeadline(context.Background(), id, newDate, "request")
	assert.NoError(s.T(), err)
}

func (s *LifecycleRepoTestSuite) TestCreateEvent_Success() {
	evt := &lifecycle.LifecycleEvent{
		PatentID:  uuid.New(),
		EventType: lifecycle.EventTypeFiling,
		EventDate: time.Now(),
		Title:     "Filed",
	}

	s.mock.ExpectQuery("INSERT INTO patent_lifecycle_events").
		WithArgs(
			evt.PatentID, evt.EventType, evt.EventDate, evt.Title, evt.Description,
			evt.ActorID, evt.ActorName, evt.RelatedDeadlineID, evt.RelatedAnnuityID,
			sqlmock.AnyArg(), sqlmock.AnyArg(), evt.Source, sqlmock.AnyArg(),
		).
		WillReturnRows(sqlmock.NewRows([]string{"id", "created_at"}).
			AddRow(uuid.New(), time.Now()))

	err := s.repo.CreateEvent(context.Background(), evt)
	assert.NoError(s.T(), err)
}

func (s *LifecycleRepoTestSuite) TestCreateCostRecord_Success() {
	cost := &lifecycle.CostRecord{
		PatentID:     uuid.New(),
		CostType:     "filing",
		Amount:       500,
		Currency:     "USD",
		IncurredDate: time.Now(),
	}

	s.mock.ExpectQuery("INSERT INTO patent_cost_records").
		WithArgs(
			cost.PatentID, cost.CostType, cost.Amount, cost.Currency,
			cost.AmountUSD, cost.ExchangeRate, cost.IncurredDate,
			cost.Description, cost.InvoiceReference,
			cost.RelatedAnnuityID, cost.RelatedEventID, sqlmock.AnyArg(),
		).
		WillReturnRows(sqlmock.NewRows([]string{"id", "created_at"}).
			AddRow(uuid.New(), time.Now()))

	err := s.repo.CreateCostRecord(context.Background(), cost)
	assert.NoError(s.T(), err)
}

func (s *LifecycleRepoTestSuite) TestWithTx_Commit() {
	s.mock.ExpectBegin()
	s.mock.ExpectQuery("INSERT INTO patent_annuities").WillReturnRows(sqlmock.NewRows([]string{"id", "created_at", "updated_at"}).AddRow(uuid.New(), time.Now(), time.Now()))
	s.mock.ExpectCommit()

	err := s.repo.WithTx(context.Background(), func(r lifecycle.LifecycleRepository) error {
		return r.CreateAnnuity(context.Background(), &lifecycle.Annuity{
			PatentID: uuid.New(),
			Status: lifecycle.AnnuityStatusUpcoming,
		})
	})
	assert.NoError(s.T(), err)
}

func (s *LifecycleRepoTestSuite) TestWithTx_Rollback() {
	s.mock.ExpectBegin()
	s.mock.ExpectQuery("INSERT INTO patent_annuities").WillReturnError(errors.New(errors.ErrCodeDatabaseError, "fail"))
	s.mock.ExpectRollback()

	err := s.repo.WithTx(context.Background(), func(r lifecycle.LifecycleRepository) error {
		return r.CreateAnnuity(context.Background(), &lifecycle.Annuity{
			PatentID: uuid.New(),
			Status: lifecycle.AnnuityStatusUpcoming,
		})
	})
	assert.Error(s.T(), err)
}

func TestLifecycleRepo(t *testing.T) {
	suite.Run(t, new(LifecycleRepoTestSuite))
}
//Personal.AI order the ending
