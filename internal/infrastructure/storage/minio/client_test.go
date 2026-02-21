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
			Documents: "docs",
		},
	}
	client := &MinIOClient{config: cfg}

	assert.Equal(t, "docs", client.GetBucketName("documents"))
	assert.Equal(t, "default", client.GetBucketName("unknown"))
}

//Personal.AI order the ending
