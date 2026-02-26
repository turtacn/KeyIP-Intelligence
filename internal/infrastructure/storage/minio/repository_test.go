package minio

import (
	"context"
	"io"
	"net/url"
	"testing"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/tags"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
)

type MockMinIOAPI struct {
	mock.Mock
}

func (m *MockMinIOAPI) PutObject(ctx context.Context, bucketName, objectName string, reader io.Reader, objectSize int64, opts minio.PutObjectOptions) (minio.UploadInfo, error) {
	args := m.Called(ctx, bucketName, objectName, reader, objectSize, opts)
	return args.Get(0).(minio.UploadInfo), args.Error(1)
}

func (m *MockMinIOAPI) GetObject(ctx context.Context, bucketName, objectName string, opts minio.GetObjectOptions) (*minio.Object, error) {
	args := m.Called(ctx, bucketName, objectName, opts)
	// Returning nil object pointer might cause panic if not carefully mocked or used.
	// But GetObject returns *minio.Object which is struct.
	// We cannot construct a functional *minio.Object without real connection.
	// This limits unit testing of Download without integration test.
	// For now, we mock error case or skip deep download test in unit test.
	return nil, args.Error(1)
}

func (m *MockMinIOAPI) RemoveObject(ctx context.Context, bucketName, objectName string, opts minio.RemoveObjectOptions) error {
	args := m.Called(ctx, bucketName, objectName, opts)
	return args.Error(0)
}

func (m *MockMinIOAPI) RemoveObjects(ctx context.Context, bucketName string, objectsCh <-chan minio.ObjectInfo, opts minio.RemoveObjectsOptions) <-chan minio.RemoveObjectError {
	args := m.Called(ctx, bucketName, objectsCh, opts)
	return args.Get(0).(<-chan minio.RemoveObjectError)
}

func (m *MockMinIOAPI) StatObject(ctx context.Context, bucketName, objectName string, opts minio.StatObjectOptions) (minio.ObjectInfo, error) {
	args := m.Called(ctx, bucketName, objectName, opts)
	return args.Get(0).(minio.ObjectInfo), args.Error(1)
}

func (m *MockMinIOAPI) ListObjects(ctx context.Context, bucketName string, opts minio.ListObjectsOptions) <-chan minio.ObjectInfo {
	args := m.Called(ctx, bucketName, opts)
	return args.Get(0).(<-chan minio.ObjectInfo)
}

func (m *MockMinIOAPI) CopyObject(ctx context.Context, dst minio.CopyDestOptions, src minio.CopySrcOptions) (minio.UploadInfo, error) {
	args := m.Called(ctx, dst, src)
	return args.Get(0).(minio.UploadInfo), args.Error(1)
}

func (m *MockMinIOAPI) PresignedGetObject(ctx context.Context, bucketName, objectName string, expires time.Duration, reqParams url.Values) (*url.URL, error) {
	args := m.Called(ctx, bucketName, objectName, expires, reqParams)
	if args.Get(0) == nil { return nil, args.Error(1) }
	return args.Get(0).(*url.URL), args.Error(1)
}

func (m *MockMinIOAPI) PresignedPutObject(ctx context.Context, bucketName, objectName string, expires time.Duration) (*url.URL, error) {
	args := m.Called(ctx, bucketName, objectName, expires)
	if args.Get(0) == nil { return nil, args.Error(1) }
	return args.Get(0).(*url.URL), args.Error(1)
}

func (m *MockMinIOAPI) PutObjectTagging(ctx context.Context, bucketName, objectName string, otags *tags.Tags, opts minio.PutObjectTaggingOptions) error {
	args := m.Called(ctx, bucketName, objectName, otags, opts)
	return args.Error(0)
}

func (m *MockMinIOAPI) GetObjectTagging(ctx context.Context, bucketName, objectName string, opts minio.GetObjectTaggingOptions) (*tags.Tags, error) {
	args := m.Called(ctx, bucketName, objectName, opts)
	if args.Get(0) == nil { return nil, args.Error(1) }
	return args.Get(0).(*tags.Tags), args.Error(1)
}

type RepositoryTestSuite struct {
	suite.Suite
	mockAPI *MockMinIOAPI
	repo    ObjectRepository
	log     logging.Logger
}

func (s *RepositoryTestSuite) SetupTest() {
	s.mockAPI = new(MockMinIOAPI)
	s.log = logging.NewNopLogger()
	s.repo = NewMinIORepositoryWithAPI(s.mockAPI, s.log)
}

func (s *RepositoryTestSuite) TestUpload_Success() {
	s.mockAPI.On("PutObject", mock.Anything, "bucket", "key", mock.Anything, mock.Anything, mock.Anything).
		Return(minio.UploadInfo{Bucket: "bucket", Key: "key", ETag: "etag", Size: 100}, nil)

	req := &UploadRequest{
		Bucket:    "bucket",
		ObjectKey: "key",
		Data:      []byte("test data"),
	}
	res, err := s.repo.Upload(context.Background(), req)
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), "bucket", res.Bucket)
	assert.Equal(s.T(), "etag", res.ETag)
}

func (s *RepositoryTestSuite) TestDelete_Success() {
	s.mockAPI.On("RemoveObject", mock.Anything, "bucket", "key", mock.Anything).Return(nil)
	err := s.repo.Delete(context.Background(), "bucket", "key")
	assert.NoError(s.T(), err)
}

func (s *RepositoryTestSuite) TestExists_True() {
	s.mockAPI.On("StatObject", mock.Anything, "bucket", "key", mock.Anything).
		Return(minio.ObjectInfo{Key: "key"}, nil)
	exists, err := s.repo.Exists(context.Background(), "bucket", "key")
	assert.NoError(s.T(), err)
	assert.True(s.T(), exists)
}

func (s *RepositoryTestSuite) TestExists_False() {
	errResp := minio.ErrorResponse{Code: "NoSuchKey"}
	s.mockAPI.On("StatObject", mock.Anything, "bucket", "key", mock.Anything).
		Return(minio.ObjectInfo{}, errResp)
	exists, err := s.repo.Exists(context.Background(), "bucket", "key")
	assert.NoError(s.T(), err)
	assert.False(s.T(), exists)
}

func (s *RepositoryTestSuite) TestList_Success() {
	ch := make(chan minio.ObjectInfo, 1)
	ch <- minio.ObjectInfo{Key: "obj1", Size: 100}
	close(ch)

	s.mockAPI.On("ListObjects", mock.Anything, "bucket", mock.Anything).Return((<-chan minio.ObjectInfo)(ch))

	res, err := s.repo.List(context.Background(), "bucket", "", nil)
	assert.NoError(s.T(), err)
	assert.Len(s.T(), res.Objects, 1)
	assert.Equal(s.T(), "obj1", res.Objects[0].ObjectKey)
}

func (s *RepositoryTestSuite) TestDeleteBatch() {
	ch := make(chan minio.RemoveObjectError)
	close(ch)

	s.mockAPI.On("RemoveObjects", mock.Anything, "bucket", mock.Anything, mock.Anything).Return((<-chan minio.RemoveObjectError)(ch))

	errs, err := s.repo.DeleteBatch(context.Background(), "bucket", []string{"k1", "k2"})
	assert.NoError(s.T(), err)
	assert.Empty(s.T(), errs)
}

func TestRepositorySuite(t *testing.T) {
	suite.Run(t, new(RepositoryTestSuite))
}
//Personal.AI order the ending
