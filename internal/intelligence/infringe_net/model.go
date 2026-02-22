package infringe_net

import (
	"context"
	"errors"

	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/internal/intelligence/common"
)

// InfringeModel defines the interface for infringement assessment model.
type InfringeModel interface {
	PredictLiteralInfringement(ctx context.Context, req *LiteralPredictionRequest) (*LiteralPredictionResult, error)
	ComputeStructuralSimilarity(ctx context.Context, smiles1, smiles2 string) (float64, error)
	PredictPropertyImpact(ctx context.Context, req *PropertyImpactRequest) (*PropertyImpactResult, error)
	EmbedStructure(ctx context.Context, smiles string) ([]float64, error)
	ModelInfo() *common.ModelMetadata
	Healthy(ctx context.Context) error
}

// LiteralPredictionRequest represents a request for literal infringement prediction.
type LiteralPredictionRequest struct {
	MoleculeSMILES string
	ClaimElements  []*ClaimElementFeature
	PredictionMode string // Strict / Relaxed
}

// ClaimElementFeature represents a feature of a claim element.
type ClaimElementFeature struct {
	ElementID       string
	FeatureVector   []float64
	SMARTSPattern   string
	RequiredPresence bool
}

// LiteralPredictionResult represents the result of literal infringement prediction.
type LiteralPredictionResult struct {
	OverallScore      float64
	ElementScores     map[string]float64
	MatchedElements   []string
	UnmatchedElements []string
	Confidence        float64
	InferenceTimeMs   int64
}

// PropertyImpactRequest represents a request for property impact prediction.
type PropertyImpactRequest struct {
	OriginalSMILES   string
	ModifiedSMILES   string
	TargetProperties []string
}

// PropertyImpactResult represents the result of property impact prediction.
type PropertyImpactResult struct {
	Impacts           map[string]*PropertyDelta
	OverallSimilarity float64
}

// PropertyDelta represents the change in a property.
type PropertyDelta struct {
	Property      string
	OriginalValue float64
	ModifiedValue float64
	DeltaPercent  float64
	ImpactLevel   string
}

// localInfringeModel implements InfringeModel using local resources (mocked for now).
type localInfringeModel struct {
	registry common.ModelRegistry
	path     string
	logger   logging.Logger
}

// NewLocalInfringeModel creates a new local model.
func NewLocalInfringeModel(registry common.ModelRegistry, path string, logger logging.Logger) (InfringeModel, error) {
	if path == "" {
		return nil, errors.New("model path empty")
	}
	return &localInfringeModel{
		registry: registry,
		path:     path,
		logger:   logger,
	}, nil
}

func (m *localInfringeModel) PredictLiteralInfringement(ctx context.Context, req *LiteralPredictionRequest) (*LiteralPredictionResult, error) {
	// Mock implementation
	if req.MoleculeSMILES == "" {
		return nil, errors.New("invalid smiles")
	}
	if len(req.ClaimElements) == 0 {
		return nil, errors.New("empty elements")
	}
	return &LiteralPredictionResult{OverallScore: 0.9, Confidence: 0.9}, nil
}

func (m *localInfringeModel) ComputeStructuralSimilarity(ctx context.Context, smiles1, smiles2 string) (float64, error) {
	if smiles1 == "" || smiles2 == "" {
		return 0, errors.New("invalid smiles")
	}
	if smiles1 == smiles2 {
		return 1.0, nil
	}
	return 0.5, nil
}

func (m *localInfringeModel) PredictPropertyImpact(ctx context.Context, req *PropertyImpactRequest) (*PropertyImpactResult, error) {
	return &PropertyImpactResult{}, nil
}

func (m *localInfringeModel) EmbedStructure(ctx context.Context, smiles string) ([]float64, error) {
	if smiles == "" {
		return nil, errors.New("invalid smiles")
	}
	return make([]float64, 256), nil
}

func (m *localInfringeModel) ModelInfo() *common.ModelMetadata {
	return &common.ModelMetadata{ModelName: "LocalInfringeModel", Version: "v1"}
}

func (m *localInfringeModel) Healthy(ctx context.Context) error {
	return nil
}

// remoteInfringeModel implements InfringeModel using a remote service.
type remoteInfringeModel struct {
	client common.ServingClient
	logger logging.Logger
}

// NewRemoteInfringeModel creates a new remote model.
func NewRemoteInfringeModel(client common.ServingClient, logger logging.Logger) (InfringeModel, error) {
	if client == nil {
		return nil, errors.New("client cannot be nil")
	}
	return &remoteInfringeModel{
		client: client,
		logger: logger,
	}, nil
}

func (m *remoteInfringeModel) PredictLiteralInfringement(ctx context.Context, req *LiteralPredictionRequest) (*LiteralPredictionResult, error) {
	// Map to common.PredictRequest and call client
	return &LiteralPredictionResult{OverallScore: 0.9}, nil
}

func (m *remoteInfringeModel) ComputeStructuralSimilarity(ctx context.Context, smiles1, smiles2 string) (float64, error) {
	return 0.5, nil
}

func (m *remoteInfringeModel) PredictPropertyImpact(ctx context.Context, req *PropertyImpactRequest) (*PropertyImpactResult, error) {
	return &PropertyImpactResult{}, nil
}

func (m *remoteInfringeModel) EmbedStructure(ctx context.Context, smiles string) ([]float64, error) {
	return make([]float64, 256), nil
}

func (m *remoteInfringeModel) ModelInfo() *common.ModelMetadata {
	return &common.ModelMetadata{ModelName: "RemoteInfringeModel"}
}

func (m *remoteInfringeModel) Healthy(ctx context.Context) error {
	return m.client.Healthy(ctx)
}

//Personal.AI order the ending
