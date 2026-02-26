package minio

import (
	"context"
	"io"
	"net/url"
	"testing"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/lifecycle"
	"github.com/minio/minio-go/v7/pkg/tags"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
)

// MockMinIO
type MockMinIO struct {
	mock.Mock
}

func (m *MockMinIO) ListBuckets(ctx context.Context) ([]minio.BucketInfo, error) {
	args := m.Called(ctx)
	return args.Get(0).([]minio.BucketInfo), args.Error(1)
}
func (m *MockMinIO) BucketExists(ctx context.Context, bucketName string) (bool, error) {
	args := m.Called(ctx, bucketName)
	return args.Bool(0), args.Error(1)
}
func (m *MockMinIO) MakeBucket(ctx context.Context, bucketName string, opts minio.MakeBucketOptions) error {
	return m.Called(ctx, bucketName, opts).Error(0)
}
func (m *MockMinIO) SetBucketLifecycle(ctx context.Context, bucketName string, config *lifecycle.Configuration) error {
	return m.Called(ctx, bucketName, config).Error(0)
}
func (m *MockMinIO) ListObjects(ctx context.Context, bucketName string, opts minio.ListObjectsOptions) <-chan minio.ObjectInfo {
	args := m.Called(ctx, bucketName, opts)
	return args.Get(0).(<-chan minio.ObjectInfo)
}
func (m *MockMinIO) PresignedGetObject(ctx context.Context, bucketName, objectName string, expiry time.Duration, reqParams url.Values) (*url.URL, error) {
	args := m.Called(ctx, bucketName, objectName, expiry, reqParams)
	return args.Get(0).(*url.URL), args.Error(1)
}
func (m *MockMinIO) PresignedPutObject(ctx context.Context, bucketName, objectName string, expiry time.Duration) (*url.URL, error) {
	args := m.Called(ctx, bucketName, objectName, expiry)
	return args.Get(0).(*url.URL), args.Error(1)
}
// ... Implement other methods as needed or stub them
func (m *MockMinIO) PutObject(ctx context.Context, bucketName, objectName string, reader io.Reader, objectSize int64, opts minio.PutObjectOptions) (minio.UploadInfo, error) {
	args := m.Called(ctx, bucketName, objectName, reader, objectSize, opts)
	return args.Get(0).(minio.UploadInfo), args.Error(1)
}
func (m *MockMinIO) GetObject(ctx context.Context, bucketName, objectName string, opts minio.GetObjectOptions) (*minio.Object, error) {
	args := m.Called(ctx, bucketName, objectName, opts)
	return args.Get(0).(*minio.Object), args.Error(1)
}
func (m *MockMinIO) RemoveObject(ctx context.Context, bucketName, objectName string, opts minio.RemoveObjectOptions) error {
	return m.Called(ctx, bucketName, objectName, opts).Error(0)
}
func (m *MockMinIO) RemoveObjects(ctx context.Context, bucketName string, objectsCh <-chan minio.ObjectInfo, opts minio.RemoveObjectsOptions) <-chan minio.RemoveObjectError {
	args := m.Called(ctx, bucketName, objectsCh, opts)
	return args.Get(0).(<-chan minio.RemoveObjectError)
}
func (m *MockMinIO) StatObject(ctx context.Context, bucketName, objectName string, opts minio.StatObjectOptions) (minio.ObjectInfo, error) {
	args := m.Called(ctx, bucketName, objectName, opts)
	return args.Get(0).(minio.ObjectInfo), args.Error(1)
}
func (m *MockMinIO) CopyObject(ctx context.Context, dst minio.CopyDestOptions, src minio.CopySrcOptions) (minio.UploadInfo, error) {
	args := m.Called(ctx, dst, src)
	return args.Get(0).(minio.UploadInfo), args.Error(1)
}
func (m *MockMinIO) PutObjectTagging(ctx context.Context, bucketName, objectName string, ot *tags.Tags, opts minio.PutObjectTaggingOptions) error {
	return m.Called(ctx, bucketName, objectName, ot, opts).Error(0)
}
func (m *MockMinIO) GetObjectTagging(ctx context.Context, bucketName, objectName string, opts minio.GetObjectTaggingOptions) (*tags.Tags, error) {
	args := m.Called(ctx, bucketName, objectName, opts)
	return args.Get(0).(*tags.Tags), args.Error(1)
}

func TestEnsureBuckets_AllNew(t *testing.T) {
	mockMinio := new(MockMinIO)
	client := &MinIOClient{
		client: mockMinio,
		config: &MinIOConfig{
			Region: "us-east-1",
			Buckets: BucketConfig{
				Documents: "docs",
				Models:    "models",
				Reports:   "reports",
				Exports:   "exports",
				Temp:      "temp",
				Attachments: "attach",
			},
		},
		logger: logging.NewNopLogger(),
	}

	mockMinio.On("BucketExists", mock.Anything, mock.Anything).Return(false, nil)
	mockMinio.On("MakeBucket", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	err := client.EnsureBuckets(context.Background())
	assert.NoError(t, err)
	mockMinio.AssertNumberOfCalls(t, "MakeBucket", 6)
}

func TestGetBucketName(t *testing.T) {
	cfg := &MinIOConfig{
		DefaultBucket: "default",
		Buckets: BucketConfig{
			Documents:   "docs",
			Models:      "models",
			Reports:     "reports",
			Exports:     "exports",
			Temp:        "temp",
			Attachments: "attachments",
		},
	}
	client := &MinIOClient{config: cfg}

	tests := []struct {
		bucketType string
		expected   string
	}{
		{"documents", "docs"},
		{"models", "models"},
		{"reports", "reports"},
		{"exports", "exports"},
		{"temp", "temp"},
		{"attachments", "attachments"},
		{"unknown", "default"},
		{"", "default"},
	}

	for _, tt := range tests {
		t.Run(tt.bucketType, func(t *testing.T) {
			assert.Equal(t, tt.expected, client.GetBucketName(tt.bucketType))
		})
	}
}

func TestApplyDefaults(t *testing.T) {
	cfg := &MinIOConfig{}
	applyDefaults(cfg)

	assert.Equal(t, "us-east-1", cfg.Region)
	assert.Equal(t, int64(16*1024*1024), cfg.PartSize)
	assert.Equal(t, 3, cfg.MaxRetries)
	assert.Equal(t, 1*time.Hour, cfg.PresignExpiry)
	assert.Equal(t, 7, cfg.TempFileExpiry)
	assert.Equal(t, "keyip-documents", cfg.DefaultBucket)
	assert.Equal(t, "keyip-documents", cfg.Buckets.Documents)
	assert.Equal(t, "keyip-models", cfg.Buckets.Models)
	assert.Equal(t, "keyip-reports", cfg.Buckets.Reports)
	assert.Equal(t, "keyip-exports", cfg.Buckets.Exports)
	assert.Equal(t, "keyip-temp", cfg.Buckets.Temp)
	assert.Equal(t, "keyip-attachments", cfg.Buckets.Attachments)
}

func TestApplyDefaults_PreservesValues(t *testing.T) {
	cfg := &MinIOConfig{
		Region:       "eu-west-1",
		PartSize:     32 * 1024 * 1024,
		MaxRetries:   5,
		PresignExpiry: 2 * time.Hour,
		TempFileExpiry: 14,
		DefaultBucket: "custom-bucket",
		Buckets: BucketConfig{
			Documents:   "custom-docs",
			Models:      "custom-models",
			Reports:     "custom-reports",
			Exports:     "custom-exports",
			Temp:        "custom-temp",
			Attachments: "custom-attachments",
		},
	}
	applyDefaults(cfg)

	// Pre-set values should be preserved
	assert.Equal(t, "eu-west-1", cfg.Region)
	assert.Equal(t, int64(32*1024*1024), cfg.PartSize)
	assert.Equal(t, 5, cfg.MaxRetries)
	assert.Equal(t, 2*time.Hour, cfg.PresignExpiry)
	assert.Equal(t, 14, cfg.TempFileExpiry)
	assert.Equal(t, "custom-bucket", cfg.DefaultBucket)
	assert.Equal(t, "custom-docs", cfg.Buckets.Documents)
}

func TestMinIOClient_Close(t *testing.T) {
	client := &MinIOClient{}
	err := client.Close()
	assert.NoError(t, err)
	assert.True(t, client.closed)
}

func TestMinIOClient_GetClient(t *testing.T) {
	mockMinio := new(MockMinIO)
	client := &MinIOClient{client: mockMinio}

	result := client.GetClient()
	assert.Equal(t, mockMinio, result)
}

func TestMinIOClient_HealthCheck_Healthy(t *testing.T) {
	mockMinio := new(MockMinIO)
	cfg := &MinIOConfig{
		Buckets: BucketConfig{
			Documents:   "docs",
			Models:      "models",
			Reports:     "reports",
			Exports:     "exports",
			Temp:        "temp",
			Attachments: "attachments",
		},
	}
	client := &MinIOClient{
		client: mockMinio,
		config: cfg,
		logger: logging.NewNopLogger(),
	}

	mockMinio.On("ListBuckets", mock.Anything).Return([]minio.BucketInfo{}, nil)
	mockMinio.On("BucketExists", mock.Anything, mock.Anything).Return(true, nil)

	status, err := client.HealthCheck(context.Background())

	assert.NoError(t, err)
	assert.True(t, status.Healthy)
	assert.NotZero(t, status.Latency)
	assert.Empty(t, status.Error)
}

func TestMinIOClient_HealthCheck_MissingBucket(t *testing.T) {
	mockMinio := new(MockMinIO)
	cfg := &MinIOConfig{
		Buckets: BucketConfig{
			Documents:   "docs",
			Models:      "models",
			Reports:     "reports",
			Exports:     "exports",
			Temp:        "temp",
			Attachments: "attachments",
		},
	}
	client := &MinIOClient{
		client: mockMinio,
		config: cfg,
		logger: logging.NewNopLogger(),
	}

	mockMinio.On("ListBuckets", mock.Anything).Return([]minio.BucketInfo{}, nil)
	mockMinio.On("BucketExists", mock.Anything, "docs").Return(true, nil)
	mockMinio.On("BucketExists", mock.Anything, "models").Return(false, nil) // Missing
	mockMinio.On("BucketExists", mock.Anything, mock.Anything).Return(true, nil)

	status, err := client.HealthCheck(context.Background())

	assert.NoError(t, err)
	assert.False(t, status.Healthy)
	assert.Contains(t, status.Error, "bucket")
}

func TestMinIOClient_GetBucketStats_Success(t *testing.T) {
	mockMinio := new(MockMinIO)
	cfg := &MinIOConfig{}
	client := &MinIOClient{
		client: mockMinio,
		config: cfg,
		logger: logging.NewNopLogger(),
	}

	objCh := make(chan minio.ObjectInfo, 2)
	objCh <- minio.ObjectInfo{Size: 1000, LastModified: time.Now()}
	objCh <- minio.ObjectInfo{Size: 2000, LastModified: time.Now().Add(time.Hour)}
	close(objCh)

	mockMinio.On("BucketExists", mock.Anything, "test-bucket").Return(true, nil)
	mockMinio.On("ListObjects", mock.Anything, "test-bucket", mock.Anything).Return((<-chan minio.ObjectInfo)(objCh))

	stats, err := client.GetBucketStats(context.Background(), "test-bucket")

	assert.NoError(t, err)
	assert.NotNil(t, stats)
	assert.Equal(t, int64(2), stats.ObjectCount)
	assert.Equal(t, int64(3000), stats.TotalSize)
}

func TestMinIOClient_GetBucketStats_NotFound(t *testing.T) {
	mockMinio := new(MockMinIO)
	client := &MinIOClient{
		client: mockMinio,
		config: &MinIOConfig{},
		logger: logging.NewNopLogger(),
	}

	mockMinio.On("BucketExists", mock.Anything, "missing-bucket").Return(false, nil)

	stats, err := client.GetBucketStats(context.Background(), "missing-bucket")

	assert.Error(t, err)
	assert.Nil(t, stats)
	assert.Equal(t, ErrBucketNotFound, err)
}

func TestMinIOClient_GeneratePresignedGetURL(t *testing.T) {
	mockMinio := new(MockMinIO)
	cfg := &MinIOConfig{PresignExpiry: 1 * time.Hour}
	client := &MinIOClient{
		client: mockMinio,
		config: cfg,
		logger: logging.NewNopLogger(),
	}

	expectedURL, _ := url.Parse("https://minio.example.com/bucket/object?signed=true")
	mockMinio.On("PresignedGetObject", mock.Anything, "bucket", "object", 1*time.Hour, mock.Anything).Return(expectedURL, nil)

	resultURL, err := client.GeneratePresignedGetURL(context.Background(), "bucket", "object", 0)

	assert.NoError(t, err)
	assert.Equal(t, expectedURL.String(), resultURL)
}

func TestMinIOClient_GeneratePresignedGetURL_CustomExpiry(t *testing.T) {
	mockMinio := new(MockMinIO)
	cfg := &MinIOConfig{PresignExpiry: 1 * time.Hour}
	client := &MinIOClient{
		client: mockMinio,
		config: cfg,
		logger: logging.NewNopLogger(),
	}

	expectedURL, _ := url.Parse("https://minio.example.com/bucket/object?signed=true")
	customExpiry := 30 * time.Minute
	mockMinio.On("PresignedGetObject", mock.Anything, "bucket", "object", customExpiry, mock.Anything).Return(expectedURL, nil)

	resultURL, err := client.GeneratePresignedGetURL(context.Background(), "bucket", "object", customExpiry)

	assert.NoError(t, err)
	assert.Equal(t, expectedURL.String(), resultURL)
}

func TestMinIOClient_GeneratePresignedPutURL(t *testing.T) {
	mockMinio := new(MockMinIO)
	cfg := &MinIOConfig{PresignExpiry: 1 * time.Hour}
	client := &MinIOClient{
		client: mockMinio,
		config: cfg,
		logger: logging.NewNopLogger(),
	}

	expectedURL, _ := url.Parse("https://minio.example.com/bucket/object?upload=true")
	mockMinio.On("PresignedPutObject", mock.Anything, "bucket", "object", 1*time.Hour).Return(expectedURL, nil)

	resultURL, err := client.GeneratePresignedPutURL(context.Background(), "bucket", "object", 0)

	assert.NoError(t, err)
	assert.Equal(t, expectedURL.String(), resultURL)
}

func TestMinIOClient_SetupLifecycleRules(t *testing.T) {
	mockMinio := new(MockMinIO)
	cfg := &MinIOConfig{
		TempFileExpiry: 7,
		Buckets: BucketConfig{
			Temp:    "temp",
			Exports: "exports",
		},
	}
	client := &MinIOClient{
		client: mockMinio,
		config: cfg,
		logger: logging.NewNopLogger(),
	}

	mockMinio.On("SetBucketLifecycle", mock.Anything, "temp", mock.Anything).Return(nil)
	mockMinio.On("SetBucketLifecycle", mock.Anything, "exports", mock.Anything).Return(nil)

	err := client.SetupLifecycleRules(context.Background())

	assert.NoError(t, err)
	mockMinio.AssertExpectations(t)
}

func TestEnsureBuckets_AllExist(t *testing.T) {
	mockMinio := new(MockMinIO)
	client := &MinIOClient{
		client: mockMinio,
		config: &MinIOConfig{
			Region: "us-east-1",
			Buckets: BucketConfig{
				Documents:   "docs",
				Models:      "models",
				Reports:     "reports",
				Exports:     "exports",
				Temp:        "temp",
				Attachments: "attach",
			},
		},
		logger: logging.NewNopLogger(),
	}

	// All buckets exist
	mockMinio.On("BucketExists", mock.Anything, mock.Anything).Return(true, nil)

	err := client.EnsureBuckets(context.Background())
	assert.NoError(t, err)
	// MakeBucket should not be called since all exist
	mockMinio.AssertNumberOfCalls(t, "MakeBucket", 0)
}

func TestHealthStatus(t *testing.T) {
	status := &HealthStatus{
		Healthy:        true,
		Latency:        100 * time.Millisecond,
		BucketStatuses: map[string]bool{"bucket1": true, "bucket2": true},
	}

	assert.True(t, status.Healthy)
	assert.Equal(t, 100*time.Millisecond, status.Latency)
	assert.True(t, status.BucketStatuses["bucket1"])
}

func TestBucketStats(t *testing.T) {
	now := time.Now()
	stats := &BucketStats{
		ObjectCount:  10,
		TotalSize:    1024 * 1024,
		LastModified: now,
	}

	assert.Equal(t, int64(10), stats.ObjectCount)
	assert.Equal(t, int64(1024*1024), stats.TotalSize)
	assert.Equal(t, now, stats.LastModified)
}

func TestBucketConfig(t *testing.T) {
	cfg := BucketConfig{
		Documents:   "docs",
		Models:      "models",
		Reports:     "reports",
		Exports:     "exports",
		Temp:        "temp",
		Attachments: "attachments",
	}

	assert.Equal(t, "docs", cfg.Documents)
	assert.Equal(t, "models", cfg.Models)
	assert.Equal(t, "reports", cfg.Reports)
	assert.Equal(t, "exports", cfg.Exports)
	assert.Equal(t, "temp", cfg.Temp)
	assert.Equal(t, "attachments", cfg.Attachments)
}

func TestMinIOConfig(t *testing.T) {
	cfg := MinIOConfig{
		Endpoint:        "localhost:9000",
		AccessKeyID:     "access-key",
		SecretAccessKey: "secret-key",
		UseSSL:          true,
		Region:          "us-west-2",
		DefaultBucket:   "default",
	}

	assert.Equal(t, "localhost:9000", cfg.Endpoint)
	assert.Equal(t, "access-key", cfg.AccessKeyID)
	assert.Equal(t, "secret-key", cfg.SecretAccessKey)
	assert.True(t, cfg.UseSSL)
	assert.Equal(t, "us-west-2", cfg.Region)
	assert.Equal(t, "default", cfg.DefaultBucket)
}

func TestErrMinIOClientClosed(t *testing.T) {
	assert.Error(t, ErrMinIOClientClosed)
	assert.Contains(t, ErrMinIOClientClosed.Error(), "closed")
}

func TestErrBucketNotFound(t *testing.T) {
	assert.Error(t, ErrBucketNotFound)
	assert.Contains(t, ErrBucketNotFound.Error(), "bucket not found")
}

//Personal.AI order the ending
