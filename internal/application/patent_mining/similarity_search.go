// Phase 10 - File 218 of 349
// Phase: 应用层 - 业务服务
// SubModule: patent_mining
// File: internal/application/patent_mining/similarity_search.go
//
// Generation Plan:
// - 功能定位: 分子/专利相似性检索应用服务，支持结构相似性、指纹相似性、语义相似性多维检索
// - 核心实现:
//   - SimilaritySearchService 接口: SearchByStructure / SearchByFingerprint / SearchBySemantic / SearchByPatent / GetSearchHistory
//   - similaritySearchServiceImpl 结构体: 注入 FingerprintEngine, VectorStore, PatentIndex, SearchHistoryStore, Logger
//   - 结构相似性: 基于分子指纹 Tanimoto 系数计算
//   - 语义相似性: 基于向量嵌入的 cosine similarity
//   - 专利相似性: 结合结构+文本的混合检索
// - 依赖: pkg/errors, pkg/types
// - 被依赖: API handler, patent_mining workflow
// - 强制约束: 文件最后一行必须为 //Personal.AI order the ending

package patent_mining

import (
	"context"
	"fmt"
	"sort"
	"time"

	apperrors "github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

// SimilarityMetric defines the similarity metric type.
type SimilarityMetric string

const (
	MetricTanimoto SimilarityMetric = "tanimoto"
	MetricCosine   SimilarityMetric = "cosine"
	MetricDice     SimilarityMetric = "dice"
	MetricEuclidean SimilarityMetric = "euclidean"
)

// SimilaritySearchType defines the type of similarity search.
type SimilaritySearchType string

const (
	SearchTypeStructure   SimilaritySearchType = "structure"
	SearchTypeFingerprint SimilaritySearchType = "fingerprint"
	SearchTypeSemantic    SimilaritySearchType = "semantic"
	SearchTypeHybrid      SimilaritySearchType = "hybrid"
)

// SimilarityHit represents a single search result.
type SimilarityHit struct {
	ID         string           `json:"id"`
	Type       string           `json:"type"` // "molecule", "patent"
	SMILES     string           `json:"smiles,omitempty"`
	InChIKey   string           `json:"inchi_key,omitempty"`
	Name       string           `json:"name,omitempty"`
	PatentNum  string           `json:"patent_number,omitempty"`
	Score      float64          `json:"score"`
	Metric     SimilarityMetric `json:"metric"`
	Highlights []string         `json:"highlights,omitempty"`
}

// SimilaritySearchResult holds the full search result.
type SimilaritySearchResult struct {
	QueryID          string              `json:"query_id"`
	SearchType       SimilaritySearchType `json:"search_type"`
	Hits             []SimilarityHit     `json:"hits"`
	TotalHits        int                 `json:"total_hits"`
	ProcessingTimeMs int64               `json:"processing_time_ms"`
	Metadata         map[string]string   `json:"metadata,omitempty"`
}

// SearchByStructureRequest is the request for structure-based search.
type SearchByStructureRequest struct {
	SMILES       string           `json:"smiles"`
	Metric       SimilarityMetric `json:"metric"`
	Threshold    float64          `json:"threshold"`
	MaxResults   int              `json:"max_results"`
	TargetDBs    []string         `json:"target_dbs,omitempty"`
}

// SearchByFingerprintRequest is the request for fingerprint-based search.
type SearchByFingerprintRequest struct {
	SMILES         string           `json:"smiles"`
	FingerprintType string          `json:"fingerprint_type"` // "morgan", "maccs", "topological"
	Radius         int              `json:"radius,omitempty"`
	NBits          int              `json:"n_bits,omitempty"`
	Metric         SimilarityMetric `json:"metric"`
	Threshold      float64          `json:"threshold"`
	MaxResults     int              `json:"max_results"`
}

// SearchBySemanticRequest is the request for semantic similarity search.
type SearchBySemanticRequest struct {
	Query      string  `json:"query"`
	EmbedModel string  `json:"embed_model,omitempty"`
	Threshold  float64 `json:"threshold"`
	MaxResults int     `json:"max_results"`
	Filters    map[string]string `json:"filters,omitempty"`
}

// SearchByPatentRequest is the request for patent-based hybrid search.
type SearchByPatentRequest struct {
	PatentID       string  `json:"patent_id"`
	StructureWeight float64 `json:"structure_weight"` // 0.0-1.0
	TextWeight     float64  `json:"text_weight"`      // 0.0-1.0
	Threshold      float64  `json:"threshold"`
	MaxResults     int      `json:"max_results"`
}

// SearchHistoryEntry records a past search.
type SearchHistoryEntry struct {
	QueryID    string               `json:"query_id"`
	SearchType SimilaritySearchType `json:"search_type"`
	Query      string               `json:"query"`
	HitCount   int                  `json:"hit_count"`
	CreatedAt  time.Time            `json:"created_at"`
}

// ---------------------------------------------------------------------------
// Port interfaces
// ---------------------------------------------------------------------------

// FingerprintEngine computes molecular fingerprints and similarity.
type FingerprintEngine interface {
	ComputeFingerprint(ctx context.Context, smiles string, fpType string, radius int, nBits int) ([]byte, error)
	ComputeSimilarity(ctx context.Context, fp1 []byte, fp2 []byte, metric SimilarityMetric) (float64, error)
	SearchSimilar(ctx context.Context, queryFP []byte, metric SimilarityMetric, threshold float64, maxResults int) ([]SimilarityHit, error)
}

// VectorStore provides vector-based similarity search.
type VectorStore interface {
	SearchByVector(ctx context.Context, vector []float64, threshold float64, maxResults int, filters map[string]string) ([]SimilarityHit, error)
	EmbedText(ctx context.Context, text string, model string) ([]float64, error)
	EmbedMolecule(ctx context.Context, smiles string) ([]float64, error)
}

// PatentIndexForSearch provides patent-level search capabilities.
type PatentIndexForSearch interface {
	GetPatentMolecules(ctx context.Context, patentID string) ([]string, error) // returns SMILES list
	GetPatentText(ctx context.Context, patentID string) (string, error)
	SearchByText(ctx context.Context, query string, maxResults int) ([]SimilarityHit, error)
}

// SearchHistoryStore persists search history.
type SearchHistoryStore interface {
	Save(ctx context.Context, entry *SearchHistoryEntry) error
	ListByUser(ctx context.Context, userID string, limit int) ([]SearchHistoryEntry, error)
}

// SimilaritySearchLogger abstracts logging.
type SimilaritySearchLogger interface {
	Info(msg string, fields ...interface{})
	Error(msg string, fields ...interface{})
	Warn(msg string, fields ...interface{})
	Debug(msg string, fields ...interface{})
}

// ---------------------------------------------------------------------------
// Service interface
// ---------------------------------------------------------------------------

// SimilaritySearchService provides similarity search capabilities.
type SimilaritySearchService interface {
	SearchByStructure(ctx context.Context, req *SearchByStructureRequest) (*SimilaritySearchResult, error)
	SearchByFingerprint(ctx context.Context, req *SearchByFingerprintRequest) (*SimilaritySearchResult, error)
	SearchBySemantic(ctx context.Context, req *SearchBySemanticRequest) (*SimilaritySearchResult, error)
	SearchByPatent(ctx context.Context, req *SearchByPatentRequest) (*SimilaritySearchResult, error)
	GetSearchHistory(ctx context.Context, userID string, limit int) ([]SearchHistoryEntry, error)
}

// ---------------------------------------------------------------------------
// Dependencies
// ---------------------------------------------------------------------------

// SimilaritySearchDeps holds all dependencies.
type SimilaritySearchDeps struct {
	FPEngine     FingerprintEngine
	VectorStore  VectorStore
	PatentIndex  PatentIndexForSearch
	HistoryStore SearchHistoryStore
	Logger       SimilaritySearchLogger
}

// ---------------------------------------------------------------------------
// Implementation
// ---------------------------------------------------------------------------

type similaritySearchServiceImpl struct {
	fpEngine     FingerprintEngine
	vectorStore  VectorStore
	patentIndex  PatentIndexForSearch
	historyStore SearchHistoryStore
	logger       SimilaritySearchLogger
}

// NewSimilaritySearchService creates a new SimilaritySearchService.
func NewSimilaritySearchService(deps SimilaritySearchDeps) SimilaritySearchService {
	return &similaritySearchServiceImpl{
		fpEngine:     deps.FPEngine,
		vectorStore:  deps.VectorStore,
		patentIndex:  deps.PatentIndex,
		historyStore: deps.HistoryStore,
		logger:       deps.Logger,
	}
}

func (s *similaritySearchServiceImpl) SearchByStructure(ctx context.Context, req *SearchByStructureRequest) (*SimilaritySearchResult, error) {
	if req == nil {
		return nil, apperrors.NewValidationError("request", "request cannot be nil")
	}
	if req.SMILES == "" {
		return nil, apperrors.NewValidationError("smiles", "SMILES is required")
	}

	startTime := time.Now()
	s.logger.Info("structure similarity search", "smiles", req.SMILES, "metric", req.Metric)

	metric := req.Metric
	if metric == "" {
		metric = MetricTanimoto
	}
	threshold := req.Threshold
	if threshold <= 0 {
		threshold = 0.70
	}
	maxResults := req.MaxResults
	if maxResults <= 0 {
		maxResults = 50
	}

	// Compute query fingerprint using default Morgan FP
	queryFP, err := s.fpEngine.ComputeFingerprint(ctx, req.SMILES, "morgan", 2, 2048)
	if err != nil {
		return nil, fmt.Errorf("compute fingerprint: %w", err)
	}

	hits, err := s.fpEngine.SearchSimilar(ctx, queryFP, metric, threshold, maxResults)
	if err != nil {
		return nil, fmt.Errorf("similarity search: %w", err)
	}

	queryID := generateSearchQueryID()
	result := &SimilaritySearchResult{
		QueryID:          queryID,
		SearchType:       SearchTypeStructure,
		Hits:             hits,
		TotalHits:        len(hits),
		ProcessingTimeMs: time.Since(startTime).Milliseconds(),
	}

	s.recordHistory(ctx, queryID, SearchTypeStructure, req.SMILES, len(hits))
	return result, nil
}

func (s *similaritySearchServiceImpl) SearchByFingerprint(ctx context.Context, req *SearchByFingerprintRequest) (*SimilaritySearchResult, error) {
	if req == nil {
		return nil, apperrors.NewValidationError("request", "request cannot be nil")
	}
	if req.SMILES == "" {
		return nil, apperrors.NewValidationError("smiles", "SMILES is required")
	}

	startTime := time.Now()

	fpType := req.FingerprintType
	if fpType == "" {
		fpType = "morgan"
	}
	radius := req.Radius
	if radius <= 0 {
		radius = 2
	}
	nBits := req.NBits
	if nBits <= 0 {
		nBits = 2048
	}
	metric := req.Metric
	if metric == "" {
		metric = MetricTanimoto
	}
	threshold := req.Threshold
	if threshold <= 0 {
		threshold = 0.70
	}
	maxResults := req.MaxResults
	if maxResults <= 0 {
		maxResults = 50
	}

	s.logger.Info("fingerprint similarity search", "smiles", req.SMILES, "fp_type", fpType, "metric", metric)

	queryFP, err := s.fpEngine.ComputeFingerprint(ctx, req.SMILES, fpType, radius, nBits)
	if err != nil {
		return nil, fmt.Errorf("compute fingerprint: %w", err)
	}

	hits, err := s.fpEngine.SearchSimilar(ctx, queryFP, metric, threshold, maxResults)
	if err != nil {
		return nil, fmt.Errorf("fingerprint search: %w", err)
	}

	queryID := generateSearchQueryID()
	result := &SimilaritySearchResult{
		QueryID:          queryID,
		SearchType:       SearchTypeFingerprint,
		Hits:             hits,
		TotalHits:        len(hits),
		ProcessingTimeMs: time.Since(startTime).Milliseconds(),
		Metadata: map[string]string{
			"fingerprint_type": fpType,
			"radius":           fmt.Sprintf("%d", radius),
			"n_bits":           fmt.Sprintf("%d", nBits),
		},
	}

	s.recordHistory(ctx, queryID, SearchTypeFingerprint, req.SMILES, len(hits))
	return result, nil
}

func (s *similaritySearchServiceImpl) SearchBySemantic(ctx context.Context, req *SearchBySemanticRequest) (*SimilaritySearchResult, error) {
	if req == nil {
		return nil, apperrors.NewValidationError("request", "request cannot be nil")
	}
	if req.Query == "" {
		return nil, apperrors.NewValidationError("query", "query is required")
	}

	startTime := time.Now()

	threshold := req.Threshold
	if threshold <= 0 {
		threshold = 0.60
	}
	maxResults := req.MaxResults
	if maxResults <= 0 {
		maxResults = 50
	}
	embedModel := req.EmbedModel
	if embedModel == "" {
		embedModel = "default"
	}

	s.logger.Info("semantic similarity search", "query_len", len(req.Query), "model", embedModel)

	vector, err := s.vectorStore.EmbedText(ctx, req.Query, embedModel)
	if err != nil {
		return nil, fmt.Errorf("embed query: %w", err)
	}

	hits, err := s.vectorStore.SearchByVector(ctx, vector, threshold, maxResults, req.Filters)
	if err != nil {
		return nil, fmt.Errorf("vector search: %w", err)
	}

	queryID := generateSearchQueryID()
	result := &SimilaritySearchResult{
		QueryID:          queryID,
		SearchType:       SearchTypeSemantic,
		Hits:             hits,
		TotalHits:        len(hits),
		ProcessingTimeMs: time.Since(startTime).Milliseconds(),
		Metadata: map[string]string{
			"embed_model": embedModel,
		},
	}

	s.recordHistory(ctx, queryID, SearchTypeSemantic, req.Query, len(hits))
	return result, nil
}

func (s *similaritySearchServiceImpl) SearchByPatent(ctx context.Context, req *SearchByPatentRequest) (*SimilaritySearchResult, error) {
	if req == nil {
		return nil, apperrors.NewValidationError("request", "request cannot be nil")
	}
	if req.PatentID == "" {
		return nil, apperrors.NewValidationError("patent_id", "patent ID is required")
	}

	startTime := time.Now()

	structWeight := req.StructureWeight
	textWeight := req.TextWeight
	if structWeight <= 0 && textWeight <= 0 {
		structWeight = 0.5
		textWeight = 0.5
	}
	// Normalize weights
	totalWeight := structWeight + textWeight
	if totalWeight > 0 {
		structWeight = structWeight / totalWeight
		textWeight = textWeight / totalWeight
	}

	threshold := req.Threshold
	if threshold <= 0 {
		threshold = 0.60
	}
	maxResults := req.MaxResults
	if maxResults <= 0 {
		maxResults = 50
	}

	s.logger.Info("patent hybrid similarity search", "patent_id", req.PatentID, "struct_w", structWeight, "text_w", textWeight)

	// Collect structure-based hits
	var structHits []SimilarityHit
	if structWeight > 0 {
		smilesList, err := s.patentIndex.GetPatentMolecules(ctx, req.PatentID)
		if err != nil {
			s.logger.Warn("failed to get patent molecules", "error", err)
		} else {
			for _, smi := range smilesList {
				fp, fpErr := s.fpEngine.ComputeFingerprint(ctx, smi, "morgan", 2, 2048)
				if fpErr != nil {
					continue
				}
				hits, searchErr := s.fpEngine.SearchSimilar(ctx, fp, MetricTanimoto, threshold, maxResults)
				if searchErr != nil {
					continue
				}
				structHits = append(structHits, hits...)
			}
		}
	}

	// Collect text-based hits
	var textHits []SimilarityHit
	if textWeight > 0 {
		patentText, err := s.patentIndex.GetPatentText(ctx, req.PatentID)
		if err != nil {
			s.logger.Warn("failed to get patent text", "error", err)
		} else {
			hits, searchErr := s.patentIndex.SearchByText(ctx, patentText, maxResults)
			if searchErr != nil {
				s.logger.Warn("text search failed", "error", searchErr)
			} else {
				textHits = hits
			}
		}
	}

	// Merge and re-rank
	merged := mergeAndRankHits(structHits, textHits, structWeight, textWeight, threshold, maxResults)

	queryID := generateSearchQueryID()
	result := &SimilaritySearchResult{
		QueryID:          queryID,
		SearchType:       SearchTypeHybrid,
		Hits:             merged,
		TotalHits:        len(merged),
		ProcessingTimeMs: time.Since(startTime).Milliseconds(),
		Metadata: map[string]string{
			"patent_id":        req.PatentID,
			"structure_weight": fmt.Sprintf("%.2f", structWeight),
			"text_weight":      fmt.Sprintf("%.2f", textWeight),
		},
	}

	s.recordHistory(ctx, queryID, SearchTypeHybrid, req.PatentID, len(merged))
	return result, nil
}

func (s *similaritySearchServiceImpl) GetSearchHistory(ctx context.Context, userID string, limit int) ([]SearchHistoryEntry, error) {
	if userID == "" {
		return nil, apperrors.NewValidationError("user_id", "user ID is required")
	}
	if limit <= 0 {
		limit = 20
	}
	return s.historyStore.ListByUser(ctx, userID, limit)
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

func (s *similaritySearchServiceImpl) recordHistory(ctx context.Context, queryID string, searchType SimilaritySearchType, query string, hitCount int) {
	entry := &SearchHistoryEntry{
		QueryID:    queryID,
		SearchType: searchType,
		Query:      truncateString(query, 500),
		HitCount:   hitCount,
		CreatedAt:  time.Now(),
	}
	if err := s.historyStore.Save(ctx, entry); err != nil {
		s.logger.Warn("failed to save search history", "error", err)
	}
}

func mergeAndRankHits(structHits, textHits []SimilarityHit, structWeight, textWeight, threshold float64, maxResults int) []SimilarityHit {
	scoreMap := make(map[string]float64)
	hitMap := make(map[string]SimilarityHit)

	for _, h := range structHits {
		key := h.ID
		scoreMap[key] += h.Score * structWeight
		if _, exists := hitMap[key]; !exists {
			hitMap[key] = h
		}
	}

	for _, h := range textHits {
		key := h.ID
		scoreMap[key] += h.Score * textWeight
		if _, exists := hitMap[key]; !exists {
			hitMap[key] = h
		}
	}

	var merged []SimilarityHit
	for id, score := range scoreMap {
		if score >= threshold {
			hit := hitMap[id]
			hit.Score = score
			merged = append(merged, hit)
		}
	}

	sort.Slice(merged, func(i, j int) bool {
		return merged[i].Score > merged[j].Score
	})

	if len(merged) > maxResults {
		merged = merged[:maxResults]
	}

	return merged
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}

var searchQueryIDCounter int64

func generateSearchQueryID() string {
	searchQueryIDCounter++
	return fmt.Sprintf("sq-%d-%d", time.Now().UnixMilli(), searchQueryIDCounter)
}

//Personal.AI order the ending
