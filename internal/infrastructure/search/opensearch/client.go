package opensearch

import (
	"context"
	"crypto/tls"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/opensearch-project/opensearch-go/v2"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

var (
	ErrInvalidConfig    = errors.New(errors.ErrCodeValidation, "invalid configuration")
	ErrConnectionFailed = errors.New(errors.ErrCodeInternal, "connection failed")
)

// ClientConfig holds the configuration for the OpenSearch client.
type ClientConfig struct {
	Addresses             []string
	Username              string
	Password              string
	TLSEnabled            bool
	TLSCertPath           string
	MaxRetries            int
	RetryBackoff          time.Duration
	RequestTimeout        time.Duration
	MaxIdleConnsPerHost   int
	HealthCheckInterval   time.Duration
	BulkFlushInterval     time.Duration
	BulkFlushBytes        int
}

// Client manages the OpenSearch client connection.
type Client struct {
	client *opensearch.Client
	config ClientConfig
	logger logging.Logger
	healthy atomic.Bool
	cancel context.CancelFunc
}

// NewClient creates a new OpenSearch client.
func NewClient(cfg ClientConfig, logger logging.Logger) (*Client, error) {
	if err := ValidateConfig(cfg); err != nil {
		return nil, err
	}

	// Fill defaults
	if cfg.MaxRetries == 0 {
		cfg.MaxRetries = 3
	}
	if cfg.RetryBackoff == 0 {
		cfg.RetryBackoff = 100 * time.Millisecond
	}
	if cfg.RequestTimeout == 0 {
		cfg.RequestTimeout = 30 * time.Second
	}
	if cfg.MaxIdleConnsPerHost == 0 {
		cfg.MaxIdleConnsPerHost = 10
	}
	if cfg.HealthCheckInterval == 0 {
		cfg.HealthCheckInterval = 30 * time.Second
	}
	if cfg.BulkFlushInterval == 0 {
		cfg.BulkFlushInterval = 5 * time.Second
	}
	if cfg.BulkFlushBytes == 0 {
		cfg.BulkFlushBytes = 5 * 1024 * 1024 // 5MB
	}

	transport := &http.Transport{
		MaxIdleConnsPerHost: cfg.MaxIdleConnsPerHost,
	}

	if cfg.TLSEnabled {
		transport.TLSClientConfig = &tls.Config{
			InsecureSkipVerify: true,
		}
	}

	osCfg := opensearch.Config{
		Addresses:     cfg.Addresses,
		Username:      cfg.Username,
		Password:      cfg.Password,
		MaxRetries:    cfg.MaxRetries,
		RetryBackoff:  func(i int) time.Duration { return cfg.RetryBackoff },
		Transport:     transport,
		RetryOnStatus: []int{502, 503, 504, 429},
	}

	client, err := opensearch.NewClient(osCfg)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeInternal, "failed to create opensearch client")
	}

	ctx, cancel := context.WithCancel(context.Background())
	c := &Client{
		client: client,
		config: cfg,
		logger: logger,
		cancel: cancel,
	}

	// Verify connectivity
	if err := c.Ping(ctx); err != nil {
		cancel()
		return nil, ErrConnectionFailed
	}

	go c.startHealthCheck(ctx)

	return c, nil
}

// Ping checks the connection to OpenSearch.
func (c *Client) Ping(ctx context.Context) error {
	resp, err := c.client.Ping(
		c.client.Ping.WithContext(ctx),
	)
	if err != nil {
		c.healthy.Store(false)
		c.logger.Warn("OpenSearch ping failed", logging.Error(err))
		return err
	}
	defer resp.Body.Close()

	if resp.IsError() {
		c.healthy.Store(false)
		c.logger.Warn("OpenSearch ping returned error status", logging.Int("status", resp.StatusCode))
		return errors.New(errors.ErrCodeInternal, "ping returned error status")
	}

	c.healthy.Store(true)
	return nil
}

// IsHealthy returns the current health status of the client.
func (c *Client) IsHealthy() bool {
	return c.healthy.Load()
}

// GetClient returns the underlying OpenSearch client.
func (c *Client) GetClient() *opensearch.Client {
	return c.client
}

// Close closes the client and stops the health check.
func (c *Client) Close() error {
	c.cancel()
	c.logger.Info("OpenSearch client closed")
	return nil
}

func (c *Client) startHealthCheck(ctx context.Context) {
	ticker := time.NewTicker(c.config.HealthCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			prev := c.healthy.Load()
			err := c.Ping(ctx)
			curr := c.healthy.Load()

			if prev && !curr {
				c.logger.Error("OpenSearch cluster became unhealthy", logging.Error(err))
			} else if !prev && curr {
				c.logger.Info("OpenSearch cluster recovered")
			}
		}
	}
}

// ValidateConfig validates the client configuration.
func ValidateConfig(cfg ClientConfig) error {
	if len(cfg.Addresses) == 0 {
		return ErrInvalidConfig
	}
	if cfg.MaxRetries < 0 {
		return errors.New(errors.ErrCodeValidation, "MaxRetries must be >= 0")
	}
	if cfg.RequestTimeout < 0 {
		return errors.New(errors.ErrCodeValidation, "RequestTimeout must be >= 0")
	}
	if cfg.RequestTimeout == 0 {
		return errors.New(errors.ErrCodeValidation, "RequestTimeout must be > 0")
	}

	if cfg.TLSEnabled && cfg.TLSCertPath == "" {
		return errors.New(errors.ErrCodeValidation, "TLSCertPath required when TLSEnabled is true")
	}
	return nil
}

//Personal.AI order the ending
