//go:build integration

package minio

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
)

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

func skipIfNoEndpoint(t *testing.T) string {
	t.Helper()
	endpoint := os.Getenv("MINIO_ENDPOINT")
	if endpoint == "" {
		t.Skip("Skipping integration test: MINIO_ENDPOINT environment variable not set")
	}
	return endpoint
}

func getTestConfig() *MinIOConfig {
	endpoint := os.Getenv("MINIO_ENDPOINT")
	accessKey := os.Getenv("MINIO_ACCESS_KEY")
	secretKey := os.Getenv("MINIO_SECRET_KEY")

	if accessKey == "" {
		accessKey = "minioadmin"
	}
	if secretKey == "" {
		secretKey = "minioadmin"
	}

	return &MinIOConfig{
		Endpoint:        endpoint,
		AccessKeyID:     accessKey,
		SecretAccessKey: secretKey,
		UseSSL:          false,
		Region:          "us-east-1",
		PartSize:        5 * 1024 * 1024, // 5 MB minimum part size for multipart
		MaxRetries:      3,
		PresignExpiry:   1 * time.Hour,
		DefaultBucket:   "keyip-documents",
		TempFileExpiry:  7,
	}
}

// integrationFixture holds shared resources for all integration tests.
// Each test gets a dedicated bucket that is automatically cleaned up.
type integrationFixture struct {
	rawClient   *minio.Client
	minioClient *MinIOClient
	repo        ObjectStorageRepository
	bucketName  string
	cfg         *MinIOConfig
	ctx         context.Context
}

func newIntegrationFixture(t *testing.T) *integrationFixture {
	t.Helper()
	skipIfNoEndpoint(t)

	cfg := getTestConfig()
	rawClient, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
		Secure: cfg.UseSSL,
		Region: cfg.Region,
	})
	require.NoError(t, err, "failed to create raw minio client")

	logger := logging.NewNopLogger()

	mc := &MinIOClient{
		client: rawClient,
		config: cfg,
		logger: logger,
	}

	repo := NewMinIORepository(mc, logger)

	// Create a unique test bucket with nanosecond timestamp to avoid collisions
	bucketName := fmt.Sprintf("test-int-%d", time.Now().UnixNano())

	err = rawClient.MakeBucket(context.Background(), bucketName, minio.MakeBucketOptions{Region: cfg.Region})
	require.NoError(t, err, "failed to create test bucket")

	// Cleanup: remove all objects then the bucket
	t.Cleanup(func() {
		ctx := context.Background()
		objCh := rawClient.ListObjects(ctx, bucketName, minio.ListObjectsOptions{Recursive: true})
		for obj := range objCh {
			_ = rawClient.RemoveObject(ctx, bucketName, obj.Key, minio.RemoveObjectOptions{})
		}
		_ = rawClient.RemoveBucket(ctx, bucketName)
	})

	return &integrationFixture{
		rawClient:   rawClient,
		minioClient: mc,
		repo:        repo,
		bucketName:  bucketName,
		cfg:         cfg,
		ctx:         context.Background(),
	}
}

// ---------------------------------------------------------------------------
// 1. Bucket CRUD: create, exists, list, delete
// ---------------------------------------------------------------------------

func TestIntegration_BucketCRUD(t *testing.T) {
	f := newIntegrationFixture(t)
	ctx := f.ctx

	testBucket := fmt.Sprintf("test-crud-%d", time.Now().UnixNano())

	// ---- Create ----
	err := f.rawClient.MakeBucket(ctx, testBucket, minio.MakeBucketOptions{Region: f.cfg.Region})
	require.NoError(t, err, "MakeBucket should succeed")

	// ---- Exists ----
	exists, err := f.rawClient.BucketExists(ctx, testBucket)
	assert.NoError(t, err)
	assert.True(t, exists, "bucket should exist after creation")

	// ---- List ----
	buckets, err := f.rawClient.ListBuckets(ctx)
	assert.NoError(t, err)
	found := false
	for _, b := range buckets {
		if b.Name == testBucket {
			found = true
			break
		}
	}
	assert.True(t, found, "new bucket should appear in ListBuckets results")

	// ---- Delete ----
	err = f.rawClient.RemoveBucket(ctx, testBucket)
	assert.NoError(t, err, "RemoveBucket should succeed")

	// ---- Verify deletion ----
	exists, err = f.rawClient.BucketExists(ctx, testBucket)
	assert.NoError(t, err)
	assert.False(t, exists, "bucket should not exist after deletion")
}

// ---------------------------------------------------------------------------
// 2. Object CRUD: put, get, delete, stat (via MinIOAPI)
// ---------------------------------------------------------------------------

func TestIntegration_ObjectCRUD(t *testing.T) {
	f := newIntegrationFixture(t)
	api := f.minioClient.GetClient()

	objectKey := "test-object.txt"
	content := []byte("Hello, MinIO integration test!")
	ctx := f.ctx

	// ---- Put ----
	info, err := api.PutObject(ctx, f.bucketName, objectKey, bytes.NewReader(content),
		int64(len(content)), minio.PutObjectOptions{ContentType: "text/plain"})
	require.NoError(t, err, "PutObject should succeed")
	assert.Equal(t, f.bucketName, info.Bucket)
	assert.Equal(t, objectKey, info.Key)
	assert.NotEmpty(t, info.ETag)

	// ---- Stat ----
	oinfo, err := api.StatObject(ctx, f.bucketName, objectKey, minio.StatObjectOptions{})
	require.NoError(t, err)
	assert.Equal(t, int64(len(content)), oinfo.Size)
	assert.Equal(t, objectKey, oinfo.Key)

	// ---- Get ----
	obj, err := api.GetObject(ctx, f.bucketName, objectKey, minio.GetObjectOptions{})
	require.NoError(t, err)
	defer obj.Close()

	downloaded, err := io.ReadAll(obj)
	require.NoError(t, err)
	assert.Equal(t, content, downloaded)

	// ---- Delete ----
	err = api.RemoveObject(ctx, f.bucketName, objectKey, minio.RemoveObjectOptions{})
	require.NoError(t, err, "RemoveObject should succeed")

	// Verify deletion via StatObject
	_, err = api.StatObject(ctx, f.bucketName, objectKey, minio.StatObjectOptions{})
	require.Error(t, err, "StatObject should fail after deletion")
	errResp := minio.ToErrorResponse(err)
	assert.Equal(t, "NoSuchKey", errResp.Code)
}

// ---------------------------------------------------------------------------
// 3. Object CRUD via Repository layer
// ---------------------------------------------------------------------------

func TestIntegration_RepositoryObjectCRUD(t *testing.T) {
	f := newIntegrationFixture(t)
	ctx := f.ctx

	objectKey := "repo-test.txt"
	objectData := []byte("repository layer integration test")
	metadata := map[string]string{"env": "integration", "source": "gotest"}

	// ---- Upload ----
	result, err := f.repo.Upload(ctx, &UploadRequest{
		Bucket:      f.bucketName,
		ObjectKey:   objectKey,
		Data:        objectData,
		ContentType: "text/plain",
		Metadata:    metadata,
	})
	require.NoError(t, err)
	assert.Equal(t, objectKey, result.ObjectKey)
	assert.Equal(t, f.bucketName, result.Bucket)

	// ---- Exists ----
	exists, err := f.repo.Exists(ctx, f.bucketName, objectKey)
	require.NoError(t, err)
	assert.True(t, exists)

	// ---- GetMetadata ----
	meta, err := f.repo.GetMetadata(ctx, f.bucketName, objectKey)
	require.NoError(t, err)
	assert.Equal(t, int64(len(objectData)), meta.Size)
	assert.NotEmpty(t, meta.ETag)
	assert.Equal(t, objectKey, meta.ObjectKey)

	// ---- Download ----
	download, err := f.repo.Download(ctx, f.bucketName, objectKey)
	require.NoError(t, err)
	assert.Equal(t, objectData, download.Data)
	assert.Equal(t, "text/plain", download.ContentType)
	assert.Equal(t, int64(len(objectData)), download.Size)
	assert.Equal(t, metadata["env"], download.Metadata["env"])

	// ---- DownloadToWriter ----
	var buf bytes.Buffer
	err = f.repo.DownloadToWriter(ctx, f.bucketName, objectKey, &buf)
	require.NoError(t, err)
	assert.Equal(t, objectData, buf.Bytes())

	// ---- Copy ----
	copyKey := "repo-test-copy.txt"
	err = f.repo.Copy(ctx, f.bucketName, objectKey, f.bucketName, copyKey)
	require.NoError(t, err, "Copy should succeed")

	copyExists, err := f.repo.Exists(ctx, f.bucketName, copyKey)
	require.NoError(t, err)
	assert.True(t, copyExists)

	copyData, err := f.repo.Download(ctx, f.bucketName, copyKey)
	require.NoError(t, err)
	assert.Equal(t, objectData, copyData.Data)

	// ---- Move ----
	moveKey := "repo-test-moved.txt"
	err = f.repo.Move(ctx, f.bucketName, copyKey, f.bucketName, moveKey)
	require.NoError(t, err, "Move should succeed")

	moveExists, err := f.repo.Exists(ctx, f.bucketName, moveKey)
	require.NoError(t, err)
	assert.True(t, moveExists, "destination should exist after move")

	copyGone, err := f.repo.Exists(ctx, f.bucketName, copyKey)
	require.NoError(t, err)
	assert.False(t, copyGone, "source should not exist after move")

	// ---- List ----
	listResult, err := f.repo.List(ctx, f.bucketName, "repo-", nil)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, listResult.TotalCount, 2,
		"should list at least the original and moved objects")

	// ---- Delete ----
	err = f.repo.Delete(ctx, f.bucketName, objectKey)
	require.NoError(t, err)
	err = f.repo.Delete(ctx, f.bucketName, moveKey)
	require.NoError(t, err)

	// ---- DeleteBatch ----
	keys := []string{"batch-delete-1.txt", "batch-delete-2.txt"}
	for _, k := range keys {
		_, err := f.repo.Upload(ctx, &UploadRequest{
			Bucket:    f.bucketName,
			ObjectKey: k,
			Data:      []byte(k),
		})
		require.NoError(t, err)
	}

	errs, err := f.repo.DeleteBatch(ctx, f.bucketName, keys)
	require.NoError(t, err)
	assert.Empty(t, errs, "batch delete should have no errors")

	for _, k := range keys {
		exists, err = f.repo.Exists(ctx, f.bucketName, k)
		require.NoError(t, err)
		assert.False(t, exists, "%s should be deleted", k)
	}
}

// ---------------------------------------------------------------------------
// 4. Large file multipart upload
// ---------------------------------------------------------------------------

func TestIntegration_LargeFileUpload(t *testing.T) {
	f := newIntegrationFixture(t)

	// Create a large payload (~12 MB) that exceeds the minio minimum part size
	// of 5 MB, forcing the client to use multipart upload internally
	size := int64(12 * 1024 * 1024) // 12 MB
	reader := io.LimitReader(rand.Reader, size)

	objectKey := "large-file.bin"

	// Upload via stream with known size; client will split into parts
	result, err := f.repo.UploadStream(f.ctx, &StreamUploadRequest{
		Bucket:    f.bucketName,
		ObjectKey: objectKey,
		Reader:    reader,
		Size:      size,
	})
	require.NoError(t, err, "large file upload should succeed")
	assert.Equal(t, objectKey, result.ObjectKey)
	assert.Equal(t, size, result.Size)

	// Verify size via GetMetadata
	meta, err := f.repo.GetMetadata(f.ctx, f.bucketName, objectKey)
	require.NoError(t, err)
	assert.Equal(t, size, meta.Size, "stored object size should match")

	// Download and verify size
	download, err := f.repo.Download(f.ctx, f.bucketName, objectKey)
	require.NoError(t, err)
	assert.Equal(t, size, int64(len(download.Data)),
		"downloaded content length should match original")
	assert.Equal(t, size, download.Size)
}

// ---------------------------------------------------------------------------
// 5. Presigned URL generation and usage
// ---------------------------------------------------------------------------

func TestIntegration_PresignedURL(t *testing.T) {
	f := newIntegrationFixture(t)
	ctx := f.ctx

	objectKey := "presigned-test.txt"
	content := []byte("presigned URL integration test")
	expiry := 15 * time.Minute

	// Upload a test object
	_, err := f.minioClient.GetClient().PutObject(ctx, f.bucketName, objectKey,
		bytes.NewReader(content), int64(len(content)),
		minio.PutObjectOptions{ContentType: "text/plain"})
	require.NoError(t, err)

	// ---- Presigned GET URL ----
	getURL, err := f.minioClient.GeneratePresignedGetURL(ctx, f.bucketName, objectKey, expiry)
	require.NoError(t, err)
	assert.Contains(t, getURL, f.bucketName, "URL should reference the bucket")
	assert.Contains(t, getURL, objectKey, "URL should reference the object key")

	// Verify the presigned GET URL works via HTTP
	resp, err := http.Get(getURL)
	require.NoError(t, err, "presigned GET URL should be reachable")
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode, "GET should return 200")

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.Equal(t, content, body, "downloaded content should match")

	// ---- Presigned PUT URL ----
	putKey := "presigned-put-test.txt"
	putURL, err := f.minioClient.GeneratePresignedPutURL(ctx, f.bucketName, putKey, expiry)
	require.NoError(t, err)
	assert.Contains(t, putURL, f.bucketName)
	assert.Contains(t, putURL, putKey)

	// Verify the presigned PUT URL works via HTTP
	newContent := []byte("uploaded via presigned PUT URL")
	putReq, err := http.NewRequest(http.MethodPut, putURL, bytes.NewReader(newContent))
	require.NoError(t, err)
	putReq.Header.Set("Content-Type", "text/plain")

	putResp, err := http.DefaultClient.Do(putReq)
	require.NoError(t, err)
	putResp.Body.Close()
	assert.Equal(t, http.StatusOK, putResp.StatusCode, "PUT should return 200")

	// Confirm the uploaded content
	download, err := f.repo.Download(ctx, f.bucketName, putKey)
	require.NoError(t, err)
	assert.Equal(t, newContent, download.Data,
		"content uploaded via presigned PUT should match")

	// ---- Repository presigned URL methods ----
	repoGetURL, err := f.repo.GetPresignedDownloadURL(ctx, f.bucketName, objectKey, expiry)
	require.NoError(t, err)
	assert.Contains(t, repoGetURL, f.bucketName)

	repoPutURL, err := f.repo.GetPresignedUploadURL(ctx, f.bucketName, "repo-presigned-put.txt", expiry)
	require.NoError(t, err)
	assert.Contains(t, repoPutURL, f.bucketName)
}

// ---------------------------------------------------------------------------
// 6. Error handling
// ---------------------------------------------------------------------------

func TestIntegration_ErrorHandling(t *testing.T) {
	f := newIntegrationFixture(t)
	ctx := f.ctx
	api := f.minioClient.GetClient()

	t.Run("object_not_found_via_stat", func(t *testing.T) {
		_, err := api.StatObject(ctx, f.bucketName, "non-existent-key",
			minio.StatObjectOptions{})
		require.Error(t, err)
		assert.Equal(t, "NoSuchKey", minio.ToErrorResponse(err).Code)
	})

	t.Run("object_not_found_via_get", func(t *testing.T) {
		obj, err := api.GetObject(ctx, f.bucketName, "non-existent-key",
			minio.GetObjectOptions{})
		require.NoError(t, err) // GetObject returns the object immediately
		defer obj.Close()

		// Stat on the object reader returns the error
		_, err = obj.Stat()
		require.Error(t, err)
		assert.Equal(t, "NoSuchKey", minio.ToErrorResponse(err).Code)
	})

	t.Run("bucket_not_found", func(t *testing.T) {
		_, err := api.PutObject(ctx, "non-existent-bucket-for-test", "key",
			bytes.NewReader([]byte("data")), 4, minio.PutObjectOptions{})
		require.Error(t, err, "PutObject to non-existent bucket should fail")
		errResp := minio.ToErrorResponse(err)
		assert.Equal(t, "NoSuchBucket", errResp.Code)
	})

	t.Run("repository_download_object_not_found", func(t *testing.T) {
		_, err := f.repo.Download(ctx, f.bucketName, "non-existent-key")
		require.Error(t, err)
		assert.Equal(t, ErrObjectNotFound, err)
	})

	t.Run("repository_invalid_request_empty_fields", func(t *testing.T) {
		_, err := f.repo.Upload(ctx, &UploadRequest{
			Bucket:    "",
			ObjectKey: "",
		})
		require.Error(t, err)
		assert.Equal(t, ErrInvalidRequest, err)
	})

	t.Run("repository_bucket_not_found", func(t *testing.T) {
		_, err := f.repo.Upload(ctx, &UploadRequest{
			Bucket:    "non-existent-bucket-for-repo",
			ObjectKey: "key",
			Data:      []byte("data"),
		})
		require.Error(t, err, "upload to non-existent bucket should fail")
	})
}

// ---------------------------------------------------------------------------
// 7. Concurrent upload test
// ---------------------------------------------------------------------------

func TestIntegration_ConcurrentUpload(t *testing.T) {
	f := newIntegrationFixture(t)
	ctx := f.ctx

	const numGoroutines = 10
	const objectsPerGoroutine = 5
	totalObjects := numGoroutines * objectsPerGoroutine

	var wg sync.WaitGroup
	errCh := make(chan error, totalObjects)

	// Launch multiple goroutines that each upload several objects concurrently
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(routineID int) {
			defer wg.Done()
			for j := 0; j < objectsPerGoroutine; j++ {
				key := fmt.Sprintf("concurrent/r%d-f%d.txt", routineID, j)
				data := []byte(fmt.Sprintf("data from goroutine %d file %d", routineID, j))
				_, err := f.repo.Upload(ctx, &UploadRequest{
					Bucket:    f.bucketName,
					ObjectKey: key,
					Data:      data,
				})
				if err != nil {
					errCh <- fmt.Errorf("upload failed for %s: %w", key, err)
					return
				}
			}
		}(i)
	}

	wg.Wait()
	close(errCh)

	// Collect any errors
	var errs []error
	for e := range errCh {
		errs = append(errs, e)
	}
	assert.Empty(t, errs, "all concurrent uploads should succeed without error")

	// Verify all objects are present via listing
	listResult, err := f.repo.List(ctx, f.bucketName, "concurrent/",
		&ListOptions{MaxKeys: totalObjects, Recursive: true})
	require.NoError(t, err)
	assert.Equal(t, totalObjects, listResult.TotalCount,
		"all concurrently uploaded objects should be listed")

	// Verify each object's content
	for i := 0; i < numGoroutines; i++ {
		for j := 0; j < objectsPerGoroutine; j++ {
			key := fmt.Sprintf("concurrent/r%d-f%d.txt", i, j)
			expected := []byte(fmt.Sprintf("data from goroutine %d file %d", i, j))

			download, err := f.repo.Download(ctx, f.bucketName, key)
			if !assert.NoError(t, err, "should download %s", key) {
				continue
			}
			assert.Equal(t, expected, download.Data, "content should match for %s", key)
		}
	}
}

// ---------------------------------------------------------------------------
// 8. Tags: SetTags and GetTags via repository
// ---------------------------------------------------------------------------

func TestIntegration_ObjectTags(t *testing.T) {
	f := newIntegrationFixture(t)
	ctx := f.ctx

	objectKey := "tagged-object.txt"
	_, err := f.repo.Upload(ctx, &UploadRequest{
		Bucket:    f.bucketName,
		ObjectKey: objectKey,
		Data:      []byte("tagged object"),
	})
	require.NoError(t, err)

	// Set tags
	tagMap := map[string]string{
		"env":     "integration",
		"project": "keyip",
		"type":    "test",
	}
	err = f.repo.SetTags(ctx, f.bucketName, objectKey, tagMap)
	require.NoError(t, err, "SetTags should succeed")

	// Get tags
	retrievedTags, err := f.repo.GetTags(ctx, f.bucketName, objectKey)
	require.NoError(t, err)
	assert.Equal(t, tagMap["env"], retrievedTags["env"])
	assert.Equal(t, tagMap["project"], retrievedTags["project"])
	assert.Equal(t, tagMap["type"], retrievedTags["type"])
}

// ---------------------------------------------------------------------------
// 9. Health check via MinIOClient
// ---------------------------------------------------------------------------

func TestIntegration_HealthCheck(t *testing.T) {
	f := newIntegrationFixture(t)
	ctx := f.ctx

	status, err := f.minioClient.HealthCheck(ctx)
	require.NoError(t, err)
	assert.True(t, status.Healthy, "MinIO should report healthy")
	assert.NotZero(t, status.Latency, "latency should be measurable")
	assert.NotNil(t, status.BucketStatuses)
}

// ---------------------------------------------------------------------------
// 10. Streaming upload with unknown size
// ---------------------------------------------------------------------------

func TestIntegration_StreamUpload(t *testing.T) {
	f := newIntegrationFixture(t)
	ctx := f.ctx

	content := []byte("stream upload integration test")
	objectKey := "stream-upload.txt"

	result, err := f.repo.UploadStream(ctx, &StreamUploadRequest{
		Bucket:    f.bucketName,
		ObjectKey: objectKey,
		Reader:    bytes.NewReader(content),
		Size:      -1, // unknown size, forces chunked upload
	})
	require.NoError(t, err)
	assert.Equal(t, objectKey, result.ObjectKey)

	// Verify
	download, err := f.repo.Download(ctx, f.bucketName, objectKey)
	require.NoError(t, err)
	assert.Equal(t, content, download.Data)
}
