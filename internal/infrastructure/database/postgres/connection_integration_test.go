//go:build integration

// Package postgres_test provides integration tests for the PostgreSQL
// connection management functionality.
package postgres_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/turtacn/KeyIP-Intelligence/internal/config"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/database/postgres"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
)

// ─────────────────────────────────────────────────────────────────────────────
// TestWithTransaction — transaction behavior (requires database)
// ─────────────────────────────────────────────────────────────────────────────

func TestWithTransaction_CommitsOnSuccess(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Execute a transaction that should commit.
	err := postgres.WithTransaction(ctx, pool, func(tx pgx.Tx, txCtx context.Context) error {
		_, err := tx.Exec(txCtx, "CREATE TEMP TABLE test_commit (id INT)")
		require.NoError(t, err)
		_, err = tx.Exec(txCtx, "INSERT INTO test_commit VALUES (1)")
		return err
	})

	require.NoError(t, err)

	// Verify the data was committed (temp tables are session-scoped).
	var count int
	err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM test_commit").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

func TestWithTransaction_RollsBackOnError(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Create a temp table outside the transaction for verification.
	_, err := pool.Exec(ctx, "CREATE TEMP TABLE test_rollback (id INT PRIMARY KEY)")
	require.NoError(t, err)

	// Execute a transaction that should rollback due to error.
	err = postgres.WithTransaction(ctx, pool, func(tx pgx.Tx, txCtx context.Context) error {
		_, err := tx.Exec(txCtx, "INSERT INTO test_rollback VALUES (1)")
		require.NoError(t, err)
		// Return an error to trigger rollback.
		return fmt.Errorf("intentional error for rollback test")
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "intentional error")

	// Verify the data was rolled back.
	var count int
	err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM test_rollback").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestWithTransaction_RollsBackOnPanic(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Create a temp table outside the transaction.
	_, err := pool.Exec(ctx, "CREATE TEMP TABLE test_panic (id INT)")
	require.NoError(t, err)

	// Execute a transaction that panics.
	assert.Panics(t, func() {
		_ = postgres.WithTransaction(ctx, pool, func(tx pgx.Tx, txCtx context.Context) error {
			_, _ = tx.Exec(txCtx, "INSERT INTO test_panic VALUES (1)")
			panic("intentional panic")
		})
	})

	// Verify the data was rolled back.
	var count int
	err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM test_panic").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestWithTransaction_NestedTransactions(t *testing.T) {
	pool, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Create a temp table.
	_, err := pool.Exec(ctx, "CREATE TEMP TABLE test_nested (id INT)")
	require.NoError(t, err)

	// Outer transaction that commits.
	err = postgres.WithTransaction(ctx, pool, func(outerTx pgx.Tx, outerCtx context.Context) error {
		_, err := outerTx.Exec(outerCtx, "INSERT INTO test_nested VALUES (1)")
		require.NoError(t, err)

		// Inner transaction (savepoint) that rolls back.
		innerErr := postgres.WithTransaction(outerCtx, pool, func(innerTx pgx.Tx, innerCtx context.Context) error {
			_, err := innerTx.Exec(innerCtx, "INSERT INTO test_nested VALUES (2)")
			require.NoError(t, err)
			return fmt.Errorf("inner transaction error")
		})
		assert.Error(t, innerErr)

		// Outer transaction should still be able to commit.
		return nil
	})

	require.NoError(t, err)

	// Only the outer transaction's insert should be visible.
	var count int
	err = pool.QueryRow(ctx, "SELECT COUNT(*) FROM test_nested").Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}

// ─────────────────────────────────────────────────────────────────────────────
// Test helpers
// ─────────────────────────────────────────────────────────────────────────────

func setupTestDB(t *testing.T) (*pgxpool.Pool, func()) {
	t.Helper()

	// These tests require a PostgreSQL instance.
	// Set INTEGRATION_TEST_DB_URL environment variable to run them.
	dbURL := os.Getenv("INTEGRATION_TEST_DB_URL")
	if dbURL == "" {
		t.Skip("INTEGRATION_TEST_DB_URL not set; skipping integration test")
	}

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

	cleanup := func() {
		postgres.Close(pool)
	}

	return pool, cleanup
}
