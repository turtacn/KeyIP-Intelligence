package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/lib/pq"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/metrics"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// anyDB matches sql.DB return type for mocking
type anyDB interface {
	PingContext(ctx context.Context) error
	SetMaxOpenConns(n int)
	SetMaxIdleConns(n int)
	SetConnMaxLifetime(d time.Duration)
	SetConnMaxIdleTime(d time.Duration)
	Close() error
	Stats() sql.DBStats
}

// sqlOpen is a variable to allow mocking in tests.
var sqlOpen = func(driverName, dataSourceName string) (*sql.DB, error) {
	return sql.Open(driverName, dataSourceName)
}

// PostgresConfig holds the database configuration.
type PostgresConfig struct {
	Host             string        `yaml:"host" env:"POSTGRES_HOST"`
	Port             int           `yaml:"port" env:"POSTGRES_PORT"`
	Database         string        `yaml:"database" env:"POSTGRES_DB"`
	Username         string        `yaml:"username" env:"POSTGRES_USER"`
	Password         string        `yaml:"password" env:"POSTGRES_PASSWORD"`
	SSLMode          string        `yaml:"ssl_mode" env:"POSTGRES_SSL_MODE"`
	MaxOpenConns     int           `yaml:"max_open_conns"`
	MaxIdleConns     int           `yaml:"max_idle_conns"`
	ConnMaxLifetime  time.Duration `yaml:"conn_max_lifetime"`
	ConnMaxIdleTime  time.Duration `yaml:"conn_max_idle_time"`
	StatementTimeout time.Duration `yaml:"statement_timeout"`
	LockTimeout      time.Duration `yaml:"lock_timeout"`
}

// Connection manages the PostgreSQL database connection pool.
type Connection struct {
	db     *sql.DB
	cfg    PostgresConfig
	logger logging.Logger
	once   sync.Once

	// poolMetrics holds the OpenTelemetry instruments for pool monitoring.
	// Set via AttachPoolMetrics. If non-nil, pool stats are recorded during
	// HealthCheck and periodic collection.
	poolMetrics *metrics.PoolMetrics
}

// NewConnection establishes a connection to the PostgreSQL database.
func NewConnection(cfg PostgresConfig, log logging.Logger) (*Connection, error) {
	dsn := buildDSN(cfg)

	db, err := sqlOpen("postgres", dsn)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to open database connection")
	}

	// Set connection pool settings
	if cfg.MaxOpenConns > 0 {
		db.SetMaxOpenConns(cfg.MaxOpenConns)
	} else {
		db.SetMaxOpenConns(25) // Default
	}

	if cfg.MaxIdleConns > 0 {
		db.SetMaxIdleConns(cfg.MaxIdleConns)
	} else {
		db.SetMaxIdleConns(10) // Default
	}

	if cfg.ConnMaxLifetime > 0 {
		db.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	} else {
		db.SetConnMaxLifetime(30 * time.Minute) // Default
	}

	if cfg.ConnMaxIdleTime > 0 {
		db.SetConnMaxIdleTime(cfg.ConnMaxIdleTime)
	} else {
		db.SetConnMaxIdleTime(5 * time.Minute) // Default
	}

	// Verify connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to ping database")
	}

	log.Info("Connected to PostgreSQL database",
		logging.String("host", cfg.Host),
		logging.Int("port", cfg.Port),
		logging.String("database", cfg.Database),
	)

	return &Connection{
		db:     db,
		cfg:    cfg,
		logger: log,
	}, nil
}

// DB returns the underlying sql.DB instance.
func (c *Connection) DB() *sql.DB {
	return c.db
}

// NewConnectionWithDB creates a Connection with an existing sql.DB (for testing).
func NewConnectionWithDB(db *sql.DB, log logging.Logger) *Connection {
	return &Connection{
		db:     db,
		logger: log,
	}
}

// HealthCheck verifies the database connection status and records pool metrics.
// It checks both the basic connectivity (Ping) and connection pool saturation.
// If pool metrics are attached, a stats snapshot is recorded automatically.
func (c *Connection) HealthCheck(ctx context.Context) error {
	if err := c.db.PingContext(ctx); err != nil {
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "database health check failed")
	}

	// Check pool stats
	stats := c.Stats()
	if stats.OpenConnections > 0 {
		usage := float64(stats.InUse) / float64(stats.OpenConnections)
		if usage > 0.8 {
			c.logger.Warn("High database connection pool usage",
				logging.Int("in_use", stats.InUse),
				logging.Int("open", stats.OpenConnections),
				logging.Float64("usage", usage),
			)
		}
	}

	// Check pool saturation against max connections for capacity issues.
	if stats.MaxOpenConnections > 0 {
		saturation := float64(stats.InUse) / float64(stats.MaxOpenConnections)
		if saturation > 0.9 {
			c.logger.Error("Database connection pool is saturated",
				logging.Int("in_use", stats.InUse),
				logging.Int("max_open", stats.MaxOpenConnections),
				logging.Float64("saturation", saturation),
			)
		} else if saturation > 0.8 {
			c.logger.Warn("Database connection pool nearing saturation",
				logging.Int("in_use", stats.InUse),
				logging.Int("max_open", stats.MaxOpenConnections),
				logging.Float64("saturation", saturation),
			)
		}
	}

	// Record pool metrics if metrics collector is attached.
	if c.poolMetrics != nil {
		c.poolMetrics.RecordPoolStats(ctx, stats)
	}

	return nil
}

// Stats returns database statistics.
func (c *Connection) Stats() sql.DBStats {
	return c.db.Stats()
}

// AttachPoolMetrics attaches an OpenTelemetry PoolMetrics instance to this
// connection. When attached, pool statistics are automatically recorded during
// HealthCheck calls and periodic metrics collection.
func (c *Connection) AttachPoolMetrics(pm *metrics.PoolMetrics) {
	c.poolMetrics = pm
}

// StartMetricsCollection launches a background goroutine that periodically
// collects connection pool statistics and records them as OpenTelemetry metrics.
// The collection runs until the provided context is cancelled. AttachPoolMetrics
// should be called before this method to set up the metric instruments.
func (c *Connection) StartMetricsCollection(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				stats := c.Stats()
				if c.poolMetrics != nil {
					c.poolMetrics.RecordPoolStats(ctx, stats)
				}
			case <-ctx.Done():
				return
			}
		}
	}()
}

// PoolSaturation returns the ratio of in-use connections to the maximum
// allowed open connections. Returns 0 if MaxOpenConns is not configured.
// A value near 1.0 indicates the pool is approaching capacity.
func (c *Connection) PoolSaturation() float64 {
	stats := c.Stats()
	if stats.MaxOpenConnections > 0 {
		return float64(stats.InUse) / float64(stats.MaxOpenConnections)
	}
	return 0
}

// PoolHealth checks whether the connection pool is healthy. It returns an
// error if the pool saturation is at or above 90% or if the database is
// unreachable. This method is suitable for use with gRPC health checkers.
func (c *Connection) PoolHealth(ctx context.Context) error {
	if err := c.db.PingContext(ctx); err != nil {
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "database pool health check failed")
	}

	saturation := c.PoolSaturation()
	if saturation >= 0.9 {
		return errors.Wrap(
			fmt.Errorf("pool saturation %.2f exceeds threshold 0.90", saturation),
			errors.ErrCodeDatabaseError,
			"database connection pool is saturated",
		)
	}

	return nil
}

// Close closes the database connection.
func (c *Connection) Close() error {
	var err error
	c.once.Do(func() {
		err = c.db.Close()
		if err == nil {
			c.logger.Info("Closed PostgreSQL database connection")
		} else {
			c.logger.Error("Failed to close PostgreSQL database connection", logging.Err(err))
		}
	})
	return err
}

// RunMigrations runs database migrations.
func (c *Connection) RunMigrations(migrationsDir string) error {
	driver, err := postgres.WithInstance(c.db, &postgres.Config{})
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeInternal, "failed to create migration driver")
	}

	m, err := migrate.NewWithDatabaseInstance(
		"file://"+migrationsDir,
		"postgres",
		driver,
	)
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeInternal, "failed to create migrate instance")
	}

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		version, _, _ := m.Version()
		return errors.Wrap(err, errors.ErrCodeInternal, fmt.Sprintf("failed to run migrations (current version: %d)", version))
	}

	version, dirty, err := m.Version()
	if err != nil && err != migrate.ErrNilVersion {
		c.logger.Warn("Failed to get migration version", logging.Err(err))
	}

	c.logger.Info("Database migrations completed",
		logging.Int64("version", int64(version)),
		logging.Bool("dirty", dirty),
	)

	return nil
}

// buildDSN constructs the PostgreSQL connection string.
func buildDSN(cfg PostgresConfig) string {
	u := url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(cfg.Username, cfg.Password),
		Host:   fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Path:   cfg.Database,
	}

	q := u.Query()
	if cfg.SSLMode != "" {
		q.Set("sslmode", cfg.SSLMode)
	} else {
		q.Set("sslmode", "disable")
	}

	if cfg.StatementTimeout > 0 {
		q.Set("statement_timeout", fmt.Sprintf("%d", cfg.StatementTimeout.Milliseconds()))
	} else {
		q.Set("statement_timeout", "30000") // Default 30s
	}

	if cfg.LockTimeout > 0 {
		q.Set("lock_timeout", fmt.Sprintf("%d", cfg.LockTimeout.Milliseconds()))
	} else {
		q.Set("lock_timeout", "10000") // Default 10s
	}

	u.RawQuery = q.Encode()
	return u.String()
}

//Personal.AI order the ending
