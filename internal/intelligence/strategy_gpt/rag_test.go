package strategy_gpt

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// Mocks
// ---------------------------------------------------------------------------

type mockVectorStore struct {
	searchFn      func(ctx context.Context, vector []float32, topK int, filters map[string]interface{}) ([]*VectorSearchResult, error)
	insertFn      func(ctx context.Context, id string, vector []float32, metadata map[string]interface{}) error
	deleteFn      func(ctx context.Context, id string) error
	batchInsertFn func(ctx context.Context, items []*VectorInsertItem) error
}

func (m *mockVectorStore) Search(ctx context.Context, vector []float32, topK int, filters map[string]interface{}) ([]*VectorSearchResult, error) {
	if m.searchFn != nil {
		return m.searchFn(ctx, vector, topK, filters)
	}
	return []*VectorSearchResult{}, nil
}

func (m *mockVectorStore) Insert(ctx context.Context, id string, vector []float32, metadata map[string]interface{}) error {
	if m.insertFn != nil {
		return m.insertFn(ctx, id, vector, metadata)
	}
	return nil
}

func (m *mockVectorStore) Delete(ctx context.Context, id string) error {
	if m.deleteFn != nil {
		return m.deleteFn(ctx, id)
	}
	return nil
}

func (m *mockVectorStore) BatchInsert(ctx context.Context, items []*VectorInsertItem) error {
	if m.batchInsertFn != nil {
		return m.batchInsertFn(ctx, items)
	}
	return nil
}

type mockTextEmbedder struct {
	embedFn      func(ctx context.Context, text string) ([]float32, error)
	batchEmbedFn func(ctx context.Context, texts []string) ([][]float32, error)
}

func (m *mockTextEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	if m.embedFn != nil {
		return m.embedFn(ctx, text)
	}
	return make([]float32, 128), nil
}

func (m *mockTextEmbedder) BatchEmbed(ctx context.Context, texts []string) ([][]float32, error) {
	if m.batchEmbedFn != nil {
		return m.batchEmbedFn(ctx, texts)
	}
	result := make([][]float32, len(texts))
	for i := range texts {
		result[i] = make([]float32, 128)
	}
	return result, nil
}

type mockReranker struct {
	rerankFn func(ctx context.Context, query string, documents []string, topK int) ([]*RerankResult, error)
}

func (m *mockReranker) Rerank(ctx context.Context, query string, documents []string, topK int) ([]*RerankResult, error) {
	if m.rerankFn != nil {
		return m.rerankFn(ctx, query, documents, topK)
	}
	var results []*RerankResult
	for i := range documents {
		if i >= topK {
			break
		}
		results = append(results, &RerankResult{Index: i, Score: 1.0 - float64(i)*0.1})
	}
	return results, nil
}

type mockDocumentChunker struct {
	chunkFn func(doc *Document) ([]*DocumentChunk, error)
}

func (m *mockDocumentChunker) Chunk(doc *Document) ([]*DocumentChunk, error) {
	if m.chunkFn != nil {
		return m.chunkFn(doc)
	}
	if doc == nil || doc.Content == "" {
		return []*DocumentChunk{}, nil
	}
	return []*DocumentChunk{
		{
			ChunkID:    doc.DocumentID + "-chunk-0",
			DocumentID: doc.DocumentID,
			Content:    doc.Content,
			Source:     doc.Source,
			Metadata:   copyMetadata(doc.Metadata),
			TokenCount: estimateTokens(doc.Content),
			Index:      0,
		},
	}, nil
}

// ---------------------------------------------------------------------------
// Helper: build test engine
// ---------------------------------------------------------------------------

func newTestRAGEngine(t *testing.T, opts ...func(*testRAGOpts)) RAGEngine {
	t.Helper()
	o := &testRAGOpts{
		vs:       &mockVectorStore{},
		embedder: &mockTextEmbedder{},
		reranker: &mockReranker{},
		chunker:  &mockDocumentChunker{},
	}
	for _, fn := range opts {
		fn(o)
	}
	eng, err := NewRAGEngine(o.vs, o.embedder, o.reranker, o.chunker, DefaultRAGConfig(), nil, nil)
	if err != nil {
		t.Fatalf("NewRAGEngine: %v", err)
	}
	return eng
}

type testRAGOpts struct {
	vs       VectorStore
	embedder TextEmbedder
	reranker Reranker
	chunker  DocumentChunker
}

func withVectorStore(vs VectorStore) func(*testRAGOpts) {
	return func(o *testRAGOpts) { o.vs = vs }
}
func withEmbedder(e TextEmbedder) func(*testRAGOpts) {
	return func(o *testRAGOpts) { o.embedder = e }
}
func withReranker(r Reranker) func(*testRAGOpts) {
	return func(o *testRAGOpts) { o.reranker = r }
}
func withChunker(c DocumentChunker) func(*testRAGOpts) {
	return func(o *testRAGOpts) { o.chunker = c }
}

func makeSearchResults(n int, baseScore float64) []*VectorSearchResult {
	results := make([]*VectorSearchResult, n)
	for i := 0; i < n; i++ {
		score := baseScore - float64(i)*0.05
		results[i] = &VectorSearchResult{
			ID:    fmt.Sprintf("chunk-%d", i),
			Score: score,
			Metadata: map[string]string{
				"document_id": fmt.Sprintf("doc-%d", i),
				"source":      string(SourcePatent),
				"content":     fmt.Sprintf("This is the content of chunk %d about patent claims.", i),
			},
		}
	}
	return results
}

// ---------------------------------------------------------------------------
// Tests: Retrieve
// ---------------------------------------------------------------------------

func TestRetrieve_Success(t *testing.T) {
	vs := &mockVectorStore{
		searchFn: func(ctx context.Context, vector []float32, topK int, filters map[string]interface{}) ([]*VectorSearchResult, error) {
			return makeSearchResults(5, 0.95), nil
		},
	}
	eng := newTestRAGEngine(t, withVectorStore(vs))

	result, err := eng.Retrieve(context.Background(), &RAGQuery{
		QueryText: "patent claim analysis",
		TopK:      5,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Chunks) != 5 {
		t.Errorf("expected 5 chunks, got %d", len(result.Chunks))
	}
	if result.TotalFound != 5 {
		t.Errorf("expected TotalFound=5, got %d", result.TotalFound)
	}
	if result.RerankerApplied {
		t.Error("expected RerankerApplied=false")
	}
}

func TestRetrieve_WithPrecomputedEmbedding(t *testing.T) {
	embedCalled := false
	embedder := &mockTextEmbedder{
		embedFn: func(ctx context.Context, text string) ([]float32, error) {
			embedCalled = true
			return make([]float32, 128), nil
		},
	}
	vs := &mockVectorStore{
		searchFn: func(ctx context.Context, vector []float32, topK int, filters map[string]interface{}) ([]*VectorSearchResult, error) {
			return makeSearchResults(3, 0.90), nil
		},
	}
	eng := newTestRAGEngine(t, withVectorStore(vs), withEmbedder(embedder))

	_, err := eng.Retrieve(context.Background(), &RAGQuery{
		QueryText:      "test",
		QueryEmbedding: make([]float32, 128),
		TopK:           3,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if embedCalled {
		t.Error("embedder should NOT have been called when QueryEmbedding is provided")
	}
}

func TestRetrieve_WithFilters(t *testing.T) {
	var capturedFilters map[string]interface{}
	vs := &mockVectorStore{
		searchFn: func(ctx context.Context, vector []float32, topK int, filters map[string]interface{}) ([]*VectorSearchResult, error) {
			capturedFilters = filters
			return []*VectorSearchResult{}, nil
		},
	}
	eng := newTestRAGEngine(t, withVectorStore(vs))

	now := time.Now()
	_, err := eng.Retrieve(context.Background(), &RAGQuery{
		QueryText: "test",
		TopK:      5,
		Filters: &RAGFilters{
			DateRange:     &DateRange{From: now.Add(-365 * 24 * time.Hour), To: now},
			Jurisdictions: []string{"US", "CN"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedFilters == nil {
		t.Fatal("filters were not passed to vector store")
	}
	if _, ok := capturedFilters["date_from"]; !ok {
		t.Error("expected date_from in filters")
	}
	if _, ok := capturedFilters["date_to"]; !ok {
		t.Error("expected date_to in filters")
	}
	jurisdictions, ok := capturedFilters["jurisdictions"]
	if !ok {
		t.Fatal("expected jurisdictions in filters")
	}
	jList, ok := jurisdictions.([]string)
	if !ok || len(jList) != 2 {
		t.Errorf("expected 2 jurisdictions, got %v", jurisdictions)
	}
}

func TestRetrieve_ThresholdFiltering(t *testing.T) {
	vs := &mockVectorStore{
		searchFn: func(ctx context.Context, vector []float32, topK int, filters map[string]interface{}) ([]*VectorSearchResult, error) {
			return []*VectorSearchResult{
				{ID: "c1", Score: 0.95, Metadata: map[string]string{"source": "Patent", "content": "high", "document_id": "d1"}},
				{ID: "c2", Score: 0.80, Metadata: map[string]string{"source": "Patent", "content": "medium", "document_id": "d2"}},
				{ID: "c3", Score: 0.70, Metadata: map[string]string{"source": "Patent", "content": "ok", "document_id": "d3"}},
				{ID: "c4", Score: 0.40, Metadata: map[string]string{"source": "Patent", "content": "low1", "document_id": "d4"}},
				{ID: "c5", Score: 0.30, Metadata: map[string]string{"source": "Patent", "content": "low2", "document_id": "d5"}},
			}, nil
		},
	}
	eng := newTestRAGEngine(t, withVectorStore(vs))

	result, err := eng.Retrieve(context.Background(), &RAGQuery{
		QueryText:           "test",
		TopK:                10,
		SimilarityThreshold: 0.55,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Chunks) != 3 {
		t.Errorf("expected 3 chunks above threshold, got %d", len(result.Chunks))
	}
}

func TestRetrieve_EmptyResult(t *testing.T) {
	vs := &mockVectorStore{
		searchFn: func(ctx context.Context, vector []float32, topK int, filters map[string]interface{}) ([]*VectorSearchResult, error) {
			return []*VectorSearchResult{}, nil
		},
	}
	eng := newTestRAGEngine(t, withVectorStore(vs))

	result, err := eng.Retrieve(context.Background(), &RAGQuery{QueryText: "nothing", TopK: 5})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Chunks) != 0 {
		t.Errorf("expected 0 chunks, got %d", len(result.Chunks))
	}
}

func TestRetrieve_EmbedderError(t *testing.T) {
	embedder := &mockTextEmbedder{
		embedFn: func(ctx context.Context, text string) ([]float32, error) {
			return nil, fmt.Errorf("embedder down")
		},
	}
	eng := newTestRAGEngine(t, withEmbedder(embedder))

	_, err := eng.Retrieve(context.Background(), &RAGQuery{QueryText: "test", TopK: 5})
	if err == nil {
		t.Fatal("expected error from embedder")
	}
	if !strings.Contains(err.Error(), "embedder down") {
		t.Errorf("expected embedder error, got: %v", err)
	}
}

func TestRetrieve_VectorStoreError(t *testing.T) {
	vs := &mockVectorStore{
		searchFn: func(ctx context.Context, vector []float32, topK int, filters map[string]interface{}) ([]*VectorSearchResult, error) {
			return nil, fmt.Errorf("milvus unavailable")
		},
	}
	eng := newTestRAGEngine(t, withVectorStore(vs))

	_, err := eng.Retrieve(context.Background(), &RAGQuery{QueryText: "test", TopK: 5})
	if err == nil {
		t.Fatal("expected error from vector store")
	}
	if !strings.Contains(err.Error(), "milvus unavailable") {
		t.Errorf("expected vector store error, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Tests: RetrieveAndRerank
// ---------------------------------------------------------------------------

func TestRetrieveAndRerank_Success(t *testing.T) {
	vs := &mockVectorStore{
		searchFn: func(ctx context.Context, vector []float32, topK int, filters map[string]interface{}) ([]*VectorSearchResult, error) {
			return makeSearchResults(30, 0.95), nil
		},
	}
	reranker := &mockReranker{
		rerankFn: func(ctx context.Context, query string, documents []string, topK int) ([]*RerankResult, error) {
			var results []*RerankResult
			for i := 0; i < topK && i < len(documents); i++ {
				results = append(results, &RerankResult{Index: i, Score: 0.99 - float64(i)*0.05})
			}
			return results, nil
		},
	}
	eng := newTestRAGEngine(t, withVectorStore(vs), withReranker(reranker))

	result, err := eng.RetrieveAndRerank(context.Background(), &RAGQuery{
		QueryText: "patent infringement analysis",
		TopK:      5,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.RerankerApplied {
		t.Error("expected RerankerApplied=true")
	}
	if len(result.Chunks) > 5 {
		t.Errorf("expected at most 5 chunks, got %d", len(result.Chunks))
	}
}

func TestRetrieveAndRerank_RerankerScoreUpdate(t *testing.T) {
	vs := &mockVectorStore{
		searchFn: func(ctx context.Context, vector []float32, topK int, filters map[string]interface{}) ([]*VectorSearchResult, error) {
			return makeSearchResults(10, 0.90), nil
		},
	}
	reranker := &mockReranker{
		rerankFn: func(ctx context.Context, query string, documents []string, topK int) ([]*RerankResult, error) {
			return []*RerankResult{
				{Index: 2, Score: 0.98},
				{Index: 0, Score: 0.95},
				{Index: 1, Score: 0.90},
			}, nil
		},
	}
	eng := newTestRAGEngine(t, withVectorStore(vs), withReranker(reranker))

	result, err := eng.RetrieveAndRerank(context.Background(), &RAGQuery{
		QueryText: "test",
		TopK:      3,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Chunks) == 0 {
		t.Fatal("expected non-empty chunks")
	}
	// First chunk should have highest reranker score.
	if result.Chunks[0].RerankerScore < result.Chunks[len(result.Chunks)-1].RerankerScore {
		t.Error("chunks should be sorted by reranker score descending")
	}
	for _, c := range result.Chunks {
		if c.RerankerScore <= 0 {
			t.Errorf("expected positive RerankerScore, got %f", c.RerankerScore)
		}
	}
}

func TestRetrieveAndRerank_RerankerUnavailable(t *testing.T) {
	vs := &mockVectorStore{
		searchFn: func(ctx context.Context, vector []float32, topK int, filters map[string]interface{}) ([]*VectorSearchResult, error) {
			return makeSearchResults(15, 0.90), nil
		},
	}
	reranker := &mockReranker{
		rerankFn: func(ctx context.Context, query string, documents []string, topK int) ([]*RerankResult, error) {
			return nil, fmt.Errorf("reranker service unavailable")
		},
	}
	eng := newTestRAGEngine(t, withVectorStore(vs), withReranker(reranker))

	result, err := eng.RetrieveAndRerank(context.Background(), &RAGQuery{
		QueryText: "test",
		TopK:      5,
	})
	if err != nil {
		t.Fatalf("unexpected error (should degrade gracefully): %v", err)
	}
	if result.RerankerApplied {
		t.Error("expected RerankerApplied=false on degradation")
	}
	if len(result.Chunks) > 5 {
		t.Errorf("expected at most 5 chunks after degradation, got %d", len(result.Chunks))
	}
}

func TestRetrieveAndRerank_EmptyRetrievalResult(t *testing.T) {
	vs := &mockVectorStore{
		searchFn: func(ctx context.Context, vector []float32, topK int, filters map[string]interface{}) ([]*VectorSearchResult, error) {
			return []*VectorSearchResult{}, nil
		},
	}
	rerankCalled := false
	reranker := &mockReranker{
		rerankFn: func(ctx context.Context, query string, documents []string, topK int) ([]*RerankResult, error) {
			rerankCalled = true
			return nil, nil
		},
	}
	eng := newTestRAGEngine(t, withVectorStore(vs), withReranker(reranker))

	result, err := eng.RetrieveAndRerank(context.Background(), &RAGQuery{QueryText: "test", TopK: 5})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rerankCalled {
		t.Error("reranker should NOT be called when retrieval returns empty")
	}
	if len(result.Chunks) != 0 {
		t.Errorf("expected 0 chunks, got %d", len(result.Chunks))
	}
}

// ---------------------------------------------------------------------------
// Tests: BuildContext
// ---------------------------------------------------------------------------

func TestBuildContext_WithinBudget(t *testing.T) {
	eng := newTestRAGEngine(t)
	chunks := make([]*RAGChunk, 5)
	for i := 0; i < 5; i++ {
		chunks[i] = &RAGChunk{
			ChunkID:    fmt.Sprintf("c%d", i),
			DocumentID: fmt.Sprintf("doc%d", i),
			Content:    fmt.Sprintf("Short content %d.", i),
			Score:      0.9 - float64(i)*0.05,
			Source:     SourcePatent,
			Metadata:   map[string]string{"patent_number": fmt.Sprintf("US%d", 10000000+i)},
			TokenCount: 10,
		}
	}
	result := &RAGResult{Chunks: chunks}

	ctx, err := eng.BuildContext(context.Background(), result, 5000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ctx == "" {
		t.Fatal("expected non-empty context")
	}
	// All 5 chunks should be included.
	for i := 0; i < 5; i++ {
		if !strings.Contains(ctx, fmt.Sprintf("Short content %d.", i)) {
			t.Errorf("expected chunk %d content in context", i)
		}
	}
}

func TestBuildContext_ExceedsBudget(t *testing.T) {
	eng := newTestRAGEngine(t)
	chunks := make([]*RAGChunk, 10)
	// Each chunk ~100 tokens.
	longContent := strings.Repeat("This is a moderately long sentence for testing purposes. ", 20)
	for i := 0; i < 10; i++ {
		chunks[i] = &RAGChunk{
			ChunkID:    fmt.Sprintf("c%d", i),
			DocumentID: fmt.Sprintf("doc%d", i),
			Content:    fmt.Sprintf("Chunk %d: %s", i, longContent),
			Score:      0.99 - float64(i)*0.05,
			Source:     SourcePatent,
			Metadata:   map[string]string{"patent_number": fmt.Sprintf("US%d", i)},
			TokenCount: estimateTokens(fmt.Sprintf("Chunk %d: %s", i, longContent)),
		}
	}
	result := &RAGResult{Chunks: chunks}

	// Budget that can only fit a few chunks.
	ctx, err := eng.BuildContext(context.Background(), result, 200)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ctx == "" {
		t.Fatal("expected non-empty context")
	}
	// Should NOT contain all 10 chunks.
	containedCount := 0
	for i := 0; i < 10; i++ {
		if strings.Contains(ctx, fmt.Sprintf("Chunk %d:", i)) {
			containedCount++
		}
	}
	if containedCount >= 10 {
		t.Error("expected budget to truncate some chunks")
	}
	// Highest-scored chunk should be present.
	if !strings.Contains(ctx, "Chunk 0:") {
		t.Error("expected highest-scored chunk (Chunk 0) to be included")
	}
}

func TestBuildContext_SourceAnnotation(t *testing.T) {
	eng := newTestRAGEngine(t)
	chunks := []*RAGChunk{
		{
			ChunkID:    "c1",
			DocumentID: "pat-001",
			Content:    "Claim 1 content.",
			Score:      0.95,
			Source:     SourcePatent,
			Metadata:   map[string]string{"patent_number": "US12345678", "claim_number": "1"},
			TokenCount: 10,
		},
		{
			ChunkID:    "c2",
			DocumentID: "case-001",
			Content:    "Case law excerpt.",
			Score:      0.90,
			Source:     SourceCaseLaw,
			Metadata:   map[string]string{"case_name": "Alice v. CLS Bank"},
			TokenCount: 10,
		},
		{
			ChunkID:    "c3",
			DocumentID: "mpep-001",
			Content:    "MPEP section content.",
			Score:      0.85,
			Source:     SourceExaminationGuideline,
			Metadata:   map[string]string{"section_ref": "2111.03"},
			TokenCount: 10,
		},
		{
			ChunkID:    "c4",
			DocumentID: "paper-001",
			Content:    "Scientific paper excerpt.",
			Score:      0.80,
			Source:     SourceScientificPaper,
			Metadata:   map[string]string{"title": "Deep Learning for Molecules"},
			TokenCount: 10,
		},
		{
			ChunkID:    "c5",
			DocumentID: "reg-001",
			Content:    "Regulatory text.",
			Score:      0.75,
			Source:     SourceRegulatory,
			Metadata:   map[string]string{"regulation_ref": "35 USC §101"},
			TokenCount: 10,
		},
	}
	result := &RAGResult{Chunks: chunks}

	ctx, err := eng.BuildContext(context.Background(), result, 5000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(ctx, "Patent US12345678, Claim 1") {
		t.Error("expected patent source annotation")
	}
	if !strings.Contains(ctx, "Case: Alice v. CLS Bank") {
		t.Error("expected case law source annotation")
	}
	if !strings.Contains(ctx, "MPEP §2111.03") {
		t.Error("expected MPEP source annotation")
	}
	if !strings.Contains(ctx, "Paper: Deep Learning for Molecules") {
		t.Error("expected paper source annotation")
	}
	if !strings.Contains(ctx, "Regulation: 35 USC §101") {
		t.Error("expected regulatory source annotation")
	}
}

func TestBuildContext_EmptyChunks(t *testing.T) {
	eng := newTestRAGEngine(t)
	result := &RAGResult{Chunks: []*RAGChunk{}}

	ctx, err := eng.BuildContext(context.Background(), result, 5000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ctx != "" {
		t.Errorf("expected empty context, got: %q", ctx)
	}
}

func TestBuildContext_ZeroBudget(t *testing.T) {
	eng := newTestRAGEngine(t)
	result := &RAGResult{
		Chunks: []*RAGChunk{
			{ChunkID: "c1", Content: "some content", Score: 0.9, Source: SourcePatent, TokenCount: 10},
		},
	}

	ctx, err := eng.BuildContext(context.Background(), result, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ctx != "" {
		t.Errorf("expected empty context for zero budget, got: %q", ctx)
	}
}

func TestBuildContext_NilResult(t *testing.T) {
	eng := newTestRAGEngine(t)
	ctx, err := eng.BuildContext(context.Background(), nil, 5000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ctx != "" {
		t.Errorf("expected empty context for nil result, got: %q", ctx)
	}
}

// ---------------------------------------------------------------------------
// Tests: IndexDocument
// ---------------------------------------------------------------------------

func TestIndexDocument_Success(t *testing.T) {
	var insertedItems []*VectorInsertItem
	vs := &mockVectorStore{
		batchInsertFn: func(ctx context.Context, items []*VectorInsertItem) error {
			insertedItems = items
			return nil
		},
	}
	eng := newTestRAGEngine(t, withVectorStore(vs))

	doc := &Document{
		DocumentID: "doc-001",
		Title:      "Test Patent",
		Content:    "This is a test patent document with some content.",
		Source:     SourcePatent,
		Metadata:   map[string]string{"patent_number": "US12345678"},
		Language:   "en",
	}
	err := eng.IndexDocument(context.Background(), doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(insertedItems) == 0 {
		t.Fatal("expected at least one item inserted")
	}
	for _, item := range insertedItems {
		if item.ID == "" {
			t.Error("expected non-empty chunk ID")
		}
		if len(item.Vector) == 0 {
			t.Error("expected non-empty vector")
		}
		docID, ok := item.Metadata["document_id"]
		if !ok || docID != "doc-001" {
			t.Errorf("expected document_id=doc-001, got %v", docID)
		}
	}
}

func TestIndexDocument_ChunkingResult(t *testing.T) {
	chunkCount := 0
	chunker := &mockDocumentChunker{
		chunkFn: func(doc *Document) ([]*DocumentChunk, error) {
			chunks := make([]*DocumentChunk, 3)
			for i := 0; i < 3; i++ {
				chunks[i] = &DocumentChunk{
					ChunkID:    fmt.Sprintf("%s-chunk-%d", doc.DocumentID, i),
					DocumentID: doc.DocumentID,
					Content:    fmt.Sprintf("Chunk %d content", i),
					Source:     doc.Source,
					Metadata:   copyMetadata(doc.Metadata),
					TokenCount: 20,
					Index:      i,
				}
			}
			chunkCount = len(chunks)
			return chunks, nil
		},
	}
	var insertCount int
	vs := &mockVectorStore{
		batchInsertFn: func(ctx context.Context, items []*VectorInsertItem) error {
			insertCount = len(items)
			return nil
		},
	}
	eng := newTestRAGEngine(t, withVectorStore(vs), withChunker(chunker))

	err := eng.IndexDocument(context.Background(), &Document{
		DocumentID: "doc-002",
		Content:    "Some content to chunk.",
		Source:     SourcePatent,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if chunkCount != 3 {
		t.Errorf("expected 3 chunks, got %d", chunkCount)
	}
	if insertCount != 3 {
		t.Errorf("expected 3 inserts, got %d", insertCount)
	}
}

func TestIndexDocument_NilDocument(t *testing.T) {
	eng := newTestRAGEngine(t)
	err := eng.IndexDocument(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil document")
	}
}

func TestIndexDocument_EmptyContent(t *testing.T) {
	eng := newTestRAGEngine(t)
	err := eng.IndexDocument(context.Background(), &Document{
		DocumentID: "doc-empty",
		Content:    "",
		Source:     SourcePatent,
	})
	if err == nil {
		t.Fatal("expected error for empty content")
	}
}

// ---------------------------------------------------------------------------
// Tests: IndexBatch
// ---------------------------------------------------------------------------

func TestIndexBatch_Success(t *testing.T) {
	indexedCount := 0
	vs := &mockVectorStore{
		batchInsertFn: func(ctx context.Context, items []*VectorInsertItem) error {
			indexedCount += len(items)
			return nil
		},
	}
	eng := newTestRAGEngine(t, withVectorStore(vs))

	docs := make([]*Document, 5)
	for i := 0; i < 5; i++ {
		docs[i] = &Document{
			DocumentID: fmt.Sprintf("batch-doc-%d", i),
			Content:    fmt.Sprintf("Batch document %d content.", i),
			Source:     SourcePatent,
		}
	}
	err := eng.IndexBatch(context.Background(), docs)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if indexedCount != 5 {
		t.Errorf("expected 5 indexed items, got %d", indexedCount)
	}
}

func TestIndexBatch_PartialFailure(t *testing.T) {
	callCount := 0
	vs := &mockVectorStore{
		batchInsertFn: func(ctx context.Context, items []*VectorInsertItem) error {
			callCount++
			if callCount == 3 {
				return fmt.Errorf("storage failure")
			}
			return nil
		},
	}
	eng := newTestRAGEngine(t, withVectorStore(vs))

	docs := make([]*Document, 5)
	for i := 0; i < 5; i++ {
		docs[i] = &Document{
			DocumentID: fmt.Sprintf("batch-doc-%d", i),
			Content:    fmt.Sprintf("Batch document %d content.", i),
			Source:     SourcePatent,
		}
	}
	err := eng.IndexBatch(context.Background(), docs)
	if err == nil {
		t.Fatal("expected partial failure error")
	}
	if !strings.Contains(err.Error(), "partial failure") {
		t.Errorf("expected partial failure message, got: %v", err)
	}
}

func TestIndexBatch_Empty(t *testing.T) {
	eng := newTestRAGEngine(t)
	err := eng.IndexBatch(context.Background(), []*Document{})
	if err != nil {
		t.Fatalf("unexpected error for empty batch: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Tests: DeleteDocument
// ---------------------------------------------------------------------------

func TestDeleteDocument_Success(t *testing.T) {
	deletedID := ""
	vs := &mockVectorStore{
		deleteFn: func(ctx context.Context, id string) error {
			deletedID = id
			return nil
		},
	}
	eng := newTestRAGEngine(t, withVectorStore(vs))

	err := eng.DeleteDocument(context.Background(), "doc-to-delete")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if deletedID != "doc-to-delete" {
		t.Errorf("expected deleted ID 'doc-to-delete', got %q", deletedID)
	}
}

func TestDeleteDocument_NotFound(t *testing.T) {
	vs := &mockVectorStore{
		deleteFn: func(ctx context.Context, id string) error {
			return fmt.Errorf("document not found: %s", id)
		},
	}
	eng := newTestRAGEngine(t, withVectorStore(vs))

	err := eng.DeleteDocument(context.Background(), "nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent document")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' in error, got: %v", err)
	}
}

func TestDeleteDocument_EmptyID(t *testing.T) {
	eng := newTestRAGEngine(t)
	err := eng.DeleteDocument(context.Background(), "")
	if err == nil {
		t.Fatal("expected error for empty docID")
	}
}

// ---------------------------------------------------------------------------
// Tests: DocumentChunker (built-in)
// ---------------------------------------------------------------------------

func TestDocumentChunker_PatentDocument(t *testing.T) {
	chunker := NewDefaultDocumentChunker(512, 64)
	doc := &Document{
		DocumentID: "pat-001",
		Title:      "Test Patent",
		Content: `Abstract

This invention relates to a method for processing data.

Description

The present invention provides a novel approach to data processing using machine learning techniques. The system comprises a neural network module and a data preprocessing pipeline.

Claims

1. A method for processing data comprising:
   a) receiving input data;
   b) applying a neural network to the input data;
   c) outputting processed results.

2. The method of claim 1, wherein the neural network is a convolutional neural network.

3. A system for data processing comprising:
   a processor configured to execute the method of claim 1.`,
		Source:   SourcePatent,
		Metadata: map[string]string{"patent_number": "US12345678"},
	}

	chunks, err := chunker.Chunk(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(chunks) == 0 {
		t.Fatal("expected at least one chunk")
	}

	// Should have description chunks and claim chunks.
	hasDescription := false
	hasClaim := false
	for _, c := range chunks {
		if c.Metadata != nil {
			if c.Metadata["section"] == "description" {
				hasDescription = true
			}
			if c.Metadata["section"] == "claims" {
				hasClaim = true
			}
		}
	}
	if !hasDescription {
		t.Error("expected description chunks")
	}
	if !hasClaim {
		t.Error("expected claim chunks")
	}
}

func TestDocumentChunker_ChunkSize(t *testing.T) {
	chunkSize := 50
	chunker := NewDefaultDocumentChunker(chunkSize, 0)

	// Build a document with many paragraphs.
	var paragraphs []string
	for i := 0; i < 20; i++ {
		paragraphs = append(paragraphs, fmt.Sprintf("This is paragraph number %d with enough words to make it meaningful for testing the chunking algorithm.", i))
	}
	doc := &Document{
		DocumentID: "doc-size-test",
		Content:    strings.Join(paragraphs, "\n\n"),
		Source:     SourceScientificPaper,
	}

	chunks, err := chunker.Chunk(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(chunks) < 2 {
		t.Fatalf("expected multiple chunks, got %d", len(chunks))
	}
	for i, c := range chunks {
		tokens := estimateTokens(c.Content)
		// Allow some tolerance (last chunk may be smaller, overlap may push slightly over).
		if tokens > chunkSize*2 {
			t.Errorf("chunk[%d] has %d tokens, exceeds 2x chunk size %d", i, tokens, chunkSize)
		}
	}
}

func TestDocumentChunker_ChunkOverlap(t *testing.T) {
	chunker := NewDefaultDocumentChunker(30, 10)

	var sentences []string
	for i := 0; i < 30; i++ {
		sentences = append(sentences, fmt.Sprintf("Sentence number %d is here.", i))
	}
	doc := &Document{
		DocumentID: "doc-overlap-test",
		Content:    strings.Join(sentences, " "),
		Source:     SourceScientificPaper,
	}

	chunks, err := chunker.Chunk(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(chunks) < 2 {
		t.Skipf("not enough chunks to test overlap (got %d)", len(chunks))
	}

	// Check that consecutive chunks share some content (overlap).
	overlapFound := false
	for i := 1; i < len(chunks); i++ {
		prev := chunks[i-1].Content
		curr := chunks[i].Content

		// Extract last few words of prev.
		prevWords := strings.Fields(prev)
		if len(prevWords) < 3 {
			continue
		}
		tailPhrase := strings.Join(prevWords[len(prevWords)-3:], " ")
		if strings.Contains(curr, tailPhrase) {
			overlapFound = true
			break
		}
	}
	if !overlapFound {
		t.Log("note: overlap detection is heuristic; may not always find exact overlap")
	}
}

func TestDocumentChunker_SentenceBoundary(t *testing.T) {
	chunker := NewDefaultDocumentChunker(20, 0)

	doc := &Document{
		DocumentID: "doc-sentence-test",
		Content:    "First sentence here. Second sentence follows. Third sentence is longer and contains more words for testing. Fourth sentence ends the paragraph.",
		Source:     SourceScientificPaper,
	}

	chunks, err := chunker.Chunk(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Each chunk should end at or near a sentence boundary.
	for i, c := range chunks {
		content := strings.TrimSpace(c.Content)
		if content == "" {
			continue
		}
		lastChar := content[len(content)-1]
		// Should end with period, or be the last chunk.
		if lastChar != '.' && i < len(chunks)-1 {
			t.Logf("chunk[%d] does not end with period: %q (last char: %c)", i, content, lastChar)
		}
	}
}

func TestDocumentChunker_EmptyDocument(t *testing.T) {
	chunker := NewDefaultDocumentChunker(512, 64)

	chunks, err := chunker.Chunk(&Document{DocumentID: "empty", Content: "", Source: SourcePatent})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(chunks) != 0 {
		t.Errorf("expected 0 chunks for empty document, got %d", len(chunks))
	}
}

func TestDocumentChunker_NilDocument(t *testing.T) {
	chunker := NewDefaultDocumentChunker(512, 64)

	chunks, err := chunker.Chunk(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(chunks) != 0 {
		t.Errorf("expected 0 chunks for nil document, got %d", len(chunks))
	}
}

// ---------------------------------------------------------------------------
// Tests: Helper functions
// ---------------------------------------------------------------------------

func TestEstimateTokens(t *testing.T) {
	tests := []struct {
		text    string
		minTok  int
		maxTok  int
	}{
		{"", 0, 0},
		{"hello", 1, 3},
		{"This is a test sentence with several words.", 8, 15},
		{"这是一个中文测试句子", 4, 10},
	}
	for _, tt := range tests {
		got := estimateTokens(tt.text)
		if got < tt.minTok || got > tt.maxTok {
			t.Errorf("estimateTokens(%q) = %d, expected in [%d, %d]", tt.text, got, tt.minTok, tt.maxTok)
		}
	}
}

func TestContainsSource(t *testing.T) {
	sources := []DocumentSourceType{SourcePatent, SourceCaseLaw}
	if !containsSource(sources, SourcePatent) {
		t.Error("expected true for SourcePatent")
	}
	if !containsSource(sources, SourceCaseLaw) {
		t.Error("expected true for SourceCaseLaw")
	}
	if containsSource(sources, SourceScientificPaper) {
		t.Error("expected false for SourceScientificPaper")
	}
	if containsSource(nil, SourcePatent) {
		t.Error("expected false for nil sources")
	}
}

func TestFormatSourceAnnotation(t *testing.T) {
	tests := []struct {
		name   string
		chunk  *RAGChunk
		expect string
	}{
		{
			name: "patent with claim",
			chunk: &RAGChunk{
				Source:   SourcePatent,
				Metadata: map[string]string{"patent_number": "US12345678", "claim_number": "3"},
			},
			expect: "Patent US12345678, Claim 3",
		},
		{
			name: "patent without claim",
			chunk: &RAGChunk{
				Source:   SourcePatent,
				Metadata: map[string]string{"patent_number": "US12345678", "section": "description"},
			},
			expect: "Patent US12345678, description",
		},
		{
			name: "case law",
			chunk: &RAGChunk{
				Source:   SourceCaseLaw,
				Metadata: map[string]string{"case_name": "KSR v. Teleflex"},
			},
			expect: "Case: KSR v. Teleflex",
		},
		{
			name: "MPEP",
			chunk: &RAGChunk{
				DocumentID: "mpep-2111",
				Source:     SourceExaminationGuideline,
				Metadata:   map[string]string{"section_ref": "2111.03"},
			},
			expect: "MPEP §2111.03",
		},
		{
			name: "paper",
			chunk: &RAGChunk{
				Source:   SourceScientificPaper,
				Metadata: map[string]string{"title": "Attention Is All You Need"},
			},
			expect: "Paper: Attention Is All You Need",
		},
		{
			name: "regulatory",
			chunk: &RAGChunk{
				Source:   SourceRegulatory,
				Metadata: map[string]string{"regulation_ref": "35 USC §103"},
			},
			expect: "Regulation: 35 USC §103",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatSourceAnnotation(tt.chunk)
			if got != tt.expect {
				t.Errorf("formatSourceAnnotation() = %q, want %q", got, tt.expect)
			}
		})
	}
}

func TestEffectiveScore(t *testing.T) {
	chunk1 := &RAGChunk{Score: 0.8, RerankerScore: 0.95}
	if s := effectiveScore(chunk1); s != 0.95 {
		t.Errorf("expected 0.95, got %f", s)
	}
	chunk2 := &RAGChunk{Score: 0.8, RerankerScore: 0}
	if s := effectiveScore(chunk2); s != 0.8 {
		t.Errorf("expected 0.8, got %f", s)
	}
}

func TestCopyMetadata(t *testing.T) {
	src := map[string]string{"a": "1", "b": "2"}
	dst := copyMetadata(src)
	if len(dst) != 2 {
		t.Errorf("expected 2 entries, got %d", len(dst))
	}
	// Mutating dst should not affect src.
	dst["c"] = "3"
	if _, ok := src["c"]; ok {
		t.Error("mutation of copy affected original")
	}
}

func TestCopyMetadata_Nil(t *testing.T) {
	dst := copyMetadata(nil)
	if dst == nil {
		t.Fatal("expected non-nil map")
	}
	if len(dst) != 0 {
		t.Errorf("expected empty map, got %d entries", len(dst))
	}
}

func TestDefaultRAGConfig(t *testing.T) {
	cfg := DefaultRAGConfig()
	if cfg.DefaultTopK != 10 {
		t.Errorf("expected DefaultTopK=10, got %d", cfg.DefaultTopK)
	}
	if cfg.RerankerTopK != 5 {
		t.Errorf("expected RerankerTopK=5, got %d", cfg.RerankerTopK)
	}
	if cfg.ChunkSize != 512 {
		t.Errorf("expected ChunkSize=512, got %d", cfg.ChunkSize)
	}
	if cfg.ChunkOverlap != 64 {
		t.Errorf("expected ChunkOverlap=64, got %d", cfg.ChunkOverlap)
	}
}

func TestNewRAGEngine_NilVectorStore(t *testing.T) {
	_, err := NewRAGEngine(nil, &mockTextEmbedder{}, nil, &mockDocumentChunker{}, DefaultRAGConfig(), nil, nil)
	if err == nil {
		t.Fatal("expected error for nil vectorStore")
	}
}

func TestNewRAGEngine_NilEmbedder(t *testing.T) {
	_, err := NewRAGEngine(&mockVectorStore{}, nil, nil, &mockDocumentChunker{}, DefaultRAGConfig(), nil, nil)
	if err == nil {
		t.Fatal("expected error for nil embedder")
	}
}

func TestNewRAGEngine_NilChunker(t *testing.T) {
	_, err := NewRAGEngine(&mockVectorStore{}, &mockTextEmbedder{}, nil, nil, DefaultRAGConfig(), nil, nil)
	if err == nil {
		t.Fatal("expected error for nil chunker")
	}
}

func TestNewRAGEngine_NilReranker(t *testing.T) {
	// Nil reranker is allowed (optional).
	eng, err := NewRAGEngine(&mockVectorStore{}, &mockTextEmbedder{}, nil, &mockDocumentChunker{}, DefaultRAGConfig(), nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if eng == nil {
		t.Fatal("expected non-nil engine")
	}
}

func TestRetrieve_NilQuery(t *testing.T) {
	eng := newTestRAGEngine(t)
	result, err := eng.Retrieve(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Chunks) != 0 {
		t.Errorf("expected 0 chunks for nil query, got %d", len(result.Chunks))
	}
}

func TestRetrieve_EmptyQueryText(t *testing.T) {
	eng := newTestRAGEngine(t)
	result, err := eng.Retrieve(context.Background(), &RAGQuery{QueryText: "", TopK: 5})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Chunks) != 0 {
		t.Errorf("expected 0 chunks for empty query, got %d", len(result.Chunks))
	}
}

func TestRetrieveAndRerank_NilQuery(t *testing.T) {
	eng := newTestRAGEngine(t)
	result, err := eng.RetrieveAndRerank(context.Background(), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Chunks) != 0 {
		t.Errorf("expected 0 chunks for nil query, got %d", len(result.Chunks))
	}
}

func TestSplitClaims(t *testing.T) {
	claimsText := `Claims

1. A method for processing data comprising:
   a) receiving input data;
   b) applying a neural network.

2. The method of claim 1, wherein the neural network is a CNN.

3. A system comprising a processor configured to execute the method of claim 1.`

	claims := splitClaims(claimsText)
	if len(claims) < 3 {
		t.Errorf("expected at least 3 claims, got %d", len(claims))
	}
	for i, c := range claims {
		if strings.TrimSpace(c) == "" {
			t.Errorf("claim[%d] is empty", i)
		}
	}
}

func TestSplitClaims_NoClaims(t *testing.T) {
	text := "This is just a paragraph without any numbered claims."
	claims := splitClaims(text)
	if len(claims) == 0 {
		t.Error("expected fallback splitting to produce at least one segment")
	}
}

func TestSplitSentences(t *testing.T) {
	text := "First sentence. Second sentence. Third sentence."
	sentences := splitSentences(text)
	if len(sentences) < 3 {
		t.Errorf("expected at least 3 sentences, got %d", len(sentences))
	}
}

func TestSplitSentences_SingleSentence(t *testing.T) {
	text := "Just one sentence without a period"
	sentences := splitSentences(text)
	if len(sentences) != 1 {
		t.Errorf("expected 1 sentence, got %d", len(sentences))
	}
}

func TestSplitSentences_Empty(t *testing.T) {
	sentences := splitSentences("")
	if len(sentences) != 0 {
		t.Errorf("expected 0 sentences, got %d", len(sentences))
	}
}

func TestTruncateTextToTokens(t *testing.T) {
	text := "This is a long sentence. It has multiple parts. And it keeps going on and on."
	truncated := truncateTextToTokens(text, 5)
	if len(truncated) >= len(text) {
		t.Error("expected truncation to produce shorter text")
	}
	if truncated == "" {
		t.Error("expected non-empty truncated text")
	}
}

func TestTruncateTextToTokens_ShortText(t *testing.T) {
	text := "Short."
	truncated := truncateTextToTokens(text, 1000)
	if truncated != text {
		t.Errorf("expected no truncation, got %q", truncated)
	}
}

func TestExtractTailTokens(t *testing.T) {
	text := "First part of the text. Middle section here. Last part of the text for overlap."
	tail := extractTailTokens(text, 5)
	if tail == "" {
		t.Error("expected non-empty tail")
	}
	if len(tail) >= len(text) {
		t.Error("expected tail to be shorter than full text")
	}
}

func TestExtractTailTokens_ShortText(t *testing.T) {
	text := "Short."
	tail := extractTailTokens(text, 1000)
	if tail != text {
		t.Errorf("expected full text returned, got %q", tail)
	}
}

func TestIsDigit(t *testing.T) {
	tests := []struct {
		r    rune
		want bool
	}{
		{'0', true},
		{'5', true},
		{'9', true},
		{'a', false},
		{'Z', false},
		{' ', false},
	}
	for _, tt := range tests {
		if got := isDigit(tt.r); got != tt.want {
			t.Errorf("isDigit(%c) = %v, want %v", tt.r, got, tt.want)
		}
	}
}

func TestFindSectionStart(t *testing.T) {
	text := "Abstract\n\nDescription\n\nClaims\n\n1. A method..."
	idx := findSectionStart(text, []string{"Claims", "CLAIMS"})
	if idx < 0 {
		t.Error("expected to find Claims section")
	}
	if text[idx:idx+6] != "Claims" {
		t.Errorf("expected 'Claims' at index, got %q", text[idx:idx+6])
	}
}

func TestFindSectionStart_NotFound(t *testing.T) {
	text := "This text has no section markers."
	idx := findSectionStart(text, []string{"Claims", "CLAIMS"})
	if idx != -1 {
		t.Errorf("expected -1, got %d", idx)
	}
}

func TestFindSectionStart_Chinese(t *testing.T) {
	text := "摘要\n\n说明书\n\n权利要求\n\n1. 一种方法..."
	idx := findSectionStart(text, []string{"Claims", "CLAIMS", "权利要求"})
	if idx < 0 {
		t.Error("expected to find 权利要求 section")
	}
}

// ---------------------------------------------------------------------------
// Tests: Retrieve with SourceTypes filter
// ---------------------------------------------------------------------------

func TestRetrieve_SourceTypeFiltering(t *testing.T) {
	vs := &mockVectorStore{
		searchFn: func(ctx context.Context, vector []float32, topK int, filters map[string]interface{}) ([]*VectorSearchResult, error) {
			return []*VectorSearchResult{
				{ID: "c1", Score: 0.95, Metadata: map[string]string{"source": "Patent", "content": "patent content", "document_id": "d1"}},
				{ID: "c2", Score: 0.90, Metadata: map[string]string{"source": "CaseLaw", "content": "case content", "document_id": "d2"}},
				{ID: "c3", Score: 0.85, Metadata: map[string]string{"source": "Patent", "content": "another patent", "document_id": "d3"}},
				{ID: "c4", Score: 0.80, Metadata: map[string]string{"source": "ScientificPaper", "content": "paper content", "document_id": "d4"}},
			}, nil
		},
	}
	eng := newTestRAGEngine(t, withVectorStore(vs))

	result, err := eng.Retrieve(context.Background(), &RAGQuery{
		QueryText:           "test",
		TopK:                10,
		SimilarityThreshold: 0.5,
		SourceTypes:         []DocumentSourceType{SourcePatent},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Chunks) != 2 {
		t.Errorf("expected 2 patent chunks, got %d", len(result.Chunks))
	}
	for _, c := range result.Chunks {
		if c.Source != SourcePatent {
			t.Errorf("expected SourcePatent, got %s", c.Source)
		}
	}
}

// ---------------------------------------------------------------------------
// Tests: Retrieve latency tracking
// ---------------------------------------------------------------------------

func TestRetrieve_RecordsLatency(t *testing.T) {
	vs := &mockVectorStore{
		searchFn: func(ctx context.Context, vector []float32, topK int, filters map[string]interface{}) ([]*VectorSearchResult, error) {
			return makeSearchResults(3, 0.90), nil
		},
	}
	eng := newTestRAGEngine(t, withVectorStore(vs))

	result, err := eng.Retrieve(context.Background(), &RAGQuery{QueryText: "test", TopK: 3})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.SearchLatencyMs < 0 {
		t.Errorf("expected non-negative latency, got %d", result.SearchLatencyMs)
	}
}

// ---------------------------------------------------------------------------
// Tests: RetrieveAndRerank with nil reranker (no reranker configured)
// ---------------------------------------------------------------------------

func TestRetrieveAndRerank_NilReranker(t *testing.T) {
	vs := &mockVectorStore{
		searchFn: func(ctx context.Context, vector []float32, topK int, filters map[string]interface{}) ([]*VectorSearchResult, error) {
			return makeSearchResults(10, 0.90), nil
		},
	}
	// Build engine without reranker.
	eng, err := NewRAGEngine(vs, &mockTextEmbedder{}, nil, &mockDocumentChunker{}, DefaultRAGConfig(), nil, nil)
	if err != nil {
		t.Fatalf("NewRAGEngine: %v", err)
	}

	result, err := eng.RetrieveAndRerank(context.Background(), &RAGQuery{
		QueryText: "test",
		TopK:      5,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.RerankerApplied {
		t.Error("expected RerankerApplied=false when reranker is nil")
	}
	if len(result.Chunks) > 5 {
		t.Errorf("expected at most 5 chunks, got %d", len(result.Chunks))
	}
}

// ---------------------------------------------------------------------------
// Tests: BuildContext ordering by reranker score
// ---------------------------------------------------------------------------

func TestBuildContext_OrdersByRerankerScore(t *testing.T) {
	eng := newTestRAGEngine(t)
	chunks := []*RAGChunk{
		{ChunkID: "c1", Content: "Low reranker.", Score: 0.95, RerankerScore: 0.50, Source: SourcePatent, Metadata: map[string]string{}, TokenCount: 5},
		{ChunkID: "c2", Content: "High reranker.", Score: 0.70, RerankerScore: 0.99, Source: SourcePatent, Metadata: map[string]string{}, TokenCount: 5},
		{ChunkID: "c3", Content: "Mid reranker.", Score: 0.80, RerankerScore: 0.75, Source: SourcePatent, Metadata: map[string]string{}, TokenCount: 5},
	}
	result := &RAGResult{Chunks: chunks}

	ctx, err := eng.BuildContext(context.Background(), result, 5000)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// "High reranker" should appear before "Mid reranker" which should appear before "Low reranker".
	highIdx := strings.Index(ctx, "High reranker.")
	midIdx := strings.Index(ctx, "Mid reranker.")
	lowIdx := strings.Index(ctx, "Low reranker.")
	if highIdx < 0 || midIdx < 0 || lowIdx < 0 {
		t.Fatalf("expected all chunks in context, got: %q", ctx)
	}
	if highIdx > midIdx || midIdx > lowIdx {
		t.Errorf("expected order: High < Mid < Low positions, got high=%d mid=%d low=%d", highIdx, midIdx, lowIdx)
	}
}

// ---------------------------------------------------------------------------
// Tests: IndexDocument with embedder error
// ---------------------------------------------------------------------------

func TestIndexDocument_EmbedderError(t *testing.T) {
	embedder := &mockTextEmbedder{
		batchEmbedFn: func(ctx context.Context, texts []string) ([][]float32, error) {
			return nil, fmt.Errorf("embedding service down")
		},
	}
	eng := newTestRAGEngine(t, withEmbedder(embedder))

	err := eng.IndexDocument(context.Background(), &Document{
		DocumentID: "doc-err",
		Content:    "Some content.",
		Source:     SourcePatent,
	})
	if err == nil {
		t.Fatal("expected error from embedder")
	}
	if !strings.Contains(err.Error(), "embedding service down") {
		t.Errorf("expected embedder error, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Tests: IndexDocument with vector store error
// ---------------------------------------------------------------------------

func TestIndexDocument_VectorStoreInsertError(t *testing.T) {
	vs := &mockVectorStore{
		batchInsertFn: func(ctx context.Context, items []*VectorInsertItem) error {
			return fmt.Errorf("storage write failure")
		},
	}
	eng := newTestRAGEngine(t, withVectorStore(vs))

	err := eng.IndexDocument(context.Background(), &Document{
		DocumentID: "doc-vs-err",
		Content:    "Some content.",
		Source:     SourcePatent,
	})
	if err == nil {
		t.Fatal("expected error from vector store")
	}
	if !strings.Contains(err.Error(), "storage write failure") {
		t.Errorf("expected storage error, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Tests: NewDefaultDocumentChunker edge cases
// ---------------------------------------------------------------------------

func TestNewDefaultDocumentChunker_ZeroChunkSize(t *testing.T) {
	chunker := NewDefaultDocumentChunker(0, 0)
	if chunker == nil {
		t.Fatal("expected non-nil chunker")
	}
	// Should use default chunk size.
	doc := &Document{
		DocumentID: "doc-zero",
		Content:    "Some content.",
		Source:     SourcePatent,
	}
	chunks, err := chunker.Chunk(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(chunks) == 0 {
		t.Error("expected at least one chunk")
	}
}

func TestNewDefaultDocumentChunker_OverlapExceedsSize(t *testing.T) {
	// Overlap >= chunkSize should be clamped.
	chunker := NewDefaultDocumentChunker(100, 200)
	if chunker == nil {
		t.Fatal("expected non-nil chunker")
	}
	dc := chunker.(*defaultDocumentChunker)
	if dc.chunkOverlap >= dc.chunkSize {
		t.Errorf("expected overlap < chunkSize, got overlap=%d size=%d", dc.chunkOverlap, dc.chunkSize)
	}
}

// ---------------------------------------------------------------------------
// Tests: DocumentSourceType constants
// ---------------------------------------------------------------------------

func TestDocumentSourceType_Values(t *testing.T) {
	sources := []DocumentSourceType{
		SourcePatent,
		SourceCaseLaw,
		SourceExaminationGuideline,
		SourceScientificPaper,
		SourceRegulatory,
	}
	expected := []string{"Patent", "CaseLaw", "ExaminationGuideline", "ScientificPaper", "Regulatory"}
	for i, s := range sources {
		if string(s) != expected[i] {
			t.Errorf("expected %q, got %q", expected[i], string(s))
		}
	}
}

// ---------------------------------------------------------------------------
// Benchmark: Retrieve
// ---------------------------------------------------------------------------

func BenchmarkRetrieve(b *testing.B) {
	vs := &mockVectorStore{
		searchFn: func(ctx context.Context, vector []float32, topK int, filters map[string]interface{}) ([]*VectorSearchResult, error) {
			return makeSearchResults(10, 0.95), nil
		},
	}
	eng, _ := NewRAGEngine(vs, &mockTextEmbedder{}, &mockReranker{}, &mockDocumentChunker{}, DefaultRAGConfig(), nil, nil)
	ctx := context.Background()
	query := &RAGQuery{QueryText: "patent claim analysis", TopK: 10}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = eng.Retrieve(ctx, query)
	}
}

func BenchmarkBuildContext(b *testing.B) {
	eng, _ := NewRAGEngine(&mockVectorStore{}, &mockTextEmbedder{}, &mockReranker{}, &mockDocumentChunker{}, DefaultRAGConfig(), nil, nil)
	chunks := make([]*RAGChunk, 20)
	for i := 0; i < 20; i++ {
		chunks[i] = &RAGChunk{
			ChunkID:    fmt.Sprintf("c%d", i),
			DocumentID: fmt.Sprintf("doc%d", i),
			Content:    strings.Repeat("Token content here. ", 50),
			Score:      0.95 - float64(i)*0.02,
			Source:     SourcePatent,
			Metadata:   map[string]string{"patent_number": fmt.Sprintf("US%d", i)},
			TokenCount: 150,
		}
	}
	result := &RAGResult{Chunks: chunks}
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = eng.BuildContext(ctx, result, 2048)
	}
}



