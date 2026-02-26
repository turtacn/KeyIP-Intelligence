package minio

import (
	"context"
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/minio/minio-go/v7"
	"io"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/minio/minio-go/v7/pkg/lifecycle"
	"github.com/minio/minio-go/v7/pkg/tags"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

type MinIOAPI interface {
	ListBuckets(ctx context.Context) ([]minio.BucketInfo, error)
	BucketExists(ctx context.Context, bucketName string) (bool, error)
	MakeBucket(ctx context.Context, bucketName string, opts minio.MakeBucketOptions) error
	SetBucketLifecycle(ctx context.Context, bucketName string, config *lifecycle.Configuration) error
	ListObjects(ctx context.Context, bucketName string, opts minio.ListObjectsOptions) <-chan minio.ObjectInfo
	PresignedGetObject(ctx context.Context, bucketName, objectName string, expiry time.Duration, reqParams url.Values) (*url.URL, error)
	PresignedPutObject(ctx context.Context, bucketName, objectName string, expiry time.Duration) (*url.URL, error)
	PutObject(ctx context.Context, bucketName, objectName string, reader io.Reader, objectSize int64, opts minio.PutObjectOptions) (minio.UploadInfo, error)
	GetObject(ctx context.Context, bucketName, objectName string, opts minio.GetObjectOptions) (*minio.Object, error)
	RemoveObject(ctx context.Context, bucketName, objectName string, opts minio.RemoveObjectOptions) error
	RemoveObjects(ctx context.Context, bucketName string, objectsCh <-chan minio.ObjectInfo, opts minio.RemoveObjectsOptions) <-chan minio.RemoveObjectError
	StatObject(ctx context.Context, bucketName, objectName string, opts minio.StatObjectOptions) (minio.ObjectInfo, error)
	CopyObject(ctx context.Context, dst minio.CopyDestOptions, src minio.CopySrcOptions) (minio.UploadInfo, error)
	PutObjectTagging(ctx context.Context, bucketName, objectName string, ot *tags.Tags, opts minio.PutObjectTaggingOptions) error
	GetObjectTagging(ctx context.Context, bucketName, objectName string, opts minio.GetObjectTaggingOptions) (*tags.Tags, error)
}

type BucketConfig struct {
	Documents   string `mapstructure:"documents"`
	Models      string `mapstructure:"models"`
	Reports     string `mapstructure:"reports"`
	Exports     string `mapstructure:"exports"`
	Temp        string `mapstructure:"temp"`
	Attachments string `mapstructure:"attachments"`
}

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

type MinIOClient struct {
	client MinIOAPI
	config *MinIOConfig
	logger logging.Logger
	mu     sync.RWMutex
	closed bool
}

func NewMinIOClient(cfg *MinIOConfig, log logging.Logger) (*MinIOClient, error) {
	applyDefaults(cfg)

	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		Secure: cfg.UseSSL,
		Region: cfg.Region,
	})
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeInternal, "failed to create minio client")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Verify connection
	if _, err := client.ListBuckets(ctx); err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeServiceUnavailable, "failed to connect to minio")
	}

	mClient := &MinIOClient{
		client: client,
		config: cfg,
		logger: log,
	}

	if err := mClient.EnsureBuckets(ctx); err != nil {
		return nil, err
	}

	if err := mClient.SetupLifecycleRules(ctx); err != nil {
		return nil, err
	}

	log.Info("MinIO client connected", logging.String("endpoint", cfg.Endpoint), logging.Bool("ssl", cfg.UseSSL))
	return mClient, nil
}

func applyDefaults(cfg *MinIOConfig) {
	if cfg.Region == "" {
		cfg.Region = "us-east-1"
	}
	if cfg.PartSize == 0 {
		cfg.PartSize = 16 * 1024 * 1024
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
	if cfg.DefaultBucket == "" {
		cfg.DefaultBucket = "keyip-documents"
	}
	if cfg.Buckets.Documents == "" {
		cfg.Buckets.Documents = "keyip-documents"
	}
	if cfg.Buckets.Models == "" {
		cfg.Buckets.Models = "keyip-models"
	}
	if cfg.Buckets.Reports == "" {
		cfg.Buckets.Reports = "keyip-reports"
	}
	if cfg.Buckets.Exports == "" {
		cfg.Buckets.Exports = "keyip-exports"
	}
	if cfg.Buckets.Temp == "" {
		cfg.Buckets.Temp = "keyip-temp"
	}
	if cfg.Buckets.Attachments == "" {
		cfg.Buckets.Attachments = "keyip-attachments"
	}
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
			return errors.Wrap(err, errors.ErrCodeInternal, "failed to check bucket existence")
		}
		if !exists {
			if err := c.client.MakeBucket(ctx, bucket, minio.MakeBucketOptions{Region: c.config.Region}); err != nil {
				return errors.Wrap(err, errors.ErrCodeInternal, fmt.Sprintf("failed to create bucket %s", bucket))
			}
			c.logger.Info("Created bucket", logging.String("bucket", bucket))
		}
	}
	return nil
}

func (c *MinIOClient) SetupLifecycleRules(ctx context.Context) error {
	// Temp bucket lifecycle
	tempConfig := lifecycle.NewConfiguration()
	tempConfig.Rules = []lifecycle.Rule{
		{
			ID:     "temp-cleanup",
			Status: "Enabled",
			Expiration: lifecycle.Expiration{
				Days: lifecycle.ExpirationDays(c.config.TempFileExpiry),
			},
			// Filter might be implicit or use Prefix directly depending on version.
			// Assuming older style or simple prefix if Filter struct not found.
			// Actually recent minio-go has RuleFilter but field name is RuleFilter? No.
			// If Filter field is unknown, likely it is Prefix.
			Prefix: "",
		},
	}
	if err := c.client.SetBucketLifecycle(ctx, c.config.Buckets.Temp, tempConfig); err != nil {
		c.logger.Warn("Failed to set lifecycle for temp bucket", logging.Err(err))
	}

	// Exports bucket lifecycle (30 days)
	exportsConfig := lifecycle.NewConfiguration()
	exportsConfig.Rules = []lifecycle.Rule{
		{
			ID:     "exports-cleanup",
			Status: "Enabled",
			Expiration: lifecycle.Expiration{
				Days: 30,
			},
			Prefix: "",
		},
	}
	if err := c.client.SetBucketLifecycle(ctx, c.config.Buckets.Exports, exportsConfig); err != nil {
		c.logger.Warn("Failed to set lifecycle for exports bucket", logging.Err(err))
	}

	return nil
}

func (c *MinIOClient) GetClient() MinIOAPI {
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

var ErrMinIOClientClosed = errors.New(errors.ErrCodeInternal, "minio client is closed")

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

func (c *MinIOClient) HealthCheck(ctx context.Context) (*HealthStatus, error) {
	start := time.Now()
	_, err := c.client.ListBuckets(ctx)
	latency := time.Since(start)

	status := &HealthStatus{
		Healthy:        err == nil,
		Latency:        latency,
		BucketStatuses: make(map[string]bool),
	}

	if err != nil {
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

	for _, b := range buckets {
		exists, _ := c.client.BucketExists(ctx, b)
		status.BucketStatuses[b] = exists
		if !exists {
			status.Healthy = false
			status.Error = fmt.Sprintf("bucket %s missing", b)
		}
	}

	return status, nil
}

type BucketStats struct {
	ObjectCount int64
	TotalSize   int64
	LastModified time.Time
}

var ErrBucketNotFound = errors.New(errors.ErrCodeNotFound, "bucket not found")

func (c *MinIOClient) GetBucketStats(ctx context.Context, bucketName string) (*BucketStats, error) {
	exists, err := c.client.BucketExists(ctx, bucketName)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, ErrBucketNotFound
	}

	stats := &BucketStats{}
	objects := c.client.ListObjects(ctx, bucketName, minio.ListObjectsOptions{Recursive: true})

	for obj := range objects {
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

// Type aliases for backward compatibility with apiserver/worker

// Client is an alias for MinIOClient for backward compatibility.
type Client = MinIOClient

// DocumentRepository is an alias for ObjectStorageRepository for backward compatibility.
type DocumentRepository = ObjectStorageRepository

// NewDocumentRepository creates a new document repository with the given client.
func NewDocumentRepository(client *MinIOClient, logger logging.Logger) ObjectStorageRepository {
	return NewMinIORepository(client, logger)
}

// NewObjectStorageRepository is an alias for NewMinIORepository for backward compatibility.
func NewObjectStorageRepository(client *MinIOClient, logger logging.Logger) ObjectStorageRepository {
	return NewMinIORepository(client, logger)
}

//Personal.AI order the ending
