package infringe_net

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
	"github.com/turtacn/KeyIP-Intelligence/internal/intelligence/common"
)

type MockServingClient struct {
	common.MockServingClient
	mock.Mock
}

func (m *MockServingClient) Healthy(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func TestNewLocalInfringeModel(t *testing.T) {
	m, err := NewLocalInfringeModel(nil, "path", new(MockLogger))
	assert.NoError(t, err)
	assert.NotNil(t, m)
}

func TestNewRemoteInfringeModel(t *testing.T) {
	m, err := NewRemoteInfringeModel(&common.MockServingClient{}, new(MockLogger))
	assert.NoError(t, err)
	assert.NotNil(t, m)
}

func TestLocalModel_PredictLiteral(t *testing.T) {
	m, _ := NewLocalInfringeModel(nil, "path", new(MockLogger))
	res, err := m.PredictLiteralInfringement(context.Background(), &LiteralPredictionRequest{MoleculeSMILES: "C", ClaimElements: []*ClaimElementFeature{{}}})
	assert.NoError(t, err)
	assert.Equal(t, 0.9, res.OverallScore)
}

func TestLocalModel_ComputeSimilarity(t *testing.T) {
	m, _ := NewLocalInfringeModel(nil, "path", new(MockLogger))
	sim, err := m.ComputeStructuralSimilarity(context.Background(), "C", "C")
	assert.NoError(t, err)
	assert.Equal(t, 1.0, sim)
}

// Reuse MockLogger
type MockLogger struct {
	logging.Logger
	mock.Mock
}
func (m *MockLogger) Info(msg string, fields ...logging.Field) {}
//Personal.AI order the ending
