//go:build integration

package repositories_test

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
)

// noopLogger is a minimal no-op logger satisfying logging.Logger for collaboration tests.
type noopLogger struct{}

func (n noopLogger) Debug(msg string, fields ...logging.Field)      {}
func (n noopLogger) Info(msg string, fields ...logging.Field)       {}
func (n noopLogger) Warn(msg string, fields ...logging.Field)       {}
func (n noopLogger) Error(msg string, fields ...logging.Field)      {}
func (n noopLogger) Fatal(msg string, fields ...logging.Field)      {}
func (n noopLogger) With(fields ...logging.Field) logging.Logger    { return n }
func (n noopLogger) WithContext(ctx context.Context) logging.Logger  { return n }
func (n noopLogger) WithError(err error) logging.Logger             { return n }
func (n noopLogger) Sync() error                                    { return nil }

// startPostgres creates a pgxpool.Pool for collaboration tests.
func startPostgres(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://postgres:postgres@localhost:5432/keyip_test?sslmode=disable"
	}
	pool, err := pgxpool.New(context.Background(), dsn)
	if err != nil {
		t.Fatalf("Failed to create pool: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}
