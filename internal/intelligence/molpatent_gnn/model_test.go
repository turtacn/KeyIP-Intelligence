package molpatent_gnn

import (
	"context"
	"fmt"
	"testing"

	"github.com/turtacn/KeyIP-Intelligence/internal/intelligence/common"
)

func TestDefaultGNNModelConfig(t *testing.T) {
	cfg := DefaultGNNModelConfig()
	if cfg.ModelID == "" {
		t.Error("expected non-empty model_id")
	}
	if cfg.EmbeddingDim != 256 {
		t.Errorf("expected embedding_dim 256, got %d", cfg.EmbeddingDim)
	}
	if cfg.NumLayers != 5 {
		t.Errorf("expected num_layers 5, got %d", cfg.NumLayers)
	}
	if cfg.HiddenDim != 512 {
		t.Errorf("expected hidden_dim 512, got %d", cfg.HiddenDim)
	}
	if cfg.Readout != ReadoutAttention {
		t.Errorf("expected readout attention, got %s", cfg.Readout)
	}
}

func TestGNNModelConfig_Validate_Valid(t *testing.T) {
	cfg := DefaultGNNModelConfig()
	if err := cfg.Validate(); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestGNNModelConfig_Validate_EmptyModelID(t *testing.T) {
	cfg := DefaultGNNModelConfig()
	cfg.ModelID = ""
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for empty model_id")
	}
}

func TestGNNModelConfig_Validate_InvalidEmbeddingDim(t *testing.T) {
	cfg := DefaultGNNModelConfig()
	cfg.EmbeddingDim = 0
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for zero embedding_dim")
	}
}

func TestGNNModelConfig_Validate_InvalidDropout(t *testing.T) {
	cfg := DefaultGNNModelConfig()
	cfg.DropoutRate = 1.0
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for dropout_rate >= 1")
	}
	cfg.DropoutRate = -0.1
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for negative dropout_rate")
	}
}

func TestGNNModelConfig_Validate_InvalidNumLayers(t *testing.T) {
	cfg := DefaultGNNModelConfig()
	cfg.NumLayers = -1
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for negative num_layers")
	}
}

func TestModelState_String(t *testing.T) {
	tests := []struct {
		state ModelState
		want  string
	}{
		{ModelStateUnloaded, "UNLOADED"},
		{ModelStateLoading, "LOADING"},
		{ModelStateReady, "READY"},
		{ModelStateError, "ERROR"},
		{ModelStateUnloading, "UNLOADING"},
		{ModelState(99), "UNKNOWN"},
	}
	for _, tt := range tests {
		got := tt.state.String()
		if got != tt.want {
			t.Errorf("ModelState(%d).String() = %s, want %s", tt.state, got, tt.want)
		}
	}
}

func TestGNNModelManager_Load_Success(t *testing.T) {
	backend := &mockModelBackendModel{}
	cfg := DefaultGNNModelConfig()
	mgr, err := NewGNNModelManager(cfg, backend, nil, nil)
	if err != nil {
		t.Fatalf("NewGNNModelManager: %v", err)
	}
	if err := mgr.Load(context.Background()); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if mgr.State() != ModelStateReady {
		t.Errorf("expected READY, got %s", mgr.State())
	}
}

func TestGNNModelManager_Load_AlreadyReady(t *testing.T) {
	backend := &mockModelBackendModel{}
	cfg := DefaultGNNModelConfig()
	mgr, _ := NewGNNModelManager(cfg, backend, nil, nil)
	_ = mgr.Load(context.Background())
	if err := mgr.Load(context.Background()); err != nil {
		t.Fatalf("second Load should be no-op: %v", err)
	}
}

func TestGNNModelManager_Load_BackendUnhealthy(t *testing.T) {
	// Override Healthy
	unhealthyBackend := &unhealthyMockBackend{}
	cfg := DefaultGNNModelConfig()
	mgr, _ := NewGNNModelManager(cfg, unhealthyBackend, nil, nil)
	if err := mgr.Load(context.Background()); err == nil {
		t.Fatal("expected error for unhealthy backend")
	}
	if mgr.State() != ModelStateError {
		t.Errorf("expected ERROR state, got %s", mgr.State())
	}
	if mgr.LastError() == nil {
		t.Error("expected non-nil LastError")
	}
}

func TestGNNModelManager_Unload(t *testing.T) {
	backend := &mockModelBackendModel{}
	cfg := DefaultGNNModelConfig()
	mgr, _ := NewGNNModelManager(cfg, backend, nil, nil)
	_ = mgr.Load(context.Background())
	if err := mgr.Unload(context.Background()); err != nil {
		t.Fatalf("Unload: %v", err)
	}
	if mgr.State() != ModelStateUnloaded {
		t.Errorf("expected UNLOADED, got %s", mgr.State())
	}
}

func TestGNNModelManager_Unload_AlreadyUnloaded(t *testing.T) {
	backend := &mockModelBackendModel{}
	cfg := DefaultGNNModelConfig()
	mgr, _ := NewGNNModelManager(cfg, backend, nil, nil)
	if err := mgr.Unload(context.Background()); err != nil {
		t.Fatalf("Unload on unloaded should be no-op: %v", err)
	}
}

func TestGNNModelManager_Config(t *testing.T) {
	cfg := DefaultGNNModelConfig()
	mgr, _ := NewGNNModelManager(cfg, &mockModelBackendModel{}, nil, nil)
	if mgr.Config().ModelID != cfg.ModelID {
		t.Error("Config() returned different config")
	}
}

func TestNewGNNModelManager_NilConfig(t *testing.T) {
	_, err := NewGNNModelManager(nil, &mockModelBackendModel{}, nil, nil)
	if err == nil {
		t.Fatal("expected error for nil config")
	}
}

func TestNewGNNModelManager_NilBackend(t *testing.T) {
	_, err := NewGNNModelManager(DefaultGNNModelConfig(), nil, nil, nil)
	if err == nil {
		t.Fatal("expected error for nil backend")
	}
}

func TestNewGNNModelManager_InvalidConfig(t *testing.T) {
	cfg := DefaultGNNModelConfig()
	cfg.EmbeddingDim = -1
	_, err := NewGNNModelManager(cfg, &mockModelBackendModel{}, nil, nil)
	if err == nil {
		t.Fatal("expected error for invalid config")
	}
}

// unhealthyMockBackend always fails health checks.
type unhealthyMockBackend struct{}

func (u *unhealthyMockBackend) Predict(ctx context.Context, req *common.PredictRequest) (*common.PredictResponse, error) {
	return nil, fmt.Errorf("unhealthy")
}
func (u *unhealthyMockBackend) PredictStream(ctx context.Context, req *common.PredictRequest) (<-chan *common.PredictResponse, error) {
	return nil, fmt.Errorf("unhealthy")
}
func (u *unhealthyMockBackend) Healthy(ctx context.Context) error { return fmt.Errorf("unhealthy") }
func (u *unhealthyMockBackend) Close() error                      { return nil }

func TestMolecularGraph_Fields(t *testing.T) {
	g := &MolecularGraph{
		NodeFeatures:   [][]float32{{1, 0}, {0, 1}},
		EdgeIndex:      [][2]int{{0, 1}, {1, 0}},
		EdgeFeatures:   [][]float32{{1.0}, {1.0}},
		GlobalFeatures: []float32{12.0, 6.0},
		NumAtoms:       2,
		NumBonds:       1,
		SMILES:         "CC",
	}
	if g.NumAtoms != 2 {
		t.Errorf("expected 2 atoms, got %d", g.NumAtoms)
	}
	if len(g.EdgeIndex) != 2 {
		t.Errorf("expected 2 edge entries, got %d", len(g.EdgeIndex))
	}
}

func TestAggregationType_Values(t *testing.T) {
	if AggregationSum != "sum" {
		t.Error("AggregationSum mismatch")
	}
	if AggregationMean != "mean" {
		t.Error("AggregationMean mismatch")
	}
	if AggregationMax != "max" {
		t.Error("AggregationMax mismatch")
	}
}

func TestReadoutType_Values(t *testing.T) {
	if ReadoutMeanPool != "mean_pool" {
		t.Error("ReadoutMeanPool mismatch")
	}
	if ReadoutAttention != "attention" {
		t.Error("ReadoutAttention mismatch")
	}
	if ReadoutSet2Set != "set2set" {
		t.Error("ReadoutSet2Set mismatch")
	}
}

// ---------------------------------------------------------------------------
// Mock Backend Definition
// ---------------------------------------------------------------------------

type mockModelBackendModel struct {
	predictFn func(ctx context.Context, req *common.PredictRequest) (*common.PredictResponse, error)
}

func (m *mockModelBackendModel) Predict(ctx context.Context, req *common.PredictRequest) (*common.PredictResponse, error) {
	if m.predictFn != nil {
		return m.predictFn(ctx, req)
	}
	return &common.PredictResponse{Outputs: map[string][]byte{"out": []byte("{}")}}, nil
}

func (m *mockModelBackendModel) PredictStream(ctx context.Context, req *common.PredictRequest) (<-chan *common.PredictResponse, error) {
	return nil, nil
}

func (m *mockModelBackendModel) Healthy(ctx context.Context) error {
	return nil
}

func (m *mockModelBackendModel) Close() error {
	return nil
}
