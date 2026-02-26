package repositories

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/pgvector/pgvector-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/turtacn/KeyIP-Intelligence/internal/domain/patent"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/database/postgres"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
)

type PatentRepoTestSuite struct {
	suite.Suite
	db   *sql.DB
	mock sqlmock.Sqlmock
	repo patent.PatentRepository
	log  logging.Logger
}

func (s *PatentRepoTestSuite) SetupTest() {
	var err error
	s.db, s.mock, err = sqlmock.New()
	require.NoError(s.T(), err)

	s.log = logging.NewNopLogger()
	conn := postgres.NewConnectionWithDB(s.db, s.log)
	s.repo = NewPostgresPatentRepo(conn, s.log)
}

func (s *PatentRepoTestSuite) TearDownTest() {
	s.db.Close()
}

func (s *PatentRepoTestSuite) TestCreate_Success() {
	p := &patent.Patent{
		ID:           uuid.New(),
		PatentNumber: "US123456",
		Title:        "Test Patent",
		IPCCodes:     []string{"A01B"},
	}

	s.mock.ExpectQuery("INSERT INTO patents").
		WithArgs(
			p.ID, p.PatentNumber, p.Title, p.TitleEn, p.Abstract, p.AbstractEn,
			p.Type, p.Status, p.FilingDate, p.PublicationDate, p.GrantDate,
			p.ExpiryDate, p.PriorityDate, p.AssigneeID, p.AssigneeName, p.Jurisdiction,
			pq.Array(p.IPCCodes), pq.Array(p.CPCCodes), pq.Array(p.KeyIPTechCodes),
			p.FamilyID, p.ApplicationNumber, p.FullTextHash, p.Source, sqlmock.AnyArg(), sqlmock.AnyArg(),
		).
		WillReturnRows(sqlmock.NewRows([]string{"created_at", "updated_at"}).
			AddRow(time.Now(), time.Now()))

	err := s.repo.Create(context.Background(), p)
	assert.NoError(s.T(), err)
}

func (s *PatentRepoTestSuite) TestGetByID_Success() {
	id := uuid.New()

	// Expect main query
	s.mock.ExpectQuery("SELECT .* FROM patents WHERE id =").
		WithArgs(id).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "patent_number", "title", "title_en", "abstract", "abstract_en",
			"patent_type", "status", "filing_date", "publication_date", "grant_date",
			"expiry_date", "priority_date", "assignee_id", "assignee_name", "jurisdiction",
			"ipc_codes", "cpc_codes", "keyip_tech_codes", "family_id", "application_number",
			"full_text_hash", "source", "raw_data", "metadata",
			"created_at", "updated_at", "deleted_at",
		}).AddRow(
			id, "US123", "Title", "", "", "",
			"invention", patent.PatentStatusGranted, nil, nil, nil,
			nil, nil, nil, "", "US",
			pq.Array([]string{}), pq.Array([]string{}), pq.Array([]string{}), "", "",
			"", "manual", nil, nil,
			time.Now(), time.Now(), nil,
		))

	// Expect preloads
	// Claims
	s.mock.ExpectQuery("SELECT .* FROM patent_claims WHERE patent_id =").
		WithArgs(id).
		WillReturnRows(sqlmock.NewRows([]string{}))
	// Inventors
	s.mock.ExpectQuery("SELECT .* FROM patent_inventors WHERE patent_id =").
		WithArgs(id).
		WillReturnRows(sqlmock.NewRows([]string{}))
	// Priority
	s.mock.ExpectQuery("SELECT .* FROM patent_priority_claims WHERE patent_id =").
		WithArgs(id).
		WillReturnRows(sqlmock.NewRows([]string{}))

	p, err := s.repo.GetByID(context.Background(), id)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), id, p.ID)
}

func (s *PatentRepoTestSuite) TestCreateClaim_Success() {
	claim := &patent.Claim{
		ID:             uuid.New(),
		PatentID:       uuid.New(),
		Number:         1,
		Type:           patent.ClaimTypeIndependent,
		Text:           "Claim text",
		ScopeEmbedding: []float32{0.1, 0.2},
	}

	embedding := pgvector.NewVector(claim.ScopeEmbedding)

	s.mock.ExpectQuery("INSERT INTO patent_claims").
		WithArgs(
			claim.ID, claim.PatentID, claim.Number, "independent", claim.ParentClaimID,
			claim.Text, claim.TextEn, sqlmock.AnyArg(), sqlmock.AnyArg(), embedding,
		).
		WillReturnRows(sqlmock.NewRows([]string{"created_at", "updated_at"}).
			AddRow(time.Now(), time.Now()))

	err := s.repo.CreateClaim(context.Background(), claim)
	assert.NoError(s.T(), err)
}

func (s *PatentRepoTestSuite) TestSearch_Keyword() {
	query := patent.SearchQuery{
		Keyword: "OLED",
		Limit:   10,
	}

	// Count
	s.mock.ExpectQuery("SELECT COUNT").
		WithArgs("OLED").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))

	// Select
	s.mock.ExpectQuery("SELECT .* FROM patents").
		WithArgs("OLED", 10, 0).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "patent_number", "title", "title_en", "abstract", "abstract_en",
			"patent_type", "status", "filing_date", "publication_date", "grant_date",
			"expiry_date", "priority_date", "assignee_id", "assignee_name", "jurisdiction",
			"ipc_codes", "cpc_codes", "keyip_tech_codes", "family_id", "application_number",
			"full_text_hash", "source", "raw_data", "metadata",
			"created_at", "updated_at", "deleted_at",
		}).AddRow(
			uuid.New(), "US123", "OLED Display", "", "", "",
			"invention", patent.PatentStatusGranted, nil, nil, nil,
			nil, nil, nil, "", "US",
			pq.Array([]string{}), pq.Array([]string{}), pq.Array([]string{}), "", "",
			"", "manual", nil, nil,
			time.Now(), time.Now(), nil,
		))

	res, err := s.repo.Search(context.Background(), query)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), int64(1), res.TotalCount)
}

func TestPatentRepo(t *testing.T) {
	suite.Run(t, new(PatentRepoTestSuite))
}
//Personal.AI order the ending
