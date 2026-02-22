package strategy_gpt

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/internal/intelligence/common"
	types "github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
)

// DocumentSourceType defines the source type of a document.
type DocumentSourceType string

const (
	SourcePatent                DocumentSourceType = "Patent"
	SourceCaseLaw               DocumentSourceType = "CaseLaw"
	SourceExaminationGuideline  DocumentSourceType = "ExaminationGuideline"
	SourceScientificPaper       DocumentSourceType = "ScientificPaper"
	SourceRegulatory            DocumentSourceType = "Regulatory"
)

// RAGEngine defines the interface for RAG operations.
type RAGEngine interface {
	Retrieve(ctx context.Context, query *RAGQuery) (*RAGResult, error)
	RetrieveAndRerank(ctx context.Context, query *RAGQuery) (*RAGResult, error)
	BuildContext(ctx context.Context, result *RAGResult, budget int) (string, error)
	IndexDocument(ctx context.Context, doc *Document) error
	IndexBatch(ctx context.Context, docs []*Document) error
	DeleteDocument(ctx context.Context, docID string) error
}

// RAGQuery represents a query for RAG.
type RAGQuery struct {
	QueryText           string
	QueryEmbedding      []float32
	Filters             *RAGFilters
	TopK                int
	SimilarityThreshold float64
	SourceTypes         []DocumentSourceType
}

// RAGFilters defines filters for retrieval.
type RAGFilters struct {
	DateRange             *types.DateRange
	Jurisdictions         []string
	PatentClassifications []string
	Assignees             []string
	ExcludeDocIDs         []string
}

// RAGResult represents the result of retrieval.
type RAGResult struct {
	Chunks          []*RAGChunk
	TotalFound      int
	SearchLatencyMs int64
	RerankerApplied bool
}

// Document represents a document to be indexed.
type Document struct {
	DocumentID string
	Title      string
	Content    string
	Source     DocumentSourceType
	Metadata   map[string]string
	Language   string
}

// VectorStore defines the interface for vector database interaction.
type VectorStore interface {
	Search(ctx context.Context, vector []float32, topK int, filters map[string]interface{}) ([]*VectorSearchResult, error)
	Insert(ctx context.Context, id string, vector []float32, metadata map[string]interface{}) error
	Delete(ctx context.Context, id string) error
	BatchInsert(ctx context.Context, items []*VectorInsertItem) error
}

// TextEmbedder defines the interface for text embedding.
type TextEmbedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
	BatchEmbed(ctx context.Context, texts []string) ([][]float32, error)
}

// Reranker defines the interface for reranking.
type Reranker interface {
	Rerank(ctx context.Context, query string, documents []string, topK int) ([]*RerankResult, error)
}

// DocumentChunker defines the interface for document chunking.
type DocumentChunker interface {
	Chunk(doc *Document) ([]*DocumentChunk, error)
}

// Helper types
type VectorSearchResult struct {
	ID       string
	Score    float64
	Metadata map[string]interface{}
}

type VectorInsertItem struct {
	ID       string
	Vector   []float32
	Metadata map[string]interface{}
}

type RerankResult struct {
	Index int
	Score float64
}

type DocumentChunk struct {
	ID      string
	Content string
}

// ragEngineImpl implements RAGEngine.
type ragEngineImpl struct {
	vectorStore VectorStore
	embedder    TextEmbedder
	reranker    Reranker
	chunker     DocumentChunker
	config      RAGConfig
	metrics     common.IntelligenceMetrics
	logger      logging.Logger
}

// NewRAGEngine creates a new RAGEngine.
func NewRAGEngine(
	vs VectorStore,
	embedder TextEmbedder,
	reranker Reranker,
	chunker DocumentChunker,
	config RAGConfig,
	metrics common.IntelligenceMetrics,
	logger logging.Logger,
) RAGEngine {
	return &ragEngineImpl{
		vectorStore: vs,
		embedder:    embedder,
		reranker:    reranker,
		chunker:     chunker,
		config:      config,
		metrics:     metrics,
		logger:      logger,
	}
}

func (e *ragEngineImpl) Retrieve(ctx context.Context, query *RAGQuery) (*RAGResult, error) {
	start := time.Now()

	vector := query.QueryEmbedding
	if len(vector) == 0 {
		var err error
		vector, err = e.embedder.Embed(ctx, query.QueryText)
		if err != nil {
			return nil, err
		}
	}

	// Convert filters to map (simplified)
	filtersMap := make(map[string]interface{})
	// ... mapping logic

	topK := query.TopK
	if topK == 0 {
		topK = e.config.TopK
	}

	results, err := e.vectorStore.Search(ctx, vector, topK, filtersMap)
	if err != nil {
		return nil, err
	}

	var chunks []*RAGChunk
	for _, res := range results {
		if res.Score < query.SimilarityThreshold {
			continue
		}

		// Map metadata to string map
		meta := make(map[string]string)
		for k, v := range res.Metadata {
			if s, ok := v.(string); ok {
				meta[k] = s
			}
		}

		chunks = append(chunks, &RAGChunk{
			ChunkID:    res.ID,
			DocumentID: meta["document_id"],
			Content:    meta["content"], // Assuming content stored in metadata
			Score:      res.Score,
			Source:     meta["source"],
			Metadata:   meta,
		})
	}

	return &RAGResult{
		Chunks:          chunks,
		TotalFound:      len(chunks),
		SearchLatencyMs: time.Since(start).Milliseconds(),
		RerankerApplied: false,
	}, nil
}

func (e *ragEngineImpl) RetrieveAndRerank(ctx context.Context, query *RAGQuery) (*RAGResult, error) {
	// First retrieve with larger K
	originalK := query.TopK
	if originalK == 0 {
		originalK = e.config.TopK
	}

	// Fetch 3x candidates
	query.TopK = originalK * 3

	result, err := e.Retrieve(ctx, query)
	if err != nil {
		return nil, err
	}

	if len(result.Chunks) == 0 {
		return result, nil
	}

	if !e.config.RerankerEnabled || e.reranker == nil {
		// Truncate to original K
		if len(result.Chunks) > originalK {
			result.Chunks = result.Chunks[:originalK]
		}
		return result, nil
	}

	// Rerank
	docs := make([]string, len(result.Chunks))
	for i, c := range result.Chunks {
		docs[i] = c.Content
	}

	reranked, err := e.reranker.Rerank(ctx, query.QueryText, docs, originalK)
	if err != nil {
		e.logger.Error("Reranker failed, falling back to vector scores", logging.Err(err))
		if len(result.Chunks) > originalK {
			result.Chunks = result.Chunks[:originalK]
		}
		return result, nil
	}

	// Reorder and filter chunks
	var reorderedChunks []*RAGChunk
	for _, rr := range reranked {
		if rr.Index < len(result.Chunks) {
			chunk := result.Chunks[rr.Index]
			chunk.Score = rr.Score // Update score with reranker score
			reorderedChunks = append(reorderedChunks, chunk)
		}
	}

	result.Chunks = reorderedChunks
	result.RerankerApplied = true
	return result, nil
}

func (e *ragEngineImpl) BuildContext(ctx context.Context, result *RAGResult, budget int) (string, error) {
	if len(result.Chunks) == 0 {
		return "", nil
	}

	var sb strings.Builder
	currentTokens := 0

	// Sort by score desc just in case
	sort.Slice(result.Chunks, func(i, j int) bool {
		return result.Chunks[i].Score > result.Chunks[j].Score
	})

	for _, chunk := range result.Chunks {
		// Estimate tokens: 4 chars / token
		estTokens := len(chunk.Content)/4 + 20 // + overhead for source tag
		if budget > 0 && currentTokens+estTokens > budget {
			break
		}

		sb.WriteString(fmt.Sprintf("[%s]: %s\n\n", chunk.Source, chunk.Content))
		currentTokens += estTokens
	}

	return sb.String(), nil
}

func (e *ragEngineImpl) IndexDocument(ctx context.Context, doc *Document) error {
	chunks, err := e.chunker.Chunk(doc)
	if err != nil {
		return err
	}

	var batch []*VectorInsertItem
	var texts []string

	for _, c := range chunks {
		texts = append(texts, c.Content)
	}

	vectors, err := e.embedder.BatchEmbed(ctx, texts)
	if err != nil {
		return err
	}

	for i, c := range chunks {
		meta := make(map[string]interface{})
		for k, v := range doc.Metadata {
			meta[k] = v
		}
		meta["content"] = c.Content
		meta["document_id"] = doc.DocumentID
		meta["source"] = string(doc.Source)

		batch = append(batch, &VectorInsertItem{
			ID:       c.ID,
			Vector:   vectors[i],
			Metadata: meta,
		})
	}

	return e.vectorStore.BatchInsert(ctx, batch)
}

func (e *ragEngineImpl) IndexBatch(ctx context.Context, docs []*Document) error {
	for _, doc := range docs {
		if err := e.IndexDocument(ctx, doc); err != nil {
			return err
		}
	}
	return nil
}

func (e *ragEngineImpl) DeleteDocument(ctx context.Context, docID string) error {
	return e.vectorStore.Delete(ctx, docID)
}

//Personal.AI order the ending
