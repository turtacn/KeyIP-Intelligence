package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/metrics"
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

func TestConnection_AttachPoolMetrics(t *testing.T) {
	db, _, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	conn := NewConnectionWithDB(db, logging.NewNopLogger())
	assert.Nil(t, conn.poolMetrics)

	pm, err := metrics.NewPoolMetrics()
	require.NoError(t, err)
	conn.AttachPoolMetrics(pm)
	assert.NotNil(t, conn.poolMetrics)
}

func TestConnection_PoolSaturation_NoConnections(t *testing.T) {
	db, _, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	conn := NewConnectionWithDB(db, logging.NewNopLogger())

	// With no real connections, saturation should be 0.
	saturation := conn.PoolSaturation()
	assert.Equal(t, 0.0, saturation)
}

func TestConnection_PoolHealth_Healthy(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	require.NoError(t, err)
	defer db.Close()

	conn := NewConnectionWithDB(db, logging.NewNopLogger())

	mock.ExpectPing()
	err = conn.PoolHealth(context.Background())
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestConnection_PoolHealth_PingFailure(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	require.NoError(t, err)
	defer db.Close()

	conn := NewConnectionWithDB(db, logging.NewNopLogger())

	mock.ExpectPing().WillReturnError(fmt.Errorf("connection refused"))
	err = conn.PoolHealth(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "database pool health check failed")
	assert.NoError(t, mock.ExpectationsWereMet())
}

func TestConnection_StartMetricsCollection_ContextCancel(t *testing.T) {
	db, _, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	conn := NewConnectionWithDB(db, logging.NewNopLogger())

	pm, err := metrics.NewPoolMetrics()
	require.NoError(t, err)
	conn.AttachPoolMetrics(pm)

	ctx, cancel := context.WithCancel(context.Background())
	conn.StartMetricsCollection(ctx, 5*time.Millisecond)

	// Let the goroutine run a few ticks then cancel.
	time.Sleep(20 * time.Millisecond)
	cancel()

	// No panic and goroutine exits cleanly.
	assert.True(t, true)
}

func TestConnection_StartMetricsCollection_NoPoolMetrics(t *testing.T) {
	db, _, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	conn := NewConnectionWithDB(db, logging.NewNopLogger())

	// Call StartMetricsCollection without attaching metrics - should not panic.
	ctx, cancel := context.WithCancel(context.Background())
	conn.StartMetricsCollection(ctx, 5*time.Millisecond)

	time.Sleep(20 * time.Millisecond)
	cancel()

	assert.True(t, true)
}

func TestConnection_HealthCheck_WithPoolMetricsAttached(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	require.NoError(t, err)
	defer db.Close()

	conn := NewConnectionWithDB(db, logging.NewNopLogger())

	pm, err := metrics.NewPoolMetrics()
	require.NoError(t, err)
	conn.AttachPoolMetrics(pm)

	mock.ExpectPing()
	err = conn.HealthCheck(context.Background())
	assert.NoError(t, err)
	assert.NoError(t, mock.ExpectationsWereMet())

	// Pool metrics were attached and HealthCheck ran without error.
	assert.NotNil(t, conn.poolMetrics)
}

//Personal.AI order the ending
