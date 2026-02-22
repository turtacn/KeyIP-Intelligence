package strategy_gpt

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// ---------------------------------------------------------------------------
// DocumentSourceType
// ---------------------------------------------------------------------------

// DocumentSourceType enumerates the origin of a document in the RAG corpus.
type DocumentSourceType string

const (
	SourcePatent                DocumentSourceType = "Patent"
	SourceCaseLaw               DocumentSourceType = "CaseLaw"
	SourceExaminationGuideline  DocumentSourceType = "ExaminationGuideline"
	SourceScientificPaper       DocumentSourceType = "ScientificPaper"
	SourceRegulatory            DocumentSourceType = "Regulatory"
)

// ---------------------------------------------------------------------------
// Core data structures
// ---------------------------------------------------------------------------

// DateRange represents a time window for filtering.
type DateRange struct {
	From time.Time `json:"from"`
	To   time.Time `json:"to"`
}

// RAGFilters constrains which documents are eligible for retrieval.
type RAGFilters struct {
	DateRange              *DateRange `json:"date_range,omitempty"`
	Jurisdictions          []string   `json:"jurisdictions,omitempty"`
	PatentClassifications  []string   `json:"patent_classifications,omitempty"`
	DocumentTypes          []string   `json:"document_types,omitempty"`
	Assignees              []string   `json:"assignees,omitempty"`
	ExcludeDocIDs          []string   `json:"exclude_doc_ids,omitempty"`
}

// RAGQuery is the input to a retrieval call.
type RAGQuery struct {
	QueryText           string             `json:"query_text"`
	QueryEmbedding      []float32          `json:"query_embedding,omitempty"`
	Filters             *RAGFilters        `json:"filters,omitempty"`
	TopK                int                `json:"top_k"`
	SimilarityThreshold float64            `json:"similarity_threshold"`
	SourceTypes         []DocumentSourceType `json:"source_types,omitempty"`
}

// RAGResult is the output of a retrieval call.
type RAGResult struct {
	Chunks          []*RAGChunk `json:"chunks"`
	TotalFound      int         `json:"total_found"`
	SearchLatencyMs int64       `json:"search_latency_ms"`
	RerankerApplied bool        `json:"reranker_applied"`
}

// Document is a full document to be indexed into the RAG corpus.
type Document struct {
	DocumentID string             `json:"document_id"`
	Title      string             `json:"title"`
	Content    string             `json:"content"`
	Source     DocumentSourceType `json:"source"`
	Metadata   map[string]string  `json:"metadata,omitempty"`
	Language   string             `json:"language,omitempty"`
}

// DocumentChunk is a fragment produced by the chunker.
type DocumentChunk struct {
	ChunkID    string             `json:"chunk_id"`
	DocumentID string             `json:"document_id"`
	Content    string             `json:"content"`
	Source     DocumentSourceType `json:"source"`
	Metadata   map[string]string  `json:"metadata,omitempty"`
	TokenCount int                `json:"token_count"`
	Index      int                `json:"index"`
}

// VectorSearchResult is a single hit from the vector store.
type VectorSearchResult struct {
	ID       string            `json:"id"`
	Score    float64           `json:"score"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// VectorInsertItem is a single item to insert into the vector store.
type VectorInsertItem struct {
	ID       string                 `json:"id"`
	Vector   []float32              `json:"vector"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// RerankResult is a single reranked document.
type RerankResult struct {
	Index int     `json:"index"`
	Score float64 `json:"score"`
}

// ---------------------------------------------------------------------------
// RAGConfig (moved to model.go to avoid redeclaration)

// ---------------------------------------------------------------------------
// Dependency interfaces
// ---------------------------------------------------------------------------

// VectorStore abstracts the underlying vector database (e.g. Milvus).
type VectorStore interface {
	Search(ctx context.Context, vector []float32, topK int, filters map[string]interface{}) ([]*VectorSearchResult, error)
	Insert(ctx context.Context, id string, vector []float32, metadata map[string]interface{}) error
	Delete(ctx context.Context, id string) error
	BatchInsert(ctx context.Context, items []*VectorInsertItem) error
}

// TextEmbedder encodes text into dense vectors.
type TextEmbedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
	BatchEmbed(ctx context.Context, texts []string) ([][]float32, error)
}

// Reranker applies a cross-encoder to re-score candidate documents.
type Reranker interface {
	Rerank(ctx context.Context, query string, documents []string, topK int) ([]*RerankResult, error)
}

// DocumentChunker splits a document into indexable fragments.
type DocumentChunker interface {
	Chunk(doc *Document) ([]*DocumentChunk, error)
}

// RAGLogger is a minimal logging interface.
type RAGLogger interface {
	Info(msg string, keysAndValues ...interface{})
	Warn(msg string, keysAndValues ...interface{})
	Error(msg string, keysAndValues ...interface{})
}

// RAGMetrics records operational metrics.
type RAGMetrics interface {
	RecordRetrievalLatency(ctx context.Context, durationMs float64, source string)
	RecordRerankLatency(ctx context.Context, durationMs float64)
	RecordIndexLatency(ctx context.Context, durationMs float64)
	RecordChunkCount(ctx context.Context, count int)
}

// ---------------------------------------------------------------------------
// RAGEngine interface
// ---------------------------------------------------------------------------

// RAGEngine defines the retrieval-augmented generation capabilities.
type RAGEngine interface {
	Retrieve(ctx context.Context, query *RAGQuery) (*RAGResult, error)
	RetrieveAndRerank(ctx context.Context, query *RAGQuery) (*RAGResult, error)
	BuildContext(ctx context.Context, result *RAGResult, budget int) (string, error)
	IndexDocument(ctx context.Context, doc *Document) error
	IndexBatch(ctx context.Context, docs []*Document) error
	DeleteDocument(ctx context.Context, docID string) error
}

// ---------------------------------------------------------------------------
// ragEngineImpl
// ---------------------------------------------------------------------------

type ragEngineImpl struct {
	vectorStore VectorStore
	embedder    TextEmbedder
	reranker    Reranker
	chunker     DocumentChunker
	config      RAGConfig
	metrics     RAGMetrics
	logger      RAGLogger
}

// NewRAGEngine constructs a production RAGEngine.
func NewRAGEngine(
	vectorStore VectorStore,
	embedder TextEmbedder,
	reranker Reranker,
	chunker DocumentChunker,
	config RAGConfig,
	metrics RAGMetrics,
	logger RAGLogger,
) (RAGEngine, error) {
	if vectorStore == nil {
		return nil, errors.NewInvalidInputError("vectorStore is required")
	}
	if embedder == nil {
		return nil, errors.NewInvalidInputError("embedder is required")
	}
	if chunker == nil {
		return nil, errors.NewInvalidInputError("chunker is required")
	}
	if logger == nil {
		logger = &noopRAGLogger{}
	}
	if metrics == nil {
		metrics = &noopRAGMetrics{}
	}
	return &ragEngineImpl{
		vectorStore: vectorStore,
		embedder:    embedder,
		reranker:    reranker,
		chunker:     chunker,
		config:      config,
		metrics:     metrics,
		logger:      logger,
	}, nil
}

// ---------------------------------------------------------------------------
// Retrieve
// ---------------------------------------------------------------------------

func (r *ragEngineImpl) Retrieve(ctx context.Context, query *RAGQuery) (*RAGResult, error) {
	if query == nil || (query.QueryText == "" && len(query.QueryEmbedding) == 0) {
		return &RAGResult{Chunks: []*RAGChunk{}}, nil
	}
	start := time.Now()

	// 1. Encode query if embedding not provided.
	embedding := query.QueryEmbedding
	if len(embedding) == 0 {
		var err error
		embedding, err = r.embedder.Embed(ctx, query.QueryText)
		if err != nil {
			return nil, fmt.Errorf("embedding query: %w", err)
		}
	}

	// 2. Build vector store filters.
	filters := r.buildVectorFilters(query)

	// 3. Determine topK.
	topK := query.TopK
	if topK <= 0 {
		topK = r.config.DefaultTopK
	}

	// 4. Search.
	hits, err := r.vectorStore.Search(ctx, embedding, topK, filters)
	if err != nil {
		return nil, fmt.Errorf("vector search: %w", err)
	}

	// 5. Threshold filtering.
	threshold := query.SimilarityThreshold
	if threshold <= 0 {
		threshold = r.config.SimilarityThreshold
	}

	var chunks []*RAGChunk
	for _, hit := range hits {
		if hit.Score < threshold {
			continue
		}
		// Source type filtering.
		src := DocumentSourceType(hit.Metadata["source"])
		if len(query.SourceTypes) > 0 && !containsSource(query.SourceTypes, src) {
			continue
		}
		chunk := &RAGChunk{
			ChunkID:    hit.ID,
			DocumentID: hit.Metadata["document_id"],
			Content:    hit.Metadata["content"],
			Score:      hit.Score,
			Source:     src,
			Metadata:   hit.Metadata,
			TokenCount: estimateTokens(hit.Metadata["content"]),
		}
		chunks = append(chunks, chunk)
	}
	if chunks == nil {
		chunks = []*RAGChunk{}
	}

	elapsed := time.Since(start).Milliseconds()
	r.metrics.RecordRetrievalLatency(ctx, float64(elapsed), "retrieve")
	r.metrics.RecordChunkCount(ctx, len(chunks))

	return &RAGResult{
		Chunks:          chunks,
		TotalFound:      len(hits),
		SearchLatencyMs: elapsed,
		RerankerApplied: false,
	}, nil
}

// ---------------------------------------------------------------------------
// RetrieveAndRerank
// ---------------------------------------------------------------------------

func (r *ragEngineImpl) RetrieveAndRerank(ctx context.Context, query *RAGQuery) (*RAGResult, error) {
	if query == nil || (query.QueryText == "" && len(query.QueryEmbedding) == 0) {
		return &RAGResult{Chunks: []*RAGChunk{}}, nil
	}

	// 1. Retrieve with expanded topK for more candidates.
	expandedQuery := *query
	multiplier := r.config.RerankerMultiplier
	if multiplier <= 0 {
		multiplier = 3
	}
	originalTopK := query.TopK
	if originalTopK <= 0 {
		originalTopK = r.config.DefaultTopK
	}
	expandedQuery.TopK = originalTopK * multiplier

	retrievalResult, err := r.Retrieve(ctx, &expandedQuery)
	if err != nil {
		return nil, err
	}

	// 2. If no results or no reranker, return as-is.
	if len(retrievalResult.Chunks) == 0 {
		return retrievalResult, nil
	}
	if r.reranker == nil {
		r.logger.Warn("reranker not configured, returning raw retrieval results")
		// Trim to original topK.
		if len(retrievalResult.Chunks) > originalTopK {
			retrievalResult.Chunks = retrievalResult.Chunks[:originalTopK]
		}
		return retrievalResult, nil
	}

	// 3. Rerank.
	queryText := query.QueryText
	documents := make([]string, len(retrievalResult.Chunks))
	for i, c := range retrievalResult.Chunks {
		documents[i] = c.Content
	}

	rerankerTopK := r.config.RerankerTopK
	if rerankerTopK <= 0 {
		rerankerTopK = originalTopK
	}

	rerankStart := time.Now()
	rerankResults, err := r.reranker.Rerank(ctx, queryText, documents, rerankerTopK)
	rerankElapsed := float64(time.Since(rerankStart).Milliseconds())
	r.metrics.RecordRerankLatency(ctx, rerankElapsed)

	if err != nil {
		// Graceful degradation: use original scores.
		r.logger.Warn("reranker unavailable, degrading to raw scores", "error", err)
		if len(retrievalResult.Chunks) > originalTopK {
			retrievalResult.Chunks = retrievalResult.Chunks[:originalTopK]
		}
		retrievalResult.RerankerApplied = false
		return retrievalResult, nil
	}

	// 4. Map reranker scores back to chunks.
	reranked := make([]*RAGChunk, 0, len(rerankResults))
	for _, rr := range rerankResults {
		if rr.Index < 0 || rr.Index >= len(retrievalResult.Chunks) {
			continue
		}
		chunk := retrievalResult.Chunks[rr.Index]
		chunk.RerankerScore = rr.Score
		reranked = append(reranked, chunk)
	}

	// Sort by reranker score descending.
	sort.Slice(reranked, func(i, j int) bool {
		return reranked[i].RerankerScore > reranked[j].RerankerScore
	})

	if len(reranked) > rerankerTopK {
		reranked = reranked[:rerankerTopK]
	}

	return &RAGResult{
		Chunks:          reranked,
		TotalFound:      retrievalResult.TotalFound,
		SearchLatencyMs: retrievalResult.SearchLatencyMs + int64(rerankElapsed),
		RerankerApplied: true,
	}, nil
}

// ---------------------------------------------------------------------------
// BuildContext
// ---------------------------------------------------------------------------

func (r *ragEngineImpl) BuildContext(ctx context.Context, result *RAGResult, budget int) (string, error) {
	if result == nil || len(result.Chunks) == 0 || budget <= 0 {
		return "", nil
	}

	// Sort by effective score descending (prefer reranker score if available).
	sorted := make([]*RAGChunk, len(result.Chunks))
	copy(sorted, result.Chunks)
	sort.Slice(sorted, func(i, j int) bool {
		si := effectiveScore(sorted[i])
		sj := effectiveScore(sorted[j])
		return si > sj
	})

	var builder strings.Builder
	usedTokens := 0

	for idx, chunk := range sorted {
		annotation := formatSourceAnnotation(chunk)
		// Each chunk is formatted as:
		//   [Source Annotation]
		//   <content>
		//
		entry := fmt.Sprintf("[%s]\n%s\n\n", annotation, strings.TrimSpace(chunk.Content))
		entryTokens := estimateTokens(entry)

		if usedTokens+entryTokens > budget {
			// Try to fit a truncated version.
			remaining := budget - usedTokens
			if remaining > 20 { // minimum useful size
				truncated := truncateTextToTokens(entry, remaining)
				builder.WriteString(truncated)
			}
			break
		}

		builder.WriteString(entry)
		usedTokens += entryTokens
		_ = idx
	}

	return strings.TrimSpace(builder.String()), nil
}

// ---------------------------------------------------------------------------
// IndexDocument
// ---------------------------------------------------------------------------

func (r *ragEngineImpl) IndexDocument(ctx context.Context, doc *Document) error {
	if doc == nil || doc.DocumentID == "" {
		return errors.NewInvalidInputError("document with ID is required")
	}
	if doc.Content == "" {
		return errors.NewInvalidInputError("document content is required")
	}

	start := time.Now()

	// 1. Chunk.
	chunks, err := r.chunker.Chunk(doc)
	if err != nil {
		return fmt.Errorf("chunking document %s: %w", doc.DocumentID, err)
	}
	if len(chunks) == 0 {
		return nil
	}

	// 2. Embed all chunks.
	texts := make([]string, len(chunks))
	for i, c := range chunks {
		texts[i] = c.Content
	}
	embeddings, err := r.embedder.BatchEmbed(ctx, texts)
	if err != nil {
		return fmt.Errorf("embedding chunks for document %s: %w", doc.DocumentID, err)
	}
	if len(embeddings) != len(chunks) {
		return fmt.Errorf("embedding count mismatch: got %d, expected %d", len(embeddings), len(chunks))
	}

	// 3. Insert into vector store.
	items := make([]*VectorInsertItem, len(chunks))
	for i, c := range chunks {
		meta := map[string]interface{}{
			"document_id": c.DocumentID,
			"source":      string(c.Source),
			"content":     c.Content,
			"chunk_index": c.Index,
		}
		for k, v := range c.Metadata {
			meta[k] = v
		}
		items[i] = &VectorInsertItem{
			ID:       c.ChunkID,
			Vector:   embeddings[i],
			Metadata: meta,
		}
	}

	if err := r.vectorStore.BatchInsert(ctx, items); err != nil {
		return fmt.Errorf("inserting chunks for document %s: %w", doc.DocumentID, err)
	}

	elapsed := float64(time.Since(start).Milliseconds())
	r.metrics.RecordIndexLatency(ctx, elapsed)
	r.logger.Info("indexed document", "document_id", doc.DocumentID, "chunks", len(chunks), "duration_ms", elapsed)
	return nil
}

// ---------------------------------------------------------------------------
// IndexBatch
// ---------------------------------------------------------------------------

func (r *ragEngineImpl) IndexBatch(ctx context.Context, docs []*Document) error {
	if len(docs) == 0 {
		return nil
	}

	var (
		mu     sync.Mutex
		errs   []error
		wg     sync.WaitGroup
		sem    = make(chan struct{}, r.config.IndexBatchSize)
	)
	if r.config.IndexBatchSize <= 0 {
		sem = make(chan struct{}, 8)
	}

	for _, doc := range docs {
		wg.Add(1)
		sem <- struct{}{}
		go func(d *Document) {
			defer wg.Done()
			defer func() { <-sem }()
			if err := r.IndexDocument(ctx, d); err != nil {
				mu.Lock()
				errs = append(errs, fmt.Errorf("document %s: %w", d.DocumentID, err))
				mu.Unlock()
			}
		}(doc)
	}
	wg.Wait()

	if len(errs) > 0 {
		msgs := make([]string, len(errs))
		for i, e := range errs {
			msgs[i] = e.Error()
		}
		return fmt.Errorf("batch index partial failure (%d/%d): %s", len(errs), len(docs), strings.Join(msgs, "; "))
	}
	return nil
}

// ---------------------------------------------------------------------------
// DeleteDocument
// ---------------------------------------------------------------------------

func (r *ragEngineImpl) DeleteDocument(ctx context.Context, docID string) error {
	if docID == "" {
		return errors.NewInvalidInputError("docID is required")
	}
	if err := r.vectorStore.Delete(ctx, docID); err != nil {
		return fmt.Errorf("deleting document %s: %w", docID, err)
	}
	r.logger.Info("deleted document index", "document_id", docID)
	return nil
}

// ---------------------------------------------------------------------------
// Built-in DocumentChunker: defaultDocumentChunker
// ---------------------------------------------------------------------------

type defaultDocumentChunker struct {
	chunkSize    int
	chunkOverlap int
}

// NewDefaultDocumentChunker creates a chunker with the given size and overlap (in estimated tokens).
func NewDefaultDocumentChunker(chunkSize, chunkOverlap int) DocumentChunker {
	if chunkSize <= 0 {
		chunkSize = 512
	}
	if chunkOverlap < 0 {
		chunkOverlap = 0
	}
	if chunkOverlap >= chunkSize {
		chunkOverlap = chunkSize / 4
	}
	return &defaultDocumentChunker{
		chunkSize:    chunkSize,
		chunkOverlap: chunkOverlap,
	}
}

func (d *defaultDocumentChunker) Chunk(doc *Document) ([]*DocumentChunk, error) {
	if doc == nil || doc.Content == "" {
		return []*DocumentChunk{}, nil
	}

	// Patent-specific: split claims individually.
	if doc.Source == SourcePatent {
		return d.chunkPatent(doc)
	}

	return d.chunkGeneric(doc)
}

func (d *defaultDocumentChunker) chunkPatent(doc *Document) ([]*DocumentChunk, error) {
	var chunks []*DocumentChunk

	// Try to split into claims section and description section.
	claimsStart := findSectionStart(doc.Content, []string{"Claims", "CLAIMS", "权利要求"})
	descriptionContent := doc.Content
	claimsContent := ""

	if claimsStart >= 0 {
		descriptionContent = doc.Content[:claimsStart]
		claimsContent = doc.Content[claimsStart:]
	}

	// Chunk description by paragraphs.
	descChunks := d.splitByParagraphs(descriptionContent, doc)
	for i, dc := range descChunks {
		dc.ChunkID = fmt.Sprintf("%s-desc-%d", doc.DocumentID, i)
		dc.Index = i
		if dc.Metadata == nil {
			dc.Metadata = make(map[string]string)
		}
		dc.Metadata["section"] = "description"
		chunks = append(chunks, dc)
	}

	// Chunk claims individually.
	if claimsContent != "" {
		claims := splitClaims(claimsContent)
		baseIdx := len(descChunks)
		for i, claim := range claims {
			claim = strings.TrimSpace(claim)
			if claim == "" {
				continue
			}
			meta := copyMetadata(doc.Metadata)
			meta["section"] = "claims"
			meta["claim_number"] = fmt.Sprintf("%d", i+1)
			chunks = append(chunks, &DocumentChunk{
				ChunkID:    fmt.Sprintf("%s-claim-%d", doc.DocumentID, i+1),
				DocumentID: doc.DocumentID,
				Content:    claim,
				Source:     doc.Source,
				Metadata:   meta,
				TokenCount: estimateTokens(claim),
				Index:      baseIdx + i,
			})
		}
	}

	if len(chunks) == 0 {
		chunks = []*DocumentChunk{}
	}
	return chunks, nil
}

func (d *defaultDocumentChunker) chunkGeneric(doc *Document) ([]*DocumentChunk, error) {
	paragraphChunks := d.splitByParagraphs(doc.Content, doc)
	if len(paragraphChunks) == 0 {
		return []*DocumentChunk{}, nil
	}
	for i, c := range paragraphChunks {
		c.ChunkID = fmt.Sprintf("%s-chunk-%d", doc.DocumentID, i)
		c.Index = i
	}
	return paragraphChunks, nil
}

func (d *defaultDocumentChunker) splitByParagraphs(text string, doc *Document) []*DocumentChunk {
	paragraphs := strings.Split(text, "\n\n")
	var chunks []*DocumentChunk
	var currentContent strings.Builder
	currentTokens := 0

	flush := func() {
		content := strings.TrimSpace(currentContent.String())
		if content == "" {
			return
		}
		chunks = append(chunks, &DocumentChunk{
			DocumentID: doc.DocumentID,
			Content:    content,
			Source:     doc.Source,
			Metadata:   copyMetadata(doc.Metadata),
			TokenCount: estimateTokens(content),
		})
		// Handle overlap: keep the tail of the current content.
		if d.chunkOverlap > 0 {
			overlapText := extractTailTokens(content, d.chunkOverlap)
			currentContent.Reset()
			currentContent.WriteString(overlapText)
			currentTokens = estimateTokens(overlapText)
		} else {
			currentContent.Reset()
			currentTokens = 0
		}
	}

	for _, para := range paragraphs {
		para = strings.TrimSpace(para)
		if para == "" {
			continue
		}
		paraTokens := estimateTokens(para)

		// If a single paragraph exceeds chunk size, split by sentences.
		if paraTokens > d.chunkSize {
			if currentTokens > 0 {
				flush()
			}
			sentenceChunks := d.splitBySentences(para, doc)
			chunks = append(chunks, sentenceChunks...)
			continue
		}

		if currentTokens+paraTokens > d.chunkSize {
			flush()
		}
		if currentContent.Len() > 0 {
			currentContent.WriteString("\n\n")
		}
		currentContent.WriteString(para)
		currentTokens += paraTokens
	}

	if currentContent.Len() > 0 {
		content := strings.TrimSpace(currentContent.String())
		if content != "" {
			chunks = append(chunks, &DocumentChunk{
				DocumentID: doc.DocumentID,
				Content:    content,
				Source:     doc.Source,
				Metadata:   copyMetadata(doc.Metadata),
				TokenCount: estimateTokens(content),
			})
		}
	}

	return chunks
}

func (d *defaultDocumentChunker) splitBySentences(text string, doc *Document) []*DocumentChunk {
	sentences := splitSentences(text)
	var chunks []*DocumentChunk
	var currentContent strings.Builder
	currentTokens := 0

	for _, sent := range sentences {
		sent = strings.TrimSpace(sent)
		if sent == "" {
			continue
		}
		sentTokens := estimateTokens(sent)

		if currentTokens+sentTokens > d.chunkSize && currentContent.Len() > 0 {
			content := strings.TrimSpace(currentContent.String())
			chunks = append(chunks, &DocumentChunk{
				DocumentID: doc.DocumentID,
				Content:    content,
				Source:     doc.Source,
				Metadata:   copyMetadata(doc.Metadata),
				TokenCount: estimateTokens(content),
			})
			if d.chunkOverlap > 0 {
				overlapText := extractTailTokens(content, d.chunkOverlap)
				currentContent.Reset()
				currentContent.WriteString(overlapText)
				currentTokens = estimateTokens(overlapText)
			} else {
				currentContent.Reset()
				currentTokens = 0
			}
		}

		if currentContent.Len() > 0 {
			currentContent.WriteString(" ")
		}
		currentContent.WriteString(sent)
		currentTokens += sentTokens
	}

	if currentContent.Len() > 0 {
		content := strings.TrimSpace(currentContent.String())
		if content != "" {
			chunks = append(chunks, &DocumentChunk{
				DocumentID: doc.DocumentID,
				Content:    content,
				Source:     doc.Source,
				Metadata:   copyMetadata(doc.Metadata),
				TokenCount: estimateTokens(content),
			})
		}
	}

	return chunks
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func (r *ragEngineImpl) buildVectorFilters(query *RAGQuery) map[string]interface{} {
	filters := make(map[string]interface{})
	if query.Filters == nil {
		return filters
	}
	f := query.Filters
	if f.DateRange != nil {
		filters["date_from"] = f.DateRange.From.Format(time.RFC3339)
		filters["date_to"] = f.DateRange.To.Format(time.RFC3339)
	}
	if len(f.Jurisdictions) > 0 {
		filters["jurisdictions"] = f.Jurisdictions
	}
	if len(f.PatentClassifications) > 0 {
		filters["patent_classifications"] = f.PatentClassifications
	}
	if len(f.DocumentTypes) > 0 {
		filters["document_types"] = f.DocumentTypes
	}
	if len(f.Assignees) > 0 {
		filters["assignees"] = f.Assignees
	}
	if len(f.ExcludeDocIDs) > 0 {
		filters["exclude_doc_ids"] = f.ExcludeDocIDs
	}
	if len(query.SourceTypes) > 0 {
		sources := make([]string, len(query.SourceTypes))
		for i, s := range query.SourceTypes {
			sources[i] = string(s)
		}
		filters["source_types"] = sources
	}
	return filters
}

func effectiveScore(chunk *RAGChunk) float64 {
	if chunk.RerankerScore > 0 {
		return chunk.RerankerScore
	}
	return chunk.Score
}

func formatSourceAnnotation(chunk *RAGChunk) string {
	switch chunk.Source {
	case SourcePatent:
		patentNo := chunk.Metadata["patent_number"]
		section := chunk.Metadata["section"]
		claimNum := chunk.Metadata["claim_number"]
		if patentNo == "" {
			patentNo = chunk.DocumentID
		}
		if claimNum != "" {
			return fmt.Sprintf("Patent %s, Claim %s", patentNo, claimNum)
		}
		if section != "" {
			return fmt.Sprintf("Patent %s, %s", patentNo, section)
		}
		return fmt.Sprintf("Patent %s", patentNo)

	case SourceCaseLaw:
		caseName := chunk.Metadata["case_name"]
		if caseName == "" {
			caseName = chunk.DocumentID
		}
		return fmt.Sprintf("Case: %s", caseName)

	case SourceExaminationGuideline:
		sectionRef := chunk.Metadata["section_ref"]
		if sectionRef != "" {
			return fmt.Sprintf("MPEP §%s", sectionRef)
		}
		return fmt.Sprintf("Examination Guideline: %s", chunk.DocumentID)

	case SourceScientificPaper:
		title := chunk.Metadata["title"]
		if title == "" {
			title = chunk.DocumentID
		}
		return fmt.Sprintf("Paper: %s", title)

	case SourceRegulatory:
		regRef := chunk.Metadata["regulation_ref"]
		if regRef != "" {
			return fmt.Sprintf("Regulation: %s", regRef)
		}
		return fmt.Sprintf("Regulatory: %s", chunk.DocumentID)

	default:
		return fmt.Sprintf("Source: %s (%s)", chunk.Source, chunk.DocumentID)
	}
}

// estimateTokens provides a rough token count. English ≈ 1 token per 4 chars;
// CJK ≈ 1 token per 1.5 chars. We use a blended heuristic.
func estimateTokens(text string) int {
	if text == "" {
		return 0
	}
	runeCount := utf8.RuneCountInString(text)
	byteCount := len(text)

	// If average bytes-per-rune > 2, assume CJK-heavy.
	if runeCount > 0 && float64(byteCount)/float64(runeCount) > 2.0 {
		return int(math.Ceil(float64(runeCount) / 1.5))
	}
	// English / Latin approximation.
	return int(math.Ceil(float64(byteCount) / 4.0))
}

func truncateTextToTokens(text string, maxTokens int) string {
	runes := []rune(text)
	// Rough: take proportional slice.
	totalTokens := estimateTokens(text)
	if totalTokens <= maxTokens {
		return text
	}
	ratio := float64(maxTokens) / float64(totalTokens)
	cutoff := int(float64(len(runes)) * ratio)
	if cutoff > len(runes) {
		cutoff = len(runes)
	}
	// Try to cut at a sentence boundary.
	truncated := string(runes[:cutoff])
	lastDot := strings.LastIndexAny(truncated, ".。!?")
	if lastDot > len(truncated)/2 {
		truncated = truncated[:lastDot+1]
	}
	return truncated + "..."
}

func extractTailTokens(text string, tokenCount int) string {
	runes := []rune(text)
	totalTokens := estimateTokens(text)
	if totalTokens <= tokenCount {
		return text
	}
	ratio := float64(tokenCount) / float64(totalTokens)
	startIdx := len(runes) - int(float64(len(runes))*ratio)
	if startIdx < 0 {
		startIdx = 0
	}
	// Try to start at a sentence boundary.
	tail := string(runes[startIdx:])
	firstDot := strings.IndexAny(tail, ".。!?")
	if firstDot >= 0 && firstDot < len(tail)/2 {
		tail = tail[firstDot+1:]
	}
	return strings.TrimSpace(tail)
}

func containsSource(sources []DocumentSourceType, target DocumentSourceType) bool {
	for _, s := range sources {
		if s == target {
			return true
		}
	}
	return false
}

func findSectionStart(text string, markers []string) int {
	for _, marker := range markers {
		idx := strings.Index(text, marker)
		if idx >= 0 {
			return idx
		}
	}
	return -1
}

func splitClaims(claimsText string) []string {
	// Try numbered claims: "1.", "2.", etc.
	var claims []string
	lines := strings.Split(claimsText, "\n")
	var current strings.Builder
	inClaim := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if inClaim && current.Len() > 0 {
				current.WriteString("\n")
			}
			continue
		}
		// Detect claim start: line starts with a digit followed by "." or "、"
		if len(trimmed) > 1 && isDigit(rune(trimmed[0])) && (trimmed[1] == '.' || trimmed[1] == 0xE3) {
			if current.Len() > 0 {
				claims = append(claims, current.String())
				current.Reset()
			}
			inClaim = true
		}
		if inClaim {
			if current.Len() > 0 {
				current.WriteString(" ")
			}
			current.WriteString(trimmed)
		}
	}
	if current.Len() > 0 {
		claims = append(claims, current.String())
	}

	// Fallback: if no numbered claims found, split by double newline.
	if len(claims) == 0 {
		parts := strings.Split(claimsText, "\n\n")
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p != "" {
				claims = append(claims, p)
			}
		}
	}

	return claims
}

func splitSentences(text string) []string {
	// Simple sentence splitter respecting common abbreviations.
	var sentences []string
	var current strings.Builder
	runes := []rune(text)

	for i := 0; i < len(runes); i++ {
		current.WriteRune(runes[i])
		if isSentenceEnd(runes, i) {
			sentences = append(sentences, current.String())
			current.Reset()
		}
	}
	if current.Len() > 0 {
		sentences = append(sentences, current.String())
	}
	return sentences
}

func isSentenceEnd(runes []rune, i int) bool {
	r := runes[i]
	if r != '.' && r != '。' && r != '!' && r != '?' {
		return false
	}
	// Check next char is space or end.
	if i+1 >= len(runes) {
		return true
	}
	next := runes[i+1]
	return next == ' ' || next == '\n' || next == '\t'
}

func isDigit(r rune) bool {
	return r >= '0' && r <= '9'
}

func copyMetadata(src map[string]string) map[string]string {
	if src == nil {
		return make(map[string]string)
	}
	dst := make(map[string]string, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

// ---------------------------------------------------------------------------
// Noop implementations
// ---------------------------------------------------------------------------

type noopRAGLogger struct{}

func (n *noopRAGLogger) Info(msg string, keysAndValues ...interface{})  {}
func (n *noopRAGLogger) Warn(msg string, keysAndValues ...interface{})  {}
func (n *noopRAGLogger) Error(msg string, keysAndValues ...interface{}) {}

type noopRAGMetrics struct{}

func (n *noopRAGMetrics) RecordRetrievalLatency(ctx context.Context, durationMs float64, source string) {}
func (n *noopRAGMetrics) RecordRerankLatency(ctx context.Context, durationMs float64)                   {}
func (n *noopRAGMetrics) RecordIndexLatency(ctx context.Context, durationMs float64)                    {}
func (n *noopRAGMetrics) RecordChunkCount(ctx context.Context, count int)                               {}


