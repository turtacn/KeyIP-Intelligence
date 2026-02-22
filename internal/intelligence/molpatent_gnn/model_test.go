package molpatent_gnn

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/turtacn/KeyIP-Intelligence/internal/intelligence/common"
)

type TypedMockRegistry struct {
	mock.Mock
}

func (m *TypedMockRegistry) Register(ctx context.Context, meta *common.ModelMetadata) error {
	args := m.Called(ctx, meta)
	return args.Error(0)
}

func (m *TypedMockRegistry) Unregister(ctx context.Context, modelID string, version string) error { return nil }
func (m *TypedMockRegistry) GetModel(ctx context.Context, modelID string) (*common.RegisteredModel, error) { return nil, nil }
func (m *TypedMockRegistry) GetModelVersion(ctx context.Context, modelID string, version string) (*common.RegisteredModel, error) { return nil, nil }
func (m *TypedMockRegistry) ListModels(ctx context.Context) ([]*common.RegisteredModel, error) { return nil, nil }
func (m *TypedMockRegistry) ListVersions(ctx context.Context, modelID string) ([]*common.ModelVersion, error) { return nil, nil }
func (m *TypedMockRegistry) SetActiveVersion(ctx context.Context, modelID string, version string) error { return nil }
func (m *TypedMockRegistry) Rollback(ctx context.Context, modelID string) error { return nil }
func (m *TypedMockRegistry) ConfigureABTest(ctx context.Context, config *common.ABTestConfig) error { return nil }
func (m *TypedMockRegistry) ResolveModel(ctx context.Context, modelID string, requestID string) (*common.RegisteredModel, error) { return nil, nil }
func (m *TypedMockRegistry) HealthCheck(ctx context.Context) (*common.RegistryHealth, error) { return nil, nil }

func TestNewGNNModelConfig_Defaults(t *testing.T) {
	c := NewGNNModelConfig()
	assert.Equal(t, 256, c.EmbeddingDim)
	assert.Equal(t, 64, c.MaxBatchSize)
}

func TestGNNModelConfig_Validate(t *testing.T) {
	c := NewGNNModelConfig()
	assert.NoError(t, c.Validate())

	c.EmbeddingDim = -1
	assert.Error(t, c.Validate())
}

func TestGNNModelConfig_Register(t *testing.T) {
	c := NewGNNModelConfig()
	c.ModelID = "gnn-test"
	c.ModelPath = "path"

	registry := new(TypedMockRegistry)
	registry.On("Register", mock.Anything, mock.Anything).Return(nil)

	err := c.RegisterToRegistry(registry)
	assert.NoError(t, err)
	registry.AssertCalled(t, "Register", mock.Anything, mock.Anything)
}
//Personal.AI order the ending
