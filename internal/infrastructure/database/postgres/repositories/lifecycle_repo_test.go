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
	mock         sqlmock.Sqlmock
	db           *sql.DB
	lifecycleRepo lifecycle.LifecycleRepository
	annuityRepo   lifecycle.AnnuityRepository
	deadlineRepo  lifecycle.DeadlineRepository
	logger       logging.Logger
}

func (s *LifecycleRepoTestSuite) SetupTest() {
	var err error
	s.db, s.mock, err = sqlmock.New()
	s.NoError(err)

	s.logger = logging.NewNopLogger()
	conn := postgres.NewConnectionWithDB(s.db, s.logger)

	s.lifecycleRepo = NewPostgresLifecycleRepo(conn, s.logger)
	s.annuityRepo = NewPostgresAnnuityRepo(conn, s.logger)
	s.deadlineRepo = NewPostgresDeadlineRepo(conn, s.logger)
}

func (s *LifecycleRepoTestSuite) TearDownTest() {
	s.db.Close()
}

func (s *LifecycleRepoTestSuite) TestSaveAnnuity_Success() {
	id := uuid.New().String()
	annuity := &lifecycle.AnnuityRecord{
		ID:         id,
		PatentID:   uuid.New().String(),
		YearNumber: 1,
		DueDate:    time.Now().Add(time.Hour * 24 * 365),
		Status:     lifecycle.AnnuityStatusUpcoming,
		Currency:   "USD",
		Amount:     lifecycle.NewMoney(1000, "USD"),
		Metadata:   map[string]any{"key": "value"},
	}
	metaJSON, _ := json.Marshal(annuity.Metadata)

	s.mock.ExpectQuery("INSERT INTO patent_annuities").
		WithArgs(annuity.PatentID, annuity.YearNumber, annuity.DueDate, annuity.GraceDeadline, annuity.Status,
			annuity.Amount.Amount, annuity.Currency, int64(0), annuity.PaidDate, annuity.PaymentReference,
			annuity.AgentName, annuity.AgentReference, annuity.Notes, annuity.ReminderSentAt, annuity.ReminderCount, metaJSON).
		WillReturnRows(sqlmock.NewRows([]string{"id", "created_at", "updated_at"}).AddRow(id, time.Now(), time.Now()))

	err := s.annuityRepo.Save(context.Background(), annuity)
	s.NoError(err)
	s.Equal(id, annuity.ID)
}

func (s *LifecycleRepoTestSuite) TestGetAnnuityByID_Found() {
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

	a, err := s.annuityRepo.FindByID(context.Background(), id)
	s.NoError(err)
	if a != nil {
		s.Equal(id, a.ID)
		s.Equal(patentID, a.PatentID)
	}
}

func (s *LifecycleRepoTestSuite) TestGetAnnuityByID_NotFound() {
	id := uuid.New().String()
	s.mock.ExpectQuery("SELECT \\* FROM patent_annuities WHERE id = \\$1").
		WithArgs(id).
		WillReturnError(sql.ErrNoRows)

	a, err := s.annuityRepo.FindByID(context.Background(), id)
	s.Error(err)
	s.Nil(a)
	s.True(errors.IsCode(err, errors.ErrCodeNotFound))
}

func (s *LifecycleRepoTestSuite) TestGetPendingAnnuities() {
	amount := int64(100)
	// Query is SELECT * FROM ... WHERE status ... AND due_date <= $1
	s.mock.ExpectQuery("SELECT \\* FROM patent_annuities").
		WithArgs(sqlmock.AnyArg()).
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

	list, err := s.annuityRepo.FindPending(context.Background(), time.Now().AddDate(0, 1, 0))
	s.NoError(err)
	s.Len(list, 1)
}

func TestLifecycleRepoTestSuite(t *testing.T) {
	suite.Run(t, new(LifecycleRepoTestSuite))
}

//Personal.AI order the ending
