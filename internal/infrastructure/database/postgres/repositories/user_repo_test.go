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
	"github.com/turtacn/KeyIP-Intelligence/internal/domain/user"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/database/postgres"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
)

type UserRepoTestSuite struct {
	suite.Suite
	db   *sql.DB
	mock sqlmock.Sqlmock
	repo user.UserRepository
	log  logging.Logger
}

func (s *UserRepoTestSuite) SetupTest() {
	var err error
	s.db, s.mock, err = sqlmock.New()
	require.NoError(s.T(), err)

	s.log = logging.NewNopLogger()
	conn := postgres.NewConnectionWithDB(s.db, s.log)
	s.repo = NewPostgresUserRepo(conn, s.log)
}

func (s *UserRepoTestSuite) TearDownTest() {
	s.db.Close()
}

func (s *UserRepoTestSuite) TestCreateUser_Success() {
	u := &user.User{
		ID:       uuid.New(),
		Email:    "test@example.com",
		Username: "testuser",
		Status:   "active",
	}

	s.mock.ExpectQuery("INSERT INTO users").
		WithArgs(
			u.ID, u.Email, u.Username, u.DisplayName, u.PasswordHash, u.Status, u.AvatarURL,
			u.Locale, u.Timezone, u.MFAEnabled, u.MFASecret, sqlmock.AnyArg(), sqlmock.AnyArg(),
		).
		WillReturnRows(sqlmock.NewRows([]string{"created_at", "updated_at"}).
			AddRow(time.Now(), time.Now()))

	err := s.repo.Create(context.Background(), u)
	assert.NoError(s.T(), err)
}

func (s *UserRepoTestSuite) TestGetByID_Found() {
	id := uuid.New()
	s.mock.ExpectQuery("SELECT .* FROM users WHERE id =").
		WithArgs(id).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "email", "username", "display_name", "password_hash", "status", "avatar_url",
			"locale", "timezone", "last_login_at", "last_login_ip", "login_count", "failed_login_count",
			"locked_until", "email_verified_at", "mfa_enabled", "mfa_secret", "preferences", "metadata",
			"created_at", "updated_at", "deleted_at",
		}).AddRow(
			id, "email", "username", "name", "hash", "active", "",
			"zh-CN", "UTC", nil, "", 0, 0,
			nil, nil, false, "", nil, nil,
			time.Now(), time.Now(), nil,
		))

	u, err := s.repo.GetByID(context.Background(), id)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), id, u.ID)
	// PasswordHash should be empty as scanUser(false)
	assert.Empty(s.T(), u.PasswordHash)
}

func (s *UserRepoTestSuite) TestGetByEmailForAuth_Found() {
	email := "test@example.com"
	s.mock.ExpectQuery("SELECT .* FROM users WHERE email =").
		WithArgs(email).
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "email", "username", "display_name", "password_hash", "status", "avatar_url",
			"locale", "timezone", "last_login_at", "last_login_ip", "login_count", "failed_login_count",
			"locked_until", "email_verified_at", "mfa_enabled", "mfa_secret", "preferences", "metadata",
			"created_at", "updated_at", "deleted_at",
		}).AddRow(
			uuid.New(), email, "username", "name", "hash123", "active", "",
			"zh-CN", "UTC", nil, "", 0, 0,
			nil, nil, false, "", nil, nil,
			time.Now(), time.Now(), nil,
		))

	u, err := s.repo.GetByEmailForAuth(context.Background(), email)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), "hash123", u.PasswordHash)
}

func (s *UserRepoTestSuite) TestCreateOrganization_Success() {
	org := &user.Organization{
		ID:   uuid.New(),
		Name: "Test Org",
		Slug: "test-org",
	}

	s.mock.ExpectQuery("INSERT INTO organizations").
		WithArgs(
			org.ID, org.Name, org.Slug, org.Description, org.LogoURL, org.Plan,
			org.MaxMembers, org.MaxPatents, sqlmock.AnyArg(),
		).
		WillReturnRows(sqlmock.NewRows([]string{"created_at", "updated_at"}).
			AddRow(time.Now(), time.Now()))

	err := s.repo.CreateOrganization(context.Background(), org)
	assert.NoError(s.T(), err)
}

func (s *UserRepoTestSuite) TestAddMember_Success() {
	orgID := uuid.New()
	userID := uuid.New()

	s.mock.ExpectExec("INSERT INTO organization_members").
		WithArgs(orgID, userID, "admin", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err := s.repo.AddMember(context.Background(), orgID, userID, "admin", uuid.New())
	assert.NoError(s.T(), err)
}

func (s *UserRepoTestSuite) TestAssignRole_Success() {
	userID := uuid.New()
	roleID := uuid.New()

	s.mock.ExpectExec("INSERT INTO user_roles").
		WithArgs(userID, roleID, sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 1))

	err := s.repo.AssignRole(context.Background(), userID, roleID, nil, uuid.New())
	assert.NoError(s.T(), err)
}

func (s *UserRepoTestSuite) TestCreateAPIKey_Success() {
	key := &user.APIKey{
		ID:        uuid.New(),
		UserID:    uuid.New(),
		Name:      "Test Key",
		KeyHash:   "hash",
		KeyPrefix: "kip_",
		Scopes:    []string{"read"},
	}

	s.mock.ExpectQuery("INSERT INTO api_keys").
		WithArgs(
			key.ID, key.UserID, key.OrganizationID, key.Name, key.KeyHash, key.KeyPrefix,
			pq.Array(key.Scopes), key.RateLimit, key.ExpiresAt,
		).
		WillReturnRows(sqlmock.NewRows([]string{"created_at", "updated_at"}).
			AddRow(time.Now(), time.Now()))

	err := s.repo.CreateAPIKey(context.Background(), key)
	assert.NoError(s.T(), err)
}

func (s *UserRepoTestSuite) TestCreateAuditLog_Success() {
	log := &user.AuditLog{
		ID:     uuid.New(),
		Action: "login",
	}

	s.mock.ExpectQuery("INSERT INTO audit_logs").
		WithArgs(
			log.ID, log.UserID, log.OrganizationID, log.Action, log.ResourceType, log.ResourceID,
			log.IPAddress, log.UserAgent, log.RequestID, sqlmock.AnyArg(), sqlmock.AnyArg(), sqlmock.AnyArg(),
		).
		WillReturnRows(sqlmock.NewRows([]string{"created_at"}).
			AddRow(time.Now()))

	err := s.repo.CreateAuditLog(context.Background(), log)
	assert.NoError(s.T(), err)
}

func TestUserRepo(t *testing.T) {
	suite.Run(t, new(UserRepoTestSuite))
}
//Personal.AI order the ending
