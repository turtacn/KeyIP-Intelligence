package molpatent_gnn

import (
	"context"
	"encoding/binary"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/turtacn/KeyIP-Intelligence/internal/intelligence/common"
)

// MockModelBackend
type MockModelBackend struct {
	mock.Mock
}

func (m *MockModelBackend) Predict(ctx context.Context, req *common.PredictRequest) (*common.PredictResponse, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*common.PredictResponse), args.Error(1)
}

func (m *MockModelBackend) PredictStream(ctx context.Context, req *common.PredictRequest) (<-chan *common.PredictResponse, error) {
	return nil, nil
}

func (m *MockModelBackend) Healthy(ctx context.Context) error { return nil }
func (m *MockModelBackend) Close() error { return nil }

func TestEmbed_Success(t *testing.T) {
	backend := new(MockModelBackend)
	cfg := NewGNNModelConfig()
	logger := new(MockLogger)
	pre := NewGNNPreprocessor(cfg, logger)
	post := NewGNNPostprocessor(cfg)
	metrics := common.NewNoopIntelligenceMetrics()

	engine := NewGNNInferenceEngine(backend, pre, post, metrics, logger, cfg)

	// Create non-zero embedding
	embedding := make([]byte, 256*4)
	for i := 0; i < 256; i++ {
		bits := math.Float32bits(1.0)
		binary.LittleEndian.PutUint32(embedding[i*4:], bits)
	}

	backend.On("Predict", mock.Anything, mock.Anything).Return(&common.PredictResponse{
		Outputs: map[string][]byte{"embedding": embedding},
	}, nil)

	res, err := engine.Embed(context.Background(), &EmbedRequest{SMILES: "C"})
	assert.NoError(t, err)
	assert.NotNil(t, res)
	assert.Equal(t, float32(1.0/16.0), res.Vector[0]) // 256 ones, norm sqrt(256)=16. 1/16.
}
//Personal.AI order the ending
