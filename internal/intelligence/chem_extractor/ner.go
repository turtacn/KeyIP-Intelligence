package chem_extractor

import (
	"context"
	"math"

	"github.com/turtacn/KeyIP-Intelligence/internal/intelligence/common"
)

// NERModel defines the interface for Named Entity Recognition.
type NERModel interface {
	Predict(ctx context.Context, text string) (*NERPrediction, error)
	PredictBatch(ctx context.Context, texts []string) ([]*NERPrediction, error)
	GetLabelSet() []string
}

// NERPrediction represents the result of NER prediction.
type NERPrediction struct {
	Tokens        []string
	Labels        []string
	Probabilities [][]float64
	Entities      []*NEREntity
}

// NEREntity represents an extracted entity.
type NEREntity struct {
	Text       string
	Label      string
	StartToken int
	EndToken   int
	StartChar  int
	EndChar    int
	Score      float64
}

// BackendType defines the type of inference backend.
type BackendType string

const (
	BackendONNX   BackendType = "onnx"
	BackendTriton BackendType = "triton"
	BackendVLLM   BackendType = "vllm"
)

// NERModelConfig holds configuration for the NER model.
type NERModelConfig struct {
	ModelID             string      `json:"model_id"`
	ModelPath           string      `json:"model_path"`
	BackendType         BackendType `json:"backend_type"`
	MaxSequenceLength   int         `json:"max_sequence_length"`
	LabelSet            []string    `json:"label_set"`
	ConfidenceThreshold float64     `json:"confidence_threshold"`
	UseCRF              bool        `json:"use_crf"`
	TimeoutMs           int         `json:"timeout_ms"`
	MaxBatchSize        int         `json:"max_batch_size"`
}

// nerModelImpl implements NERModel.
type nerModelImpl struct {
	config  *NERModelConfig
	backend common.ModelBackend
}

// NewNERModel creates a new NERModel.
func NewNERModel(config *NERModelConfig, backend common.ModelBackend) NERModel {
	return &nerModelImpl{
		config:  config,
		backend: backend,
	}
}

func (m *nerModelImpl) Predict(ctx context.Context, text string) (*NERPrediction, error) {
	// Simplified prediction logic
	// In reality, this would tokenize, call backend, decode (Viterbi if CRF), and extract entities.

	// Mock behavior for now as backend implementation is complex
	// We assume backend returns raw logits/probabilities.

	tokens := []string{"aspirin"} // Placeholder tokenization
	labels := []string{"B-COMMON"} // Placeholder labels
	probs := [][]float64{{0.9}} // Placeholder probs
	entities := []*NEREntity{{
		Text: "aspirin",
		Label: "COMMON",
		StartToken: 0,
		EndToken: 1,
		Score: 0.9,
	}}

	return &NERPrediction{
		Tokens:        tokens,
		Labels:        labels,
		Probabilities: probs,
		Entities:      entities,
	}, nil
}

func (m *nerModelImpl) PredictBatch(ctx context.Context, texts []string) ([]*NERPrediction, error) {
	var results []*NERPrediction
	for _, text := range texts {
		res, err := m.Predict(ctx, text)
		if err != nil {
			return nil, err
		}
		results = append(results, res)
	}
	return results, nil
}

func (m *nerModelImpl) GetLabelSet() []string {
	return m.config.LabelSet
}

// Calculate confidence score (geometric mean)
func calculateConfidence(probs []float64) float64 {
	if len(probs) == 0 {
		return 0
	}
	product := 1.0
	for _, p := range probs {
		product *= p
	}
	return math.Pow(product, 1.0/float64(len(probs)))
}

//Personal.AI order the ending
