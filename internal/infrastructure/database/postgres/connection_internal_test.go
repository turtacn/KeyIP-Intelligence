package postgres

import (
	"database/sql"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
)

func TestConnection_NewConnection(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	require.NoError(t, err)
	defer db.Close()

	// Mock sqlOpen
	origOpen := sqlOpen
	defer func() { sqlOpen = origOpen }()
	sqlOpen = func(driverName, dataSourceName string) (*sql.DB, error) {
		return db, nil
	}

	mock.ExpectPing()

	logger := logging.NewNopLogger()
	cfg := PostgresConfig{
		Host: "localhost",
		Port: 5432,
	}

	conn, err := NewConnection(cfg, logger)
	require.NoError(t, err)
	require.NotNil(t, conn)

	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestBuildDSN(t *testing.T) {
	cfg := PostgresConfig{
		Host:     "localhost",
		Port:     5432,
		Database: "testdb",
		Username: "user",
		Password: "password",
	}
	dsn := buildDSN(cfg)
	// dsn format is URL: postgres://user:password@localhost:5432/testdb...
	assert.Contains(t, dsn, "postgres://user:password@localhost:5432/testdb")
	assert.Contains(t, dsn, "sslmode=disable")
}

// Check pool configuration logic
func TestConnection_PoolConfig(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	require.NoError(t, err)
	defer db.Close()

	origOpen := sqlOpen
	defer func() { sqlOpen = origOpen }()
	sqlOpen = func(driverName, dataSourceName string) (*sql.DB, error) {
		return db, nil
	}

	mock.ExpectPing()

	cfg := PostgresConfig{
		MaxOpenConns:    10,
		MaxIdleConns:    5,
		ConnMaxLifetime: 1 * time.Hour,
		ConnMaxIdleTime: 10 * time.Minute,
	}

	conn, err := NewConnection(cfg, logging.NewNopLogger())
	require.NoError(t, err)
	require.NotNil(t, conn)

	// We can't easily assert db settings on *sql.DB without reflection or stats,
	// but we can ensure no panic and logic runs.
	stats := conn.Stats()
	assert.Equal(t, 10, stats.MaxOpenConnections)
	// MaxIdle is not directly exposed in stats struct in older Go versions, but MaxOpen is.
}

//Personal.AI order the ending
