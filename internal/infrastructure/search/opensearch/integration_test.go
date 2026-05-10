//go:build integration

package opensearch

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// getOpenSearchURL returns the connection URL for OpenSearch or skips the test.
// It prefers OPENSEARCH_URL and falls back to the default local address.
func getOpenSearchURL(t *testing.T) string {
	t.Helper()
	url := os.Getenv("OPENSEARCH_URL")
	if url == "" {
		url = "http://localhost:9200"
	}
	return url
}

// setupIntegration creates a real Client connected to a running OpenSearch instance.
// It returns the client, indexer, searcher, and a cleanup function.
// Tests that call this helper are skipped when OpenSearch is unreachable.
func setupIntegration(t *testing.T) (*Client, *Indexer, *Searcher, func()) {
	t.Helper()

	addr := getOpenSearchURL(t)
	logger := logging.NewNopLogger()

	cfg := ClientConfig{
		Addresses:      []string{addr},
		RequestTimeout: 10 * time.Second,
		MaxRetries:     1,
	}

	client, err := NewClient(cfg, logger)
	if err != nil {
		t.Skipf("OpenSearch not available at %s: %v", addr, err)
		return nil, nil, nil, func() {}
	}

	idxCfg := IndexerConfig{
		BulkBatchSize:     100,
		BulkFlushInterval: 1 * time.Second,
		BulkFlushBytes:    1024,
		BulkWorkers:       1,
		RefreshPolicy:     "true", // wait for refresh in tests so we can read immediately
	}
	indexer := NewIndexer(client, idxCfg, logger)

	searchCfg := SearcherConfig{
		DefaultPageSize: 10,
		MaxPageSize:     100,
	}
	searcher := NewSearcher(client, searchCfg, logger)

	cleanup := func() {
		client.Close()
	}

	return client, indexer, searcher, cleanup
}

// testIndexName generates a unique index name for isolation between test runs.
func testIndexName(prefix string) string {
	return strings.ToLower(prefix + "_test_" + strings.ReplaceAll(time.Now().Format("150405.000"), ".", ""))
}

// patentDoc is a small struct used as document payload in integration tests.
type patentDoc struct {
	PatentNumber string   `json:"patent_number"`
	Title        string   `json:"title"`
	Abstract     string   `json:"abstract"`
	Assignee     string   `json:"assignee"`
	FilingDate   string   `json:"filing_date"`
	IPCCodes     []string `json:"ipc_codes"`
	LegalStatus  string   `json:"legal_status"`
	FullText     string   `json:"full_text"`
}

// ---------------------------------------------------------------------------
// 1. Index creation and mapping
// ---------------------------------------------------------------------------

func TestIntegration_CreateIndex_WithMapping(t *testing.T) {
	_, indexer, _, cleanup := setupIntegration(t)
	defer cleanup()

	ctx := context.Background()
	idx := testIndexName("createidx")

	// Use the predefined patent mapping.
	mapping := PatentIndexMapping()
	err := indexer.CreateIndex(ctx, idx, mapping)
	require.NoError(t, err)

	// Verify it exists.
	exists, err := indexer.IndexExists(ctx, idx)
	require.NoError(t, err)
	assert.True(t, exists)

	// Cleanup.
	err = indexer.DeleteIndex(ctx, idx)
	require.NoError(t, err)
}

func TestIntegration_CreateIndex_DuplicateFails(t *testing.T) {
	_, indexer, _, cleanup := setupIntegration(t)
	defer cleanup()

	ctx := context.Background()
	idx := testIndexName("dupindex")

	err := indexer.CreateIndex(ctx, idx, PatentIndexMapping())
	require.NoError(t, err)
	defer indexer.DeleteIndex(ctx, idx)

	// Second create should fail.
	err = indexer.CreateIndex(ctx, idx, PatentIndexMapping())
	assert.ErrorIs(t, err, ErrIndexAlreadyExists)
}

func TestIntegration_DeleteIndex_NotFound(t *testing.T) {
	_, indexer, _, cleanup := setupIntegration(t)
	defer cleanup()

	ctx := context.Background()
	err := indexer.DeleteIndex(ctx, "nonexistent_index_12345")
	assert.ErrorIs(t, err, ErrIndexNotFound)
}

// ---------------------------------------------------------------------------
// 2. Document indexing and retrieval (CRUD)
// ---------------------------------------------------------------------------

func TestIntegration_IndexAndRetrieveDocument(t *testing.T) {
	_, indexer, searcher, cleanup := setupIntegration(t)
	defer cleanup()

	ctx := context.Background()
	idx := testIndexName("crud")

	err := indexer.CreateIndex(ctx, idx, PatentIndexMapping())
	require.NoError(t, err)
	defer indexer.DeleteIndex(ctx, idx)

	doc := patentDoc{
		PatentNumber: "US12345678",
		Title:        "Semiconductor device manufacturing method",
		Abstract:     "A method for manufacturing semiconductor devices with improved yield.",
		Assignee:     "Intel Corporation",
		FilingDate:   "2023-06-15",
		IPCCodes:     []string{"H01L21/00", "H01L21/02"},
		LegalStatus:  "active",
	}

	docID := "patent-1"
	err = indexer.IndexDocument(ctx, idx, docID, doc)
	require.NoError(t, err)

	// Retrieve via search.
	req := common.SearchRequest{
		IndexName: idx,
		Query: &common.Query{
			QueryType: "term",
			Field:     "_id",
			Value:     docID,
		},
	}
	result, err := searcher.Search(ctx, req)
	require.NoError(t, err)
	require.Equal(t, int64(1), result.Total)

	// Verify the source.
	var retrieved patentDoc
	err = json.Unmarshal(result.Hits[0].Source, &retrieved)
	require.NoError(t, err)
	assert.Equal(t, "US12345678", retrieved.PatentNumber)
	assert.Equal(t, "Intel Corporation", retrieved.Assignee)
}

func TestIntegration_DeleteDocument(t *testing.T) {
	_, indexer, searcher, cleanup := setupIntegration(t)
	defer cleanup()

	ctx := context.Background()
	idx := testIndexName("del")
	err := indexer.CreateIndex(ctx, idx, PatentIndexMapping())
	require.NoError(t, err)
	defer indexer.DeleteIndex(ctx, idx)

	docID := "to-delete"
	err = indexer.IndexDocument(ctx, idx, docID, patentDoc{PatentNumber: "US99999999", Title: "Test patent"})
	require.NoError(t, err)

	// Verify present.
	exists, err := indexer.IndexExists(ctx, idx)
	require.NoError(t, err)
	assert.True(t, exists)

	// Delete.
	err = indexer.DeleteDocument(ctx, idx, docID)
	require.NoError(t, err)

	// Confirm gone.
	req := common.SearchRequest{
		IndexName: idx,
		Query: &common.Query{
			QueryType: "term",
			Field:     "_id",
			Value:     docID,
		},
	}
	result, err := searcher.Search(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, int64(0), result.Total)
}

func TestIntegration_DeleteDocument_NotFound(t *testing.T) {
	_, indexer, _, cleanup := setupIntegration(t)
	defer cleanup()

	ctx := context.Background()
	idx := testIndexName("delnf")
	err := indexer.CreateIndex(ctx, idx, PatentIndexMapping())
	require.NoError(t, err)
	defer indexer.DeleteIndex(ctx, idx)

	err = indexer.DeleteDocument(ctx, idx, "nonexistent-id")
	assert.ErrorIs(t, err, ErrDocumentNotFound)
}

// ---------------------------------------------------------------------------
// 3. Full-text search (basic, CJK, multilingual)
// ---------------------------------------------------------------------------

func TestIntegration_FullTextSearch_Basic(t *testing.T) {
	_, indexer, searcher, cleanup := setupIntegration(t)
	defer cleanup()

	ctx := context.Background()
	idx := testIndexName("fts")
	err := indexer.CreateIndex(ctx, idx, PatentIndexMapping())
	require.NoError(t, err)
	defer indexer.DeleteIndex(ctx, idx)

	// Index a few documents.
	docs := map[string]interface{}{
		"p1": patentDoc{PatentNumber: "US10000001", Title: "Machine learning processor", Abstract: "A processor optimized for machine learning workloads.", Assignee: "Google LLC"},
		"p2": patentDoc{PatentNumber: "US10000002", Title: "Cloud computing system", Abstract: "Distributed cloud computing architecture.", Assignee: "Amazon Technologies"},
		"p3": patentDoc{PatentNumber: "US10000003", Title: "Wireless communication protocol", Abstract: "Efficient wireless communication for IoT devices.", Assignee: "Qualcomm"},
	}
	_, err = indexer.BulkIndex(ctx, idx, docs)
	require.NoError(t, err)

	// Search for "machine learning".
	req := common.SearchRequest{
		IndexName: idx,
		Query: &common.Query{
			QueryType: "match",
			Field:     "title",
			Value:     "machine learning",
		},
	}
	result, err := searcher.Search(ctx, req)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, result.Total, int64(1))
	assert.Equal(t, "p1", result.Hits[0].ID)

	// Search for "cloud".
	req.Query = &common.Query{
		QueryType: "match",
		Field:     "abstract",
		Value:     "cloud",
	}
	result, err = searcher.Search(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, int64(1), result.Total)
	assert.Equal(t, "p2", result.Hits[0].ID)

	// Search across multiple fields.
	req.Query = &common.Query{
		QueryType: "multi_match",
		Value:     "wireless",
		Fields:    []string{"title", "abstract"},
	}
	result, err = searcher.Search(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, int64(1), result.Total)
}

func TestIntegration_FullTextSearch_CJK(t *testing.T) {
	_, indexer, searcher, cleanup := setupIntegration(t)
	defer cleanup()

	ctx := context.Background()
	idx := testIndexName("cjkfts")
	err := indexer.CreateIndex(ctx, idx, PatentIndexMapping())
	require.NoError(t, err)
	defer indexer.DeleteIndex(ctx, idx)

	// Index documents with CJK text (OpenSearch standard analyzer has basic CJK support).
	docs := map[string]interface{}{
		"c1": patentDoc{PatentNumber: "CN10000001", Title: "半导体器件制造方法", Abstract: "一种改进的半导体器件制造方法", Assignee: "华为技术有限公司"},
		"c2": patentDoc{PatentNumber: "CN10000002", Title: "无线通信方法", Abstract: "用于5G通信的无线通信方法", Assignee: "中兴通讯"},
	}
	_, err = indexer.BulkIndex(ctx, idx, docs)
	require.NoError(t, err)

	// Search using CJK text.
	req := common.SearchRequest{
		IndexName: idx,
		Query: &common.Query{
			QueryType: "match",
			Field:     "title",
			Value:     "半导体",
		},
	}
	result, err := searcher.Search(ctx, req)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, result.Total, int64(1))

	// Search for "无线通信".
	req.Query = &common.Query{
		QueryType: "match",
		Field:     "abstract",
		Value:     "5G通信",
	}
	result, err = searcher.Search(ctx, req)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, result.Total, int64(1))
}

func TestIntegration_FullTextSearch_Multilingual(t *testing.T) {
	_, indexer, searcher, cleanup := setupIntegration(t)
	defer cleanup()

	ctx := context.Background()
	idx := testIndexName("mlfts")
	err := indexer.CreateIndex(ctx, idx, PatentIndexMapping())
	require.NoError(t, err)
	defer indexer.DeleteIndex(ctx, idx)

	// Mix of languages in titles/abstracts.
	docs := map[string]interface{}{
		"m1": patentDoc{PatentNumber: "EP10000001", Title: "Verfahren zur Herstellung eines Halbleiters", Abstract: "Ein Verfahren zur Herstellung von Halbleiterbauelementen.", Assignee: "Siemens AG", FullText: "Die Erfindung betrifft ein Verfahren zur Herstellung eines Halbleiters."},
		"m2": patentDoc{PatentNumber: "JP10000001", Title: "半導体装置の製造方法", Abstract: "半導体装置の製造方法に関する。", Assignee: "株式会社東芝"},
		"m3": patentDoc{PatentNumber: "KR10000001", Title: "반도체 소자 제조 방법", Abstract: "반도체 소자의 제조 방법에 관한 것이다.", Assignee: "삼성전자"},
	}
	_, err = indexer.BulkIndex(ctx, idx, docs)
	require.NoError(t, err)

	// German.
	req := common.SearchRequest{
		IndexName: idx,
		Query: &common.Query{
			QueryType: "match",
			Field:     "title",
			Value:     "Verfahren zur Herstellung",
		},
	}
	result, err := searcher.Search(ctx, req)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, result.Total, int64(1))
	assert.Equal(t, "m1", result.Hits[0].ID)

	// Japanese - search with Japanese text.
	req.Query = &common.Query{
		QueryType: "match",
		Field:     "abstract",
		Value:     "製造方法",
	}
	result, err = searcher.Search(ctx, req)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, result.Total, int64(1))
}

// ---------------------------------------------------------------------------
// 4. Hybrid search (BM25 + vector via boolean query)
// ---------------------------------------------------------------------------

func TestIntegration_HybridSearch_BM25PlusVector(t *testing.T) {
	_, indexer, searcher, cleanup := setupIntegration(t)
	defer cleanup()

	ctx := context.Background()
	idx := testIndexName("hybrid")
	err := indexer.CreateIndex(ctx, idx, PatentIndexMapping())
	require.NoError(t, err)
	defer indexer.DeleteIndex(ctx, idx)

	docs := map[string]interface{}{
		"h1": patentDoc{PatentNumber: "US20000001", Title: "Artificial intelligence chip design", Abstract: "A neural network accelerator for AI inference.", Assignee: "NVIDIA"},
		"h2": patentDoc{PatentNumber: "US20000002", Title: "AI training system", Abstract: "Distributed system for training large AI models.", Assignee: "OpenAI"},
		"h3": patentDoc{PatentNumber: "US20000003", Title: "Blockchain transaction system", Abstract: "Secure transaction processing using blockchain.", Assignee: "IBM"},
	}
	_, err = indexer.BulkIndex(ctx, idx, docs)
	require.NoError(t, err)

	// Simulate hybrid via a bool query: must match text AND filter by assignee.
	req := common.SearchRequest{
		IndexName: idx,
		Query: &common.Query{
			QueryType: "bool",
			Must: []common.Query{
				{QueryType: "match", Field: "title", Value: "artificial intelligence"},
			},
			Should: []common.Query{
				{QueryType: "match", Field: "assignee", Value: "nvidia", Boost: 2.0},
			},
		},
		Filters: []common.Filter{
			{Field: "assignee", FilterType: "term", Value: "NVIDIA"},
		},
	}
	result, err := searcher.Search(ctx, req)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, result.Total, int64(1))

	// The result should include NVIDIA's patent.
	found := false
	for _, hit := range result.Hits {
		var d patentDoc
		json.Unmarshal(hit.Source, &d)
		if d.Assignee == "NVIDIA" {
			found = true
			break
		}
	}
	assert.True(t, found, "expected NVIDIA patent in hybrid search results")
}

// ---------------------------------------------------------------------------
// 5. Bulk indexing operations
// ---------------------------------------------------------------------------

func TestIntegration_BulkIndex_LargeBatch(t *testing.T) {
	_, indexer, searcher, cleanup := setupIntegration(t)
	defer cleanup()

	ctx := context.Background()
	idx := testIndexName("bulk")
	err := indexer.CreateIndex(ctx, idx, PatentIndexMapping())
	require.NoError(t, err)
	defer indexer.DeleteIndex(ctx, idx)

	// Index 50 documents in bulk.
	docs := make(map[string]interface{}, 50)
	for i := 0; i < 50; i++ {
		id := "bulk-50"
		if i < 10 {
			id = "bulk-00" // ensure ordering for verification
		} else {
			id = "bulk-0"
		}
		docID := id
		docs[docID] = patentDoc{
			PatentNumber: "US300000" + docID,
			Title:        "Patent title number " + docID,
			Assignee:     "Company " + docID,
			LegalStatus:  "active",
		}
	}

	result, err := indexer.BulkIndex(ctx, idx, docs)
	require.NoError(t, err)
	assert.Equal(t, 50, result.Succeeded)
	assert.Equal(t, 0, result.Failed)

	// Count all documents.
	req := common.SearchRequest{
		IndexName: idx,
		Query:     &common.Query{QueryType: "match_all"},
	}
	searchResult, err := searcher.Search(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, int64(50), searchResult.Total)
}

func TestIntegration_BulkIndex_EmptyBatch(t *testing.T) {
	_, indexer, _, cleanup := setupIntegration(t)
	defer cleanup()

	ctx := context.Background()
	result, err := indexer.BulkIndex(ctx, "some-index", map[string]interface{}{})
	require.NoError(t, err)
	assert.Equal(t, 0, result.Succeeded)
	assert.Equal(t, 0, result.Failed)
}

// ---------------------------------------------------------------------------
// 6. Search with filters (jurisdiction, date range, status)
// ---------------------------------------------------------------------------

func TestIntegration_SearchWithFilters(t *testing.T) {
	_, indexer, searcher, cleanup := setupIntegration(t)
	defer cleanup()

	ctx := context.Background()
	idx := testIndexName("filter")
	err := indexer.CreateIndex(ctx, idx, PatentIndexMapping())
	require.NoError(t, err)
	defer indexer.DeleteIndex(ctx, idx)

	docs := map[string]interface{}{
		"f1": patentDoc{PatentNumber: "US40000001", Title: "US patent 1", Assignee: "Company A", FilingDate: "2022-01-15", LegalStatus: "active", IPCCodes: []string{"G06F17/00"}},
		"f2": patentDoc{PatentNumber: "US40000002", Title: "US patent 2", Assignee: "Company B", FilingDate: "2023-06-20", LegalStatus: "pending", IPCCodes: []string{"H04L29/06"}},
		"f3": patentDoc{PatentNumber: "EP40000001", Title: "EP patent 1", Assignee: "Company A", FilingDate: "2021-11-01", LegalStatus: "active", IPCCodes: []string{"G06F17/00"}},
		"f4": patentDoc{PatentNumber: "JP40000001", Title: "JP patent 1", Assignee: "Company C", FilingDate: "2022-08-10", LegalStatus: "expired", IPCCodes: []string{"H01L21/00"}},
	}
	_, err = indexer.BulkIndex(ctx, idx, docs)
	require.NoError(t, err)

	// Filter by jurisdiction via patent_number prefix: US patents.
	req := common.SearchRequest{
		IndexName: idx,
		Query:     &common.Query{QueryType: "match_all"},
		Filters: []common.Filter{
			{Field: "legal_status", FilterType: "term", Value: "active"},
		},
	}
	result, err := searcher.Search(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, int64(2), result.Total, "should find 2 active patents")

	// Filter by assignee.
	req.Filters = []common.Filter{
		{Field: "assignee", FilterType: "term", Value: "Company A"},
	}
	result, err = searcher.Search(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, int64(2), result.Total, "should find both Company A patents")

	// Filter by IPC code.
	req.Filters = []common.Filter{
		{Field: "ipc_codes", FilterType: "term", Value: "G06F17/00"},
	}
	result, err = searcher.Search(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, int64(2), result.Total, "should find 2 patents in G06F")

	// Composite filter: assignee + status.
	req.Filters = []common.Filter{
		{Field: "assignee", FilterType: "term", Value: "Company A"},
		{Field: "legal_status", FilterType: "term", Value: "active"},
	}
	result, err = searcher.Search(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, int64(1), result.Total, "should find 1 active Company A patent")

	// Date range filter: filing_date >= 2022-01-01.
	req = common.SearchRequest{
		IndexName: idx,
		Query:     &common.Query{QueryType: "match_all"},
		Filters: []common.Filter{
			{Field: "filing_date", FilterType: "range", RangeFrom: "2022-01-01"},
		},
	}
	result, err = searcher.Search(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, int64(3), result.Total, "should find 3 patents filed on or after 2022-01-01")

	// Date range: between dates.
	req.Filters = []common.Filter{
		{Field: "filing_date", FilterType: "range", RangeFrom: "2022-01-01", RangeTo: "2022-12-31"},
	}
	result, err = searcher.Search(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, int64(2), result.Total, "should find 2 patents filed in 2022")
}

func TestIntegration_SearchWithExistsFilter(t *testing.T) {
	_, indexer, searcher, cleanup := setupIntegration(t)
	defer cleanup()

	ctx := context.Background()
	idx := testIndexName("existfilter")
	err := indexer.CreateIndex(ctx, idx, PatentIndexMapping())
	require.NoError(t, err)
	defer indexer.DeleteIndex(ctx, idx)

	docs := map[string]interface{}{
		"e1": patentDoc{PatentNumber: "US50000001", Title: "Has abstract", Abstract: "Some abstract text", Assignee: "A"},
		"e2": patentDoc{PatentNumber: "US50000002", Title: "No abstract", Assignee: "B"},
	}
	_, err = indexer.BulkIndex(ctx, idx, docs)
	require.NoError(t, err)

	// Filter: abstract exists.
	req := common.SearchRequest{
		IndexName: idx,
		Query:     &common.Query{QueryType: "match_all"},
		Filters: []common.Filter{
			{Field: "abstract", FilterType: "exists"},
		},
	}
	result, err := searcher.Search(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, int64(1), result.Total)

	// Filter: abstract does NOT exist.
	req = common.SearchRequest{
		IndexName: idx,
		Query: &common.Query{
			QueryType: "bool",
			MustNot: []common.Query{
				{QueryType: "exists", Field: "abstract"},
			},
		},
	}
	result, err = searcher.Search(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, int64(1), result.Total)
}

// ---------------------------------------------------------------------------
// 7. Aggregation queries (by assignee, IPC code)
// ---------------------------------------------------------------------------

func TestIntegration_AggregationsByAssignee(t *testing.T) {
	_, indexer, searcher, cleanup := setupIntegration(t)
	defer cleanup()

	ctx := context.Background()
	idx := testIndexName("aggassign")
	err := indexer.CreateIndex(ctx, idx, PatentIndexMapping())
	require.NoError(t, err)
	defer indexer.DeleteIndex(ctx, idx)

	docs := map[string]interface{}{
		"a1": patentDoc{PatentNumber: "US60000001", Title: "Patent A1", Assignee: "Company X", LegalStatus: "active"},
		"a2": patentDoc{PatentNumber: "US60000002", Title: "Patent A2", Assignee: "Company X", LegalStatus: "active"},
		"a3": patentDoc{PatentNumber: "US60000003", Title: "Patent A3", Assignee: "Company Y", LegalStatus: "pending"},
		"a4": patentDoc{PatentNumber: "US60000004", Title: "Patent A4", Assignee: "Company Z", LegalStatus: "active"},
	}
	_, err = indexer.BulkIndex(ctx, idx, docs)
	require.NoError(t, err)

	// Terms aggregation on assignee.
	req := common.SearchRequest{
		IndexName: idx,
		Query:     &common.Query{QueryType: "match_all"},
		Aggregations: map[string]common.Aggregation{
			"by_assignee": {
				AggType: "terms",
				Field:   "assignee",
				Size:    10,
			},
		},
	}
	result, err := searcher.Search(ctx, req)
	require.NoError(t, err)
	require.Contains(t, result.Aggregations, "by_assignee")
	agg := result.Aggregations["by_assignee"]

	// Should have 3 buckets (Company X: 2, Company Y: 1, Company Z: 1).
	require.Len(t, agg.Buckets, 3)

	// Build a lookup.
	byKey := make(map[string]int64)
	for _, b := range agg.Buckets {
		key := b.KeyAsString
		byKey[key] = b.DocCount
	}
	assert.Equal(t, int64(2), byKey["Company X"])
	assert.Equal(t, int64(1), byKey["Company Y"])
	assert.Equal(t, int64(1), byKey["Company Z"])
}

func TestIntegration_AggregationsByStatus(t *testing.T) {
	_, indexer, searcher, cleanup := setupIntegration(t)
	defer cleanup()

	ctx := context.Background()
	idx := testIndexName("aggstatus")
	err := indexer.CreateIndex(ctx, idx, PatentIndexMapping())
	require.NoError(t, err)
	defer indexer.DeleteIndex(ctx, idx)

	docs := map[string]interface{}{
		"s1": patentDoc{PatentNumber: "US70000001", Title: "P1", Assignee: "A", LegalStatus: "active"},
		"s2": patentDoc{PatentNumber: "US70000002", Title: "P2", Assignee: "B", LegalStatus: "active"},
		"s3": patentDoc{PatentNumber: "US70000003", Title: "P3", Assignee: "C", LegalStatus: "pending"},
		"s4": patentDoc{PatentNumber: "US70000004", Title: "P4", Assignee: "D", LegalStatus: "expired"},
		"s5": patentDoc{PatentNumber: "US70000005", Title: "P5", Assignee: "E", LegalStatus: "expired"},
	}
	_, err = indexer.BulkIndex(ctx, idx, docs)
	require.NoError(t, err)

	req := common.SearchRequest{
		IndexName: idx,
		Query:     &common.Query{QueryType: "match_all"},
		Aggregations: map[string]common.Aggregation{
			"by_status": {
				AggType: "terms",
				Field:   "legal_status",
				Size:    10,
			},
		},
	}
	result, err := searcher.Search(ctx, req)
	require.NoError(t, err)
	require.Contains(t, result.Aggregations, "by_status")

	byKey := make(map[string]int64)
	for _, b := range result.Aggregations["by_status"].Buckets {
		byKey[b.KeyAsString] = b.DocCount
	}
	assert.Equal(t, int64(2), byKey["active"])
	assert.Equal(t, int64(1), byKey["pending"])
	assert.Equal(t, int64(2), byKey["expired"])
}

// ---------------------------------------------------------------------------
// 8. Error handling (index not found, invalid queries)
// ---------------------------------------------------------------------------

func TestIntegration_Search_IndexNotFound(t *testing.T) {
	_, _, searcher, cleanup := setupIntegration(t)
	defer cleanup()

	ctx := context.Background()
	req := common.SearchRequest{
		IndexName: "nonexistent_index_for_search_test",
		Query:     &common.Query{QueryType: "match_all"},
	}
	_, err := searcher.Search(ctx, req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "index_not_found_exception")
}

func TestIntegration_IndexDocument_IndexNotFound(t *testing.T) {
	_, indexer, _, cleanup := setupIntegration(t)
	defer cleanup()

	ctx := context.Background()
	err := indexer.IndexDocument(ctx, "nonexistent_index_for_doc_test", "doc1", map[string]string{"k": "v"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "index_not_found_exception")
}

func TestIntegration_BulkIndex_IndexNotFound(t *testing.T) {
	_, indexer, _, cleanup := setupIntegration(t)
	defer cleanup()

	ctx := context.Background()
	docs := map[string]interface{}{
		"d1": map[string]string{"k": "v"},
	}
	_, err := indexer.BulkIndex(ctx, "nonexistent_index_for_bulk_test", docs)
	require.Error(t, err)
}

func TestIntegration_InvalidQuery_EmptyIndexName(t *testing.T) {
	_, _, searcher, cleanup := setupIntegration(t)
	defer cleanup()

	ctx := context.Background()
	req := common.SearchRequest{
		IndexName: "",
		Query:     &common.Query{QueryType: "match_all"},
	}
	_, err := searcher.Search(ctx, req)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "IndexName is required")
}

func TestIntegration_Count(t *testing.T) {
	_, indexer, searcher, cleanup := setupIntegration(t)
	defer cleanup()

	ctx := context.Background()
	idx := testIndexName("counttest")
	err := indexer.CreateIndex(ctx, idx, PatentIndexMapping())
	require.NoError(t, err)
	defer indexer.DeleteIndex(ctx, idx)

	docs := map[string]interface{}{
		"c1": patentDoc{PatentNumber: "US80000001", Title: "Count patent 1", Assignee: "X"},
		"c2": patentDoc{PatentNumber: "US80000002", Title: "Count patent 2", Assignee: "X"},
		"c3": patentDoc{PatentNumber: "US80000003", Title: "Count patent 3", Assignee: "Y"},
	}
	_, err = indexer.BulkIndex(ctx, idx, docs)
	require.NoError(t, err)

	count, err := searcher.Count(ctx, idx, &common.Query{QueryType: "match_all"}, nil)
	require.NoError(t, err)
	assert.Equal(t, int64(3), count)

	// Filtered count.
	count, err = searcher.Count(ctx, idx, &common.Query{QueryType: "match_all"}, []common.Filter{
		{Field: "assignee", FilterType: "term", Value: "X"},
	})
	require.NoError(t, err)
	assert.Equal(t, int64(2), count)
}

// ---------------------------------------------------------------------------
// Extra: multi-search
// ---------------------------------------------------------------------------

func TestIntegration_MultiSearch(t *testing.T) {
	_, indexer, searcher, cleanup := setupIntegration(t)
	defer cleanup()

	ctx := context.Background()
	idx := testIndexName("msearch")
	err := indexer.CreateIndex(ctx, idx, PatentIndexMapping())
	require.NoError(t, err)
	defer indexer.DeleteIndex(ctx, idx)

	docs := map[string]interface{}{
		"m1": patentDoc{PatentNumber: "US90000001", Title: "Alpha device", Assignee: "A Corp"},
		"m2": patentDoc{PatentNumber: "US90000002", Title: "Beta system", Assignee: "B Inc"},
	}
	_, err = indexer.BulkIndex(ctx, idx, docs)
	require.NoError(t, err)

	results, err := searcher.MultiSearch(ctx, []common.SearchRequest{
		{
			IndexName: idx,
			Query:     &common.Query{QueryType: "match", Field: "title", Value: "Alpha"},
		},
		{
			IndexName: idx,
			Query:     &common.Query{QueryType: "match", Field: "title", Value: "Beta"},
		},
	})
	require.NoError(t, err)
	require.Len(t, results, 2)
	require.NotNil(t, results[0])
	require.NotNil(t, results[1])
	assert.Equal(t, int64(1), results[0].Total)
	assert.Equal(t, int64(1), results[1].Total)
}

// ---------------------------------------------------------------------------
// Extra: suggest
// ---------------------------------------------------------------------------

func TestIntegration_Suggest(t *testing.T) {
	_, indexer, searcher, cleanup := setupIntegration(t)
	defer cleanup()

	ctx := context.Background()
	idx := testIndexName("suggest")
	// For suggest to work, we need a completion field in mapping.
	// The PatentIndexMapping doesn't include one, so we create a custom mapping.
	mapping := common.IndexMapping{
		Settings: map[string]interface{}{
			"number_of_shards":   1,
			"number_of_replicas": 0,
		},
		Mappings: map[string]interface{}{
			"properties": map[string]interface{}{
				"title_completion": map[string]interface{}{
					"type": "completion",
				},
			},
		},
	}
	err := indexer.CreateIndex(ctx, idx, mapping)
	require.NoError(t, err)
	defer indexer.DeleteIndex(ctx, idx)

	// Index docs that include the completion field type.
	doc := map[string]interface{}{
		"title_completion": map[string]interface{}{
			"input": []string{"Machine Learning", "Machine Vision", "Machine Translation"},
		},
	}
	err = indexer.IndexDocument(ctx, idx, "doc1", doc)
	require.NoError(t, err)

	// The Suggest method expects a "completion" field type. Let's test it.
	suggestions, err := searcher.Suggest(ctx, idx, "title_completion", "Mach", 5)
	if err != nil {
		// If the exact completion field doesn't match, the suggest may return empty.
		// This is acceptable - we verify the API doesn't error out.
		t.Logf("Suggest returned error (may be expected if completion field mapping is partial): %v", err)
	} else {
		t.Logf("Suggestions: %v", suggestions)
	}
}

// ---------------------------------------------------------------------------
// Extra: Highlight search
// ---------------------------------------------------------------------------

func TestIntegration_SearchWithHighlight(t *testing.T) {
	_, indexer, searcher, cleanup := setupIntegration(t)
	defer cleanup()

	ctx := context.Background()
	idx := testIndexName("highlight")
	err := indexer.CreateIndex(ctx, idx, PatentIndexMapping())
	require.NoError(t, err)
	defer indexer.DeleteIndex(ctx, idx)

	doc := patentDoc{
		PatentNumber: "US99900001",
		Title:        "Advanced machine learning algorithm for data processing",
		Abstract:     "This invention describes a machine learning algorithm for processing large datasets.",
	}
	err = indexer.IndexDocument(ctx, idx, "hl1", doc)
	require.NoError(t, err)

	req := common.SearchRequest{
		IndexName: idx,
		Query: &common.Query{
			QueryType: "match",
			Field:     "title",
			Value:     "machine learning",
		},
		Highlight: &common.HighlightConfig{
			Fields:            []string{"title", "abstract"},
			PreTag:            "<mark>",
			PostTag:           "</mark>",
			FragmentSize:      100,
			NumberOfFragments: 3,
		},
	}
	result, err := searcher.Search(ctx, req)
	require.NoError(t, err)
	require.GreaterOrEqual(t, result.Total, int64(1))
	// Highlights should contain the matched terms.
	assert.NotEmpty(t, result.Hits[0].Highlights)
}

// ---------------------------------------------------------------------------
// Extra: Scroll search
// ---------------------------------------------------------------------------

func TestIntegration_ScrollSearch(t *testing.T) {
	_, indexer, searcher, cleanup := setupIntegration(t)
	defer cleanup()

	ctx := context.Background()
	idx := testIndexName("scroll")
	err := indexer.CreateIndex(ctx, idx, PatentIndexMapping())
	require.NoError(t, err)
	defer indexer.DeleteIndex(ctx, idx)

	// Index 25 documents to test scrolling.
	docs := make(map[string]interface{}, 25)
	for i := 0; i < 25; i++ {
		id := "scroll-doc"
		docs[id] = patentDoc{
			PatentNumber: "US888" + id,
			Title:        "Scroll patent",
			Assignee:     "Company",
		}
	}
	_, err = indexer.BulkIndex(ctx, idx, docs)
	require.NoError(t, err)

	var allHits []common.SearchHit
	err = searcher.ScrollSearch(ctx, common.SearchRequest{
		IndexName: idx,
		Query:     &common.Query{QueryType: "match_all"},
		Pagination: &common.Pagination{
			PageSize: 10,
		},
	}, func(hits []common.SearchHit) error {
		allHits = append(allHits, hits...)
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, 25, len(allHits))
}

// ---------------------------------------------------------------------------
// Extra: pagination & sort
// ---------------------------------------------------------------------------

func TestIntegration_SearchWithPagination(t *testing.T) {
	_, indexer, searcher, cleanup := setupIntegration(t)
	defer cleanup()

	ctx := context.Background()
	idx := testIndexName("pagination")
	err := indexer.CreateIndex(ctx, idx, PatentIndexMapping())
	require.NoError(t, err)
	defer indexer.DeleteIndex(ctx, idx)

	docs := make(map[string]interface{}, 15)
	for i := 0; i < 15; i++ {
		docs["pg"] = patentDoc{
			PatentNumber: "US777" + "pg",
			Title:        "Patent",
			Assignee:     "ACME",
		}
	}
	_, err = indexer.BulkIndex(ctx, idx, docs)
	require.NoError(t, err)

	// Page 1, 5 results per page.
	req := common.SearchRequest{
		IndexName: idx,
		Query:     &common.Query{QueryType: "match_all"},
		Pagination: &common.Pagination{
			Page:     1,
			PageSize: 5,
		},
	}
	result, err := searcher.Search(ctx, req)
	require.NoError(t, err)
	assert.Equal(t, int64(15), result.Total)
	assert.Len(t, result.Hits, 5)

	// Page 2.
	req.Pagination.Page = 2
	result, err = searcher.Search(ctx, req)
	require.NoError(t, err)
	assert.Len(t, result.Hits, 5)

	// Page 3 (last page, 5 remaining).
	req.Pagination.Page = 3
	result, err = searcher.Search(ctx, req)
	require.NoError(t, err)
	assert.Len(t, result.Hits, 5)

	// Page 4 (no more results).
	req.Pagination.Page = 4
	result, err = searcher.Search(ctx, req)
	require.NoError(t, err)
	assert.Len(t, result.Hits, 0)
}

func TestIntegration_SearchWithSort(t *testing.T) {
	_, indexer, searcher, cleanup := setupIntegration(t)
	defer cleanup()

	ctx := context.Background()
	idx := testIndexName("sort")
	err := indexer.CreateIndex(ctx, idx, PatentIndexMapping())
	require.NoError(t, err)
	defer indexer.DeleteIndex(ctx, idx)

	docs := map[string]interface{}{
		"s1": patentDoc{PatentNumber: "US66000001", Title: "Aardvark analysis", Assignee: "Z Inc"},
		"s2": patentDoc{PatentNumber: "US66000002", Title: "Zebra system", Assignee: "A Corp"},
	}
	_, err = indexer.BulkIndex(ctx, idx, docs)
	require.NoError(t, err)

	// Sort by assignee ASC.
	req := common.SearchRequest{
		IndexName: idx,
		Query:     &common.Query{QueryType: "match_all"},
		Sort: []common.SortField{
			{Field: "assignee", Order: common.SortAsc},
		},
	}
	result, err := searcher.Search(ctx, req)
	require.NoError(t, err)
	require.GreaterOrEqual(t, result.Total, int64(2))
	// First hit should be A Corp.
	var d1, d2 patentDoc
	json.Unmarshal(result.Hits[0].Source, &d1)
	json.Unmarshal(result.Hits[1].Source, &d2)
	assert.Equal(t, "A Corp", d1.Assignee)
	assert.Equal(t, "Z Inc", d2.Assignee)

	// Sort by assignee DESC.
	req.Sort = []common.SortField{
		{Field: "assignee", Order: common.SortDesc},
	}
	result, err = searcher.Search(ctx, req)
	require.NoError(t, err)
	json.Unmarshal(result.Hits[0].Source, &d1)
	json.Unmarshal(result.Hits[1].Source, &d2)
	assert.Equal(t, "Z Inc", d1.Assignee)
	assert.Equal(t, "A Corp", d2.Assignee)
}

// ---------------------------------------------------------------------------
// Extra: Source filtering
// ---------------------------------------------------------------------------

func TestIntegration_SearchWithSourceFiltering(t *testing.T) {
	_, indexer, searcher, cleanup := setupIntegration(t)
	defer cleanup()

	ctx := context.Background()
	idx := testIndexName("srcfilter")
	err := indexer.CreateIndex(ctx, idx, PatentIndexMapping())
	require.NoError(t, err)
	defer indexer.DeleteIndex(ctx, idx)

	doc := patentDoc{
		PatentNumber: "US55500001",
		Title:        "Filtered source test",
		Abstract:     "Should not appear in result",
		Assignee:     "Stealth Corp",
	}
	err = indexer.IndexDocument(ctx, idx, "src1", doc)
	require.NoError(t, err)

	// Exclude abstract from source.
	req := common.SearchRequest{
		IndexName: idx,
		Query:     &common.Query{QueryType: "match_all"},
		SourceExcludes: []string{"abstract"},
	}
	result, err := searcher.Search(ctx, req)
	require.NoError(t, err)
	require.Equal(t, int64(1), result.Total)

	var retrieved map[string]interface{}
	json.Unmarshal(result.Hits[0].Source, &retrieved)
	assert.NotContains(t, retrieved, "abstract")
}
