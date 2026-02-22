package milvus

import (
	"context"
	"testing"

	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
	"github.com/stretchr/testify/assert"
)

func newTestSearcher(mock client.Client) *Searcher {
	c := &Client{
		milvusClient: mock,
		logger:       newMockLogger(),
	}
	// We need CollectionManager for Insert (to describe collection)
	// Mock collection manager
	// We can pass a real CollectionManager with mocked client.
	cm := NewCollectionManager(c, CollectionConfig{}, newMockLogger())

	cfg := SearcherConfig{
		DefaultTopK: 10,
	}
	return NewSearcher(c, cm, cfg, newMockLogger())
}

// Extend mockCollectionClient to support Search/Insert
type mockSearchClient struct {
	mockCollectionClient // Embed previous mock capabilities

	insertFunc func(ctx context.Context, collName, partitionName string, columns ...entity.Column) (entity.Column, error)
	upsertFunc func(ctx context.Context, collName, partitionName string, columns ...entity.Column) (entity.Column, error)
	searchFunc func(ctx context.Context, collName string, partitions []string, expr string, outputFields []string, vectors []entity.Vector, vectorField string, metricType entity.MetricType, topK int, sp entity.SearchParam, opts ...client.SearchQueryOptionFunc) ([]client.SearchResult, error)
	queryByPksFunc func(ctx context.Context, collName string, partitions []string, ids entity.Column, outputFields []string, opts ...client.SearchQueryOptionFunc) (client.ResultSet, error)
	deleteFunc func(ctx context.Context, collName, partitionName string, expr string) error
}

func (m *mockSearchClient) Insert(ctx context.Context, collName, partitionName string, columns ...entity.Column) (entity.Column, error) {
	if m.insertFunc != nil {
		return m.insertFunc(ctx, collName, partitionName, columns...)
	}
	return entity.NewColumnInt64("id", []int64{1}), nil
}

func (m *mockSearchClient) Upsert(ctx context.Context, collName, partitionName string, columns ...entity.Column) (entity.Column, error) {
	if m.upsertFunc != nil {
		return m.upsertFunc(ctx, collName, partitionName, columns...)
	}
	return entity.NewColumnInt64("id", []int64{1}), nil
}

func (m *mockSearchClient) Search(ctx context.Context, collName string, partitions []string, expr string, outputFields []string, vectors []entity.Vector, vectorField string, metricType entity.MetricType, topK int, sp entity.SearchParam, opts ...client.SearchQueryOptionFunc) ([]client.SearchResult, error) {
	if m.searchFunc != nil {
		return m.searchFunc(ctx, collName, partitions, expr, outputFields, vectors, vectorField, metricType, topK, sp, opts...)
	}
	return []client.SearchResult{}, nil
}

func (m *mockSearchClient) QueryByPks(ctx context.Context, collName string, partitions []string, ids entity.Column, outputFields []string, opts ...client.SearchQueryOptionFunc) (client.ResultSet, error) {
	if m.queryByPksFunc != nil {
		return m.queryByPksFunc(ctx, collName, partitions, ids, outputFields, opts...)
	}
	return nil, nil // ResultSet is interface or struct? It's interface.
}

func (m *mockSearchClient) Delete(ctx context.Context, collName, partitionName string, expr string) error {
	if m.deleteFunc != nil {
		return m.deleteFunc(ctx, collName, partitionName, expr)
	}
	return nil
}

func TestInsert_Success(t *testing.T) {
	mock := &mockSearchClient{
		mockCollectionClient: mockCollectionClient{
			describeCollectionFunc: func(ctx context.Context, name string) (*entity.Collection, error) {
				return &entity.Collection{
					Name: name,
					Schema: &entity.Schema{
						Fields: []*entity.Field{
							{Name: "id", DataType: entity.FieldTypeInt64, AutoID: true},
							{Name: "vec", DataType: entity.FieldTypeFloatVector},
						},
					},
				}, nil
			},
		},
		insertFunc: func(ctx context.Context, collName, partitionName string, columns ...entity.Column) (entity.Column, error) {
			assert.Equal(t, "test", collName)
			return entity.NewColumnInt64("id", []int64{1}), nil
		},
	}

	s := newTestSearcher(mock)
	req := InsertRequest{
		CollectionName: "test",
		Data: []map[string]interface{}{
			{"vec": []float32{0.1, 0.2}},
		},
	}
	res, err := s.Insert(context.Background(), req)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), res.InsertedCount)
}

func TestSearch_Success(t *testing.T) {
	mock := &mockSearchClient{
		searchFunc: func(ctx context.Context, collName string, partitions []string, expr string, outputFields []string, vectors []entity.Vector, vectorField string, metricType entity.MetricType, topK int, sp entity.SearchParam, opts ...client.SearchQueryOptionFunc) ([]client.SearchResult, error) {
			return []client.SearchResult{
				{
					ResultCount: 1,
					IDs:         entity.NewColumnInt64("id", []int64{1}),
					Scores:      []float32{0.99},
				},
			}, nil
		},
	}
	s := newTestSearcher(mock)
	req := VectorSearchRequest{
		CollectionName:  "test",
		VectorFieldName: "vec",
		Vectors:         [][]float32{{0.1, 0.2}},
		TopK:            10,
	}
	res, err := s.Search(context.Background(), req)
	assert.NoError(t, err)
	assert.Len(t, res.Results, 1) // 1 query
	assert.Len(t, res.Results[0], 1) // 1 hit
	assert.Equal(t, int64(1), res.Results[0][0].ID)
}

func TestHybridSearch_RRF(t *testing.T) {
	// Mock 2 calls
	callCount := 0
	mock := &mockSearchClient{
		searchFunc: func(ctx context.Context, collName string, partitions []string, expr string, outputFields []string, vectors []entity.Vector, vectorField string, metricType entity.MetricType, topK int, sp entity.SearchParam, opts ...client.SearchQueryOptionFunc) ([]client.SearchResult, error) {
			callCount++
			if callCount == 1 {
				return []client.SearchResult{{
					ResultCount: 2,
					IDs:         entity.NewColumnInt64("id", []int64{1, 2}),
					Scores:      []float32{0.9, 0.8},
				}}, nil
			}
			return []client.SearchResult{{
				ResultCount: 2,
				IDs:         entity.NewColumnInt64("id", []int64{2, 3}),
				Scores:      []float32{0.85, 0.7},
			}}, nil
		},
	}
	s := newTestSearcher(mock)

	req1 := VectorSearchRequest{CollectionName: "test", VectorFieldName: "vec1", Vectors: [][]float32{{0.1}}}
	req2 := VectorSearchRequest{CollectionName: "test", VectorFieldName: "vec2", Vectors: [][]float32{{0.2}}}

	res, err := s.HybridSearch(context.Background(), "test", []VectorSearchRequest{req1, req2}, &RRFReranker{K: 60}, 10)
	assert.NoError(t, err)
	// ID 2 should be top because it appears in both (rank 2 and rank 1)
	// Score(2) = 1/(60+2) + 1/(60+1) = 1/62 + 1/61 ~= 0.0161 + 0.0164 = 0.0325
	// Score(1) = 1/(60+1) = 0.0164
	// Score(3) = 1/(60+2) = 0.0161

	assert.Equal(t, int64(2), res.Results[0][0].ID)
	assert.Equal(t, int64(1), res.Results[0][1].ID)
	assert.Equal(t, int64(3), res.Results[0][2].ID)
}

//Personal.AI order the ending
