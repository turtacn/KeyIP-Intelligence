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
	"github.com/turtacn/KeyIP-Intelligence/internal/domain/patent"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/database/postgres"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
)

type PatentRepoTestSuite struct {
	suite.Suite
	mock   sqlmock.Sqlmock
	db     *sql.DB
	repo   patent.PatentRepository
	logger logging.Logger
}

type NopLogger struct{}

func (l *NopLogger) Debug(msg string, fields ...logging.Field) {}
func (l *NopLogger) Info(msg string, fields ...logging.Field)  {}
func (l *NopLogger) Warn(msg string, fields ...logging.Field)  {}
func (l *NopLogger) Error(msg string, fields ...logging.Field) {}
func (l *NopLogger) Fatal(msg string, fields ...logging.Field) {}
func (l *NopLogger) With(fields ...logging.Field) logging.Logger { return l }
func (l *NopLogger) WithContext(ctx context.Context) logging.Logger { return l }
func (l *NopLogger) WithError(err error) logging.Logger { return l }
func (l *NopLogger) Sync() error { return nil }

func (s *PatentRepoTestSuite) SetupTest() {
	var err error
	s.db, s.mock, err = sqlmock.New()
	s.NoError(err)

	s.logger = &NopLogger{}
	conn := postgres.NewConnectionWithDB(s.db, s.logger)
	s.repo = NewPostgresPatentRepo(conn, s.logger)
}

func (s *PatentRepoTestSuite) TearDownTest() {
	s.db.Close()
}

func (s *PatentRepoTestSuite) TestSave_Insert_Success() {
	id := uuid.New().String()
	now := time.Now()
	p := &patent.Patent{
		ID:           id,
		PatentNumber: "US123456",
		Title:        "OLED",
		Abstract:     "Abstract",
		Status:       patent.PatentStatusFiled,
		Office:       patent.OfficeUSPTO,
		Dates: patent.PatentDate{
			FilingDate: &now,
		},
		CreatedAt: now,
		UpdatedAt: now,
		Version:   1,
	}

	// Expect Exists check
	s.mock.ExpectQuery(`SELECT COUNT\(\*\) FROM patents WHERE patent_number = \$1 AND deleted_at IS NULL`).
		WithArgs(p.PatentNumber).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	// Expect Insert
	// Columns: id, patent_number, title, abstract, status, office, applicants, inventors, ipc_codes, filing_date, pub, grant, expiry, mol_ids, family_id, created, updated, version
	s.mock.ExpectExec(`INSERT INTO patents`).
		WithArgs(
			p.ID, p.PatentNumber, p.Title, p.Abstract, p.Status.String(), p.Office,
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), // JSON fields
			p.Dates.FilingDate, p.Dates.PublicationDate, p.Dates.GrantDate, p.Dates.ExpiryDate,
			sqlmock.AnyArg(), p.FamilyID, p.CreatedAt, p.UpdatedAt, p.Version,
		).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err := s.repo.Save(context.Background(), p)
	s.NoError(err)
}

func (s *PatentRepoTestSuite) TestFindByID_Found() {
	id := uuid.New().String()
	now := time.Now()

	cols := []string{
		"id", "patent_number", "title", "abstract", "status", "office",
		"applicants", "inventors", "ipc_codes",
		"filing_date", "publication_date", "grant_date", "expiry_date",
		"molecule_ids", "family_id", "created_at", "updated_at", "version",
	}

	applicantsJSON, _ := json.Marshal([]patent.Applicant{{Name: "Test Co"}})

	row := sqlmock.NewRows(cols).AddRow(
		id, "US123", "Title", "Abstract", "Filed", "USPTO",
		applicantsJSON, []byte("[]"), []byte("[]"),
		now, nil, nil, nil,
		"{mol1}", "fam1", now, now, 1,
	)

	s.mock.ExpectQuery(`SELECT \* FROM patents WHERE id = \$1 AND deleted_at IS NULL`).
		WithArgs(id).
		WillReturnRows(row)

	p, err := s.repo.FindByID(context.Background(), id)
	s.NoError(err)
	s.NotNil(p)
	s.Equal(id, p.ID)
	s.Equal("US123", p.PatentNumber)
	s.Equal(patent.PatentStatusFiled, p.Status)
	s.Equal("Test Co", p.Applicants[0].Name)
	s.Equal("mol1", p.MoleculeIDs[0])
}

func TestPatentRepoTestSuite(t *testing.T) {
	suite.Run(t, new(PatentRepoTestSuite))
}
