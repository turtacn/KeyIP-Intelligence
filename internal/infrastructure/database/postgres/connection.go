// Package postgres provides PostgreSQL connection pool management, transaction
// handling, and health-check utilities for the KeyIP-Intelligence platform.
// The connection pool is created once at application startup and injected into
// all repository implementations.
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

// ─────────────────────────────────────────────────────────────────────────────
// Constants for connection retry and pool configuration
// ─────────────────────────────────────────────────────────────────────────────

const (
	// maxRetries is the maximum number of connection attempts before giving up.
	maxRetries = 5

	// initialRetryDelay is the starting delay between retry attempts.
	// Subsequent attempts use exponential backoff: 1s, 2s, 4s, 8s, 16s.
	initialRetryDelay = 1 * time.Second

	// defaultMaxConns is the default maximum number of connections in the pool.
	defaultMaxConns = 25

	// defaultMinConns is the default minimum number of idle connections in the pool.
	defaultMinConns = 5

	// defaultMaxConnLifetime is the maximum duration a connection can be reused.
	defaultMaxConnLifetime = 1 * time.Hour

	// defaultMaxConnIdleTime is the maximum duration a connection can be idle.
	defaultMaxConnIdleTime = 30 * time.Minute

	// defaultHealthCheckPeriod is the interval between automatic health checks.
	defaultHealthCheckPeriod = 1 * time.Minute
)

// ─────────────────────────────────────────────────────────────────────────────
// NewConnectionPool — connection pool factory with retry logic
// ─────────────────────────────────────────────────────────────────────────────

// NewConnectionPool creates and initializes a pgxpool.Pool with exponential
// backoff retry logic. The pool is ready to use upon successful return.
//
// Retry strategy:
// - Attempts up to maxRetries (5) connections
// - Initial delay: 1s, then doubles each attempt (2s, 4s, 8s, 16s)
// - Logs each attempt and final success/failure
//
// The returned pool must be closed by the caller via Close() when the
// application shuts down.
func NewConnectionPool(cfg config.DatabaseConfig, logger logging.Logger) (*pgxpool.Pool, error) {
	connString := buildConnString(cfg)

	// Parse connection string and build pool config.
	poolConfig, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, fmt.Errorf("failed to parse connection string: %w", err)
	}

	// Apply custom pool configuration.
	configurePool(poolConfig, cfg)

	// Attempt to establish connection pool with exponential backoff.
	var pool *pgxpool.Pool
	retryDelay := initialRetryDelay

	for attempt := 1; attempt <= maxRetries; attempt++ {
		logger.Info("attempting database connection",
			logging.Int("attempt", attempt),
			logging.Int("max_attempts", maxRetries),
			logging.String("host", cfg.Host),
			logging.Int("port", cfg.Port),
			logging.String("database", cfg.DBName),
		)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		pool, err = pgxpool.NewWithConfig(ctx, poolConfig)
		cancel()

		if err == nil {
			// Verify connectivity with ping.
			pingCtx, pingCancel := context.WithTimeout(context.Background(), 5*time.Second)
			err = pool.Ping(pingCtx)
			pingCancel()

			if err == nil {
				logger.Info("database connection established",
					logging.String("host", cfg.Host),
					logging.Int("port", cfg.Port),
					logging.String("database", cfg.DBName),
					logging.Int("max_conns", int(poolConfig.MaxConns)),
				)
				return pool, nil
			}

			// Ping failed; close pool and retry.
			pool.Close()
			logger.Warn("database ping failed",
				logging.Int("attempt", attempt),
				logging.Err(err),
			)
		} else {
			logger.Warn("failed to create connection pool",
				logging.Int("attempt", attempt),
				logging.Err(err),
			)
		}

		// Last attempt failed; return error.
		if attempt == maxRetries {
			return nil, fmt.Errorf("failed to connect to database after %d attempts: %w", maxRetries, err)
		}

		// Exponential backoff before next retry.
		logger.Info("retrying database connection",
			logging.Float64("delay_seconds", retryDelay.Seconds()),
		)
		time.Sleep(retryDelay)
		retryDelay *= 2
	}

	// Unreachable code; satisfies compiler.
	return nil, fmt.Errorf("connection retry logic exhausted")
}

// ─────────────────────────────────────────────────────────────────────────────
// Close — graceful connection pool shutdown
// ─────────────────────────────────────────────────────────────────────────────

// Close gracefully shuts down the connection pool, waiting for all active
// connections to be released. This should be called during application shutdown.
//
// The pool must not be used after calling Close.
func Close(pool *pgxpool.Pool) {
	if pool != nil {
		pool.Close()
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// HealthCheck — connection liveness verification
// ─────────────────────────────────────────────────────────────────────────────

// HealthCheck executes a simple `SELECT 1` query to verify that the database
// is reachable and the connection pool is healthy. This is typically called by
// health-check HTTP endpoints or monitoring probes.
func HealthCheck(ctx context.Context, pool *pgxpool.Pool) error {
	if pool == nil {
		return fmt.Errorf("connection pool is nil")
	}

	// Execute a lightweight query to verify connectivity.
	var result int
	err := pool.QueryRow(ctx, "SELECT 1").Scan(&result)
	if err != nil {
		return fmt.Errorf("health check query failed: %w", err)
	}

	if result != 1 {
		return fmt.Errorf("health check returned unexpected value: %d", result)
	}

	return nil
}

// ─────────────────────────────────────────────────────────────────────────────
// buildConnString — construct PostgreSQL connection string
// ─────────────────────────────────────────────────────────────────────────────

// buildConnString constructs a PostgreSQL connection string in the standard
// URL format from the provided DatabaseConfig.
//
// Format: postgres://user:password@host:port/dbname?sslmode=xxx
func buildConnString(cfg config.DatabaseConfig) string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		cfg.User,
		cfg.Password,
		cfg.Host,
		cfg.Port,
		cfg.DBName,
		cfg.SSLMode,
	)
}

// ─────────────────────────────────────────────────────────────────────────────
// configurePool — apply custom pool settings
// ─────────────────────────────────────────────────────────────────────────────

// configurePool applies connection pool configuration from DatabaseConfig to
// the pgxpool.Config. This function sets sensible defaults when config values
// are zero.
func configurePool(poolConfig *pgxpool.Config, cfg config.DatabaseConfig) {
	// MaxConns: maximum number of connections in the pool.
	if cfg.MaxConns > 0 {
		poolConfig.MaxConns = int32(cfg.MaxConns)
	} else {
		poolConfig.MaxConns = defaultMaxConns
	}

	// MinConns: minimum number of idle connections to maintain.
	if cfg.MinConns > 0 {
		poolConfig.MinConns = int32(cfg.MinConns)
	} else {
		poolConfig.MinConns = defaultMinConns
	}

	// MaxConnLifetime: maximum duration a connection can be reused.
	if cfg.ConnMaxLifetime > 0 {
		poolConfig.MaxConnLifetime = cfg.ConnMaxLifetime
	} else {
		poolConfig.MaxConnLifetime = defaultMaxConnLifetime
	}

	// MaxConnIdleTime: maximum duration a connection can remain idle.
	if cfg.ConnMaxIdleTime > 0 {
		poolConfig.MaxConnIdleTime = cfg.ConnMaxIdleTime
	} else {
		poolConfig.MaxConnIdleTime = defaultMaxConnIdleTime
	}

	// HealthCheckPeriod: interval between automatic connection health checks.
	poolConfig.HealthCheckPeriod = defaultHealthCheckPeriod
}

// ─────────────────────────────────────────────────────────────────────────────
// WithTransaction — transaction wrapper with savepoint support
// ─────────────────────────────────────────────────────────────────────────────

// txKey is the context key used to store the active transaction.
type txKey struct{}

// WithTransaction executes the provided function within a database transaction.
// If fn returns an error or panics, the transaction is rolled back; otherwise,
// it is committed.
//
// Nested transactions are supported via PostgreSQL savepoints. If a transaction
// is already active in the context, a savepoint is created instead of starting
// a new top-level transaction.
//
// Usage:
//
//	err := WithTransaction(ctx, pool, func(tx pgx.Tx, txCtx context.Context) error {
//	    _, err := tx.Exec(txCtx, "INSERT INTO patents (...) VALUES (...)")
//	    return err
//	})
func WithTransaction(ctx context.Context, pool *pgxpool.Pool, fn func(tx pgx.Tx, txCtx context.Context) error) (err error) {
	// Check if we are already inside a transaction.
	if activeTx, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
		// Nested transaction: use a savepoint.
		savepointName := fmt.Sprintf("sp_%d", time.Now().UnixNano())
		_, err = activeTx.Exec(ctx, "SAVEPOINT "+savepointName)
		if err != nil {
			return fmt.Errorf("failed to create savepoint: %w", err)
		}

		// Ensure the savepoint is released or rolled back.
		defer func() {
			if p := recover(); p != nil {
				_, _ = activeTx.Exec(ctx, "ROLLBACK TO SAVEPOINT "+savepointName)
				panic(p)
			} else if err != nil {
				_, _ = activeTx.Exec(ctx, "ROLLBACK TO SAVEPOINT "+savepointName)
			} else {
				_, cmtErr := activeTx.Exec(ctx, "RELEASE SAVEPOINT "+savepointName)
				if cmtErr != nil {
					err = fmt.Errorf("failed to release savepoint: %w", cmtErr)
				}
			}
		}()

		return fn(activeTx, ctx)
	}

	// Begin a new top-level transaction.
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}

	// Put the transaction in the context so nested calls can find it.
	txCtx := context.WithValue(ctx, txKey{}, tx)

	// Ensure the transaction is finalized (commit or rollback).
	defer func() {
		if p := recover(); p != nil {
			// Panic occurred; rollback and re-panic.
			_ = tx.Rollback(ctx)
			panic(p)
		} else if err != nil {
			// Function returned an error; rollback.
			if rbErr := tx.Rollback(ctx); rbErr != nil {
				err = fmt.Errorf("rollback failed: %w (original error: %v)", rbErr, err)
			}
		} else {
			// Function succeeded; commit.
			if cmtErr := tx.Commit(ctx); cmtErr != nil {
				err = fmt.Errorf("commit failed: %w", cmtErr)
			}
		}
	}()

	// Execute the user-provided function within the transaction context.
	err = fn(tx, txCtx)
	return err
}

