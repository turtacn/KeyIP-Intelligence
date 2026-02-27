package opensearch

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	opensearchgo "github.com/opensearch-project/opensearch-go/v2"
	"github.com/stretchr/testify/assert"
	"github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
)

// Reusing MockLogger and newTestClient helper logic from client_test.go
// but since they are in the same package (opensearch), they should be available if I didn't export them?
// No, tests in same package `opensearch` share symbols if in `package opensearch`.
// `client_test.go` has `package opensearch`.
// But I defined `MockLogger` there.
// If `client_test.go` is compiled with `indexer_test.go`, it's fine.
// But `go test` compiles all `*_test.go` files in the package.

func newTestIndexer(serverURL string) *Indexer {
	// Manually construct Client to avoid Ping during test setup if server handler doesn't support it
	// But tests should support Ping or we assume Client is healthy.
	// We'll manually build Client.

	osCfg := opensearchgo.Config{
		Addresses: []string{serverURL},
	}
	osClient, err := opensearchgo.NewClient(osCfg)
	if err != nil {
		panic(err)
	}

	c := &Client{
		client: osClient,
		config: ClientConfig{Addresses: []string{serverURL}},
		logger: newMockLogger(),
	}
	c.healthy.Store(true)

	idxCfg := IndexerConfig{
		BulkBatchSize:     500,
		BulkFlushInterval: 1 * time.Second,
		BulkFlushBytes:    1024,
		BulkWorkers:       1,
	}
	return NewIndexer(c, idxCfg, newMockLogger())
}

func TestCreateIndex_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "HEAD" {
			w.WriteHeader(http.StatusNotFound) // Index doesn't exist
			return
		}
		if r.Method == "PUT" && strings.Contains(r.URL.Path, "test-index") {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"acknowledged": true}`))
			return
		}
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	indexer := newTestIndexer(server.URL)
	err := indexer.CreateIndex(context.Background(), "test-index", common.IndexMapping{})
	assert.NoError(t, err)
}

func TestCreateIndex_AlreadyExists(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "HEAD" {
			w.WriteHeader(http.StatusOK) // Exists
			return
		}
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	indexer := newTestIndexer(server.URL)
	err := indexer.CreateIndex(context.Background(), "test-index", common.IndexMapping{})
	assert.Error(t, err)
	assert.Equal(t, ErrIndexAlreadyExists, err)
}

func TestDeleteIndex_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "DELETE" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"acknowledged": true}`))
			return
		}
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	indexer := newTestIndexer(server.URL)
	err := indexer.DeleteIndex(context.Background(), "test-index")
	assert.NoError(t, err)
}

func TestDeleteIndex_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	indexer := newTestIndexer(server.URL)
	err := indexer.DeleteIndex(context.Background(), "test-index")
	assert.Error(t, err)
	assert.Equal(t, ErrIndexNotFound, err)
}

func TestIndexDocument_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "PUT" && strings.Contains(r.URL.Path, "/_doc/") {
			body, _ := io.ReadAll(r.Body)
			assert.Contains(t, string(body), "value")
			w.WriteHeader(http.StatusCreated)
			w.Write([]byte(`{"_id": "1", "result": "created"}`))
			return
		}
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	indexer := newTestIndexer(server.URL)
	doc := map[string]string{"key": "value"}
	err := indexer.IndexDocument(context.Background(), "test-index", "1", doc)
	assert.NoError(t, err)
}

func TestBulkIndex_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && strings.Contains(r.URL.Path, "_bulk") {
			w.WriteHeader(http.StatusOK)
			// Mock bulk response
			w.Write([]byte(`{
				"took": 30,
				"errors": false,
				"items": [
					{"index": {"_index": "test", "_id": "1", "status": 201}},
					{"index": {"_index": "test", "_id": "2", "status": 201}}
				]
			}`))
			return
		}
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	indexer := newTestIndexer(server.URL)
	docs := map[string]interface{}{
		"1": map[string]string{"k": "v"},
		"2": map[string]string{"k": "v"},
	}
	result, err := indexer.BulkIndex(context.Background(), "test-index", docs)
	assert.NoError(t, err)
	assert.Equal(t, 2, result.Succeeded)
	assert.Equal(t, 0, result.Failed)
}

func TestBulkIndex_PartialFailure(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && strings.Contains(r.URL.Path, "_bulk") {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{
				"took": 30,
				"errors": true,
				"items": [
					{"index": {"_index": "test", "_id": "1", "status": 201}},
					{"index": {"_index": "test", "_id": "2", "status": 400, "error": {"type": "mapper_parsing_exception", "reason": "failed"}}}
				]
			}`))
			return
		}
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	indexer := newTestIndexer(server.URL)
	docs := map[string]interface{}{
		"1": map[string]string{"k": "v"},
		"2": map[string]string{"k": "v"},
	}
	result, err := indexer.BulkIndex(context.Background(), "test-index", docs)
	assert.NoError(t, err)
	assert.Equal(t, 1, result.Succeeded)
	assert.Equal(t, 1, result.Failed)
	assert.Len(t, result.Errors, 1)
	assert.Equal(t, "2", result.Errors[0].DocID)
}

func TestPatentIndexMapping(t *testing.T) {
	m := PatentIndexMapping()
	assert.NotNil(t, m.Mappings)
	assert.NotNil(t, m.Settings)

	props := m.Mappings["properties"].(map[string]interface{})
	assert.Contains(t, props, "patent_number")
	assert.Contains(t, props, "title")
	assert.Contains(t, props, "abstract")
}
