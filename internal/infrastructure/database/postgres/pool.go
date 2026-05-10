// Package postgres provides database connection pooling and transaction management
// using pgx for integration test support and advanced connection management.
package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/turtacn/KeyIP-Intelligence/internal/config"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
)

// NewConnectionPool creates a new pgxpool connection pool from config.
func NewConnectionPool(cfg config.DatabaseConfig, logger logging.Logger) (*pgxpool.Pool, error) {
	dsn := buildPoolDSN(cfg.Postgres)

	poolCfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to parse pool config: %w", err)
	}

	if cfg.Postgres.MaxOpenConns > 0 {
		poolCfg.MaxConns = int32(cfg.Postgres.MaxOpenConns)
	} else {
		poolCfg.MaxConns = 25
	}

	ctx := context.Background()
	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection pool: %w", err)
	}

	// Verify connectivity.
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	logger.Info("Connected to PostgreSQL database via pool",
		logging.String("host", cfg.Postgres.Host),
		logging.String("database", cfg.Postgres.DBName),
	)

	return pool, nil
}

// Close gracefully shuts down a pgxpool connection pool.
func Close(pool *pgxpool.Pool) {
	if pool != nil {
		pool.Close()
	}
}

// WithTransaction executes a function within a database transaction.
// If the function returns an error, the transaction is rolled back.
// If the function panics, the transaction is rolled back and the panic is re-raised.
func WithTransaction(ctx context.Context, pool *pgxpool.Pool, fn func(tx pgx.Tx, ctx context.Context) error) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	rolledBack := false
	defer func() {
		if !rolledBack {
			_ = tx.Rollback(ctx)
		}
	}()

	if err := fn(tx, ctx); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	rolledBack = true
	return nil
}

// buildPoolDSN constructs a PostgreSQL connection string for pgxpool.
func buildPoolDSN(cfg config.PostgresConfig) string {
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%d/%s",
		cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.DBName)

	if cfg.SSLMode == "" {
		dsn += "?sslmode=disable"
	} else {
		dsn += "?sslmode=" + cfg.SSLMode
	}

	return dsn
}
