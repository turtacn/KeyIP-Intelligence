package molpatent_gnn

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/turtacn/KeyIP-Intelligence/internal/intelligence/common"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// ---------------------------------------------------------------------------
// Interfaces
// ---------------------------------------------------------------------------

// GNNInferenceService defines the inference capabilities of the MolPatent-GNN.
type GNNInferenceService interface {
	Embed(ctx context.Context, req *EmbedRequest) (*EmbedResponse, error)
	BatchEmbed(ctx context.Context, req *BatchEmbedRequest) (*BatchEmbedResponse, error)
	ComputeSimilarity(ctx context.Context, req *SimilarityRequest) (*SimilarityResponse, error)
	SearchSimilar(ctx context.Context, req *SimilarSearchRequest) (*SimilarSearchResponse, error)
}

// VectorSearcher abstracts vector database search (e.g. Milvus).
type VectorSearcher interface {
	Search(ctx context.Context, vector []float32, topK int, threshold float64) ([]*VectorMatch, error)
}

// ---------------------------------------------------------------------------
// Request / Response types
// ---------------------------------------------------------------------------

// InferenceMode enumerates the supported inference modes.
type InferenceMode int

const (
	InferenceModeEmbedding InferenceMode = iota
	InferenceModeSimilarity
	InferenceModeBatchSimilarity
)

// FingerprintType enumerates supported molecular fingerprint types.
type FingerprintType string

const (
	FingerprintMorgan   FingerprintType = "morgan"
	FingerprintRDKit    FingerprintType = "rdkit"
	FingerprintAtomPair FingerprintType = "atompair"
	FingerprintGNN      FingerprintType = "gnn"
)

// EmbedRequest is the input for a single-molecule embedding.
type EmbedRequest struct {
	SMILES              string          `json:"smiles"`
	FingerprintPrefer   FingerprintType `json:"fingerprint_prefer,omitempty"`
	Mode                InferenceMode   `json:"mode,omitempty"`
}

// EmbedResponse is the output of a single-molecule embedding.
type EmbedResponse struct {
	Embedding    []float32     `json:"embedding"`
	SMILES       string        `json:"smiles"`
	Confidence   float64       `json:"confidence"`
	InferenceMs  int64         `json:"inference_ms"`
	ModelVersion string        `json:"model_version"`
}

// BatchEmbedRequest wraps multiple embed requests.
type BatchEmbedRequest struct {
	Items []*EmbedRequest `json:"items"`
}

// BatchEmbedResponse wraps multiple embed responses.
type BatchEmbedResponse struct {
	Results []*EmbedResultItem `json:"results"`
}

// EmbedResultItem is a single item in a batch embed response.
type EmbedResultItem struct {
	Index    int            `json:"index"`
	Response *EmbedResponse `json:"response,omitempty"`
	Error    string         `json:"error,omitempty"`
}

// SimilarityRequest asks for the similarity between two molecules.
type SimilarityRequest struct {
	SMILES1 string `json:"smiles1"`
	SMILES2 string `json:"smiles2"`
}

// SimilarityResponse contains multi-fingerprint similarity scores.
type SimilarityResponse struct {
	FusedScore      float64            `json:"fused_score"`
	Scores          map[string]float64 `json:"scores"`
	Level           SimilarityLevel    `json:"level"`
	InferenceMs     int64              `json:"inference_ms"`
	ModelVersion    string             `json:"model_version"`
}

// SimilarSearchRequest asks for molecules similar to a query.
type SimilarSearchRequest struct {
	SMILES    string  `json:"smiles"`
	TopK      int     `json:"top_k"`
	Threshold float64 `json:"threshold"`
}

// SimilarSearchResponse contains the search results.
type SimilarSearchResponse struct {
	Matches     []*VectorMatch `json:"matches"`
	QuerySMILES string         `json:"query_smiles"`
	InferenceMs int64          `json:"inference_ms"`
}

// VectorMatch represents a single vector search hit.
type VectorMatch struct {
	MoleculeID string  `json:"molecule_id"`
	SMILES     string  `json:"smiles"`
	Score      float64 `json:"score"`
	Level      SimilarityLevel `json:"level"`
}

// SimilarityLevel classifies a similarity score.
type SimilarityLevel string

const (
	SimilarityHigh   SimilarityLevel = "HIGH"
	SimilarityMedium SimilarityLevel = "MEDIUM"
	SimilarityLow    SimilarityLevel = "LOW"
	SimilarityNone   SimilarityLevel = "NONE"
)

// ---------------------------------------------------------------------------
// Default fusion weights
// ---------------------------------------------------------------------------

var defaultFusionWeights = map[string]float64{
	string(FingerprintMorgan):   0.30,
	string(FingerprintRDKit):    0.20,
	string(FingerprintAtomPair): 0.15,
	string(FingerprintGNN):      0.35,
}

// ---------------------------------------------------------------------------
// GNNInferenceEngine
// ---------------------------------------------------------------------------

const (
	maxBatchSize    = 64
	defaultRetries  = 2
	retryBaseDelay  = 200 * time.Millisecond
)

// GNNInferenceEngine implements GNNInferenceService.
type GNNInferenceEngine struct {
	backend       common.ModelBackend
	preprocessor  GNNPreprocessor
	postprocessor GNNPostprocessor
	vectorSearch  VectorSearcher
	metrics       common.IntelligenceMetrics
	logger        common.Logger
	config        *GNNModelConfig
	mu            sync.RWMutex
}

// NewGNNInferenceEngine creates a new inference engine with all dependencies.
func NewGNNInferenceEngine(
	backend common.ModelBackend,
	preprocessor GNNPreprocessor,
	postprocessor GNNPostprocessor,
	vectorSearch VectorSearcher,
	metrics common.IntelligenceMetrics,
	logger common.Logger,
	config *GNNModelConfig,
) (*GNNInferenceEngine, error) {
	if backend == nil {
		return nil, errors.NewInvalidInputError("model backend is required")
	}
	if preprocessor == nil {
		return nil, errors.NewInvalidInputError("preprocessor is required")
	}
	if postprocessor == nil {
		return nil, errors.NewInvalidInputError("postprocessor is required")
	}
	if config == nil {
		return nil, errors.NewInvalidInputError("config is required")
	}
	if metrics == nil {
		metrics = common.NewNoopIntelligenceMetrics()
	}
	if logger == nil {
		logger = common.NewNoopLogger()
	}
	return &GNNInferenceEngine{
		backend:       backend,
		preprocessor:  preprocessor,
		postprocessor: postprocessor,
		vectorSearch:  vectorSearch,
		metrics:       metrics,
		logger:        logger,
		config:        config,
	}, nil
}

// Embed produces an embedding vector for a single molecule.
func (e *GNNInferenceEngine) Embed(ctx context.Context, req *EmbedRequest) (*EmbedResponse, error) {
	if req == nil || req.SMILES == "" {
		return nil, errors.NewInvalidInputError("SMILES is required")
	}
	start := time.Now()

	// 1. Validate
	if err := e.preprocessor.ValidateSMILES(req.SMILES); err != nil {
		return nil, fmt.Errorf("%w: %v", errors.ErrInvalidMolecule, err)
	}

	// 2. Preprocess
	graph, err := e.preprocessor.PreprocessSMILES(ctx, req.SMILES)
	if err != nil {
		return nil, fmt.Errorf("preprocessing failed: %w", err)
	}

	// 3. Build backend request
	backendReq := &common.PredictRequest{
		ModelName:  e.config.ModelID,
		InputData:  common.EncodeMolecularGraph(graph.NodeFeatures, graph.EdgeIndex, graph.EdgeFeatures, graph.GlobalFeatures),
		InputFormat: common.FormatJSON,
		Metadata:   map[string]string{"smiles": req.SMILES},
	}

	// 4. Invoke with retry
	var backendResp *common.PredictResponse
	backendResp, err = e.predictWithRetry(ctx, backendReq)
	if err != nil {
		return nil, err
	}

	// 5. Post-process
	rawEmbedding, err := common.DecodeFloat32Vector(backendResp.Outputs["embedding"])
	if err != nil {
		return nil, fmt.Errorf("decoding embedding: %w", err)
	}

	meta := &InferenceMeta{
		OriginalSMILES: req.SMILES,
		ModelID:        e.config.ModelID,
		Timestamp:      time.Now(),
		BackendLatency: backendResp.InferenceTimeMs,
	}
	embResult, err := e.postprocessor.ProcessEmbedding(rawEmbedding, meta)
	if err != nil {
		return nil, fmt.Errorf("postprocessing failed: %w", err)
	}

	elapsed := time.Since(start).Milliseconds()

	// 6. Metrics
	e.metrics.RecordInference(ctx, &common.InferenceMetricParams{
		ModelName:    e.config.ModelID,
		ModelVersion: e.config.ModelID,
		TaskType:     "embed",
		DurationMs:   float64(elapsed),
		Success:      true,
		BatchSize:    1,
	})

	return &EmbedResponse{
		Embedding:    embResult.NormalizedVector,
		SMILES:       req.SMILES,
		Confidence:   1.0,
		InferenceMs:  elapsed,
		ModelVersion: e.config.ModelID,
	}, nil
}

// BatchEmbed embeds multiple molecules, splitting into chunks of maxBatchSize.
func (e *GNNInferenceEngine) BatchEmbed(ctx context.Context, req *BatchEmbedRequest) (*BatchEmbedResponse, error) {
	if req == nil || len(req.Items) == 0 {
		return &BatchEmbedResponse{Results: []*EmbedResultItem{}}, nil
	}

	results := make([]*EmbedResultItem, len(req.Items))
	chunks := splitEmbedRequests(req.Items, maxBatchSize)

	offset := 0
	for _, chunk := range chunks {
		var wg sync.WaitGroup
		for i, item := range chunk {
			idx := offset + i
			wg.Add(1)
			go func(index int, r *EmbedRequest) {
				defer wg.Done()
				resp, err := e.Embed(ctx, r)
				ri := &EmbedResultItem{Index: index}
				if err != nil {
					ri.Error = err.Error()
				} else {
					ri.Response = resp
				}
				results[index] = ri
			}(idx, item)
		}
		wg.Wait()
		offset += len(chunk)
	}

	return &BatchEmbedResponse{Results: results}, nil
}

// ComputeSimilarity computes multi-fingerprint fused similarity between two molecules.
func (e *GNNInferenceEngine) ComputeSimilarity(ctx context.Context, req *SimilarityRequest) (*SimilarityResponse, error) {
	if req == nil || req.SMILES1 == "" || req.SMILES2 == "" {
		return nil, errors.NewInvalidInputError("both SMILES are required")
	}
	start := time.Now()

	// Embed both molecules
	emb1, err := e.Embed(ctx, &EmbedRequest{SMILES: req.SMILES1})
	if err != nil {
		return nil, fmt.Errorf("embedding molecule 1: %w", err)
	}
	emb2, err := e.Embed(ctx, &EmbedRequest{SMILES: req.SMILES2})
	if err != nil {
		return nil, fmt.Errorf("embedding molecule 2: %w", err)
	}

	// Cosine similarity from GNN embeddings
	gnnSim, err := e.postprocessor.ComputeCosineSimilarity(emb1.Embedding, emb2.Embedding)
	if err != nil {
		return nil, fmt.Errorf("cosine similarity: %w", err)
	}

	scores := map[string]float64{
		string(FingerprintGNN): gnnSim,
	}

	// Fuse available scores
	fused, err := e.postprocessor.FuseScores(scores, defaultFusionWeights)
	if err != nil {
		// Fallback: use GNN score directly
		fused = gnnSim
	}

	level := e.postprocessor.ClassifySimilarity(fused)
	elapsed := time.Since(start).Milliseconds()

	e.metrics.RecordInference(ctx, &common.InferenceMetricParams{
		ModelName:  e.config.ModelID,
		TaskType:   "similarity",
		DurationMs: float64(elapsed),
		Success:    true,
		BatchSize:  1,
	})

	return &SimilarityResponse{
		FusedScore:   fused,
		Scores:       scores,
		Level:        level,
		InferenceMs:  elapsed,
		ModelVersion: e.config.ModelID,
	}, nil
}

// SearchSimilar retrieves the top-K similar molecules from the vector store.
func (e *GNNInferenceEngine) SearchSimilar(ctx context.Context, req *SimilarSearchRequest) (*SimilarSearchResponse, error) {
	if req == nil || req.SMILES == "" {
		return nil, errors.NewInvalidInputError("SMILES is required")
	}
	if e.vectorSearch == nil {
		return nil, errors.NewInvalidInputError("vector searcher not configured")
	}
	start := time.Now()

	topK := req.TopK
	if topK <= 0 {
		topK = 10
	}
	threshold := req.Threshold
	if threshold <= 0 {
		threshold = 0.55
	}

	emb, err := e.Embed(ctx, &EmbedRequest{SMILES: req.SMILES})
	if err != nil {
		return nil, err
	}

	matches, err := e.vectorSearch.Search(ctx, emb.Embedding, topK, threshold)
	if err != nil {
		return nil, fmt.Errorf("vector search: %w", err)
	}
	if matches == nil {
		matches = []*VectorMatch{}
	}

	// Classify each match
	for _, m := range matches {
		m.Level = e.postprocessor.ClassifySimilarity(m.Score)
	}

	elapsed := time.Since(start).Milliseconds()
	return &SimilarSearchResponse{
		Matches:     matches,
		QuerySMILES: req.SMILES,
		InferenceMs: elapsed,
	}, nil
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

func (e *GNNInferenceEngine) predictWithRetry(ctx context.Context, req *common.PredictRequest) (*common.PredictResponse, error) {
	var lastErr error
	for attempt := 0; attempt <= defaultRetries; attempt++ {
		resp, err := e.backend.Predict(ctx, req)
		if err == nil {
			return resp, nil
		}
		lastErr = err
		if !isTransient(err) {
			return nil, err
		}
		if attempt < defaultRetries {
			delay := retryBaseDelay * time.Duration(math.Pow(2, float64(attempt)))
			select {
			case <-ctx.Done():
				return nil, fmt.Errorf("%w: %v", errors.ErrInferenceTimeout, ctx.Err())
			case <-time.After(delay):
			}
		}
	}
	return nil, fmt.Errorf("%w: %v", errors.ErrModelBackendUnavailable, lastErr)
}

func isTransient(err error) bool {
	return errors.Is(err, errors.ErrServingUnavailable) ||
		errors.Is(err, errors.ErrInferenceTimeout)
}

func splitEmbedRequests(items []*EmbedRequest, chunkSize int) [][]*EmbedRequest {
	var chunks [][]*EmbedRequest
	for i := 0; i < len(items); i += chunkSize {
		end := i + chunkSize
		if end > len(items) {
			end = len(items)
		}
		chunks = append(chunks, items[i:end])
	}
	return chunks
}

//Personal.AI order the ending
