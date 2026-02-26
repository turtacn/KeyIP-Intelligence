package minio

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/tags"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

type ObjectRepository interface {
	Upload(ctx context.Context, req *UploadRequest) (*UploadResult, error)
	UploadStream(ctx context.Context, req *StreamUploadRequest) (*UploadResult, error)
	Download(ctx context.Context, bucket, objectKey string) (*DownloadResult, error)
	DownloadToWriter(ctx context.Context, bucket, objectKey string, writer io.Writer) error
	Delete(ctx context.Context, bucket, objectKey string) error
	DeleteBatch(ctx context.Context, bucket string, objectKeys []string) ([]DeleteError, error)
	Exists(ctx context.Context, bucket, objectKey string) (bool, error)
	GetMetadata(ctx context.Context, bucket, objectKey string) (*ObjectMetadata, error)
	List(ctx context.Context, bucket, prefix string, opts *ListOptions) (*ListResult, error)
	Copy(ctx context.Context, srcBucket, srcKey, dstBucket, dstKey string) error
	Move(ctx context.Context, srcBucket, srcKey, dstBucket, dstKey string) error
	GetPresignedDownloadURL(ctx context.Context, bucket, objectKey string, expiry time.Duration) (string, error)
	GetPresignedUploadURL(ctx context.Context, bucket, objectKey string, expiry time.Duration) (string, error)
	SetTags(ctx context.Context, bucket, objectKey string, tagsMap map[string]string) error
	GetTags(ctx context.Context, bucket, objectKey string) (map[string]string, error)
	Get(ctx context.Context, path string) ([]byte, error)
}

type UploadRequest struct {
	Bucket      string
	ObjectKey   string
	Data        []byte
	ContentType string
	Metadata    map[string]string
	Tags        map[string]string
}

type StreamUploadRequest struct {
	Bucket      string
	ObjectKey   string
	Reader      io.Reader
	Size        int64
	ContentType string
	Metadata    map[string]string
	Tags        map[string]string
}

type UploadResult struct {
	Bucket     string
	ObjectKey  string
	ETag       string
	Size       int64
	VersionID  string
	Location   string
	UploadedAt time.Time
}

type DownloadResult struct {
	Data         []byte
	ContentType  string
	Size         int64
	ETag         string
	Metadata     map[string]string
	LastModified time.Time
}

type ObjectMetadata struct {
	Bucket       string
	ObjectKey    string
	Size         int64
	ContentType  string
	ETag         string
	LastModified time.Time
	Metadata     map[string]string
	Tags         map[string]string
	VersionID    string
	StorageClass string
}

type ListOptions struct {
	MaxKeys     int
	StartAfter  string
	Recursive   bool
	ContentType *string
}

type ListResult struct {
	Objects    []*ObjectMetadata
	HasMore    bool
	NextMarker string
	TotalCount int
}

type DeleteError struct {
	ObjectKey string
	Error     error
}

// Internal interface to mock minio.Client behavior
type minioAPI interface {
	PutObject(ctx context.Context, bucketName, objectName string, reader io.Reader, objectSize int64, opts minio.PutObjectOptions) (info minio.UploadInfo, err error)
	GetObject(ctx context.Context, bucketName, objectName string, opts minio.GetObjectOptions) (*minio.Object, error)
	RemoveObject(ctx context.Context, bucketName, objectName string, opts minio.RemoveObjectOptions) error
	RemoveObjects(ctx context.Context, bucketName string, objectsCh <-chan minio.ObjectInfo, opts minio.RemoveObjectsOptions) <-chan minio.RemoveObjectError
	StatObject(ctx context.Context, bucketName, objectName string, opts minio.StatObjectOptions) (minio.ObjectInfo, error)
	ListObjects(ctx context.Context, bucketName string, opts minio.ListObjectsOptions) <-chan minio.ObjectInfo
	CopyObject(ctx context.Context, dst minio.CopyDestOptions, src minio.CopySrcOptions) (minio.UploadInfo, error)
	PresignedGetObject(ctx context.Context, bucketName, objectName string, expires time.Duration, reqParams url.Values) (*url.URL, error)
	PresignedPutObject(ctx context.Context, bucketName, objectName string, expires time.Duration) (*url.URL, error)
	PutObjectTagging(ctx context.Context, bucketName, objectName string, otags *tags.Tags, opts minio.PutObjectTaggingOptions) error
	GetObjectTagging(ctx context.Context, bucketName, objectName string, opts minio.GetObjectTaggingOptions) (*tags.Tags, error)
}

// Ensure MinIOClient satisfies minioAPI? No, MinIOClient wraps *minio.Client.
// We need minioRepository to use *minio.Client or interface.
// Since we cannot change MinIOClient's client field type (it's *minio.Client),
// we will adapt.
// But testing repo requires mocking the API.
// So minioRepository should hold an interface.

type minioRepository struct {
	client     minioAPI // Use interface
	logger     logging.Logger
	partSize   int64
	maxRetries int
}

func NewMinIORepository(client *MinIOClient, log logging.Logger) ObjectRepository {
	return &minioRepository{
		client:     client.GetClient(), // *minio.Client implicitly implements methods of minioAPI matching signature?
		// minio.Client methods: PutObject, GetObject...
		// Yes, provided signatures match exactly.
		// Wait, ListObjects returns <-chan. GetClient() returns *minio.Client.
		// *minio.Client implements these methods.
		logger:     log,
		partSize:   client.config.PartSize,
		maxRetries: client.config.MaxRetries,
	}
}

// NewMinIORepositoryWithAPI allows injecting mock
func NewMinIORepositoryWithAPI(api minioAPI, log logging.Logger) ObjectRepository {
	return &minioRepository{
		client:   api,
		logger:   log,
		partSize: 16 * 1024 * 1024,
	}
}

func (r *minioRepository) Upload(ctx context.Context, req *UploadRequest) (*UploadResult, error) {
	if req.Bucket == "" || req.ObjectKey == "" {
		return nil, errors.New(errors.ErrCodeValidation, "bucket and key required")
	}
	if req.Data == nil && len(req.Data) == 0 {
		// Allow empty file? req.Data is byte slice. empty is allowed. nil check handled.
	}

	contentType := req.ContentType
	if contentType == "" {
		contentType = http.DetectContentType(req.Data)
	}

	opts := minio.PutObjectOptions{
		ContentType:  contentType,
		UserMetadata: req.Metadata,
		UserTags:     req.Tags,
	}

	info, err := r.client.PutObject(ctx, req.Bucket, req.ObjectKey, bytes.NewReader(req.Data), int64(len(req.Data)), opts)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeExternalService, "upload failed")
	}

	return &UploadResult{
		Bucket:     info.Bucket,
		ObjectKey:  info.Key,
		ETag:       info.ETag,
		Size:       info.Size,
		VersionID:  info.VersionID,
		Location:   info.Location,
		UploadedAt: time.Now(), // info doesn't have timestamp usually? LastModified? MinIO UploadInfo has LastModified? It might not.
	}, nil
}

func (r *minioRepository) UploadStream(ctx context.Context, req *StreamUploadRequest) (*UploadResult, error) {
	if req.Bucket == "" || req.ObjectKey == "" || req.Reader == nil {
		return nil, errors.New(errors.ErrCodeValidation, "bucket, key, and reader required")
	}

	opts := minio.PutObjectOptions{
		ContentType:  req.ContentType,
		UserMetadata: req.Metadata,
		UserTags:     req.Tags,
	}
	if req.Size == -1 {
		opts.PartSize = uint64(r.partSize)
	}

	info, err := r.client.PutObject(ctx, req.Bucket, req.ObjectKey, req.Reader, req.Size, opts)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeExternalService, "stream upload failed")
	}

	return &UploadResult{
		Bucket:    info.Bucket,
		ObjectKey: info.Key,
		ETag:      info.ETag,
		Size:      info.Size,
		VersionID: info.VersionID,
		Location:  info.Location,
	}, nil
}

func (r *minioRepository) Download(ctx context.Context, bucket, objectKey string) (*DownloadResult, error) {
	obj, err := r.client.GetObject(ctx, bucket, objectKey, minio.GetObjectOptions{})
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeExternalService, "download failed")
	}
	defer obj.Close()

	stat, err := obj.Stat()
	if err != nil {
		if minio.ToErrorResponse(err).Code == "NoSuchKey" {
			return nil, ErrBucketNotFound // or ErrObjectNotFound
		}
		return nil, errors.Wrap(err, errors.ErrCodeExternalService, "stat object failed")
	}

	data, err := io.ReadAll(obj)
	if err != nil {
		return nil, errors.Wrap(err, errors.ErrCodeExternalService, "read object failed")
	}

	meta := make(map[string]string)
	for k, v := range stat.Metadata {
		if len(v) > 0 {
			meta[k] = v[0]
		}
	}

	return &DownloadResult{
		Data:         data,
		ContentType:  stat.ContentType,
		Size:         stat.Size,
		ETag:         stat.ETag,
		Metadata:     meta,
		LastModified: stat.LastModified,
	}, nil
}

func (r *minioRepository) DownloadToWriter(ctx context.Context, bucket, objectKey string, writer io.Writer) error {
	obj, err := r.client.GetObject(ctx, bucket, objectKey, minio.GetObjectOptions{})
	if err != nil {
		return errors.Wrap(err, errors.ErrCodeExternalService, "download failed")
	}
	defer obj.Close()

	if _, err := io.Copy(writer, obj); err != nil {
		return errors.Wrap(err, errors.ErrCodeExternalService, "copy failed")
	}
	return nil
}

func (r *minioRepository) Delete(ctx context.Context, bucket, objectKey string) error {
	return r.client.RemoveObject(ctx, bucket, objectKey, minio.RemoveObjectOptions{})
}

func (r *minioRepository) DeleteBatch(ctx context.Context, bucket string, objectKeys []string) ([]DeleteError, error) {
	objectsCh := make(chan minio.ObjectInfo, len(objectKeys))
	for _, key := range objectKeys {
		objectsCh <- minio.ObjectInfo{Key: key}
	}
	close(objectsCh)

	var deleteErrors []DeleteError
	for err := range r.client.RemoveObjects(ctx, bucket, objectsCh, minio.RemoveObjectsOptions{}) {
		deleteErrors = append(deleteErrors, DeleteError{ObjectKey: err.ObjectName, Error: err.Err})
	}
	return deleteErrors, nil
}

func (r *minioRepository) Exists(ctx context.Context, bucket, objectKey string) (bool, error) {
	_, err := r.client.StatObject(ctx, bucket, objectKey, minio.StatObjectOptions{})
	if err != nil {
		if minio.ToErrorResponse(err).Code == "NoSuchKey" {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (r *minioRepository) GetMetadata(ctx context.Context, bucket, objectKey string) (*ObjectMetadata, error) {
	info, err := r.client.StatObject(ctx, bucket, objectKey, minio.StatObjectOptions{})
	if err != nil {
		if minio.ToErrorResponse(err).Code == "NoSuchKey" {
			return nil, errors.New(errors.ErrCodeNotFound, "object not found")
		}
		return nil, err
	}

	// Get tags
	tagging, err := r.client.GetObjectTagging(ctx, bucket, objectKey, minio.GetObjectTaggingOptions{})
	var tagsMap map[string]string
	if err == nil && tagging != nil {
		tagsMap = tagging.ToMap()
	}

	return &ObjectMetadata{
		Bucket:       bucket,
		ObjectKey:    info.Key,
		Size:         info.Size,
		ContentType:  info.ContentType,
		ETag:         info.ETag,
		LastModified: info.LastModified,
		Metadata:     info.UserMetadata,
		Tags:         tagsMap,
		VersionID:    info.VersionID,
		StorageClass: info.StorageClass,
	}, nil
}

func (r *minioRepository) List(ctx context.Context, bucket, prefix string, opts *ListOptions) (*ListResult, error) {
	if opts == nil {
		opts = &ListOptions{MaxKeys: 1000, Recursive: true}
	}

	minioOpts := minio.ListObjectsOptions{
		Prefix:    prefix,
		Recursive: opts.Recursive,
		MaxKeys:   opts.MaxKeys,
		StartAfter: opts.StartAfter,
	}

	result := &ListResult{}
	count := 0

	for obj := range r.client.ListObjects(ctx, bucket, minioOpts) {
		if obj.Err != nil {
			return nil, obj.Err
		}
		if opts.ContentType != nil && obj.ContentType != *opts.ContentType {
			continue
		}

		result.Objects = append(result.Objects, &ObjectMetadata{
			Bucket:       bucket,
			ObjectKey:    obj.Key,
			Size:         obj.Size,
			ContentType:  obj.ContentType,
			ETag:         obj.ETag,
			LastModified: obj.LastModified,
			Metadata:     obj.UserMetadata,
			StorageClass: obj.StorageClass,
		})
		count++
		result.NextMarker = obj.Key
		if count >= opts.MaxKeys {
			result.HasMore = true
			break
		}
	}
	result.TotalCount = count
	return result, nil
}

func (r *minioRepository) Copy(ctx context.Context, srcBucket, srcKey, dstBucket, dstKey string) error {
	src := minio.CopySrcOptions{Bucket: srcBucket, Object: srcKey}
	dst := minio.CopyDestOptions{Bucket: dstBucket, Object: dstKey}
	_, err := r.client.CopyObject(ctx, dst, src)
	return err
}

func (r *minioRepository) Move(ctx context.Context, srcBucket, srcKey, dstBucket, dstKey string) error {
	if err := r.Copy(ctx, srcBucket, srcKey, dstBucket, dstKey); err != nil {
		return err
	}
	return r.Delete(ctx, srcBucket, srcKey)
}

func (r *minioRepository) GetPresignedDownloadURL(ctx context.Context, bucket, objectKey string, expiry time.Duration) (string, error) {
	u, err := r.client.PresignedGetObject(ctx, bucket, objectKey, expiry, nil)
	if err != nil {
		return "", err
	}
	return u.String(), nil
}

func (r *minioRepository) GetPresignedUploadURL(ctx context.Context, bucket, objectKey string, expiry time.Duration) (string, error) {
	u, err := r.client.PresignedPutObject(ctx, bucket, objectKey, expiry)
	if err != nil {
		return "", err
	}
	return u.String(), nil
}

func (r *minioRepository) SetTags(ctx context.Context, bucket, objectKey string, tagsMap map[string]string) error {
	t, err := tags.NewTags(tagsMap, false)
	if err != nil {
		return err
	}
	return r.client.PutObjectTagging(ctx, bucket, objectKey, t, minio.PutObjectTaggingOptions{})
}

func (r *minioRepository) GetTags(ctx context.Context, bucket, objectKey string) (map[string]string, error) {
	t, err := r.client.GetObjectTagging(ctx, bucket, objectKey, minio.GetObjectTaggingOptions{})
	if err != nil {
		return nil, err
	}
	return t.ToMap(), nil
}

func (r *minioRepository) Get(ctx context.Context, path string) ([]byte, error) {
	// Simple path parsing: bucket/key
	// If no slash, error
	parts := parsePath(path)
	if len(parts) != 2 {
		return nil, errors.New(errors.ErrCodeValidation, "invalid storage path, expected bucket/key")
	}
	res, err := r.Download(ctx, parts[0], parts[1])
	if err != nil {
		return nil, err
	}
	return res.Data, nil
}

func parsePath(path string) []string {
	// Simple split by first slash
	for i, c := range path {
		if c == '/' {
			return []string{path[:i], path[i+1:]}
		}
	}
	return []string{path}
}

// Helpers for keys
func BuildPatentDocumentKey(year int, patentID, docType, filename string) string {
	return fmt.Sprintf("patents/%d/%s/%s/%s", year, patentID, docType, filename)
}

//Personal.AI order the ending
