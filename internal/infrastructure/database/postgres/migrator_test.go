// Package postgres_test provides integration tests for the database migration
// functionality. These tests require a live PostgreSQL instance.
//
//go:build integration

package postgres_test

import (
	"context"
	"os"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/turtacn/KeyIP-Intelligence/internal/config"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/database/postgres"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
)

// ─────────────────────────────────────────────────────────────────────────────
// Test environment setup
// ─────────────────────────────────────────────────────────────────────────────

const (
	// testMigrationsPath is the path to test migration files.
	// Adjust this path based on your project structure.
	testMigrationsPath = "file://./migrations"
)

func getTestDBURL(t *testing.T) string {
	t.Helper()

	dbURL := os.Getenv("INTEGRATION_TEST_DB_URL")
	if dbURL == "" {
		t.Skip("INTEGRATION_TEST_DB_URL not set; skipping integration test")
	}

	return dbURL
}

// ─────────────────────────────────────────────────────────────────────────────
// TestRunMigrations — verify migrations can be applied to an empty database
// ─────────────────────────────────────────────────────────────────────────────

func TestRunMigrations_AppliesAllMigrations(t *testing.T) {
	dbURL := getTestDBURL(t)

	// Reset database to ensure clean state.
	err := postgres.ResetDatabase(dbURL, testMigrationsPath)
	require.NoError(t, err)

	// Run migrations.
	err = postgres.RunMigrations(dbURL, testMigrationsPath)
	require.NoError(t, err)

	// Verify migrations were applied by checking version.
	version, dirty, err := postgres.MigrationStatus(dbURL, testMigrationsPath)
	require.NoError(t, err)
	assert.False(t, dirty, "migration state should not be dirty")
	assert.Greater(t, version, uint(0), "version should be greater than 0 after migrations")
}

func TestRunMigrations_NoChangeWhenAlreadyUpToDate(t *testing.T) {
	dbURL := getTestDBURL(t)

	// Apply all migrations first.
	err := postgres.RunMigrations(dbURL, testMigrationsPath)
	require.NoError(t, err)

	// Run migrations again; should return no error (no change).
	err = postgres.RunMigrations(dbURL, testMigrationsPath)
	require.NoError(t, err)
}

// ─────────────────────────────────────────────────────────────────────────────
// TestRollbackMigration — verify migrations can be rolled back
// ─────────────────────────────────────────────────────────────────────────────

func TestRollbackMigration_RollsBackSpecifiedSteps(t *testing.T) {
	dbURL := getTestDBURL(t)

	// Reset and apply all migrations.
	err := postgres.ResetDatabase(dbURL, testMigrationsPath)
	require.NoError(t, err)

	initialVersion, _, err := postgres.MigrationStatus(dbURL, testMigrationsPath)
	require.NoError(t, err)

	// Rollback 1 step.
	err = postgres.RollbackMigration(dbURL, testMigrationsPath, 1)
	require.NoError(t, err)

	// Verify version decreased.
	newVersion, dirty, err := postgres.MigrationStatus(dbURL, testMigrationsPath)
	require.NoError(t, err)
	assert.False(t, dirty)
	assert.Equal(t, initialVersion-1, newVersion)
}

func TestRollbackMigration_FailsWhenStepsIsZero(t *testing.T) {
	dbURL := getTestDBURL(t)

	err := postgres.RollbackMigration(dbURL, testMigrationsPath, 0)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "steps must be greater than 0")
}

func TestRollbackMigration_FailsWhenNoMigrationsToRollback(t *testing.T) {
	dbURL := getTestDBURL(t)

	// Reset database to version 0.
	err := postgres.ResetDatabase(dbURL, testMigrationsPath)
	require.NoError(t, err)

	// Roll back all migrations.
	err = postgres.RollbackMigration(dbURL, testMigrationsPath, 100) // Attempt to rollback more than exist.
	require.Error(t, err)
}

// ─────────────────────────────────────────────────────────────────────────────
// TestMigrationStatus — verify status reporting
// ─────────────────────────────────────────────────────────────────────────────

func TestMigrationStatus_ReturnsCorrectVersion(t *testing.T) {
	dbURL := getTestDBURL(t)

	// Reset and apply all migrations.
	err := postgres.ResetDatabase(dbURL, testMigrationsPath)
	require.NoError(t, err)

	version, dirty, err := postgres.MigrationStatus(dbURL, testMigrationsPath)
	require.NoError(t, err)
	assert.False(t, dirty)
	assert.Greater(t, version, uint(0))
}

func TestMigrationStatus_ReturnsZeroWhenNoMigrationsApplied(t *testing.T) {
	dbURL := getTestDBURL(t)

	// Roll back all migrations to version 0.
	m, err := migrate.New(testMigrationsPath, dbURL)
	require.NoError(t, err)
	defer m.Close()

	_ = m.Down()

	version, dirty, err := postgres.MigrationStatus(dbURL, testMigrationsPath)
	require.NoError(t, err)
	assert.False(t, dirty)
	assert.Equal(t, uint(0), version)
}

// ─────────────────────────────────────────────────────────────────────────────
// TestResetDatabase — verify complete reset works
// ─────────────────────────────────────────────────────────────────────────────

func TestResetDatabase_DropsAndRecreatesSchema(t *testing.T) {
	dbURL := getTestDBURL(t)

	// Apply migrations first.
	err := postgres.RunMigrations(dbURL, testMigrationsPath)
	require.NoError(t, err)

	// Reset database.
	err = postgres.ResetDatabase(dbURL, testMigrationsPath)
	require.NoError(t, err)

	// Verify migrations were re-applied.
	version, dirty, err := postgres.MigrationStatus(dbURL, testMigrationsPath)
	require.NoError(t, err)
	assert.False(t, dirty)
	assert.Greater(t, version, uint(0))
}

// ─────────────────────────────────────────────────────────────────────────────
// TestForceMigrationVersion — verify version forcing (dangerous operation)
// ─────────────────────────────────────────────────────────────────────────────

func TestForceMigrationVersion_SetsVersionManually(t *testing.T) {
	dbURL := getTestDBURL(t)

	// Reset database.
	err := postgres.ResetDatabase(dbURL, testMigrationsPath)
	require.NoError(t, err)

	// Force version to 1.
	err = postgres.ForceMigrationVersion(dbURL, testMigrationsPath, 1)
	require.NoError(t, err)

	// Verify version is now 1.
	version, dirty, err := postgres.MigrationStatus(dbURL, testMigrationsPath)
	require.NoError(t, err)
	assert.Equal(t, uint(1), version)
	assert.False(t, dirty)
}

// ─────────────────────────────────────────────────────────────────────────────
// Test table existence after migration
// ─────────────────────────────────────────────────────────────────────────────

func TestRunMigrations_CreatesExpectedTables(t *testing.T) {
	dbURL := getTestDBURL(t)

	// Reset and apply all migrations.
	err := postgres.ResetDatabase(dbURL, testMigrationsPath)
	require.NoError(t, err)

	// Connect to database and verify tables exist.
	cfg := config.DatabaseConfig{
		Host:     "localhost",
		Port:     5432,
		User:     "test",
		Password: "test",
		DBName:   "test_keyip",
		SSLMode:  "disable",
	}

	logger := logging.NewNopLogger()
	pool, err := postgres.NewConnectionPool(cfg, logger)
	require.NoError(t, err)
	defer postgres.Close(pool)

	ctx := context.Background()

	expectedTables := []string{
		"patents",
		"claims",
		"markush_structures",
		"molecules",
		"portfolios",
		"patent_lifecycles",
		"workspaces",
		"audit_logs",
	}

	for _, table := range expectedTables {
		var exists bool
		query := `SELECT EXISTS (
			SELECT FROM information_schema.tables 
			WHERE table_schema = 'public' 
			AND table_name = $1
		)`
		err := pool.QueryRow(ctx, query, table).Scan(&exists)
		require.NoError(t, err)
		assert.True(t, exists, "table %s should exist after migrations", table)
	}
}

