package minio

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/minio/minio-go/v7/pkg/lifecycle"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

var (
	ErrMinIOClientClosed = errors.New(errors.ErrCodeInternal, "minio client is closed")
	ErrBucketNotFound    = errors.New(errors.ErrCodeNotFound, "bucket not found")
	ErrBucketCreationFailed = errors.New(errors.ErrCodeInternal, "bucket creation failed")
)

type MinIOConfig struct {
	Endpoint        string        `mapstructure:"endpoint"`
	AccessKeyID     string        `mapstructure:"access_key_id"`
	SecretAccessKey string        `mapstructure:"secret_access_key"`
	UseSSL          bool          `mapstructure:"use_ssl"`
	Region          string        `mapstructure:"region"`
	DefaultBucket   string        `mapstructure:"default_bucket"`
	Buckets         BucketConfig  `mapstructure:"buckets"`
	PartSize        int64         `mapstructure:"part_size"`
	MaxRetries      int           `mapstructure:"max_retries"`
	PresignExpiry   time.Duration `mapstructure:"presign_expiry"`
	TempFileExpiry  int           `mapstructure:"temp_file_expiry"`
}

type BucketConfig struct {
	Documents   string `mapstructure:"documents"`
	Models      string `mapstructure:"models"`
	Reports     string `mapstructure:"reports"`
	Exports     string `mapstructure:"exports"`
	Temp        string `mapstructure:"temp"`
	Attachments string `mapstructure:"attachments"`
}

type MinIOClient struct {
	client *minio.Client
	config *MinIOConfig
	logger logging.Logger
	mu     sync.RWMutex
	closed bool
}

// MinIOClientInterface defines the interface for MinIO client wrapper to facilitate mocking.
type MinIOClientInterface interface {
	GetClient() *minio.Client
	GetBucketName(bucketType string) string
	Ping(ctx context.Context) error
	Close() error
	HealthCheck(ctx context.Context) (*HealthStatus, error)
	GetBucketStats(ctx context.Context, bucketName string) (*BucketStats, error)
	GeneratePresignedGetURL(ctx context.Context, bucketName, objectName string, expiry time.Duration) (string, error)
	GeneratePresignedPutURL(ctx context.Context, bucketName, objectName string, expiry time.Duration) (string, error)
}

func NewMinIOClient(cfg *MinIOConfig, log logging.Logger) (*MinIOClient, error) {
	applyDefaults(cfg)

	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		Secure: cfg.UseSSL,
		Region: cfg.Region,
	})
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeExternalService, "failed to create minio client")
	}

	mClient := &MinIOClient{
		client: client,
		config: cfg,
		logger: log,
	}

	// Verify connection
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// ListBuckets as ping
	if _, err := client.ListBuckets(ctx); err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeExternalService, "failed to connect to minio")
	}

	// Ensure buckets
	if err := mClient.EnsureBuckets(ctx); err != nil {
		return nil, err
	}

	// Setup lifecycle
	if err := mClient.SetupLifecycleRules(ctx); err != nil {
		log.Warn("Failed to setup lifecycle rules", logging.Err(err))
	}

	log.Info("MinIO client connected",
		logging.String("endpoint", cfg.Endpoint),
		logging.Bool("ssl", cfg.UseSSL),
	)

	return mClient, nil
}

func applyDefaults(cfg *MinIOConfig) {
	if cfg.Region == "" {
		cfg.Region = "us-east-1"
	}
	if cfg.PartSize == 0 {
		cfg.PartSize = 16 * 1024 * 1024 // 16MB
	}
	if cfg.MaxRetries == 0 {
		cfg.MaxRetries = 3
	}
	if cfg.PresignExpiry == 0 {
		cfg.PresignExpiry = 1 * time.Hour
	}
	if cfg.TempFileExpiry == 0 {
		cfg.TempFileExpiry = 7
	}

	// Default bucket names
	if cfg.DefaultBucket == "" { cfg.DefaultBucket = "keyip-documents" }
	if cfg.Buckets.Documents == "" { cfg.Buckets.Documents = "keyip-documents" }
	if cfg.Buckets.Models == "" { cfg.Buckets.Models = "keyip-models" }
	if cfg.Buckets.Reports == "" { cfg.Buckets.Reports = "keyip-reports" }
	if cfg.Buckets.Exports == "" { cfg.Buckets.Exports = "keyip-exports" }
	if cfg.Buckets.Temp == "" { cfg.Buckets.Temp = "keyip-temp" }
	if cfg.Buckets.Attachments == "" { cfg.Buckets.Attachments = "keyip-attachments" }
}

func (c *MinIOClient) EnsureBuckets(ctx context.Context) error {
	buckets := []string{
		c.config.Buckets.Documents,
		c.config.Buckets.Models,
		c.config.Buckets.Reports,
		c.config.Buckets.Exports,
		c.config.Buckets.Temp,
		c.config.Buckets.Attachments,
	}

	for _, bucket := range buckets {
		exists, err := c.client.BucketExists(ctx, bucket)
		if err != nil {
			return errors.Wrap(err, errors.ErrCodeExternalService, fmt.Sprintf("failed to check bucket %s", bucket))
		}
		if !exists {
			err = c.client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{Region: c.config.Region})
			if err != nil {
				return errors.Wrap(err, errors.ErrCodeExternalService, fmt.Sprintf("failed to create bucket %s", bucket))
			}
			c.logger.Info("Created bucket", logging.String("bucket", bucket))
		}
	}
	return nil
}

func (c *MinIOClient) SetupLifecycleRules(ctx context.Context) error {
	// Temp bucket: clean up after TempFileExpiry days
	tempBucket := c.config.Buckets.Temp
	expiryDays := lifecycle.ExpirationDays(c.config.TempFileExpiry)

	config := lifecycle.NewConfiguration()
	config.Rules = []lifecycle.Rule{
		{
			ID:     "temp-cleanup",
			Status: "Enabled",
			Expiration: lifecycle.Expiration{
				Days: expiryDays,
			},
			// Filter: lifecycle.Filter{ Prefix: "" }, // Not compatible with older minio-go version if Filter is missing or structure differs
			// If Filter is missing, use Rule.Prefix for backward compatibility if available, or upgrade minio-go
			// Assuming older SDK:
			Prefix: "",
		},
	}

	err := c.client.SetBucketLifecycle(ctx, tempBucket, config)
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeExternalService, "failed to set temp bucket lifecycle")
	}

	// Exports bucket: 30 days
	exportsBucket := c.config.Buckets.Exports
	configExports := lifecycle.NewConfiguration()
	configExports.Rules = []lifecycle.Rule{
		{
			ID:     "exports-cleanup",
			Status: "Enabled",
			Expiration: lifecycle.Expiration{
				Days: 30,
			},
			Prefix: "",
		},
	}

	err = c.client.SetBucketLifecycle(ctx, exportsBucket, configExports)
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeExternalService, "failed to set exports bucket lifecycle")
	}

	return nil
}

func (c *MinIOClient) GetClient() *minio.Client {
	return c.client
}

func (c *MinIOClient) GetBucketName(bucketType string) string {
	switch bucketType {
	case "documents":
		return c.config.Buckets.Documents
	case "models":
		return c.config.Buckets.Models
	case "reports":
		return c.config.Buckets.Reports
	case "exports":
		return c.config.Buckets.Exports
	case "temp":
		return c.config.Buckets.Temp
	case "attachments":
		return c.config.Buckets.Attachments
	default:
		return c.config.DefaultBucket
	}
}

func (c *MinIOClient) Ping(ctx context.Context) error {
	c.mu.RLock()
	if c.closed {
		c.mu.RUnlock()
		return ErrMinIOClientClosed
	}
	c.mu.RUnlock()

	_, err := c.client.ListBuckets(ctx)
	return err
}

func (c *MinIOClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	return nil
}

type HealthStatus struct {
	Healthy        bool
	Latency        time.Duration
	BucketStatuses map[string]bool
	Error          string
}

type BucketStats struct {
	ObjectCount  int64
	TotalSize    int64
	LastModified time.Time
}

func (c *MinIOClient) HealthCheck(ctx context.Context) (*HealthStatus, error) {
	start := time.Now()
	status := &HealthStatus{
		BucketStatuses: make(map[string]bool),
	}

	if err := c.Ping(ctx); err != nil {
		status.Healthy = false
		status.Error = err.Error()
		return status, err
	}

	buckets := []string{
		c.config.Buckets.Documents,
		c.config.Buckets.Models,
		c.config.Buckets.Reports,
		c.config.Buckets.Exports,
		c.config.Buckets.Temp,
		c.config.Buckets.Attachments,
	}

	allExist := true
	for _, b := range buckets {
		exists, err := c.client.BucketExists(ctx, b)
		if err != nil || !exists {
			status.BucketStatuses[b] = false
			allExist = false
		} else {
			status.BucketStatuses[b] = true
		}
	}

	status.Healthy = allExist
	status.Latency = time.Since(start)
	return status, nil
}

func (c *MinIOClient) GetBucketStats(ctx context.Context, bucketName string) (*BucketStats, error) {
	exists, err := c.client.BucketExists(ctx, bucketName)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrBucketNotFound
	}

	stats := &BucketStats{}
	objectCh := c.client.ListObjects(ctx, bucketName, minio.ListObjectsOptions{Recursive: true})

	for obj := range objectCh {
		if obj.Err != nil {
			return nil, obj.Err
		}
		stats.ObjectCount++
		stats.TotalSize += obj.Size
		if obj.LastModified.After(stats.LastModified) {
			stats.LastModified = obj.LastModified
		}
	}
	return stats, nil
}

func (c *MinIOClient) GeneratePresignedGetURL(ctx context.Context, bucketName, objectName string, expiry time.Duration) (string, error) {
	if expiry == 0 {
		expiry = c.config.PresignExpiry
	}
	u, err := c.client.PresignedGetObject(ctx, bucketName, objectName, expiry, nil)
	if err != nil {
		return "", err
	}
	return u.String(), nil
}

func (c *MinIOClient) GeneratePresignedPutURL(ctx context.Context, bucketName, objectName string, expiry time.Duration) (string, error) {
	if expiry == 0 {
		expiry = c.config.PresignExpiry
	}
	u, err := c.client.PresignedPutObject(ctx, bucketName, objectName, expiry)
	if err != nil {
		return "", err
	}
	return u.String(), nil
}
//Personal.AI order the ending
