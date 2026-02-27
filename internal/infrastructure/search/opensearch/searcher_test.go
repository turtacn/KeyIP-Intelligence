package opensearch

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	opensearchgo "github.com/opensearch-project/opensearch-go/v2"
	"github.com/stretchr/testify/assert"
	"github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
)

func newTestSearcher(serverURL string) *Searcher {
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

	searchCfg := SearcherConfig{
		DefaultPageSize: 10,
		MaxPageSize:     100,
	}
	return NewSearcher(c, searchCfg, newMockLogger())
}

func TestSearch_SimpleMatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && strings.Contains(r.URL.Path, "_search") {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{
				"took": 10,
				"hits": {
					"total": {"value": 1},
					"max_score": 1.0,
					"hits": [
						{"_id": "1", "_score": 1.0, "_source": {"title": "test"}}
					]
				}
			}`))
			return
		}
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer server.Close()

	searcher := newTestSearcher(server.URL)
	req := common.SearchRequest{
		IndexName: "test-index",
		Query: &common.Query{
			QueryType: "match",
			Field:     "title",
			Value:     "test",
		},
	}
	result, err := searcher.Search(context.Background(), req)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), result.Total)
	assert.Len(t, result.Hits, 1)
	assert.Equal(t, "1", result.Hits[0].ID)
}

func TestSearch_WithAggregations(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"took": 10,
			"hits": {"total": {"value": 0}, "hits": []},
			"aggregations": {
				"types": {
					"buckets": [
						{"key": "A", "doc_count": 10}
					]
				}
			}
		}`))
	}))
	defer server.Close()

	searcher := newTestSearcher(server.URL)
	req := common.SearchRequest{
		IndexName: "test-index",
		Aggregations: map[string]common.Aggregation{
			"types": {
				AggType: "terms",
				Field:   "type",
			},
		},
	}
	result, err := searcher.Search(context.Background(), req)
	assert.NoError(t, err)
	assert.Contains(t, result.Aggregations, "types")
	assert.Len(t, result.Aggregations["types"].Buckets, 1)
	assert.Equal(t, "A", result.Aggregations["types"].Buckets[0].Key)
}

func TestScrollSearch_Success(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.WriteHeader(http.StatusOK)
		if strings.Contains(r.URL.Path, "_search") && !strings.Contains(r.URL.Path, "scroll") {
			// Initial search
			w.Write([]byte(`{
				"_scroll_id": "scroll1",
				"hits": {
					"hits": [{"_id": "1"}]
				}
			}`))
		} else if strings.Contains(r.URL.Path, "scroll") && r.Method != "DELETE" {
			// Next batch
			if requests == 2 {
				w.Write([]byte(`{
					"_scroll_id": "scroll1",
					"hits": {
						"hits": [{"_id": "2"}]
					}
				}`))
			} else {
				// Done
				w.Write([]byte(`{
					"_scroll_id": "scroll1",
					"hits": {
						"hits": []
					}
				}`))
			}
		} else if r.Method == "DELETE" {
			w.Write([]byte(`{"succeeded": true}`))
		}
	}))
	defer server.Close()

	searcher := newTestSearcher(server.URL)
	count := 0
	err := searcher.ScrollSearch(context.Background(), common.SearchRequest{IndexName: "test"}, func(hits []common.SearchHit) error {
		count += len(hits)
		return nil
	})
	assert.NoError(t, err)
	assert.Equal(t, 2, count)
}

func TestSuggest_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"suggest": {
				"my-suggest": [
					{
						"text": "t",
						"options": [{"text": "test"}]
					}
				]
			}
		}`))
	}))
	defer server.Close()

	searcher := newTestSearcher(server.URL)
	suggestions, err := searcher.Suggest(context.Background(), "test", "title", "t", 5)
	assert.NoError(t, err)
	assert.Len(t, suggestions, 1)
	assert.Equal(t, "test", suggestions[0])
}
