package postgres

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	pkgerrors "github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

func TestBuildDSN_DefaultConfig(t *testing.T) {
	cfg := PostgresConfig{
		Host:     "localhost",
		Port:     5432,
		Database: "keyip",
		Username: "postgres",
		Password: "password",
		SSLMode:  "disable",
	}

	dsn := buildDSN(cfg)
	expected := "postgres://postgres:password@localhost:5432/keyip?lock_timeout=10000&sslmode=disable&statement_timeout=30000"
	assert.Equal(t, expected, dsn)
}

func TestBuildDSN_CustomConfig(t *testing.T) {
	cfg := PostgresConfig{
		Host:             "db.example.com",
		Port:             5433,
		Database:         "prod_db",
		Username:         "user",
		Password:         "pass!word",
		SSLMode:          "require",
		StatementTimeout: 60 * time.Second,
		LockTimeout:      15 * time.Second,
	}

	dsn := buildDSN(cfg)
	expected := "postgres://user:pass%21word@db.example.com:5433/prod_db?lock_timeout=15000&sslmode=require&statement_timeout=60000"
	assert.Equal(t, expected, dsn)
}

func TestBuildDSN_SSLModeVariants(t *testing.T) {
	modes := []string{"disable", "require", "verify-ca", "verify-full"}
	cfg := PostgresConfig{
		Host:     "localhost",
		Port:     5432,
		Database: "test",
		Username: "user",
		Password: "pw",
	}

	for _, mode := range modes {
		cfg.SSLMode = mode
		dsn := buildDSN(cfg)
		assert.Contains(t, dsn, "sslmode="+mode)
	}
}

func TestNewConnection_Success(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	require.NoError(t, err)
	defer db.Close()

	// Mock sql.Open
	originalSqlOpen := sqlOpen
	defer func() { sqlOpen = originalSqlOpen }()
	sqlOpen = func(driverName, dataSourceName string) (*sql.DB, error) {
		return db, nil
	}

	mock.ExpectPing()

	cfg := PostgresConfig{
		Host:     "localhost",
		Port:     5432,
		Database: "test",
		Username: "user",
		Password: "pw",
	}

	log := logging.NewNopLogger()
	conn, err := NewConnection(cfg, log)

	assert.NoError(t, err)
	assert.NotNil(t, conn)
	assert.Equal(t, db, conn.DB())
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestNewConnection_PingFailure(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	require.NoError(t, err)
	defer db.Close()

	// Mock sql.Open
	originalSqlOpen := sqlOpen
	defer func() { sqlOpen = originalSqlOpen }()
	sqlOpen = func(driverName, dataSourceName string) (*sql.DB, error) {
		return db, nil
	}

	mock.ExpectPing().WillReturnError(errors.New("connection refused"))

	cfg := PostgresConfig{
		Host: "localhost",
	}

	log := logging.NewNopLogger()
	conn, err := NewConnection(cfg, log)

	assert.Error(t, err)
	assert.Nil(t, conn)
	// Check if the returned error wraps ErrDatabaseConnection (as per requirement)
	// Or check error code
	var appErr *pkgerrors.AppError
	if errors.As(err, &appErr) {
		assert.Equal(t, pkgerrors.ErrCodeDatabaseError, appErr.Code)
		assert.Equal(t, "database connection failed", appErr.Message)
		assert.Contains(t, appErr.Cause.Error(), "connection refused")
	} else {
		t.Errorf("Error should be of type *AppError")
	}
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestNewConnection_OpenFailure(t *testing.T) {
	originalSqlOpen := sqlOpen
	defer func() { sqlOpen = originalSqlOpen }()
	sqlOpen = func(driverName, dataSourceName string) (*sql.DB, error) {
		return nil, errors.New("open failed")
	}

	cfg := PostgresConfig{}
	log := logging.NewNopLogger()
	conn, err := NewConnection(cfg, log)

	assert.Error(t, err)
	assert.Nil(t, conn)
}

func TestConnection_HealthCheck_Success(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	require.NoError(t, err)
	defer db.Close()

	conn := NewConnectionWithDB(db, logging.NewNopLogger())

	mock.ExpectPing()

	err = conn.HealthCheck(context.Background())
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestConnection_HealthCheck_Failure(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	require.NoError(t, err)
	defer db.Close()

	conn := NewConnectionWithDB(db, logging.NewNopLogger())

	mock.ExpectPing().WillReturnError(errors.New("timeout"))

	err = conn.HealthCheck(context.Background())
	assert.Error(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestConnection_Stats(t *testing.T) {
	db, _, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	conn := NewConnectionWithDB(db, logging.NewNopLogger())
	stats := conn.Stats()
	assert.IsType(t, sql.DBStats{}, stats)
}

func TestConnection_Close_Success(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	conn := NewConnectionWithDB(db, logging.NewNopLogger())

	mock.ExpectClose()

	err = conn.Close()
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestConnection_Close_Idempotent(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	conn := NewConnectionWithDB(db, logging.NewNopLogger())

	mock.ExpectClose()

	err = conn.Close()
	assert.NoError(t, err)

	// Second close should not call db.Close() again
	err = conn.Close()
	assert.NoError(t, err)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestConnection_DB_ReturnsInstance(t *testing.T) {
	db, _, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	conn := NewConnectionWithDB(db, logging.NewNopLogger())
	assert.Equal(t, db, conn.DB())
}
//Personal.AI order the ending
