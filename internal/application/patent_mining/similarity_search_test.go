// Phase 10 - File 219 of 349
// Phase: 应用层 - 业务服务
// SubModule: patent_mining
// File: internal/application/patent_mining/similarity_search_test.go

package patent_mining

import (
	"context"
	"errors"
	"fmt"
	"testing"

	apperrors "github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// ---------------------------------------------------------------------------
// Mocks
// ---------------------------------------------------------------------------

type mockFingerprintEngine struct {
	computeFingerprintFn func(ctx context.Context, smiles string, fpType string, radius int, nBits int) ([]byte, error)
	computeSimilarityFn  func(ctx context.Context, fp1 []byte, fp2 []byte, metric SimilarityMetric) (float64, error)
	searchSimilarFn      func(ctx context.Context, queryFP []byte, metric SimilarityMetric, threshold float64, maxResults int) ([]SimilarityHit, error)
}

func (m *mockFingerprintEngine) ComputeFingerprint(ctx context.Context, smiles string, fpType string, radius int, nBits int) ([]byte, error) {
	if m.computeFingerprintFn != nil {
		return m.computeFingerprintFn(ctx, smiles, fpType, radius, nBits)
	}
	return []byte{0x01, 0x02, 0x03}, nil
}

func (m *mockFingerprintEngine) ComputeSimilarity(ctx context.Context, fp1 []byte, fp2 []byte, metric SimilarityMetric) (float64, error) {
	if m.computeSimilarityFn != nil {
		return m.computeSimilarityFn(ctx, fp1, fp2, metric)
	}
	return 0.85, nil
}

func (m *mockFingerprintEngine) SearchSimilar(ctx context.Context, queryFP []byte, metric SimilarityMetric, threshold float64, maxResults int) ([]SimilarityHit, error) {
	if m.searchSimilarFn != nil {
		return m.searchSimilarFn(ctx, queryFP, metric, threshold, maxResults)
	}
	return []SimilarityHit{
		{ID: "mol-001", Type: "molecule", SMILES: "c1ccc2c(c1)[nH]c1ccccc12", Score: 0.92, Metric: MetricTanimoto},
		{ID: "mol-002", Type: "molecule", SMILES: "c1ccc(-c2ccccc2)cc1", Score: 0.78, Metric: MetricTanimoto},
	}, nil
}

type mockVectorStore struct {
	searchByVectorFn func(ctx context.Context, vector []float64, threshold float64, maxResults int, filters map[string]string) ([]SimilarityHit, error)
	embedTextFn      func(ctx context.Context, text string, model string) ([]float64, error)
	embedMoleculeFn  func(ctx context.Context, smiles string) ([]float64, error)
}

func (m *mockVectorStore) SearchByVector(ctx context.Context, vector []float64, threshold float64, maxResults int, filters map[string]string) ([]SimilarityHit, error) {
	if m.searchByVectorFn != nil {
		return m.searchByVectorFn(ctx, vector, threshold, maxResults, filters)
	}
	return []SimilarityHit{
		{ID: "pat-001", Type: "patent", PatentNum: "CN115000001A", Score: 0.88, Metric: MetricCosine},
	}, nil
}

func (m *mockVectorStore) EmbedText(ctx context.Context, text string, model string) ([]float64, error) {
	if m.embedTextFn != nil {
		return m.embedTextFn(ctx, text, model)
	}
	return []float64{0.1, 0.2, 0.3, 0.4}, nil
}

func (m *mockVectorStore) EmbedMolecule(ctx context.Context, smiles string) ([]float64, error) {
	if m.embedMoleculeFn != nil {
		return m.embedMoleculeFn(ctx, smiles)
	}
	return []float64{0.5, 0.6, 0.7, 0.8}, nil
}

type mockPatentIndexForSearch struct {
	getPatentMoleculesFn func(ctx context.Context, patentID string) ([]string, error)
	getPatentTextFn      func(ctx context.Context, patentID string) (string, error)
	searchByTextFn       func(ctx context.Context, query string, maxResults int) ([]SimilarityHit, error)
}

func (m *mockPatentIndexForSearch) GetPatentMolecules(ctx context.Context, patentID string) ([]string, error) {
	if m.getPatentMoleculesFn != nil {
		return m.getPatentMoleculesFn(ctx, patentID)
	}
	return []string{"c1ccccc1"}, nil
}

func (m *mockPatentIndexForSearch) GetPatentText(ctx context.Context, patentID string) (string, error) {
	if m.getPatentTextFn != nil {
		return m.getPatentTextFn(ctx, patentID)
	}
	return "patent text content", nil
}

func (m *mockPatentIndexForSearch) SearchByText(ctx context.Context, query string, maxResults int) ([]SimilarityHit, error) {
	if m.searchByTextFn != nil {
		return m.searchByTextFn(ctx, query, maxResults)
	}
	return []SimilarityHit{
		{ID: "pat-010", Type: "patent", PatentNum: "CN116000001A", Score: 0.75, Metric: MetricCosine},
	}, nil
}

type mockSearchHistoryStore struct {
	saveFn       func(ctx context.Context, entry *SearchHistoryEntry) error
	listByUserFn func(ctx context.Context, userID string, limit int) ([]SearchHistoryEntry, error)
}

func (m *mockSearchHistoryStore) Save(ctx context.Context, entry *SearchHistoryEntry) error {
	if m.saveFn != nil {
		return m.saveFn(ctx, entry)
	}
	return nil
}

func (m *mockSearchHistoryStore) ListByUser(ctx context.Context, userID string, limit int) ([]SearchHistoryEntry, error) {
	if m.listByUserFn != nil {
		return m.listByUserFn(ctx, userID, limit)
	}
	return nil, nil
}

type mockSimSearchLogger struct{}

func (m *mockSimSearchLogger) Info(msg string, fields ...interface{})  {}
func (m *mockSimSearchLogger) Error(msg string, fields ...interface{}) {}
func (m *mockSimSearchLogger) Warn(msg string, fields ...interface{})  {}
func (m *mockSimSearchLogger) Debug(msg string, fields ...interface{}) {}

// ---------------------------------------------------------------------------
// Helper
// ---------------------------------------------------------------------------

func newTestSimilaritySearchService(
	fp FingerprintEngine,
	vs VectorStore,
	pi PatentIndexForSearch,
	hs SearchHistoryStore,
) SimilaritySearchService {
	return NewSimilaritySearchService(SimilaritySearchDeps{
		FPEngine:     fp,
		VectorStore:  vs,
		PatentIndex:  pi,
		HistoryStore: hs,
		Logger:       &mockSimSearchLogger{},
	})
}

// ===========================================================================
// Tests: SearchByStructure
// ===========================================================================

func TestSearchByStructure_Success(t *testing.T) {
	svc := newTestSimilaritySearchService(
		&mockFingerprintEngine{},
		&mockVectorStore{},
		&mockPatentIndexForSearch{},
		&mockSearchHistoryStore{},
	)

	req := &SearchByStructureRequest{
		SMILES:     "c1ccc2c(c1)[nH]c1ccccc12",
		Metric:     MetricTanimoto,
		Threshold:  0.70,
		MaxResults: 10,
	}

	result, err := svc.SearchByStructure(context.Background(), req)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.SearchType != SearchTypeStructure {
		t.Errorf("expected search type structure, got %s", result.SearchType)
	}
	if len(result.Hits) != 2 {
		t.Errorf("expected 2 hits, got %d", len(result.Hits))
	}
	if result.QueryID == "" {
		t.Error("expected non-empty query ID")
	}
	if result.ProcessingTimeMs < 0 {
		t.Error("expected non-negative processing time")
	}
}

func TestSearchByStructure_NilRequest(t *testing.T) {
	svc := newTestSimilaritySearchService(
		&mockFingerprintEngine{},
		&mockVectorStore{},
		&mockPatentIndexForSearch{},
		&mockSearchHistoryStore{},
	)

	_, err := svc.SearchByStructure(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil request")
	}
}

func TestSearchByStructure_EmptySMILES(t *testing.T) {
	svc := newTestSimilaritySearchService(
		&mockFingerprintEngine{},
		&mockVectorStore{},
		&mockPatentIndexForSearch{},
		&mockSearchHistoryStore{},
	)

	req := &SearchByStructureRequest{SMILES: ""}
	_, err := svc.SearchByStructure(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for empty SMILES")
	}
	if !apperrors.IsValidation(err) {
		t.Errorf("expected ValidationError, got: %v", err)
	}
}

func TestSearchByStructure_FingerprintError(t *testing.T) {
	fp := &mockFingerprintEngine{
		computeFingerprintFn: func(ctx context.Context, smiles string, fpType string, radius int, nBits int) ([]byte, error) {
			return nil, errors.New("invalid SMILES")
		},
	}

	svc := newTestSimilaritySearchService(fp, &mockVectorStore{}, &mockPatentIndexForSearch{}, &mockSearchHistoryStore{})

	req := &SearchByStructureRequest{SMILES: "INVALID"}
	_, err := svc.SearchByStructure(context.Background(), req)
	if err == nil {
		t.Fatal("expected error from fingerprint computation")
	}
}

func TestSearchByStructure_SearchError(t *testing.T) {
	fp := &mockFingerprintEngine{
		searchSimilarFn: func(ctx context.Context, queryFP []byte, metric SimilarityMetric, threshold float64, maxResults int) ([]SimilarityHit, error) {
			return nil, errors.New("search engine down")
		},
	}

	svc := newTestSimilaritySearchService(fp, &mockVectorStore{}, &mockPatentIndexForSearch{}, &mockSearchHistoryStore{})

	req := &SearchByStructureRequest{SMILES: "c1ccccc1"}
	_, err := svc.SearchByStructure(context.Background(), req)
	if err == nil {
		t.Fatal("expected error from search failure")
	}
}

func TestSearchByStructure_DefaultParams(t *testing.T) {
	var capturedMetric SimilarityMetric
	var capturedThreshold float64
	var capturedMax int

	fp := &mockFingerprintEngine{
		searchSimilarFn: func(ctx context.Context, queryFP []byte, metric SimilarityMetric, threshold float64, maxResults int) ([]SimilarityHit, error) {
			capturedMetric = metric
			capturedThreshold = threshold
			capturedMax = maxResults
			return nil, nil
		},
	}

	svc := newTestSimilaritySearchService(fp, &mockVectorStore{}, &mockPatentIndexForSearch{}, &mockSearchHistoryStore{})

	req := &SearchByStructureRequest{SMILES: "c1ccccc1"}
	_, err := svc.SearchByStructure(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedMetric != MetricTanimoto {
		t.Errorf("expected default metric tanimoto, got %s", capturedMetric)
	}
	if capturedThreshold != 0.70 {
		t.Errorf("expected default threshold 0.70, got %f", capturedThreshold)
	}
	if capturedMax != 50 {
		t.Errorf("expected default max 50, got %d", capturedMax)
	}
}

func TestSearchByStructure_NoHits(t *testing.T) {
	fp := &mockFingerprintEngine{
		searchSimilarFn: func(ctx context.Context, queryFP []byte, metric SimilarityMetric, threshold float64, maxResults int) ([]SimilarityHit, error) {
			return []SimilarityHit{}, nil
		},
	}

	svc := newTestSimilaritySearchService(fp, &mockVectorStore{}, &mockPatentIndexForSearch{}, &mockSearchHistoryStore{})

	req := &SearchByStructureRequest{SMILES: "c1ccccc1", Threshold: 0.99}
	result, err := svc.SearchByStructure(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.TotalHits != 0 {
		t.Errorf("expected 0 hits, got %d", result.TotalHits)
	}
}

// ===========================================================================
// Tests: SearchByFingerprint
// ===========================================================================

func TestSearchByFingerprint_Success(t *testing.T) {
	svc := newTestSimilaritySearchService(
		&mockFingerprintEngine{},
		&mockVectorStore{},
		&mockPatentIndexForSearch{},
		&mockSearchHistoryStore{},
	)

	req := &SearchByFingerprintRequest{
		SMILES:          "c1ccc2c(c1)[nH]c1ccccc12",
		FingerprintType: "maccs",
		Radius:          3,
		NBits:           1024,
		Metric:          MetricDice,
		Threshold:       0.65,
		MaxResults:      20,
	}

	result, err := svc.SearchByFingerprint(context.Background(), req)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if result.SearchType != SearchTypeFingerprint {
		t.Errorf("expected search type fingerprint, got %s", result.SearchType)
	}
	if result.Metadata["fingerprint_type"] != "maccs" {
		t.Errorf("expected metadata fp_type maccs, got %s", result.Metadata["fingerprint_type"])
	}
}

func TestSearchByFingerprint_NilRequest(t *testing.T) {
	svc := newTestSimilaritySearchService(&mockFingerprintEngine{}, &mockVectorStore{}, &mockPatentIndexForSearch{}, &mockSearchHistoryStore{})

	_, err := svc.SearchByFingerprint(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil request")
	}
}

func TestSearchByFingerprint_EmptySMILES(t *testing.T) {
	svc := newTestSimilaritySearchService(&mockFingerprintEngine{}, &mockVectorStore{}, &mockPatentIndexForSearch{}, &mockSearchHistoryStore{})

	req := &SearchByFingerprintRequest{SMILES: ""}
	_, err := svc.SearchByFingerprint(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for empty SMILES")
	}
}

func TestSearchByFingerprint_DefaultParams(t *testing.T) {
	var capturedFPType string
	var capturedRadius, capturedNBits int

	fp := &mockFingerprintEngine{
		computeFingerprintFn: func(ctx context.Context, smiles string, fpType string, radius int, nBits int) ([]byte, error) {
			capturedFPType = fpType
			capturedRadius = radius
			capturedNBits = nBits
			return []byte{0x01}, nil
		},
	}

	svc := newTestSimilaritySearchService(fp, &mockVectorStore{}, &mockPatentIndexForSearch{}, &mockSearchHistoryStore{})

	req := &SearchByFingerprintRequest{SMILES: "c1ccccc1"}
	_, err := svc.SearchByFingerprint(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedFPType != "morgan" {
		t.Errorf("expected default fp_type morgan, got %s", capturedFPType)
	}
	if capturedRadius != 2 {
		t.Errorf("expected default radius 2, got %d", capturedRadius)
	}
	if capturedNBits != 2048 {
		t.Errorf("expected default nBits 2048, got %d", capturedNBits)
	}
}

// ===========================================================================
// Tests: SearchBySemantic
// ===========================================================================

func TestSearchBySemantic_Success(t *testing.T) {
	svc := newTestSimilaritySearchService(
		&mockFingerprintEngine{},
		&mockVectorStore{},
		&mockPatentIndexForSearch{},
		&mockSearchHistoryStore{},
	)

	req := &SearchBySemanticRequest{
		Query:      "carbazole-based OLED host materials with high triplet energy",
		EmbedModel: "sci-bert",
		Threshold:  0.70,
		MaxResults: 20,
	}

	result, err := svc.SearchBySemantic(context.Background(), req)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if result.SearchType != SearchTypeSemantic {
		t.Errorf("expected search type semantic, got %s", result.SearchType)
	}
	if result.Metadata["embed_model"] != "sci-bert" {
		t.Errorf("expected embed_model sci-bert, got %s", result.Metadata["embed_model"])
	}
}

func TestSearchBySemantic_NilRequest(t *testing.T) {
	svc := newTestSimilaritySearchService(&mockFingerprintEngine{}, &mockVectorStore{}, &mockPatentIndexForSearch{}, &mockSearchHistoryStore{})

	_, err := svc.SearchBySemantic(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil request")
	}
}

func TestSearchBySemantic_EmptyQuery(t *testing.T) {
	svc := newTestSimilaritySearchService(&mockFingerprintEngine{}, &mockVectorStore{}, &mockPatentIndexForSearch{}, &mockSearchHistoryStore{})

	req := &SearchBySemanticRequest{Query: ""}
	_, err := svc.SearchBySemantic(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for empty query")
	}
}

func TestSearchBySemantic_EmbedError(t *testing.T) {
	vs := &mockVectorStore{
		embedTextFn: func(ctx context.Context, text string, model string) ([]float64, error) {
			return nil, errors.New("embedding service unavailable")
		},
	}

	svc := newTestSimilaritySearchService(&mockFingerprintEngine{}, vs, &mockPatentIndexForSearch{}, &mockSearchHistoryStore{})

	req := &SearchBySemanticRequest{Query: "test query"}
	_, err := svc.SearchBySemantic(context.Background(), req)
	if err == nil {
		t.Fatal("expected error from embed failure")
	}
}

func TestSearchBySemantic_VectorSearchError(t *testing.T) {
	vs := &mockVectorStore{
		searchByVectorFn: func(ctx context.Context, vector []float64, threshold float64, maxResults int, filters map[string]string) ([]SimilarityHit, error) {
			return nil, errors.New("vector store timeout")
		},
	}

	svc := newTestSimilaritySearchService(&mockFingerprintEngine{}, vs, &mockPatentIndexForSearch{}, &mockSearchHistoryStore{})

	req := &SearchBySemanticRequest{Query: "test query"}
	_, err := svc.SearchBySemantic(context.Background(), req)
	if err == nil {
		t.Fatal("expected error from vector search failure")
	}
}

// ===========================================================================
// Tests: SearchByPatent
// ===========================================================================

func TestSearchByPatent_Success(t *testing.T) {
	svc := newTestSimilaritySearchService(
		&mockFingerprintEngine{},
		&mockVectorStore{},
		&mockPatentIndexForSearch{},
		&mockSearchHistoryStore{},
	)

	req := &SearchByPatentRequest{
		PatentID:        "pat-001",
		StructureWeight: 0.6,
		TextWeight:      0.4,
		Threshold:       0.60,
		MaxResults:      20,
	}

	result, err := svc.SearchByPatent(context.Background(), req)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if result.SearchType != SearchTypeHybrid {
		t.Errorf("expected search type hybrid, got %s", result.SearchType)
	}
	if result.Metadata["patent_id"] != "pat-001" {
		t.Errorf("expected patent_id pat-001 in metadata")
	}
}

func TestSearchByPatent_NilRequest(t *testing.T) {
	svc := newTestSimilaritySearchService(&mockFingerprintEngine{}, &mockVectorStore{}, &mockPatentIndexForSearch{}, &mockSearchHistoryStore{})

	_, err := svc.SearchByPatent(context.Background(), nil)
	if err == nil {
		t.Fatal("expected error for nil request")
	}
}

func TestSearchByPatent_EmptyPatentID(t *testing.T) {
	svc := newTestSimilaritySearchService(&mockFingerprintEngine{}, &mockVectorStore{}, &mockPatentIndexForSearch{}, &mockSearchHistoryStore{})

	req := &SearchByPatentRequest{PatentID: ""}
	_, err := svc.SearchByPatent(context.Background(), req)
	if err == nil {
		t.Fatal("expected error for empty patent ID")
	}
}

func TestSearchByPatent_DefaultWeights(t *testing.T) {
	svc := newTestSimilaritySearchService(
		&mockFingerprintEngine{},
		&mockVectorStore{},
		&mockPatentIndexForSearch{},
		&mockSearchHistoryStore{},
	)

	req := &SearchByPatentRequest{PatentID: "pat-001"}
	result, err := svc.SearchByPatent(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Metadata["structure_weight"] != "0.50" {
		t.Errorf("expected default structure_weight 0.50, got %s", result.Metadata["structure_weight"])
	}
	if result.Metadata["text_weight"] != "0.50" {
		t.Errorf("expected default text_weight 0.50, got %s", result.Metadata["text_weight"])
	}
}

func TestSearchByPatent_MoleculesFetchError(t *testing.T) {
	pi := &mockPatentIndexForSearch{
		getPatentMoleculesFn: func(ctx context.Context, patentID string) ([]string, error) {
			return nil, errors.New("index unavailable")
		},
	}

	svc := newTestSimilaritySearchService(&mockFingerprintEngine{}, &mockVectorStore{}, pi, &mockSearchHistoryStore{})

	req := &SearchByPatentRequest{PatentID: "pat-001", StructureWeight: 0.5, TextWeight: 0.5}
	// Should not fail entirely, just degrade gracefully
	result, err := svc.SearchByPatent(context.Background(), req)
	if err != nil {
		t.Fatalf("expected graceful degradation, got error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result even with partial failure")
	}
}

// ===========================================================================
// Tests: GetSearchHistory
// ===========================================================================

func TestGetSearchHistory_Success(t *testing.T) {
	hs := &mockSearchHistoryStore{
		listByUserFn: func(ctx context.Context, userID string, limit int) ([]SearchHistoryEntry, error) {
			return []SearchHistoryEntry{
				{QueryID: "sq-1", SearchType: SearchTypeStructure, HitCount: 5},
				{QueryID: "sq-2", SearchType: SearchTypeSemantic, HitCount: 12},
			}, nil
		},
	}

	svc := newTestSimilaritySearchService(&mockFingerprintEngine{}, &mockVectorStore{}, &mockPatentIndexForSearch{}, hs)

	entries, err := svc.GetSearchHistory(context.Background(), "user-001", 10)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(entries) != 2 {
		t.Errorf("expected 2 entries, got %d", len(entries))
	}
}

func TestGetSearchHistory_EmptyUserID(t *testing.T) {
	svc := newTestSimilaritySearchService(&mockFingerprintEngine{}, &mockVectorStore{}, &mockPatentIndexForSearch{}, &mockSearchHistoryStore{})

	_, err := svc.GetSearchHistory(context.Background(), "", 10)
	if err == nil {
		t.Fatal("expected error for empty user ID")
	}
}

func TestGetSearchHistory_DefaultLimit(t *testing.T) {
	var capturedLimit int
	hs := &mockSearchHistoryStore{
		listByUserFn: func(ctx context.Context, userID string, limit int) ([]SearchHistoryEntry, error) {
			capturedLimit = limit
			return nil, nil
		},
	}

	svc := newTestSimilaritySearchService(&mockFingerprintEngine{}, &mockVectorStore{}, &mockPatentIndexForSearch{}, hs)

	_, err := svc.GetSearchHistory(context.Background(), "user-001", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if capturedLimit != 20 {
		t.Errorf("expected default limit 20, got %d", capturedLimit)
	}
}

// ===========================================================================
// Tests: mergeAndRankHits helper
// ===========================================================================

func TestMergeAndRankHits_Basic(t *testing.T) {
	structHits := []SimilarityHit{
		{ID: "a", Score: 0.90},
		{ID: "b", Score: 0.80},
	}
	textHits := []SimilarityHit{
		{ID: "a", Score: 0.85},
		{ID: "c", Score: 0.70},
	}

	merged := mergeAndRankHits(structHits, textHits, 0.5, 0.5, 0.60, 10)

	if len(merged) == 0 {
		t.Fatal("expected non-empty merged results")
	}
	// "a" should be first with highest combined score
	if merged[0].ID != "a" {
		t.Errorf("expected 'a' as top hit, got %s", merged[0].ID)
	}
	// "a" combined: 0.90*0.5 + 0.85*0.5 = 0.875
	expectedScore := 0.90*0.5 + 0.85*0.5
	if merged[0].Score < expectedScore-0.01 || merged[0].Score > expectedScore+0.01 {
		t.Errorf("expected score ~%.3f, got %.3f", expectedScore, merged[0].Score)
	}
}

func TestMergeAndRankHits_ThresholdFilter(t *testing.T) {
	structHits := []SimilarityHit{
		{ID: "low", Score: 0.30},
	}
	textHits := []SimilarityHit{
		{ID: "low", Score: 0.20},
	}

	merged := mergeAndRankHits(structHits, textHits, 0.5, 0.5, 0.60, 10)
	if len(merged) != 0 {
		t.Errorf("expected 0 hits above threshold, got %d", len(merged))
	}
}

func TestMergeAndRankHits_MaxResults(t *testing.T) {
	var structHits []SimilarityHit
	for i := 0; i < 20; i++ {
		structHits = append(structHits, SimilarityHit{
			ID:    fmt.Sprintf("mol-%03d", i),
			Score: 0.95 - float64(i)*0.01,
		})
	}

	merged := mergeAndRankHits(structHits, nil, 1.0, 0.0, 0.50, 5)
	if len(merged) != 5 {
		t.Errorf("expected 5 results after max cap, got %d", len(merged))
	}
}

func TestMergeAndRankHits_Empty(t *testing.T) {
	merged := mergeAndRankHits(nil, nil, 0.5, 0.5, 0.60, 10)
	if len(merged) != 0 {
		t.Errorf("expected 0 for empty inputs, got %d", len(merged))
	}
}

func TestTruncateString(t *testing.T) {
	if truncateString("hello", 10) != "hello" {
		t.Error("short string should not be truncated")
	}
	if truncateString("hello world", 5) != "hello" {
		t.Error("long string should be truncated to maxLen")
	}
	if truncateString("", 5) != "" {
		t.Error("empty string should remain empty")
	}
}

//Personal.AI order the ending

