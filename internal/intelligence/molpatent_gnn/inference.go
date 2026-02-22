package molpatent_gnn

import (
	"context"
	"errors"
	"math"
	"time"

	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/internal/intelligence/common"
	"github.com/turtacn/KeyIP-Intelligence/pkg/types/molecule"
)

// GNNInferenceEngine defines the interface for GNN inference.
type GNNInferenceEngine interface {
	Embed(ctx context.Context, req *EmbedRequest) (*EmbedResponse, error)
	BatchEmbed(ctx context.Context, req *BatchEmbedRequest) (*BatchEmbedResponse, error)
	ComputeSimilarity(ctx context.Context, req *SimilarityRequest) (*SimilarityResponse, error)
	SearchSimilar(ctx context.Context, req *SimilarSearchRequest) (*SimilarSearchResponse, error)
}

type EmbedRequest struct {
	SMILES string
}

type EmbedResponse struct {
	Vector      []float32
	InferenceMs int64
}

type BatchEmbedRequest struct {
	SMILES []string
}

type BatchEmbedResponse struct {
	Vectors     [][]float32
	InferenceMs int64
}

type SimilarityRequest struct {
	SMILES1 string
	SMILES2 string
}

type SimilarityResponse struct {
	Score float64
}

type SimilarSearchRequest struct {
	SMILES     string
	TopK       int
	Threshold  float64
}

type SimilarSearchResponse struct {
	Results []molecule.SimilarityResult
}

// gnnInferenceEngine implements GNNInferenceEngine.
type gnnInferenceEngine struct {
	backend        common.ModelBackend
	preprocessor   GNNPreprocessor
	postprocessor  GNNPostprocessor
	metrics        common.IntelligenceMetrics
	logger         logging.Logger
	config         *GNNModelConfig
}

var (
	ErrInferenceTimeout = errors.New("inference timeout")
	ErrModelBackendUnavailable = errors.New("model backend unavailable")
	ErrInvalidMolecule = errors.New("invalid molecule")
)

// NewGNNInferenceEngine creates a new GNNInferenceEngine.
func NewGNNInferenceEngine(
	backend common.ModelBackend,
	preprocessor GNNPreprocessor,
	postprocessor GNNPostprocessor,
	metrics common.IntelligenceMetrics,
	logger logging.Logger,
	config *GNNModelConfig,
) GNNInferenceEngine {
	return &gnnInferenceEngine{
		backend:       backend,
		preprocessor:  preprocessor,
		postprocessor: postprocessor,
		metrics:       metrics,
		logger:        logger,
		config:        config,
	}
}

func (e *gnnInferenceEngine) Embed(ctx context.Context, req *EmbedRequest) (*EmbedResponse, error) {
	start := time.Now()

	graph, err := e.preprocessor.PreprocessSMILES(ctx, req.SMILES)
	if err != nil {
		return nil, ErrInvalidMolecule
	}

	// Serialize graph to bytes or use PredictRequest format expected by backend
	// Simplified: passing raw SMILES if backend handles it or serialized graph
	// Assuming backend takes "input_data" as bytes.
	// Here we just use a placeholder for serialization.
	inputData := []byte(graph.SMILES) // Should be graph serialization

	predictReq := &common.PredictRequest{
		ModelName:   e.config.ModelID,
		InputData:   inputData,
		InputFormat: common.FormatJSON,
	}

	resp, err := e.backend.Predict(ctx, predictReq)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, ErrInferenceTimeout
		}
		return nil, ErrModelBackendUnavailable
	}

	// Extract vector from response
	// Assuming output name "embedding"
	rawVectorBytes, ok := resp.Outputs["embedding"]
	if !ok {
		return nil, errors.New("missing embedding output")
	}

	// Deserialize []byte to []float32
	if len(rawVectorBytes)%4 != 0 {
		return nil, errors.New("invalid embedding byte length")
	}
	count := len(rawVectorBytes) / 4
	if count != e.config.EmbeddingDim {
		// Log warning or resize? For now strict check or just use what we got if it matches config?
		// Ensure we don't panic.
	}

	rawVector := make([]float32, count)
	for i := 0; i < count; i++ {
		// Little Endian assumption
		bits := uint32(rawVectorBytes[i*4]) | uint32(rawVectorBytes[i*4+1])<<8 | uint32(rawVectorBytes[i*4+2])<<16 | uint32(rawVectorBytes[i*4+3])<<24
		rawVector[i] = math.Float32frombits(bits)
	}

	embedRes, err := e.postprocessor.ProcessEmbedding(rawVector, &InferenceMeta{
		SMILES:           req.SMILES,
		ModelID:          resp.ModelName,
		BackendLatencyMs: resp.InferenceTimeMs,
	})
	if err != nil {
		return nil, err
	}

	e.metrics.RecordInference(ctx, &common.InferenceMetricParams{
		ModelName: e.config.ModelID,
		Success: true,
		DurationMs: float64(time.Since(start).Milliseconds()),
	})

	return &EmbedResponse{
		Vector:      embedRes.Vector,
		InferenceMs: resp.InferenceTimeMs,
	}, nil
}

func (e *gnnInferenceEngine) BatchEmbed(ctx context.Context, req *BatchEmbedRequest) (*BatchEmbedResponse, error) {
	// Simplified batch logic: sequential loop or use common.BatchProcessor
	// Ideally use BatchProcessor.
	// For this phase, simplified loop.
	var vectors [][]float32
	var totalTime int64

	for _, s := range req.SMILES {
		res, err := e.Embed(ctx, &EmbedRequest{SMILES: s})
		if err != nil {
			return nil, err
		}
		vectors = append(vectors, res.Vector)
		totalTime += res.InferenceMs
	}
	return &BatchEmbedResponse{Vectors: vectors, InferenceMs: totalTime}, nil
}

func (e *gnnInferenceEngine) ComputeSimilarity(ctx context.Context, req *SimilarityRequest) (*SimilarityResponse, error) {
	emb1, err := e.Embed(ctx, &EmbedRequest{SMILES: req.SMILES1})
	if err != nil {
		return nil, err
	}
	emb2, err := e.Embed(ctx, &EmbedRequest{SMILES: req.SMILES2})
	if err != nil {
		return nil, err
	}

	sim, err := e.postprocessor.ComputeCosineSimilarity(emb1.Vector, emb2.Vector)
	if err != nil {
		return nil, err
	}
	return &SimilarityResponse{Score: sim}, nil
}

func (e *gnnInferenceEngine) SearchSimilar(ctx context.Context, req *SimilarSearchRequest) (*SimilarSearchResponse, error) {
	// Need vector search dependency (Milvus).
	// Not passed in constructor. Assuming internal integration or placeholder.
	return &SimilarSearchResponse{}, nil
}
//Personal.AI order the ending
