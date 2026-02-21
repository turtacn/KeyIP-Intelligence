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

// Result abstracts neo4j.ResultWithContext
type Result interface {
	Next(ctx context.Context) bool
	Record() *neo4j.Record
	Err() error
	Consume(ctx context.Context) (neo4j.ResultSummary, error)
	// Add other methods as needed
}

// Transaction abstracts neo4j.ManagedTransaction
type Transaction interface {
	Run(ctx context.Context, cypher string, params map[string]any) (Result, error)
}

// internalSession abstracts neo4j.SessionWithContext
type internalSession interface {
	ExecuteRead(ctx context.Context, work func(Transaction) (any, error)) (any, error)
	ExecuteWrite(ctx context.Context, work func(Transaction) (any, error)) (any, error)
	Close(ctx context.Context) error
}

// internalDriver abstracts neo4j.DriverWithContext
type internalDriver interface {
	VerifyConnectivity(ctx context.Context) error
	NewSession(ctx context.Context, config neo4j.SessionConfig) internalSession
	Close(ctx context.Context) error
}

// stdResult implements Result
type stdResult struct {
	res neo4j.ResultWithContext
}

func (r *stdResult) Next(ctx context.Context) bool {
	return r.res.Next(ctx)
}
func (r *stdResult) Record() *neo4j.Record {
	return r.res.Record()
}
func (r *stdResult) Err() error {
	return r.res.Err()
}
func (r *stdResult) Consume(ctx context.Context) (neo4j.ResultSummary, error) {
	return r.res.Consume(ctx)
}

// stdTransaction implements Transaction
type stdTransaction struct {
	tx neo4j.ManagedTransaction
}

func (t *stdTransaction) Run(ctx context.Context, cypher string, params map[string]any) (Result, error) {
	res, err := t.tx.Run(ctx, cypher, params)
	if err != nil {
		return nil, err
	}
	return &stdResult{res: res}, nil
}

// stdSession implements internalSession
type stdSession struct {
	s neo4j.SessionWithContext
}

func (s *stdSession) ExecuteRead(ctx context.Context, work func(Transaction) (any, error)) (any, error) {
	return s.s.ExecuteRead(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		return work(&stdTransaction{tx: tx})
	})
}

func (s *stdSession) ExecuteWrite(ctx context.Context, work func(Transaction) (any, error)) (any, error) {
	return s.s.ExecuteWrite(ctx, func(tx neo4j.ManagedTransaction) (any, error) {
		return work(&stdTransaction{tx: tx})
	})
}

func (s *stdSession) Close(ctx context.Context) error {
	return s.s.Close(ctx)
}

// stdDriver implements internalDriver
type stdDriver struct {
	d neo4j.DriverWithContext
}

func (d *stdDriver) VerifyConnectivity(ctx context.Context) error {
	return d.d.VerifyConnectivity(ctx)
}

func (d *stdDriver) NewSession(ctx context.Context, config neo4j.SessionConfig) internalSession {
	return &stdSession{s: d.d.NewSession(ctx, config)}
}

func (d *stdDriver) Close(ctx context.Context) error {
	return d.d.Close(ctx)
}

// Driver is the high-level wrapper
type Driver struct {
	driver internalDriver
	cfg    Neo4jConfig
	logger logging.Logger
	once   sync.Once
}

func NewDriver(cfg Neo4jConfig, log logging.Logger) (*Driver, error) {
	authToken := neo4j.BasicAuth(cfg.Username, cfg.Password, "")

	driver, err := neo4j.NewDriverWithContext(cfg.URI, authToken, func(c *neo4j.Config) {
		if cfg.MaxConnectionPoolSize > 0 {
			c.MaxConnectionPoolSize = cfg.MaxConnectionPoolSize
		} else {
			c.MaxConnectionPoolSize = 50
		}
		if cfg.MaxConnectionLifetime > 0 {
			c.MaxConnectionLifetime = cfg.MaxConnectionLifetime
		} else {
			c.MaxConnectionLifetime = 1 * time.Hour
		}
		if cfg.ConnectionAcquisitionTimeout > 0 {
			c.ConnectionAcquisitionTimeout = cfg.ConnectionAcquisitionTimeout
		} else {
			c.ConnectionAcquisitionTimeout = 60 * time.Second
		}
	})
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to create neo4j driver")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := driver.VerifyConnectivity(ctx); err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "failed to connect to neo4j")
	}

	log.Info("Connected to Neo4j", logging.String("uri", cfg.URI), logging.String("database", cfg.Database))

	return &Driver{
		driver: &stdDriver{d: driver},
		cfg:    cfg,
		logger: log,
	}, nil
}

func (d *Driver) Session(ctx context.Context, accessMode neo4j.AccessMode) internalSession {
	dbName := d.cfg.Database
	if dbName == "" {
		dbName = "neo4j"
	}
	return d.driver.NewSession(ctx, neo4j.SessionConfig{
		DatabaseName: dbName,
		AccessMode:   accessMode,
	})
}

func (d *Driver) ReadSession(ctx context.Context) internalSession {
	return d.Session(ctx, neo4j.AccessModeRead)
}

func (d *Driver) WriteSession(ctx context.Context) internalSession {
	return d.Session(ctx, neo4j.AccessModeWrite)
}

func (d *Driver) ExecuteRead(ctx context.Context, work func(Transaction) (interface{}, error)) (interface{}, error) {
	session := d.ReadSession(ctx)
	defer session.Close(ctx)

	result, err := session.ExecuteRead(ctx, work)
	if err != nil {
		d.logger.Error("Neo4j read transaction failed", logging.Err(err))
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "neo4j read failed")
	}
	return result, nil
}

func (d *Driver) ExecuteWrite(ctx context.Context, work func(Transaction) (interface{}, error)) (interface{}, error) {
	session := d.WriteSession(ctx)
	defer session.Close(ctx)

	result, err := session.ExecuteWrite(ctx, work)
	if err != nil {
		d.logger.Error("Neo4j write transaction failed", logging.Err(err))
		return nil, errors.Wrap(err, errors.ErrCodeDatabaseError, "neo4j write failed")
	}
	return result, nil
}

func (d *Driver) HealthCheck(ctx context.Context) error {
	if err := d.driver.VerifyConnectivity(ctx); err != nil {
		return errors.Wrap(err, errors.ErrCodeDatabaseError, "neo4j connectivity check failed")
	}

	// Execute simple query
	_, err := d.ExecuteRead(ctx, func(tx Transaction) (interface{}, error) {
		result, err := tx.Run(ctx, "RETURN 1 AS health", nil)
		if err != nil { return nil, err }
		if result.Next(ctx) {
			return result.Record().Values[0], nil
		}
		return nil, result.Err()
	})
	return err
}

func (d *Driver) Close() error {
	var err error
	d.once.Do(func() {
		err = d.driver.Close(context.Background())
		if err == nil {
			d.logger.Info("Closed Neo4j driver")
		} else {
			d.logger.Error("Failed to close Neo4j driver", logging.Err(err))
		}
	})
	return err
}

// Helpers

func ExtractSingleRecord[T any](ctx context.Context, result Result, mapper func(*neo4j.Record) (T, error)) (T, error) {
	var zero T
	if result.Next(ctx) {
		return mapper(result.Record())
	}
	if err := result.Err(); err != nil {
		return zero, err
	}
	return zero, errors.New(errors.ErrCodeNotFound, "no record found")
}

func CollectRecords[T any](ctx context.Context, result Result, mapper func(*neo4j.Record) (T, error)) ([]T, error) {
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
