package minio

import (
	"context"
	"testing"

	"github.com/minio/minio-go/v7"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
)

type RepositoryTestSuite struct {
	// ...
}

func TestUpload_Success(t *testing.T) {
	mockMinio := new(MockMinIO)
	client := &MinIOClient{client: mockMinio, config: &MinIOConfig{}, logger: logging.NewNopLogger()}
	repo := NewMinIORepository(client, logging.NewNopLogger())

	req := &UploadRequest{
		Bucket:    "bucket",
		ObjectKey: "key",
		Data:      []byte("test data"),
	}

	mockMinio.On("PutObject", mock.Anything, "bucket", "key", mock.Anything, int64(9), mock.Anything).
		Return(minio.UploadInfo{Bucket: "bucket", Key: "key", ETag: "etag"}, nil)

	res, err := repo.Upload(context.Background(), req)
	assert.NoError(t, err)
	assert.Equal(t, "etag", res.ETag)
}

func TestDownload_NotFound(t *testing.T) {
	mockMinio := new(MockMinIO)
	client := &MinIOClient{client: mockMinio, config: &MinIOConfig{}, logger: logging.NewNopLogger()}
	repo := NewMinIORepository(client, logging.NewNopLogger())

	mockMinio.On("GetObject", mock.Anything, "bucket", "key", mock.Anything).
		Return(&minio.Object{}, minio.ErrorResponse{Code: "NoSuchKey"})

	_, err := repo.Download(context.Background(), "bucket", "key")
	// GetObject returns error immediately in my mock, but real implementation returns Object.
	// But Stat() call on Object would fail.
	// My mock implementation for GetObject returns error.
	assert.Error(t, err)
}

// Ensure mock implementations in client_test.go are available here (same package).

//Personal.AI order the ending
