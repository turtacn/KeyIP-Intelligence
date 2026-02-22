package claim_bert

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/internal/intelligence/common"
)

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

type MockTokenizer struct {
	mock.Mock
}

func (m *MockTokenizer) Encode(text string) (*EncodedInput, error) {
	args := m.Called(text)
	return args.Get(0).(*EncodedInput), args.Error(1)
}
// ... other methods stubbed
func (m *MockTokenizer) Tokenize(text string) (*TokenizedOutput, error) { return nil, nil }
func (m *MockTokenizer) EncodePair(textA, textB string) (*EncodedInput, error) { return nil, nil }
func (m *MockTokenizer) Decode(ids []int) (string, error) { return "", nil }
func (m *MockTokenizer) BatchEncode(texts []string) ([]*EncodedInput, error) { return nil, nil }
func (m *MockTokenizer) VocabSize() int { return 0 }

type MockLogger struct {
	logging.Logger
}

func TestParseClaim_Success(t *testing.T) {
	backend := new(MockModelBackend)
	tokenizer := new(MockTokenizer)
	config := NewClaimBERTConfig()
	logger := new(MockLogger)

	parser := NewClaimParser(backend, config, tokenizer, logger)

	tokenizer.On("Encode", mock.Anything).Return(&EncodedInput{}, nil)
	backend.On("Predict", mock.Anything, mock.Anything).Return(&common.PredictResponse{}, nil)

	claim, err := parser.ParseClaim(context.Background(), "1. A compound comprising X.")
	assert.NoError(t, err)
	assert.Equal(t, 1, claim.ClaimNumber)
}
//Personal.AI order the ending
