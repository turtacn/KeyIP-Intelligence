package common

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
)

// MockModelLoader
type MockModelLoader struct {
	mock.Mock
}

func (m *MockModelLoader) Load(ctx context.Context, artifactPath string) (interface{}, error) {
	args := m.Called(ctx, artifactPath)
	return args.Get(0), args.Error(1)
}

func (m *MockModelLoader) Unload(ctx context.Context, modelHandle interface{}) error {
	args := m.Called(ctx, modelHandle)
	return args.Error(0)
}

func (m *MockModelLoader) Validate(ctx context.Context, artifactPath string, checksum string) error {
	args := m.Called(ctx, artifactPath, checksum)
	return args.Error(0)
}

// MockLogger (simplified)
type MockLogger struct {
	logging.Logger
	mock.Mock
}

func (m *MockLogger) Info(msg string, fields ...logging.Field) {
	m.Called(msg, fields)
}

func (m *MockLogger) Error(msg string, fields ...logging.Field) {
	m.Called(msg, fields)
}

func (m *MockLogger) Warn(msg string, fields ...logging.Field) {
	m.Called(msg, fields)
}

func TestNewModelRegistry_Success(t *testing.T) {
	loader := new(MockModelLoader)
	metrics := NewNoopIntelligenceMetrics()
	logger := new(MockLogger)

	registry, err := NewModelRegistry(loader, metrics, logger)
	assert.NoError(t, err)
	assert.NotNil(t, registry)
}

func TestRegister_Success(t *testing.T) {
	loader := new(MockModelLoader)
	loader.On("Validate", mock.Anything, "path/to/model", "checksum").Return(nil)

	logger := new(MockLogger)
	logger.On("Info", mock.Anything, mock.Anything).Return() // Allow Info calls

	registry, _ := NewModelRegistry(loader, NewNoopIntelligenceMetrics(), logger)

	meta := &ModelMetadata{
		ModelID:      "test-model",
		Version:      "v1",
		ArtifactPath: "path/to/model",
		Checksum:     "checksum",
	}

	err := registry.Register(context.Background(), meta)
	assert.NoError(t, err)
}

func TestRegister_DuplicateVersion(t *testing.T) {
	loader := new(MockModelLoader)
	loader.On("Validate", mock.Anything, "path/to/model", "checksum").Return(nil)

	logger := new(MockLogger)
	logger.On("Info", mock.Anything, mock.Anything).Return()

	registry, _ := NewModelRegistry(loader, NewNoopIntelligenceMetrics(), logger)

	meta := &ModelMetadata{
		ModelID:      "test-model",
		Version:      "v1",
		ArtifactPath: "path/to/model",
		Checksum:     "checksum",
	}

	registry.Register(context.Background(), meta)
	err := registry.Register(context.Background(), meta)
	assert.ErrorIs(t, err, ErrVersionAlreadyExists)
}

func TestSetActiveVersion_Success(t *testing.T) {
	loader := new(MockModelLoader)
	loader.On("Validate", mock.Anything, "path/to/model", "checksum").Return(nil)
	loader.On("Load", mock.Anything, "path/to/model").Return(nil, nil)

	logger := new(MockLogger)
	logger.On("Info", mock.Anything, mock.Anything).Return()

	registry, _ := NewModelRegistry(loader, NewNoopIntelligenceMetrics(), logger)

	meta := &ModelMetadata{
		ModelID:      "test-model",
		Version:      "v1",
		ArtifactPath: "path/to/model",
		Checksum:     "checksum",
	}

	registry.Register(context.Background(), meta)
	err := registry.SetActiveVersion(context.Background(), "test-model", "v1")
	assert.NoError(t, err)

	model, err := registry.GetModel(context.Background(), "test-model")
	assert.NoError(t, err)
	assert.Equal(t, "v1", model.ActiveVersion)
}

func TestResolveModel_ABTest_DeterministicRouting(t *testing.T) {
	loader := new(MockModelLoader)
	loader.On("Validate", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	loader.On("Load", mock.Anything, mock.Anything).Return(nil, nil)

	logger := new(MockLogger)
	logger.On("Info", mock.Anything, mock.Anything).Return()

	registry, _ := NewModelRegistry(loader, NewNoopIntelligenceMetrics(), logger)

	// Register v1 and v2
	registry.Register(context.Background(), &ModelMetadata{ModelID: "ab-model", Version: "v1", ArtifactPath: "p1"})
	registry.Register(context.Background(), &ModelMetadata{ModelID: "ab-model", Version: "v2", ArtifactPath: "p2"})

	registry.SetActiveVersion(context.Background(), "ab-model", "v1")

	// Configure AB Test: 50% v1, 50% v2
	config := &ABTestConfig{
		ModelID:   "ab-model",
		StartTime: time.Now().Add(-1 * time.Hour),
		EndTime:   time.Now().Add(1 * time.Hour),
		Enabled:   true,
		Variants: []*ABTestVariant{
			{Version: "v1", TrafficWeight: 50},
			{Version: "v2", TrafficWeight: 50},
		},
	}
	registry.ConfigureABTest(context.Background(), config)

	model1, _ := registry.ResolveModel(context.Background(), "ab-model", "req1")
	model2, _ := registry.ResolveModel(context.Background(), "ab-model", "req1")

	assert.Equal(t, model1.ActiveVersion, model2.ActiveVersion)
	assert.Equal(t, "v2", model1.ActiveVersion)
}
//Personal.AI order the ending
