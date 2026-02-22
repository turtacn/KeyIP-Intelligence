package molpatent_gnn

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/turtacn/KeyIP-Intelligence/internal/intelligence/common"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// ---------------------------------------------------------------------------
// GNN Model Configuration
// ---------------------------------------------------------------------------

// GNNModelConfig holds all configuration for the MolPatent-GNN model.
type GNNModelConfig struct {
	ModelID          string            `json:"model_id" yaml:"model_id"`
	ModelVersion     string            `json:"model_version" yaml:"model_version"`
	EmbeddingDim     int               `json:"embedding_dim" yaml:"embedding_dim"`
	NumLayers        int               `json:"num_layers" yaml:"num_layers"`
	HiddenDim        int               `json:"hidden_dim" yaml:"hidden_dim"`
	NumHeads         int               `json:"num_heads" yaml:"num_heads"`
	DropoutRate      float64           `json:"dropout_rate" yaml:"dropout_rate"`
	Aggregation      AggregationType   `json:"aggregation" yaml:"aggregation"`
	Readout          ReadoutType       `json:"readout" yaml:"readout"`
	MaxAtoms         int               `json:"max_atoms" yaml:"max_atoms"`
	NodeFeatureDim   int               `json:"node_feature_dim" yaml:"node_feature_dim"`
	EdgeFeatureDim   int               `json:"edge_feature_dim" yaml:"edge_feature_dim"`
	ServingEndpoint  string            `json:"serving_endpoint" yaml:"serving_endpoint"`
	TimeoutMs        int64             `json:"timeout_ms" yaml:"timeout_ms"`
	BatchSize        int               `json:"batch_size" yaml:"batch_size"`
	WarmupOnLoad     bool              `json:"warmup_on_load" yaml:"warmup_on_load"`
	Labels           map[string]string `json:"labels,omitempty" yaml:"labels,omitempty"`
}

// AggregationType enumerates GNN message-passing aggregation strategies.
type AggregationType string

const (
	AggregationSum  AggregationType = "sum"
	AggregationMean AggregationType = "mean"
	AggregationMax  AggregationType = "max"
)

// ReadoutType enumerates graph-level readout strategies.
type ReadoutType string

const (
	ReadoutMeanPool    ReadoutType = "mean_pool"
	ReadoutSumPool     ReadoutType = "sum_pool"
	ReadoutAttention   ReadoutType = "attention"
	ReadoutSet2Set     ReadoutType = "set2set"
)

// DefaultGNNModelConfig returns a sensible default configuration.
func DefaultGNNModelConfig() *GNNModelConfig {
	return &GNNModelConfig{
		ModelID:         "molpatent-gnn-v1",
		ModelVersion:    "1.0.0",
		EmbeddingDim:    256,
		NumLayers:       5,
		HiddenDim:       512,
		NumHeads:        8,
		DropoutRate:     0.1,
		Aggregation:     AggregationSum,
		Readout:         ReadoutAttention,
		MaxAtoms:        200,
		NodeFeatureDim:  78,
		EdgeFeatureDim:  12,
		ServingEndpoint: "localhost:8501",
		TimeoutMs:       5000,
		BatchSize:       32,
		WarmupOnLoad:    true,
	}
}

// Validate checks the configuration for consistency.
func (c *GNNModelConfig) Validate() error {
	if c.ModelID == "" {
		return errors.NewInvalidInputError("model_id is required")
	}
	if c.EmbeddingDim <= 0 {
		return errors.NewInvalidInputError("embedding_dim must be positive")
	}
	if c.NumLayers <= 0 {
		return errors.NewInvalidInputError("num_layers must be positive")
	}
	if c.HiddenDim <= 0 {
		return errors.NewInvalidInputError("hidden_dim must be positive")
	}
	if c.MaxAtoms <= 0 {
		return errors.NewInvalidInputError("max_atoms must be positive")
	}
	if c.NodeFeatureDim <= 0 {
		return errors.NewInvalidInputError("node_feature_dim must be positive")
	}
	if c.DropoutRate < 0 || c.DropoutRate >= 1 {
		return errors.NewInvalidInputError("dropout_rate must be in [0, 1)")
	}
	return nil
}

// ---------------------------------------------------------------------------
// Molecular Graph representation
// ---------------------------------------------------------------------------

// MolecularGraph is the graph-level representation fed into the GNN.
type MolecularGraph struct {
	NodeFeatures   [][]float32 `json:"node_features"`
	EdgeIndex      [][2]int    `json:"edge_index"`
	EdgeFeatures   [][]float32 `json:"edge_features"`
	GlobalFeatures []float32   `json:"global_features"`
	NumAtoms       int         `json:"num_atoms"`
	NumBonds       int         `json:"num_bonds"`
	SMILES         string      `json:"smiles"`
}

// MolecularInput is a generic input that can be SMILES or MOL block.
type MolecularInput struct {
	SMILES   string `json:"smiles,omitempty"`
	MOLBlock string `json:"mol_block,omitempty"`
}

// ---------------------------------------------------------------------------
// Preprocessor / Postprocessor interfaces
// ---------------------------------------------------------------------------

// GNNPreprocessor converts molecular representations into graph tensors.
type GNNPreprocessor interface {
	PreprocessSMILES(ctx context.Context, smiles string) (*MolecularGraph, error)
	PreprocessMOL(ctx context.Context, molBlock string) (*MolecularGraph, error)
	PreprocessBatch(ctx context.Context, inputs []MolecularInput) ([]*MolecularGraph, error)
	ValidateSMILES(smiles string) error
	Canonicalize(smiles string) (string, error)
}

// InferenceMeta carries metadata about an inference call.
type InferenceMeta struct {
	OriginalSMILES string
	ModelID        string
	Timestamp      time.Time
	BackendLatency int64
}

// EmbeddingResult is the post-processed embedding output.
type EmbeddingResult struct {
	NormalizedVector []float32 `json:"normalized_vector"`
	L2Norm           float64   `json:"l2_norm"`
}

// GNNPostprocessor normalizes and transforms raw model outputs.
type GNNPostprocessor interface {
	ProcessEmbedding(raw []float32, meta *InferenceMeta) (*EmbeddingResult, error)
	ProcessBatchEmbedding(raw [][]float32, meta []*InferenceMeta) ([]*EmbeddingResult, error)
	ComputeCosineSimilarity(a, b []float32) (float64, error)
	ComputeTanimotoSimilarity(a, b []byte) (float64, error)
	FuseScores(scores map[string]float64, weights map[string]float64) (float64, error)
	ClassifySimilarity(score float64) SimilarityLevel
}

// ---------------------------------------------------------------------------
// GNNModelManager manages model lifecycle
// ---------------------------------------------------------------------------

// ModelState represents the current state of a loaded model.
type ModelState int

const (
	ModelStateUnloaded ModelState = iota
	ModelStateLoading
	ModelStateReady
	ModelStateError
	ModelStateUnloading
)

func (s ModelState) String() string {
	switch s {
	case ModelStateUnloaded:
		return "UNLOADED"
	case ModelStateLoading:
		return "LOADING"
	case ModelStateReady:
		return "READY"
	case ModelStateError:
		return "ERROR"
	case ModelStateUnloading:
		return "UNLOADING"
	default:
		return "UNKNOWN"
	}
}

// GNNModelManager manages the lifecycle of the GNN model.
type GNNModelManager struct {
	config  *GNNModelConfig
	backend common.ModelBackend
	state   ModelState
	logger  common.Logger
	metrics common.IntelligenceMetrics
	mu      sync.RWMutex
	loadErr error
}

// NewGNNModelManager creates a new model manager.
func NewGNNModelManager(
	config *GNNModelConfig,
	backend common.ModelBackend,
	logger common.Logger,
	metrics common.IntelligenceMetrics,
) (*GNNModelManager, error) {
	if config == nil {
		return nil, errors.NewInvalidInputError("config is required")
	}
	if err := config.Validate(); err != nil {
		return nil, err
	}
	if backend == nil {
		return nil, errors.NewInvalidInputError("backend is required")
	}
	if logger == nil {
		logger = common.NewNoopLogger()
	}
	if metrics == nil {
		metrics = common.NewNoopIntelligenceMetrics()
	}
	return &GNNModelManager{
		config:  config,
		backend: backend,
		state:   ModelStateUnloaded,
		logger:  logger,
		metrics: metrics,
	}, nil
}

// Load initializes the model backend and optionally warms up.
func (m *GNNModelManager) Load(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.state == ModelStateReady {
		return nil
	}
	m.state = ModelStateLoading
	start := time.Now()

	if err := m.backend.Healthy(ctx); err != nil {
		m.state = ModelStateError
		m.loadErr = err
		m.metrics.RecordModelLoad(ctx, m.config.ModelID, m.config.ModelVersion, float64(time.Since(start).Milliseconds()), false)
		return fmt.Errorf("backend health check failed: %w", err)
	}

	if m.config.WarmupOnLoad {
		if err := m.warmup(ctx); err != nil {
			m.logger.Warn("warmup failed, proceeding anyway", "error", err)
		}
	}

	m.state = ModelStateReady
	m.loadErr = nil
	elapsed := float64(time.Since(start).Milliseconds())
	m.metrics.RecordModelLoad(ctx, m.config.ModelID, m.config.ModelVersion, elapsed, true)
	m.logger.Info("model loaded", "model_id", m.config.ModelID, "duration_ms", elapsed)
	return nil
}

// Unload tears down the model backend.
func (m *GNNModelManager) Unload(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.state == ModelStateUnloaded {
		return nil
	}
	m.state = ModelStateUnloading
	if err := m.backend.Close(); err != nil {
		m.state = ModelStateError
		return fmt.Errorf("backend close failed: %w", err)
	}
	m.state = ModelStateUnloaded
	m.logger.Info("model unloaded", "model_id", m.config.ModelID)
	return nil
}

// State returns the current model state.
func (m *GNNModelManager) State() ModelState {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.state
}

// Config returns the model configuration.
func (m *GNNModelManager) Config() *GNNModelConfig {
	return m.config
}

// LastError returns the last load error, if any.
func (m *GNNModelManager) LastError() error {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.loadErr
}

func (m *GNNModelManager) warmup(ctx context.Context) error {
	warmupSMILES := "C"
	dummyReq := &common.PredictRequest{
		ModelName:  m.config.ModelID,
		InputData:  []byte(fmt.Sprintf(`{"smiles":"%s"}`, warmupSMILES)),
		InputFormat: common.FormatJSON,
	}
	_, err := m.backend.Predict(ctx, dummyReq)
	return err
}

