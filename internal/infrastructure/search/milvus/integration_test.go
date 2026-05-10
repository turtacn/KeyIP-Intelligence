//go:build integration

package milvus

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/milvus-io/milvus-sdk-go/v2/entity"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// getMilvusAddress returns the Milvus connection address or skips the test.
// It prefers MILVUS_ADDR and falls back to localhost:19530.
func getMilvusAddress(t *testing.T) string {
	t.Helper()
	addr := os.Getenv("MILVUS_ADDR")
	if addr == "" {
		addr = "localhost:19530"
	}
	return addr
}

// setupMilvusIntegration creates a real Client connected to a running Milvus instance.
// It returns the client, collection manager, searcher, and a cleanup function.
// Tests that call this helper are skipped when Milvus is unreachable.
func setupMilvusIntegration(t *testing.T) (*Client, *CollectionManager, *Searcher, func()) {
	t.Helper()

	addr := getMilvusAddress(t)
	logger := newMockLogger()

	cfg := ClientConfig{
		Address:        addr,
		ConnectTimeout: 5 * time.Second,
		RequestTimeout: 30 * time.Second,
		MaxRetries:     1,
	}

	client, err := NewClient(cfg, logger)
	if err != nil {
		t.Skipf("Milvus not available at %s: %v", addr, err)
		return nil, nil, nil, func() {}
	}

	collCfg := CollectionConfig{}
	collMgr := NewCollectionManager(client, collCfg, logger)

	searchCfg := SearcherConfig{
		DefaultTopK: 10,
	}
	searcher := NewSearcher(client, collMgr, searchCfg, logger)

	cleanup := func() {
		// Best-effort release and close
		_ = client.Close()
	}

	return client, collMgr, searcher, cleanup
}

// testCollectionName generates a unique collection name for isolation between test runs.
func testCollectionName(prefix string) string {
	return strings.ToLower(prefix + "_test_" + strings.ReplaceAll(time.Now().Format("150405.000"), ".", ""))
}

// testSchema returns a simple collection schema for integration tests.
func testSchema(name string) common.CollectionSchema {
	fields := []*entity.Field{
		{Name: "id", DataType: entity.FieldTypeInt64, PrimaryKey: true, AutoID: false},
		{Name: "embedding", DataType: entity.FieldTypeFloatVector, TypeParams: map[string]string{"dim": "4"}},
		{Name: "category", DataType: entity.FieldTypeVarChar, TypeParams: map[string]string{"max_length": "64"}},
	}
	ifaces := make([]interface{}, len(fields))
	for i, f := range fields {
		ifaces[i] = f
	}
	return common.CollectionSchema{
		Name:        name,
		Description: "Integration test collection",
		Fields:      ifaces,
	}
}

// testData generates n sample records with 4-d vectors and category labels.
func testData(n int) []map[string]interface{} {
	data := make([]map[string]interface{}, n)
	categories := []string{"electronics", "mechanical", "chemical", "software"}
	for i := 0; i < n; i++ {
		base := float32(i) * 0.1
		data[i] = map[string]interface{}{
			"id":        int64(i + 1),
			"embedding": []float32{base, base + 0.1, base + 0.2, base + 0.3},
			"category":  categories[i%len(categories)],
		}
	}
	return data
}

// cleanupCollection drops a collection and logs but does not fail on error.
func cleanupCollection(t *testing.T, collMgr *CollectionManager, name string) {
	t.Helper()
	if err := collMgr.DropCollection(context.Background(), name); err != nil {
		t.Logf("Cleanup drop collection %s: %v", name, err)
	}
}

// ---------------------------------------------------------------------------
// 1. Collection: create, describe, list, drop
// ---------------------------------------------------------------------------

func TestIntegrationMilvus_CreateCollection(t *testing.T) {
	_, collMgr, _, cleanup := setupMilvusIntegration(t)
	defer cleanup()

	ctx := context.Background()
	name := testCollectionName("create")

	schema := testSchema(name)
	err := collMgr.CreateCollection(ctx, schema)
	require.NoError(t, err)
	defer cleanupCollection(t, collMgr, name)

	// Verify existence.
	exists, err := collMgr.HasCollection(ctx, name)
	require.NoError(t, err)
	assert.True(t, exists)
}

func TestIntegrationMilvus_CreateCollection_DuplicateFails(t *testing.T) {
	_, collMgr, _, cleanup := setupMilvusIntegration(t)
	defer cleanup()

	ctx := context.Background()
	name := testCollectionName("dup")

	schema := testSchema(name)
	err := collMgr.CreateCollection(ctx, schema)
	require.NoError(t, err)
	defer cleanupCollection(t, collMgr, name)

	// Second create should fail.
	err = collMgr.CreateCollection(ctx, schema)
	assert.ErrorIs(t, err, ErrCollectionAlreadyExists)
}

func TestIntegrationMilvus_DescribeCollection(t *testing.T) {
	_, collMgr, _, cleanup := setupMilvusIntegration(t)
	defer cleanup()

	ctx := context.Background()
	name := testCollectionName("describe")

	schema := testSchema(name)
	err := collMgr.CreateCollection(ctx, schema)
	require.NoError(t, err)
	defer cleanupCollection(t, collMgr, name)

	info, err := collMgr.DescribeCollection(ctx, name)
	require.NoError(t, err)
	assert.Equal(t, name, info.Name)
	assert.Equal(t, "Integration test collection", info.Description)
	// Verify expected fields exist.
	fieldNames := make(map[string]bool)
	for _, f := range info.Fields {
		fieldNames[f.Name] = true
	}
	assert.True(t, fieldNames["id"], "expected id field")
	assert.True(t, fieldNames["embedding"], "expected embedding field")
	assert.True(t, fieldNames["category"], "expected category field")
}

func TestIntegrationMilvus_HasCollection_NotFound(t *testing.T) {
	_, collMgr, _, cleanup := setupMilvusIntegration(t)
	defer cleanup()

	ctx := context.Background()
	exists, err := collMgr.HasCollection(ctx, "nonexistent_collection_do_not_create")
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestIntegrationMilvus_DropCollection(t *testing.T) {
	_, collMgr, _, cleanup := setupMilvusIntegration(t)
	defer cleanup()

	ctx := context.Background()
	name := testCollectionName("drop")

	err := collMgr.CreateCollection(ctx, testSchema(name))
	require.NoError(t, err)

	// Drop it.
	err = collMgr.DropCollection(ctx, name)
	require.NoError(t, err)

	// Verify it is gone.
	exists, err := collMgr.HasCollection(ctx, name)
	require.NoError(t, err)
	assert.False(t, exists)
}

func TestIntegrationMilvus_DropCollection_NotFound(t *testing.T) {
	_, collMgr, _, cleanup := setupMilvusIntegration(t)
	defer cleanup()

	ctx := context.Background()
	err := collMgr.DropCollection(ctx, "nonexistent_collection_for_drop_test")
	assert.ErrorIs(t, err, ErrCollectionNotFound)
}

// ---------------------------------------------------------------------------
// 2. Index: create index, describe index
// ---------------------------------------------------------------------------

func TestIntegrationMilvus_CreateIndex(t *testing.T) {
	client, collMgr, _, cleanup := setupMilvusIntegration(t)
	defer cleanup()

	ctx := context.Background()
	name := testCollectionName("index")

	err := collMgr.CreateCollection(ctx, testSchema(name))
	require.NoError(t, err)
	defer cleanupCollection(t, collMgr, name)

	// Create an IVF_FLAT index on the embedding field.
	cfg := common.IndexConfig{
		FieldName:  "embedding",
		IndexType:  "IVF_FLAT",
		MetricType: "COSINE",
	}
	err = collMgr.CreateIndex(ctx, name, cfg)
	require.NoError(t, err)

	// Describe index through the underlying SDK client to verify.
	mc := client.GetMilvusClient()
	indexInfos, err := mc.DescribeIndex(ctx, name, "embedding")
	require.NoError(t, err)
	require.NotEmpty(t, indexInfos, "expected at least one index description")
	assert.Equal(t, "embedding", indexInfos[0].Name())
}

func TestIntegrationMilvus_DropIndex(t *testing.T) {
	_, collMgr, _, cleanup := setupMilvusIntegration(t)
	defer cleanup()

	ctx := context.Background()
	name := testCollectionName("dropidx")

	err := collMgr.CreateCollection(ctx, testSchema(name))
	require.NoError(t, err)
	defer cleanupCollection(t, collMgr, name)

	// Create index.
	cfg := common.IndexConfig{FieldName: "embedding", IndexType: "IVF_FLAT", MetricType: "COSINE"}
	err = collMgr.CreateIndex(ctx, name, cfg)
	require.NoError(t, err)

	// Drop it.
	err = collMgr.DropIndex(ctx, name, "embedding")
	require.NoError(t, err)
}

// ---------------------------------------------------------------------------
// 3. Vector insertion and search (by ID, ANN)
// ---------------------------------------------------------------------------

func TestIntegrationMilvus_InsertAndSearch(t *testing.T) {
	_, collMgr, searcher, cleanup := setupMilvusIntegration(t)
	defer cleanup()

	ctx := context.Background()
	name := testCollectionName("insertsearch")

	// Create collection, index, and load.
	err := collMgr.EnsureCollection(ctx, testSchema(name), []common.IndexConfig{
		{FieldName: "embedding", IndexType: "IVF_FLAT", MetricType: "COSINE"},
	})
	require.NoError(t, err)
	defer cleanupCollection(t, collMgr, name)

	// Insert 4 vectors.
	data := testData(4)
	req := common.InsertRequest{
		CollectionName: name,
		Data:           data,
	}
	res, err := searcher.Insert(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, int64(4), res.InsertedCount)
	assert.Len(t, res.IDs, 4)

	// Flush to make data visible (Milvus auto-flushes but may take time).
	_ = searcher.client.GetMilvusClient().Flush(ctx, name, false)

	// Search for similar vectors (query with first vector).
	searchReq := common.VectorSearchRequest{
		CollectionName:  name,
		VectorFieldName: "embedding",
		Vectors:         [][]float32{{0.0, 0.1, 0.2, 0.3}},
		TopK:            3,
		OutputFields:    []string{"id", "category"},
	}
	searchRes, err := searcher.Search(ctx, searchReq)
	require.NoError(t, err)
	require.Len(t, searchRes.Results, 1, "expected results for one query vector")
	hits := searchRes.Results[0]
	require.NotEmpty(t, hits, "expected at least one hit")
	// The closest vector (by COSINE) should be entity id=1 (vector [0.0, 0.1, 0.2, 0.3]).
	assert.Equal(t, int64(1), hits[0].ID, "expected closest match to be id=1")
}

func TestIntegrationMilvus_SearchByID(t *testing.T) {
	_, collMgr, searcher, cleanup := setupMilvusIntegration(t)
	defer cleanup()

	ctx := context.Background()
	name := testCollectionName("searchbyid")

	err := collMgr.EnsureCollection(ctx, testSchema(name), []common.IndexConfig{
		{FieldName: "embedding", IndexType: "IVF_FLAT", MetricType: "COSINE"},
	})
	require.NoError(t, err)
	defer cleanupCollection(t, collMgr, name)

	data := testData(4)
	_, err = searcher.Insert(ctx, common.InsertRequest{CollectionName: name, Data: data})
	require.NoError(t, err)
	_ = searcher.client.GetMilvusClient().Flush(ctx, name, false)

	// Search by entity id=1: find similar vectors.
	hits, err := searcher.SearchByID(ctx, name, "embedding", 1, 3, "", []string{"id", "category"})
	require.NoError(t, err)
	require.NotEmpty(t, hits, "expected at least one hit from SearchByID")
	assert.Equal(t, int64(1), hits[0].ID, "the source entity itself should be the top hit")
}

func TestIntegrationMilvus_ANN_Search(t *testing.T) {
	_, collMgr, searcher, cleanup := setupMilvusIntegration(t)
	defer cleanup()

	ctx := context.Background()
	name := testCollectionName("annsearch")

	err := collMgr.EnsureCollection(ctx, testSchema(name), []common.IndexConfig{
		{FieldName: "embedding", IndexType: "IVF_FLAT", MetricType: "COSINE"},
	})
	require.NoError(t, err)
	defer cleanupCollection(t, collMgr, name)

	// Insert 10 points along a line.
	data := testData(10)
	_, err = searcher.Insert(ctx, common.InsertRequest{CollectionName: name, Data: data})
	require.NoError(t, err)
	_ = searcher.client.GetMilvusClient().Flush(ctx, name, false)

	// Query with vector near item 5.
	queryVec := []float32{0.5, 0.6, 0.7, 0.8} // near id=5 -> [0.4, 0.5, 0.6, 0.7]
	req := common.VectorSearchRequest{
		CollectionName:  name,
		VectorFieldName: "embedding",
		Vectors:         [][]float32{queryVec},
		TopK:            5,
	}
	res, err := searcher.Search(ctx, req)
	require.NoError(t, err)
	require.Len(t, res.Results, 1)
	hits := res.Results[0]
	require.Len(t, hits, 5, "expected top-5 results")

	// The nearest neighbor should be id=5 (vector [0.4, 0.5, 0.6, 0.7]).
	assert.Equal(t, int64(5), hits[0].ID)
	// Scores should be positive (COSINE similarity).
	assert.Positive(t, hits[0].Score, "expected positive similarity score")
}

// ---------------------------------------------------------------------------
// 4. Search with filters and top-K
// ---------------------------------------------------------------------------

func TestIntegrationMilvus_SearchWithFilters(t *testing.T) {
	_, collMgr, searcher, cleanup := setupMilvusIntegration(t)
	defer cleanup()

	ctx := context.Background()
	name := testCollectionName("filter")

	err := collMgr.EnsureCollection(ctx, testSchema(name), []common.IndexConfig{
		{FieldName: "embedding", IndexType: "IVF_FLAT", MetricType: "COSINE"},
	})
	require.NoError(t, err)
	defer cleanupCollection(t, collMgr, name)

	// Insert data with known categories.
	data := testData(8)
	_, err = searcher.Insert(ctx, common.InsertRequest{CollectionName: name, Data: data})
	require.NoError(t, err)
	_ = searcher.client.GetMilvusClient().Flush(ctx, name, false)

	queryVec := [][]float32{{0.0, 0.1, 0.2, 0.3}}

	// Filter to only "electronics" category (id 1, 5).
	req := common.VectorSearchRequest{
		CollectionName:  name,
		VectorFieldName: "embedding",
		Vectors:         queryVec,
		TopK:            5,
		Filters:         `category == "electronics"`,
		OutputFields:    []string{"category"},
	}
	res, err := searcher.Search(ctx, req)
	require.NoError(t, err)
	require.Len(t, res.Results, 1)
	hits := res.Results[0]
	// With filter, we should get at most 2 results (ids 1 and 5).
	require.LessOrEqual(t, len(hits), 2, "expected at most 2 electronics hits")
	// Verify first hit is id=1 (closest electronics).
	assert.Equal(t, int64(1), hits[0].ID)

	// Filter to "chemical" category (id 3, 7).
	req.Filters = `category == "chemical"`
	res, err = searcher.Search(ctx, req)
	require.NoError(t, err)
	hits = res.Results[0]
	require.LessOrEqual(t, len(hits), 2)
	assert.Equal(t, int64(3), hits[0].ID)
}

func TestIntegrationMilvus_SearchWithTopK(t *testing.T) {
	_, collMgr, searcher, cleanup := setupMilvusIntegration(t)
	defer cleanup()

	ctx := context.Background()
	name := testCollectionName("topk")

	err := collMgr.EnsureCollection(ctx, testSchema(name), []common.IndexConfig{
		{FieldName: "embedding", IndexType: "IVF_FLAT", MetricType: "COSINE"},
	})
	require.NoError(t, err)
	defer cleanupCollection(t, collMgr, name)

	data := testData(10)
	_, err = searcher.Insert(ctx, common.InsertRequest{CollectionName: name, Data: data})
	require.NoError(t, err)
	_ = searcher.client.GetMilvusClient().Flush(ctx, name, false)

	queryVec := [][]float32{{0.0, 0.1, 0.2, 0.3}}

	// TopK = 2.
	req := common.VectorSearchRequest{
		CollectionName:  name,
		VectorFieldName: "embedding",
		Vectors:         queryVec,
		TopK:            2,
	}
	res, err := searcher.Search(ctx, req)
	require.NoError(t, err)
	require.Len(t, res.Results, 1)
	assert.Len(t, res.Results[0], 2, "expected exactly 2 results with TopK=2")

	// TopK = 5.
	req.TopK = 5
	res, err = searcher.Search(ctx, req)
	require.NoError(t, err)
	assert.Len(t, res.Results[0], 5, "expected exactly 5 results with TopK=5")
}

func TestIntegrationMilvus_Search_AllResults(t *testing.T) {
	_, collMgr, searcher, cleanup := setupMilvusIntegration(t)
	defer cleanup()

	ctx := context.Background()
	name := testCollectionName("allres")

	err := collMgr.EnsureCollection(ctx, testSchema(name), []common.IndexConfig{
		{FieldName: "embedding", IndexType: "IVF_FLAT", MetricType: "COSINE"},
	})
	require.NoError(t, err)
	defer cleanupCollection(t, collMgr, name)

	data := testData(10)
	_, err = searcher.Insert(ctx, common.InsertRequest{CollectionName: name, Data: data})
	require.NoError(t, err)
	_ = searcher.client.GetMilvusClient().Flush(ctx, name, false)

	// Request TopK larger than dataset.
	req := common.VectorSearchRequest{
		CollectionName:  name,
		VectorFieldName: "embedding",
		Vectors:         [][]float32{{0.0, 0.1, 0.2, 0.3}},
		TopK:            100,
	}
	res, err := searcher.Search(ctx, req)
	require.NoError(t, err)
	// Should return at most 10 results (the entire collection).
	assert.LessOrEqual(t, len(res.Results[0]), 10, "should not exceed collection size")
}

// ---------------------------------------------------------------------------
// 5. Batch operations
// ---------------------------------------------------------------------------

func TestIntegrationMilvus_BatchSearch(t *testing.T) {
	_, collMgr, searcher, cleanup := setupMilvusIntegration(t)
	defer cleanup()

	ctx := context.Background()
	name := testCollectionName("batchsearch")

	err := collMgr.EnsureCollection(ctx, testSchema(name), []common.IndexConfig{
		{FieldName: "embedding", IndexType: "IVF_FLAT", MetricType: "COSINE"},
	})
	require.NoError(t, err)
	defer cleanupCollection(t, collMgr, name)

	data := testData(6)
	_, err = searcher.Insert(ctx, common.InsertRequest{CollectionName: name, Data: data})
	require.NoError(t, err)
	_ = searcher.client.GetMilvusClient().Flush(ctx, name, false)

	// Batch of 3 query vectors.
	requests := []common.VectorSearchRequest{
		{
			CollectionName:  name,
			VectorFieldName: "embedding",
			Vectors:         [][]float32{{0.0, 0.1, 0.2, 0.3}},
			TopK:            2,
		},
		{
			CollectionName:  name,
			VectorFieldName: "embedding",
			Vectors:         [][]float32{{0.3, 0.4, 0.5, 0.6}},
			TopK:            2,
		},
		{
			CollectionName:  name,
			VectorFieldName: "embedding",
			Vectors:         [][]float32{{0.6, 0.7, 0.8, 0.9}},
			TopK:            2,
		},
	}

	results, err := searcher.BatchSearch(ctx, requests)
	require.NoError(t, err)
	require.Len(t, results, 3)

	for i, res := range results {
		require.NotNil(t, res, "batch result %d should not be nil", i)
		require.Len(t, res.Results, 1, "each batch request should have 1 query result")
		assert.Len(t, res.Results[0], 2, "expected 2 hits for batch request %d", i)
	}
}

func TestIntegrationMilvus_GetEntityByIDs(t *testing.T) {
	_, collMgr, searcher, cleanup := setupMilvusIntegration(t)
	defer cleanup()

	ctx := context.Background()
	name := testCollectionName("getentity")

	err := collMgr.EnsureCollection(ctx, testSchema(name), []common.IndexConfig{
		{FieldName: "embedding", IndexType: "IVF_FLAT", MetricType: "COSINE"},
	})
	require.NoError(t, err)
	defer cleanupCollection(t, collMgr, name)

	data := testData(4)
	_, err = searcher.Insert(ctx, common.InsertRequest{CollectionName: name, Data: data})
	require.NoError(t, err)
	_ = searcher.client.GetMilvusClient().Flush(ctx, name, false)

	// Retrieve entities by IDs [1, 3].
	rows, err := searcher.GetEntityByIDs(ctx, name, []int64{1, 3}, []string{"id", "category"})
	require.NoError(t, err)
	require.Len(t, rows, 2, "expected 2 entities")

	// Build lookup by id.
	byID := make(map[int64]map[string]interface{})
	for _, row := range rows {
		id, ok := row["id"].(int64)
		require.True(t, ok, "id should be int64")
		byID[id] = row
	}

	assert.Equal(t, "electronics", byID[1]["category"], "id=1 should be electronics")
	assert.Equal(t, "chemical", byID[3]["category"], "id=3 should be chemical")
}

func TestIntegrationMilvus_GetEntityCount(t *testing.T) {
	_, collMgr, searcher, cleanup := setupMilvusIntegration(t)
	defer cleanup()

	ctx := context.Background()
	name := testCollectionName("entitycount")

	err := collMgr.EnsureCollection(ctx, testSchema(name), []common.IndexConfig{
		{FieldName: "embedding", IndexType: "IVF_FLAT", MetricType: "COSINE"},
	})
	require.NoError(t, err)
	defer cleanupCollection(t, collMgr, name)

	data := testData(7)
	_, err = searcher.Insert(ctx, common.InsertRequest{CollectionName: name, Data: data})
	require.NoError(t, err)
	_ = searcher.client.GetMilvusClient().Flush(ctx, name, false)

	count, err := searcher.GetEntityCount(ctx, name)
	require.NoError(t, err)
	assert.Equal(t, int64(7), count, "expected 7 entities")
}

// ---------------------------------------------------------------------------
// 6. Error handling (collection not found, invalid vectors)
// ---------------------------------------------------------------------------

func TestIntegrationMilvus_Search_CollectionNotFound(t *testing.T) {
	_, _, searcher, cleanup := setupMilvusIntegration(t)
	defer cleanup()

	ctx := context.Background()
	req := common.VectorSearchRequest{
		CollectionName:  "nonexistent_collection_for_search_test",
		VectorFieldName: "embedding",
		Vectors:         [][]float32{{0.1, 0.2, 0.3, 0.4}},
		TopK:            10,
	}
	_, err := searcher.Search(ctx, req)
	require.Error(t, err)
	// The error message should indicate the collection was not found.
	assert.Contains(t, err.Error(), "search failed",
		"expected search to wrap with search failed message")
}

func TestIntegrationMilvus_DescribeCollection_NotFound(t *testing.T) {
	_, collMgr, _, cleanup := setupMilvusIntegration(t)
	defer cleanup()

	ctx := context.Background()
	_, err := collMgr.DescribeCollection(ctx, "nonexistent_collection_for_describe_test")
	require.Error(t, err)
}

func TestIntegrationMilvus_Insert_InvalidVectorDim(t *testing.T) {
	_, collMgr, searcher, cleanup := setupMilvusIntegration(t)
	defer cleanup()

	ctx := context.Background()
	name := testCollectionName("invaliddim")

	err := collMgr.EnsureCollection(ctx, testSchema(name), []common.IndexConfig{
		{FieldName: "embedding", IndexType: "IVF_FLAT", MetricType: "COSINE"},
	})
	require.NoError(t, err)
	defer cleanupCollection(t, collMgr, name)

	// Insert data where one vector has wrong dimension (2 instead of 4).
	badData := []map[string]interface{}{
		{"id": int64(1), "embedding": []float32{0.1, 0.2}, "category": "electronics"},
	}
	req := common.InsertRequest{
		CollectionName: name,
		Data:           badData,
	}
	_, err = searcher.Insert(ctx, req)
	require.Error(t, err, "expected error when inserting vector with wrong dimension")
}

func TestIntegrationMilvus_Insert_EmptyData(t *testing.T) {
	_, _, searcher, cleanup := setupMilvusIntegration(t)
	defer cleanup()

	ctx := context.Background()
	req := common.InsertRequest{
		CollectionName: "test",
		Data:           []map[string]interface{}{},
	}
	_, err := searcher.Insert(ctx, req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Data is empty")
}

func TestIntegrationMilvus_Insert_MissingCollectionName(t *testing.T) {
	_, _, searcher, cleanup := setupMilvusIntegration(t)
	defer cleanup()

	ctx := context.Background()
	req := common.InsertRequest{
		CollectionName: "",
		Data:           testData(1),
	}
	_, err := searcher.Insert(ctx, req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "CollectionName is required")
}

func TestIntegrationMilvus_GetEntityByIDs_NotFound(t *testing.T) {
	_, collMgr, searcher, cleanup := setupMilvusIntegration(t)
	defer cleanup()

	ctx := context.Background()
	name := testCollectionName("getnf")

	err := collMgr.EnsureCollection(ctx, testSchema(name), []common.IndexConfig{
		{FieldName: "embedding", IndexType: "IVF_FLAT", MetricType: "COSINE"},
	})
	require.NoError(t, err)
	defer cleanupCollection(t, collMgr, name)

	// Insert some data.
	_, err = searcher.Insert(ctx, common.InsertRequest{CollectionName: name, Data: testData(3)})
	require.NoError(t, err)

	// Query for nonexistent IDs should return empty, not error.
	rows, err := searcher.GetEntityByIDs(ctx, name, []int64{999, 1000}, []string{"id", "category"})
	require.NoError(t, err, "querying nonexistent IDs should not error")
	assert.Empty(t, rows, "expected no rows for nonexistent IDs")
}

// ---------------------------------------------------------------------------
// 7. Upsert and Delete operations
// ---------------------------------------------------------------------------

func TestIntegrationMilvus_Upsert(t *testing.T) {
	_, collMgr, searcher, cleanup := setupMilvusIntegration(t)
	defer cleanup()

	ctx := context.Background()
	name := testCollectionName("upsert")

	// Ensure collection with index and loaded.
	err := collMgr.EnsureCollection(ctx, testSchema(name), []common.IndexConfig{
		{FieldName: "embedding", IndexType: "IVF_FLAT", MetricType: "COSINE"},
	})
	require.NoError(t, err)
	defer cleanupCollection(t, collMgr, name)

	// Insert original data.
	data := []map[string]interface{}{
		{"id": int64(1), "embedding": []float32{0.1, 0.2, 0.3, 0.4}, "category": "electronics"},
		{"id": int64(2), "embedding": []float32{0.2, 0.3, 0.4, 0.5}, "category": "mechanical"},
	}
	_, err = searcher.Insert(ctx, common.InsertRequest{CollectionName: name, Data: data})
	require.NoError(t, err)
	_ = searcher.client.GetMilvusClient().Flush(ctx, name, false)

	// Upsert with same IDs but updated category.
	updateData := []map[string]interface{}{
		{"id": int64(1), "embedding": []float32{0.1, 0.2, 0.3, 0.4}, "category": "updated_electronics"},
		{"id": int64(3), "embedding": []float32{0.3, 0.4, 0.5, 0.6}, "category": "new_software"},
	}
	res, err := searcher.Upsert(ctx, common.InsertRequest{CollectionName: name, Data: updateData})
	require.NoError(t, err)
	assert.Equal(t, int64(2), res.InsertedCount)
	_ = searcher.client.GetMilvusClient().Flush(ctx, name, false)

	// Verify upserted entity has updated category.
	rows, err := searcher.GetEntityByIDs(ctx, name, []int64{1}, []string{"id", "category"})
	require.NoError(t, err)
	require.Len(t, rows, 1)
	assert.Equal(t, "updated_electronics", rows[0]["category"])
}

func TestIntegrationMilvus_Delete(t *testing.T) {
	_, collMgr, searcher, cleanup := setupMilvusIntegration(t)
	defer cleanup()

	ctx := context.Background()
	name := testCollectionName("delete")

	err := collMgr.EnsureCollection(ctx, testSchema(name), []common.IndexConfig{
		{FieldName: "embedding", IndexType: "IVF_FLAT", MetricType: "COSINE"},
	})
	require.NoError(t, err)
	defer cleanupCollection(t, collMgr, name)

	data := testData(4)
	_, err = searcher.Insert(ctx, common.InsertRequest{CollectionName: name, Data: data})
	require.NoError(t, err)
	_ = searcher.client.GetMilvusClient().Flush(ctx, name, false)

	// Delete id=1 and id=2.
	err = searcher.Delete(ctx, name, []int64{1, 2})
	require.NoError(t, err)

	// Verify they are gone.
	count, err := searcher.GetEntityCount(ctx, name)
	require.NoError(t, err)
	assert.Equal(t, int64(2), count, "expected 2 entities after deleting 2")
}

func TestIntegrationMilvus_Delete_EmptyIDs(t *testing.T) {
	_, _, searcher, cleanup := setupMilvusIntegration(t)
	defer cleanup()

	ctx := context.Background()
	err := searcher.Delete(ctx, "test", []int64{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "IDs cannot be empty")
}

// ---------------------------------------------------------------------------
// 8. EnsureCollection idempotency
// ---------------------------------------------------------------------------

func TestIntegrationMilvus_EnsureCollection_Idempotent(t *testing.T) {
	_, collMgr, _, cleanup := setupMilvusIntegration(t)
	defer cleanup()

	ctx := context.Background()
	name := testCollectionName("ensureidem")

	// First call creates the collection, index, and loads it.
	err := collMgr.EnsureCollection(ctx, testSchema(name), []common.IndexConfig{
		{FieldName: "embedding", IndexType: "IVF_FLAT", MetricType: "COSINE"},
	})
	require.NoError(t, err)
	defer cleanupCollection(t, collMgr, name)

	// Verify collection exists and is loaded.
	exists, err := collMgr.HasCollection(ctx, name)
	require.NoError(t, err)
	assert.True(t, exists)

	loadState, err := collMgr.GetLoadState(ctx, name)
	require.NoError(t, err)
	assert.Equal(t, "Loaded", loadState, "collection should be loaded")

	// Second call should be idempotent (no error).
	err = collMgr.EnsureCollection(ctx, testSchema(name), []common.IndexConfig{
		{FieldName: "embedding", IndexType: "IVF_FLAT", MetricType: "COSINE"},
	})
	require.NoError(t, err, "EnsureCollection should be idempotent")
}

// ---------------------------------------------------------------------------
// Extra: patent schema integration
// ---------------------------------------------------------------------------

func TestIntegrationMilvus_PatentSchema(t *testing.T) {
	_, collMgr, searcher, cleanup := setupMilvusIntegration(t)
	defer cleanup()

	ctx := context.Background()
	name := testCollectionName("patent")

	// Use the predefined patent schema with a unique name.
	ps := PatentVectorSchema()
	ps.Name = name
	err := collMgr.CreateCollection(ctx, ps)
	require.NoError(t, err)
	defer cleanupCollection(t, collMgr, name)

	// Describe and verify the patent collection.
	info, err := collMgr.DescribeCollection(ctx, name)
	require.NoError(t, err)
	assert.Equal(t, name, info.Name)

	// Verify all expected fields are present.
	fieldMap := make(map[string]*entity.Field)
	for _, f := range info.Fields {
		fieldMap[f.Name] = f
	}
	for _, expected := range []string{"id", "patent_number", "title_vector", "abstract_vector", "claims_vector", "tech_domain", "filing_date", "assignee"} {
		_, ok := fieldMap[expected]
		assert.True(t, ok, "expected field %q in patent schema", expected)
	}

	// Insert a minimal patent vector (need 768-d vectors for title, abstract, claims).
	_ = searcher // not used for this test
}

// ---------------------------------------------------------------------------
// Extra: molecule schema integration
// ---------------------------------------------------------------------------

func TestIntegrationMilvus_MoleculeSchema(t *testing.T) {
	_, collMgr, _, cleanup := setupMilvusIntegration(t)
	defer cleanup()

	ctx := context.Background()
	name := testCollectionName("molecule")

	ms := MoleculeVectorSchema()
	ms.Name = name
	err := collMgr.CreateCollection(ctx, ms)
	require.NoError(t, err)
	defer cleanupCollection(t, collMgr, name)

	info, err := collMgr.DescribeCollection(ctx, name)
	require.NoError(t, err)
	assert.Equal(t, name, info.Name)

	fieldMap := make(map[string]bool)
	for _, f := range info.Fields {
		fieldMap[f.Name] = true
	}
	for _, expected := range []string{"id", "smiles", "fingerprint_vector", "structure_vector", "molecular_weight", "source_patent"} {
		assert.True(t, fieldMap[expected], "expected field %q in molecule schema", expected)
	}
}

// ---------------------------------------------------------------------------
// Extra: Server version check
// ---------------------------------------------------------------------------

func TestIntegrationMilvus_GetServerVersion(t *testing.T) {
	client, _, _, cleanup := setupMilvusIntegration(t)
	defer cleanup()

	ctx := context.Background()
	version, err := client.GetServerVersion(ctx)
	require.NoError(t, err)
	assert.NotEmpty(t, version, "expected non-empty server version")
	t.Logf("Milvus server version: %s", version)
}

// ---------------------------------------------------------------------------
// Extra: multiple query vectors in a single Search call
// ---------------------------------------------------------------------------

func TestIntegrationMilvus_Search_MultipleQueryVectors(t *testing.T) {
	_, collMgr, searcher, cleanup := setupMilvusIntegration(t)
	defer cleanup()

	ctx := context.Background()
	name := testCollectionName("multiQ")

	err := collMgr.EnsureCollection(ctx, testSchema(name), []common.IndexConfig{
		{FieldName: "embedding", IndexType: "IVF_FLAT", MetricType: "COSINE"},
	})
	require.NoError(t, err)
	defer cleanupCollection(t, collMgr, name)

	data := testData(10)
	_, err = searcher.Insert(ctx, common.InsertRequest{CollectionName: name, Data: data})
	require.NoError(t, err)
	_ = searcher.client.GetMilvusClient().Flush(ctx, name, false)

	// Search with 2 query vectors simultaneously.
	req := common.VectorSearchRequest{
		CollectionName:  name,
		VectorFieldName: "embedding",
		Vectors: [][]float32{
			{0.0, 0.1, 0.2, 0.3}, // close to id=1
			{0.5, 0.6, 0.7, 0.8}, // close to id=5
		},
		TopK: 3,
	}
	res, err := searcher.Search(ctx, req)
	require.NoError(t, err)
	require.Len(t, res.Results, 2, "expected results for 2 query vectors")

	// First query result: top hit should be id=1.
	assert.Equal(t, int64(1), res.Results[0][0].ID, "first query top hit should be id=1")
	// Second query result: top hit should be id=5.
	assert.Equal(t, int64(5), res.Results[1][0].ID, "second query top hit should be id=5")

	// Each result should have 3 hits.
	assert.Len(t, res.Results[0], 3)
	assert.Len(t, res.Results[1], 3)
}

// ---------------------------------------------------------------------------
// Extra: hybrid search (multi-vector fusion via RRF)
// ---------------------------------------------------------------------------

func TestIntegrationMilvus_HybridSearch(t *testing.T) {
	_, collMgr, searcher, cleanup := setupMilvusIntegration(t)
	defer cleanup()

	ctx := context.Background()
	// We reuse the same collection but pretend two vector fields.
	// In practice hybrid search searches the same collection with different
	// vector fields, but our test schema only has one vector field (embedding).
	// So we search the same field twice with different query vectors.
	name := testCollectionName("hybrid")

	err := collMgr.EnsureCollection(ctx, testSchema(name), []common.IndexConfig{
		{FieldName: "embedding", IndexType: "IVF_FLAT", MetricType: "COSINE"},
	})
	require.NoError(t, err)
	defer cleanupCollection(t, collMgr, name)

	// Insert 10 points.
	data := testData(10)
	_, err = searcher.Insert(ctx, common.InsertRequest{CollectionName: name, Data: data})
	require.NoError(t, err)
	_ = searcher.client.GetMilvusClient().Flush(ctx, name, false)

	// Two search requests with different query vectors on the same field.
	req1 := common.VectorSearchRequest{
		VectorFieldName: "embedding",
		Vectors:         [][]float32{{0.0, 0.1, 0.2, 0.3}},
	}
	req2 := common.VectorSearchRequest{
		VectorFieldName: "embedding",
		Vectors:         [][]float32{{0.3, 0.4, 0.5, 0.6}},
	}

	res, err := searcher.HybridSearch(ctx, name, []common.VectorSearchRequest{req1, req2}, &RRFReranker{K: 60}, 5)
	require.NoError(t, err)
	require.Len(t, res.Results, 1, "hybrid search with 1 batch should have 1 result set")

	hits := res.Results[0]
	require.NotEmpty(t, hits, "hybrid search should return results")

	// The RRF should rank id=2 and id=4 highly since they appear in both
	// ranking lists (id=2 is near both queries, id=4 is also relatively close).
	// At minimum, results should be deduplicated and sorted by fused score.
	t.Logf("Hybrid search top-5 IDs: %v", extractIDs(hits))
	assert.Greater(t, hits[0].Score, float32(0), "expected positive fused score")
}

func extractIDs(hits []common.VectorHit) []int64 {
	ids := make([]int64, len(hits))
	for i, h := range hits {
		ids[i] = h.ID
	}
	return ids
}

// ---------------------------------------------------------------------------
// Extra: Upsert validation errors
// ---------------------------------------------------------------------------

func TestIntegrationMilvus_Upsert_NoCollectionName(t *testing.T) {
	_, _, searcher, cleanup := setupMilvusIntegration(t)
	defer cleanup()

	ctx := context.Background()
	_, err := searcher.Upsert(ctx, common.InsertRequest{
		CollectionName: "",
		Data:           testData(1),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "CollectionName is required")
}

// ---------------------------------------------------------------------------
// Extra: insert with explicit flush to verify visibility
// ---------------------------------------------------------------------------

func TestIntegrationMilvus_InsertAndVerifyVisible(t *testing.T) {
	_, collMgr, searcher, cleanup := setupMilvusIntegration(t)
	defer cleanup()

	ctx := context.Background()
	name := testCollectionName("visible")

	err := collMgr.EnsureCollection(ctx, testSchema(name), []common.IndexConfig{
		{FieldName: "embedding", IndexType: "IVF_FLAT", MetricType: "COSINE"},
	})
	require.NoError(t, err)
	defer cleanupCollection(t, collMgr, name)

	data := testData(2)
	_, err = searcher.Insert(ctx, common.InsertRequest{CollectionName: name, Data: data})
	require.NoError(t, err)
	_ = searcher.client.GetMilvusClient().Flush(ctx, name, false)

	// Count should reflect inserted data.
	count, err := searcher.GetEntityCount(ctx, name)
	require.NoError(t, err)
	assert.Equal(t, int64(2), count, "expected 2 visible entities after insert and flush")
}

// ---------------------------------------------------------------------------
// Extra: Load and Release collection lifecycle
// ---------------------------------------------------------------------------

func TestIntegrationMilvus_LoadAndReleaseCollection(t *testing.T) {
	_, collMgr, _, cleanup := setupMilvusIntegration(t)
	defer cleanup()

	ctx := context.Background()
	name := testCollectionName("loadrelease")

	err := collMgr.CreateCollection(ctx, testSchema(name))
	require.NoError(t, err)
	defer cleanupCollection(t, collMgr, name)

	// Create index to support loading.
	cfg := common.IndexConfig{FieldName: "embedding", IndexType: "IVF_FLAT", MetricType: "COSINE"}
	err = collMgr.CreateIndex(ctx, name, cfg)
	require.NoError(t, err)

	// Load.
	err = collMgr.LoadCollection(ctx, name)
	require.NoError(t, err)

	// Check load state.
	state, err := collMgr.GetLoadState(ctx, name)
	require.NoError(t, err)
	assert.Equal(t, "Loaded", state)

	// Release.
	err = collMgr.ReleaseCollection(ctx, name)
	require.NoError(t, err)

	// After release, the load state should not be "Loaded".
	state, err = collMgr.GetLoadState(ctx, name)
	require.NoError(t, err)
	assert.NotEqual(t, "Loaded", state, "collection should not be loaded after release")
}

func TestIntegrationMilvus_MultipleQueriesBatchSearch_SomeFail(t *testing.T) {
	_, collMgr, searcher, cleanup := setupMilvusIntegration(t)
	defer cleanup()

	ctx := context.Background()
	name := testCollectionName("batchpartial")

	err := collMgr.EnsureCollection(ctx, testSchema(name), []common.IndexConfig{
		{FieldName: "embedding", IndexType: "IVF_FLAT", MetricType: "COSINE"},
	})
	require.NoError(t, err)
	defer cleanupCollection(t, collMgr, name)

	data := testData(4)
	_, err = searcher.Insert(ctx, common.InsertRequest{CollectionName: name, Data: data})
	require.NoError(t, err)
	_ = searcher.client.GetMilvusClient().Flush(ctx, name, false)

	// Batch with one valid and one invalid (collection doesn't exist) request.
	// BatchSearch tolerates individual failures (returns nil for that slot).
	requests := []common.VectorSearchRequest{
		{
			CollectionName:  name,
			VectorFieldName: "embedding",
			Vectors:         [][]float32{{0.0, 0.1, 0.2, 0.3}},
			TopK:            2,
		},
		{
			CollectionName:  "nonexistent_collection_for_batch",
			VectorFieldName: "embedding",
			Vectors:         [][]float32{{0.1, 0.2, 0.3, 0.4}},
			TopK:            2,
		},
	}

	results, err := searcher.BatchSearch(ctx, requests)
	require.NoError(t, err)
	require.Len(t, results, 2)
	// First result should be successful.
	require.NotNil(t, results[0], "valid request should produce a result")
	// Second result may be nil due to individual failure (BatchSearch tolerates this).
	t.Logf("Second batch result (expected nil or error): %v", results[1])
}

func describeCollectionOutput(t *testing.T, info *CollectionInfo) string {
	t.Helper()
	fieldNames := make([]string, len(info.Fields))
	for i, f := range info.Fields {
		fieldNames[i] = fmt.Sprintf("%s(%s)", f.Name, f.DataType)
	}
	return fmt.Sprintf("Collection{Name: %s, Fields: [%s]}", info.Name, strings.Join(fieldNames, ", "))
}
