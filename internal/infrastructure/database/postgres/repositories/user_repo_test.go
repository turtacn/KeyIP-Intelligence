package repositories

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"
	"github.com/turtacn/KeyIP-Intelligence/internal/domain/user"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/database/postgres"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
)

type UserRepoTestSuite struct {
	suite.Suite
	mock   sqlmock.Sqlmock
	db     *sql.DB
	repo   user.UserRepository
	logger logging.Logger
}

func (s *UserRepoTestSuite) SetupTest() {
	var err error
	s.db, s.mock, err = sqlmock.New()
	s.NoError(err)

	s.logger = logging.NewNopLogger()
	conn := postgres.NewConnectionWithDB(s.db, s.logger)
	s.repo = NewPostgresUserRepo(conn, s.logger)
}

func (s *UserRepoTestSuite) TearDownTest() {
	s.db.Close()
}

func (s *UserRepoTestSuite) TestGetByID_Found() {
	id := uuid.New()
	s.mock.ExpectQuery("SELECT id, email, .* FROM users WHERE id = \\$1").
		WithArgs(id).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "email", "username", "display_name", "status", "avatar_url", "locale", "timezone",
			"last_login_at", "last_login_ip", "login_count", "failed_login_count", "locked_until",
			"email_verified_at", "mfa_enabled", "preferences", "metadata", "created_at", "updated_at", "deleted_at",
		}).AddRow(
			id, "test@example.com", "testuser", "Test User", "active", "", "en-US", "UTC",
			nil, "", 0, 0, nil, nil, false, nil, nil, time.Now(), time.Now(), nil,
		))

	u, err := s.repo.GetByID(context.Background(), id)
	s.NoError(err)
	s.NotNil(u)
	s.Equal(id, u.ID)
	s.Equal("test@example.com", u.Email)
}

func (s *UserRepoTestSuite) TestCreate_Success() {
	id := uuid.New()
	u := &user.User{
		Email:       "new@example.com",
		Username:    "newuser",
		DisplayName: "New User",
		Status:      "pending_verification",
	}

	s.mock.ExpectQuery("INSERT INTO users").
		WillReturnRows(sqlmock.NewRows([]string{"id", "created_at", "updated_at"}).AddRow(id, time.Now(), time.Now()))

	err := s.repo.Create(context.Background(), u)
	s.NoError(err)
	s.Equal(id, u.ID)
}

func TestUserRepoTestSuite(t *testing.T) {
	suite.Run(t, new(UserRepoTestSuite))
}

//Personal.AI order the ending
