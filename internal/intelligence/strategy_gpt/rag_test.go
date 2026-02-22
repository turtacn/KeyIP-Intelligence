package strategy_gpt

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/internal/intelligence/common"
)

// Mocks
type MockVectorStore struct {
	mock.Mock
}

func (m *MockVectorStore) Search(ctx context.Context, vector []float32, topK int, filters map[string]interface{}) ([]*VectorSearchResult, error) {
	args := m.Called(ctx, vector, topK, filters)
	return args.Get(0).([]*VectorSearchResult), args.Error(1)
}

func (m *MockVectorStore) Insert(ctx context.Context, id string, vector []float32, metadata map[string]interface{}) error {
	args := m.Called(ctx, id, vector, metadata)
	return args.Error(0)
}

func (m *MockVectorStore) Delete(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockVectorStore) BatchInsert(ctx context.Context, items []*VectorInsertItem) error {
	args := m.Called(ctx, items)
	return args.Error(0)
}

type MockTextEmbedder struct {
	mock.Mock
}

func (m *MockTextEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	args := m.Called(ctx, text)
	return args.Get(0).([]float32), args.Error(1)
}

func (m *MockTextEmbedder) BatchEmbed(ctx context.Context, texts []string) ([][]float32, error) {
	args := m.Called(ctx, texts)
	return args.Get(0).([][]float32), args.Error(1)
}

type MockReranker struct {
	mock.Mock
}

func (m *MockReranker) Rerank(ctx context.Context, query string, documents []string, topK int) ([]*RerankResult, error) {
	args := m.Called(ctx, query, documents, topK)
	return args.Get(0).([]*RerankResult), args.Error(1)
}

type MockDocumentChunker struct {
	mock.Mock
}

func (m *MockDocumentChunker) Chunk(doc *Document) ([]*DocumentChunk, error) {
	args := m.Called(doc)
	return args.Get(0).([]*DocumentChunk), args.Error(1)
}

type RAGMockLogger struct {
	logging.Logger
	mock.Mock
}

func (m *RAGMockLogger) Error(msg string, fields ...logging.Field) {
	m.Called(msg, fields)
}

func TestRetrieve_Success(t *testing.T) {
	vs := new(MockVectorStore)
	embedder := new(MockTextEmbedder)
	reranker := new(MockReranker)
	chunker := new(MockDocumentChunker)
	metrics := common.NewNoopIntelligenceMetrics()
	logger := new(RAGMockLogger)

	config := RAGConfig{TopK: 5, SimilarityThreshold: 0.5}
	engine := NewRAGEngine(vs, embedder, reranker, chunker, config, metrics, logger)

	embedder.On("Embed", mock.Anything, "query").Return([]float32{0.1, 0.2}, nil)
	vs.On("Search", mock.Anything, mock.Anything, 5, mock.Anything).Return([]*VectorSearchResult{
		{ID: "1", Score: 0.9, Metadata: map[string]interface{}{"content": "doc1", "source": "s1"}},
		{ID: "2", Score: 0.4, Metadata: map[string]interface{}{"content": "doc2"}}, // Should be filtered
	}, nil)

	res, err := engine.Retrieve(context.Background(), &RAGQuery{QueryText: "query", SimilarityThreshold: 0.5})
	assert.NoError(t, err)
	assert.Equal(t, 1, len(res.Chunks))
	assert.Equal(t, "doc1", res.Chunks[0].Content)
}

func TestRetrieveAndRerank_Success(t *testing.T) {
	vs := new(MockVectorStore)
	embedder := new(MockTextEmbedder)
	reranker := new(MockReranker)
	chunker := new(MockDocumentChunker)
	metrics := common.NewNoopIntelligenceMetrics()
	logger := new(RAGMockLogger)

	config := RAGConfig{TopK: 2, SimilarityThreshold: 0.5, RerankerEnabled: true}
	engine := NewRAGEngine(vs, embedder, reranker, chunker, config, metrics, logger)

	embedder.On("Embed", mock.Anything, "query").Return([]float32{0.1, 0.2}, nil)

	// Returns 3 results (original K * 3 candidates logic applied inside RetrieveAndRerank for TopK override,
	// but Retrieve takes query.TopK. Inside RetrieveAndRerank it sets query.TopK = config.TopK * 3 = 6)
	// Mock expects 6
	vs.On("Search", mock.Anything, mock.Anything, 6, mock.Anything).Return([]*VectorSearchResult{
		{ID: "1", Score: 0.8, Metadata: map[string]interface{}{"content": "doc1"}},
		{ID: "2", Score: 0.7, Metadata: map[string]interface{}{"content": "doc2"}},
		{ID: "3", Score: 0.6, Metadata: map[string]interface{}{"content": "doc3"}},
	}, nil)

	reranker.On("Rerank", mock.Anything, "query", mock.Anything, 2).Return([]*RerankResult{
		{Index: 2, Score: 0.95}, // doc3 best
		{Index: 0, Score: 0.90}, // doc1 second
	}, nil)

	res, err := engine.RetrieveAndRerank(context.Background(), &RAGQuery{QueryText: "query"})
	assert.NoError(t, err)
	assert.Equal(t, 2, len(res.Chunks))
	assert.Equal(t, "doc3", res.Chunks[0].Content) // Reranked first
	assert.True(t, res.RerankerApplied)
}

func TestBuildContext(t *testing.T) {
	engine := NewRAGEngine(nil, nil, nil, nil, RAGConfig{}, nil, nil)
	chunks := []*RAGChunk{
		{Content: "Short", Source: "S1", Score: 0.9},
		{Content: "Longer content", Source: "S2", Score: 0.8},
	}

	ctx, err := engine.BuildContext(context.Background(), &RAGResult{Chunks: chunks}, 100)
	assert.NoError(t, err)
	assert.Contains(t, ctx, "[S1]: Short")
	assert.Contains(t, ctx, "[S2]: Longer content")
}

func TestIndexDocument_Success(t *testing.T) {
	vs := new(MockVectorStore)
	embedder := new(MockTextEmbedder)
	chunker := new(MockDocumentChunker)
	engine := NewRAGEngine(vs, embedder, nil, chunker, RAGConfig{}, nil, nil)

	doc := &Document{DocumentID: "d1", Content: "text"}
	chunks := []*DocumentChunk{{ID: "c1", Content: "text"}}

	chunker.On("Chunk", doc).Return(chunks, nil)
	embedder.On("BatchEmbed", mock.Anything, []string{"text"}).Return([][]float32{{0.1}}, nil)
	vs.On("BatchInsert", mock.Anything, mock.Anything).Return(nil)

	err := engine.IndexDocument(context.Background(), doc)
	assert.NoError(t, err)
}
//Personal.AI order the ending
