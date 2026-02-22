package chem_extractor

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/turtacn/KeyIP-Intelligence/internal/intelligence/common"
)

type MockBackend struct {}

func (m *MockBackend) Predict(ctx context.Context, req *common.PredictRequest) (*common.PredictResponse, error) {
	return &common.PredictResponse{}, nil
}

func (m *MockBackend) PredictStream(ctx context.Context, req *common.PredictRequest) (<-chan *common.PredictResponse, error) {
	return nil, nil
}

func (m *MockBackend) Healthy(ctx context.Context) error { return nil }
func (m *MockBackend) Close() error { return nil }

func TestPredict_Simple(t *testing.T) {
	config := &NERModelConfig{
		LabelSet: []string{"O", "B-COMMON", "I-COMMON"},
	}
	model := NewNERModel(config, &MockBackend{})

	prediction, err := model.Predict(context.Background(), "aspirin")
	assert.NoError(t, err)
	assert.NotNil(t, prediction)
	assert.Equal(t, "aspirin", prediction.Entities[0].Text)
}

func TestPredictBatch_Success(t *testing.T) {
	config := &NERModelConfig{}
	model := NewNERModel(config, &MockBackend{})

	texts := []string{"aspirin", "ibuprofen"}
	predictions, err := model.PredictBatch(context.Background(), texts)
	assert.NoError(t, err)
	assert.Equal(t, 2, len(predictions))
}

func TestCalculateConfidence(t *testing.T) {
	// Not testing internal function directly unless exported, but logic is simple
	assert.InDelta(t, 0.5, calculateConfidence([]float64{0.25, 1.0}), 0.001)
}
//Personal.AI order the ending
