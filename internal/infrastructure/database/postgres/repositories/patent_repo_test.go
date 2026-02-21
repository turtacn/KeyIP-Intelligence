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

func (s *PatentRepoTestSuite) SetupTest() {
	var err error
	s.db, s.mock, err = sqlmock.New()
	s.NoError(err)

	s.logger = logging.NewNopLogger()
	conn := postgres.NewConnectionWithDB(s.db, s.logger)
	s.repo = NewPostgresPatentRepo(conn, s.logger)
}

func (s *PatentRepoTestSuite) TearDownTest() {
	s.db.Close()
}

func (s *PatentRepoTestSuite) TestCreate_Success() {
	id := uuid.New()
	p := &patent.Patent{
		PatentNumber: "US123456",
		Title:        "OLED",
		Status:       patent.PatentStatusFiled,
		RawData:      map[string]any{"raw": "data"},
		Metadata:     map[string]any{"meta": "data"},
	}
	raw, _ := json.Marshal(p.RawData)
	meta, _ := json.Marshal(p.Metadata)

	s.mock.ExpectQuery("INSERT INTO patents").
		WithArgs(p.PatentNumber, p.Title, sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), p.Status.String(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
			sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(), raw, meta).
		WillReturnRows(sqlmock.NewRows([]string{"id", "created_at", "updated_at"}).AddRow(id, time.Now(), time.Now()))

	err := s.repo.Create(context.Background(), p)
	s.NoError(err)
	s.Equal(id, p.ID)
}

func (s *PatentRepoTestSuite) TestGetByID_Found() {
	id := uuid.New()

	// Columns: id, patent_number, title, title_en, abstract, abstract_en, patent_type, status,
	// filing_date, publication_date, grant_date, expiry_date, priority_date,
	// assignee_id, assignee_name, jurisdiction, ipc_codes, cpc_codes, keyip_tech_codes,
	// family_id, application_number, full_text_hash, source, raw_data, metadata,
	// created_at, updated_at, deleted_at

	cols := []string{
		"id", "patent_number", "title", "title_en", "abstract", "abstract_en", "patent_type", "status",
		"filing_date", "publication_date", "grant_date", "expiry_date", "priority_date",
		"assignee_id", "assignee_name", "jurisdiction", "ipc_codes", "cpc_codes", "keyip_tech_codes",
		"family_id", "application_number", "full_text_hash", "source", "raw_data", "metadata",
		"created_at", "updated_at", "deleted_at",
	}

	row := sqlmock.NewRows(cols).AddRow(
		id, "US123", "Title", "", "Abstract", "", "invention", "filed",
		nil, nil, nil, nil, nil,
		nil, "", "US", []uint8("{}"), []uint8("{}"), []uint8("{}"),
		"", "", "", "manual", []byte("{}"), []byte("{}"),
		time.Now(), time.Now(), nil,
	)

	s.mock.ExpectQuery("SELECT \\* FROM patents WHERE id = \\$1").
		WithArgs(id).
		WillReturnRows(row)

	// Preload calls (Claims, Inventors, PriorityClaims) - assuming GetByID impl calls them
	// The repo implementation calls GetClaimsByPatent, GetInventors, GetPriorityClaims.
	// We expect empty results for them.
	s.mock.ExpectQuery("SELECT \\* FROM patent_claims WHERE patent_id = \\$1").
		WithArgs(id).
		WillReturnRows(sqlmock.NewRows([]string{}))

	s.mock.ExpectQuery("SELECT \\* FROM patent_inventors WHERE patent_id = \\$1").
		WithArgs(id).
		WillReturnRows(sqlmock.NewRows([]string{}))

	s.mock.ExpectQuery("SELECT \\* FROM patent_priority_claims WHERE patent_id = \\$1").
		WithArgs(id).
		WillReturnRows(sqlmock.NewRows([]string{}))

	p, err := s.repo.GetByID(context.Background(), id)
	s.NoError(err)
	s.NotNil(p)
	s.Equal(id, p.ID)
	s.Equal("US123", p.PatentNumber)
}

func TestPatentRepoTestSuite(t *testing.T) {
	suite.Run(t, new(PatentRepoTestSuite))
}

//Personal.AI order the ending
