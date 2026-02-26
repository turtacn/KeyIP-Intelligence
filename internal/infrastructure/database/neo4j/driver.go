package neo4j

import (
	"context"
	"sync"
	"time"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

type Neo4jConfig struct {
	URI                          string        `yaml:"uri" env:"NEO4J_URI"`
	Username                     string        `yaml:"username" env:"NEO4J_USERNAME"`
	Password                     string        `yaml:"password" env:"NEO4J_PASSWORD"`
	Database                     string        `yaml:"database" env:"NEO4J_DATABASE"`
	MaxConnectionPoolSize        int           `yaml:"max_connection_pool_size"`
	MaxConnectionLifetime        time.Duration `yaml:"max_connection_lifetime"`
	ConnectionAcquisitionTimeout time.Duration `yaml:"connection_acquisition_timeout"`
	Encrypted                    bool          `yaml:"encrypted"`
}

// Result interface abstracts neo4j result to allow mocking
type Result interface {
	Next(ctx context.Context) bool
	Record() *neo4j.Record
	Err() error
	Consume(ctx context.Context) (neo4j.ResultSummary, error)
	Peek(ctx context.Context) bool
	// We avoid methods that might be unexported or hard to mock
}

// Transaction interface to abstract neo4j operations
type Transaction interface {
	Run(ctx context.Context, cypher string, params map[string]any) (Result, error)
}

// TransactionWork matches our interface
type TransactionWork func(tx Transaction) (any, error)

// DriverInterface abstracts the Neo4j driver wrapper
type DriverInterface interface {
	ExecuteRead(ctx context.Context, work TransactionWork) (interface{}, error)
	ExecuteWrite(ctx context.Context, work TransactionWork) (interface{}, error)
	HealthCheck(ctx context.Context) error
	Close(ctx context.Context) error
}

type Driver struct {
	driver neo4j.DriverWithContext
	cfg    Neo4jConfig
	logger logging.Logger
	once   sync.Once
}

func NewDriver(cfg Neo4jConfig, log logging.Logger) (*Driver, error) {
	if cfg.Database == "" {
		cfg.Database = "neo4j"
	}
	if cfg.MaxConnectionPoolSize == 0 {
		cfg.MaxConnectionPoolSize = 50
	}
	if cfg.MaxConnectionLifetime == 0 {
		cfg.MaxConnectionLifetime = 1 * time.Hour
	}
	if cfg.ConnectionAcquisitionTimeout == 0 {
		cfg.ConnectionAcquisitionTimeout = 60 * time.Second
	}

	auth := neo4j.BasicAuth(cfg.Username, cfg.Password, "")

	driver, err := neo4j.NewDriverWithContext(cfg.URI, auth, func(c *neo4j.Config) {
		c.MaxConnectionPoolSize = cfg.MaxConnectionPoolSize
		c.MaxConnectionLifetime = cfg.MaxConnectionLifetime
		c.ConnectionAcquisitionTimeout = cfg.ConnectionAcquisitionTimeout
	})
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to create neo4j driver")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := driver.VerifyConnectivity(ctx); err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to verify neo4j connectivity")
	}

	log.Info("Connected to Neo4j database",
		logging.String("uri", cfg.URI),
		logging.String("database", cfg.Database),
	)

	return &Driver{
		driver: driver,
		cfg:    cfg,
		logger: log,
	}, nil
}

func NewDriverWithInstance(driver neo4j.DriverWithContext, log logging.Logger) *Driver {
	return &Driver{
		driver: driver,
		logger: log,
		cfg:    Neo4jConfig{Database: "neo4j"},
	}
}

func (d *Driver) Session(ctx context.Context, accessMode neo4j.AccessMode) neo4j.SessionWithContext {
	return d.driver.NewSession(ctx, neo4j.SessionConfig{
		DatabaseName: d.cfg.Database,
		AccessMode:   accessMode,
	})
}

// adapter implements Transaction using neo4j.ExplicitTransaction
type txAdapter struct {
	tx neo4j.ExplicitTransaction
}

func (t *txAdapter) Run(ctx context.Context, cypher string, params map[string]any) (Result, error) {
	return t.tx.Run(ctx, cypher, params)
}

func (d *Driver) ExecuteRead(ctx context.Context, work TransactionWork) (interface{}, error) {
	session := d.Session(ctx, neo4j.AccessModeRead)
	defer session.Close(ctx)

	tx, err := session.BeginTransaction(ctx)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to begin read transaction")
	}
	defer tx.Close(ctx)

	res, err := work(&txAdapter{tx: tx})
	if err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to commit read transaction")
	}

	return res, nil
}

func (d *Driver) ExecuteWrite(ctx context.Context, work TransactionWork) (interface{}, error) {
	session := d.Session(ctx, neo4j.AccessModeWrite)
	defer session.Close(ctx)

	tx, err := session.BeginTransaction(ctx)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to begin write transaction")
	}
	defer tx.Close(ctx)

	res, err := work(&txAdapter{tx: tx})
	if err != nil {
		tx.Rollback(ctx)
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to commit write transaction")
	}

	return res, nil
}

func (d *Driver) HealthCheck(ctx context.Context) error {
	if err := d.driver.VerifyConnectivity(ctx); err != nil {
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "neo4j connectivity check failed")
	}

	_, err := d.ExecuteRead(ctx, func(tx Transaction) (interface{}, error) {
		res, err := tx.Run(ctx, "RETURN 1 AS health", nil)
		if err != nil {
			return nil, err
		}
		if res.Next(ctx) {
			return res.Record().Values[0], nil
		}
		return nil, res.Err()
	})

	return err
}

func (d *Driver) Close(ctx context.Context) error {
	var err error
	d.once.Do(func() {
		err = d.driver.Close(ctx)
		if err == nil {
			d.logger.Info("Closed Neo4j driver")
		} else {
			d.logger.Error("Failed to close Neo4j driver", logging.Err(err))
		}
	})
	return err
}

// Helpers

func ExtractSingleRecord[T any](result Result, ctx context.Context) (T, error) {
	var zero T
	if result.Next(ctx) {
		rec := result.Record()
		if len(rec.Values) > 0 {
			if v, ok := rec.Values[0].(T); ok {
				return v, nil
			}
			return zero, errors.New(errors.ErrCodeSerialization, "unexpected type")
		}
	}
	if err := result.Err(); err != nil {
		return zero, err
	}
	return zero, errors.New(errors.ErrCodeNotFound, "no record found")
}

func CollectRecords[T any](result Result, ctx context.Context, mapper func(*neo4j.Record) (T, error)) ([]T, error) {
	var items []T
	for result.Next(ctx) {
		item, err := mapper(result.Record())
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	if err := result.Err(); err != nil {
		return nil, err
	}
	return items, nil
}
//Personal.AI order the ending
