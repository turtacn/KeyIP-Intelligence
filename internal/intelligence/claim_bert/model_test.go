package claim_bert

import (
	"fmt"
	"strings"
	"testing"

	"github.com/turtacn/KeyIP-Intelligence/internal/intelligence/common"
)

// ---------------------------------------------------------------------------
// Mock ModelRegistry
// ---------------------------------------------------------------------------

type mockRegistry struct {
	registered []common.ModelDescriptor
	err        error
}

func (r *mockRegistry) Register(desc common.ModelDescriptor) error {
	if r.err != nil {
		return r.err
	}
	r.registered = append(r.registered, desc)
	return nil
}

func (r *mockRegistry) Lookup(modelID string) (*common.ModelDescriptor, error) {
	for _, d := range r.registered {
		if d.ModelID == modelID {
			return &d, nil
		}
	}
	return nil, fmt.Errorf("not found")
}

func (r *mockRegistry) List() ([]common.ModelDescriptor, error) {
	return r.registered, nil
}

func (r *mockRegistry) Unregister(modelID string) error { return nil }

// ---------------------------------------------------------------------------
// TestNewClaimBERTConfig_Defaults
// ---------------------------------------------------------------------------

func TestNewClaimBERTConfig_Defaults(t *testing.T) {
	cfg := NewClaimBERTConfig(nil)

	if cfg.MaxSequenceLength != 512 {
		t.Errorf("MaxSequenceLength: want 512, got %d", cfg.MaxSequenceLength)
	}
	if cfg.HiddenDim != 768 {
		t.Errorf("HiddenDim: want 768, got %d", cfg.HiddenDim)
	}
	if cfg.NumAttentionHeads != 12 {
		t.Errorf("NumAttentionHeads: want 12, got %d", cfg.NumAttentionHeads)
	}
	if cfg.NumLayers != 12 {
		t.Errorf("NumLayers: want 12, got %d", cfg.NumLayers)
	}
	if cfg.VocabSize != 31000 {
		t.Errorf("VocabSize: want 31000, got %d", cfg.VocabSize)
	}
	if cfg.PoolingStrategy != PoolingCLS {
		t.Errorf("PoolingStrategy: want CLS, got %s", cfg.PoolingStrategy)
	}
	if cfg.TimeoutMs != 3000 {
		t.Errorf("TimeoutMs: want 3000, got %d", cfg.TimeoutMs)
	}
	if cfg.MaxBatchSize != 32 {
		t.Errorf("MaxBatchSize: want 32, got %d", cfg.MaxBatchSize)
	}
	if cfg.ModelID != "claim-bert-v1.0.0" {
		t.Errorf("ModelID: want claim-bert-v1.0.0, got %s", cfg.ModelID)
	}
	if len(cfg.TaskHeads) != 4 {
		t.Errorf("TaskHeads count: want 4, got %d", len(cfg.TaskHeads))
	}
}

// ---------------------------------------------------------------------------
// TestNewClaimBERTConfig_FromConfig
// ---------------------------------------------------------------------------

func TestNewClaimBERTConfig_FromConfig(t *testing.T) {
	overrides := &ClaimBERTConfig{
		ModelID:           "claim-bert-v2.1.0",
		ModelPath:         "/models/claim-bert-v2",
		BackendType:       common.BackendONNX,
		MaxSequenceLength: 1024,
		HiddenDim:         1024,
		NumAttentionHeads: 16,
		NumLayers:         24,
		VocabSize:         50000,
		PoolingStrategy:   PoolingMean,
		TimeoutMs:         5000,
		MaxBatchSize:      64,
		TaskHeads: []TaskHeadConfig{
			{TaskName: TaskClaimClassification, OutputDim: 5, ActivationType: ActivationSoftmax, Enabled: true},
		},
		Labels: map[string]string{"env": "prod"},
	}

	cfg := NewClaimBERTConfig(overrides)

	if cfg.ModelID != "claim-bert-v2.1.0" {
		t.Errorf("ModelID: want claim-bert-v2.1.0, got %s", cfg.ModelID)
	}
	if cfg.ModelPath != "/models/claim-bert-v2" {
		t.Errorf("ModelPath mismatch")
	}
	if cfg.BackendType != common.BackendONNX {
		t.Errorf("BackendType mismatch")
	}
	if cfg.MaxSequenceLength != 1024 {
		t.Errorf("MaxSequenceLength: want 1024, got %d", cfg.MaxSequenceLength)
	}
	if cfg.HiddenDim != 1024 {
		t.Errorf("HiddenDim: want 1024, got %d", cfg.HiddenDim)
	}
	if cfg.NumAttentionHeads != 16 {
		t.Errorf("NumAttentionHeads: want 16, got %d", cfg.NumAttentionHeads)
	}
	if cfg.NumLayers != 24 {
		t.Errorf("NumLayers: want 24, got %d", cfg.NumLayers)
	}
	if cfg.VocabSize != 50000 {
		t.Errorf("VocabSize: want 50000, got %d", cfg.VocabSize)
	}
	if cfg.PoolingStrategy != PoolingMean {
		t.Errorf("PoolingStrategy: want Mean, got %s", cfg.PoolingStrategy)
	}
	if cfg.TimeoutMs != 5000 {
		t.Errorf("TimeoutMs: want 5000, got %d", cfg.TimeoutMs)
	}
	if cfg.MaxBatchSize != 64 {
		t.Errorf("MaxBatchSize: want 64, got %d", cfg.MaxBatchSize)
	}
	if len(cfg.TaskHeads) != 1 {
		t.Errorf("TaskHeads count: want 1, got %d", len(cfg.TaskHeads))
	}
	if cfg.Labels["env"] != "prod" {
		t.Errorf("Labels[env]: want prod, got %s", cfg.Labels["env"])
	}
}

// ---------------------------------------------------------------------------
// TestClaimBERTConfig_Validate_Success
// ---------------------------------------------------------------------------

func TestClaimBERTConfig_Validate_Success(t *testing.T) {
	cfg := NewClaimBERTConfig(nil)
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected valid config, got error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// TestClaimBERTConfig_Validate_InvalidSequenceLength
// ---------------------------------------------------------------------------

func TestClaimBERTConfig_Validate_InvalidSequenceLength(t *testing.T) {
	cfg := NewClaimBERTConfig(nil)
	cfg.MaxSequenceLength = 500 // not a power of 2
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for non-power-of-2 MaxSequenceLength")
	}
	if !strings.Contains(err.Error(), "power of 2") {
		t.Errorf("error should mention 'power of 2', got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// TestClaimBERTConfig_Validate_SequenceLengthTooLarge
// ---------------------------------------------------------------------------

func TestClaimBERTConfig_Validate_SequenceLengthTooLarge(t *testing.T) {
	cfg := NewClaimBERTConfig(nil)
	cfg.MaxSequenceLength = 4096 // > 2048
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for MaxSequenceLength > 2048")
	}
	if !strings.Contains(err.Error(), "2048") {
		t.Errorf("error should mention 2048 limit, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// TestClaimBERTConfig_Validate_HiddenDimNotDivisible
// ---------------------------------------------------------------------------

func TestClaimBERTConfig_Validate_HiddenDimNotDivisible(t *testing.T) {
	cfg := NewClaimBERTConfig(nil)
	cfg.HiddenDim = 700
	cfg.NumAttentionHeads = 12
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error when HiddenDim is not divisible by NumAttentionHeads")
	}
	if !strings.Contains(err.Error(), "divisible") {
		t.Errorf("error should mention divisibility, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// TestClaimBERTConfig_Validate_NoEnabledTaskHead
// ---------------------------------------------------------------------------

func TestClaimBERTConfig_Validate_NoEnabledTaskHead(t *testing.T) {
	cfg := NewClaimBERTConfig(nil)
	// Disable all task heads.
	for i := range cfg.TaskHeads {
		cfg.TaskHeads[i].Enabled = false
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error when no task head is enabled")
	}
	if !strings.Contains(err.Error(), "enabled") {
		t.Errorf("error should mention enabled, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// TestClaimBERTConfig_Validate_ZeroVocabSize
// ---------------------------------------------------------------------------

func TestClaimBERTConfig_Validate_ZeroVocabSize(t *testing.T) {
	cfg := NewClaimBERTConfig(nil)
	cfg.VocabSize = 0
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for zero VocabSize")
	}
	if !strings.Contains(err.Error(), "vocab_size") {
		t.Errorf("error should mention vocab_size, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// TestClaimBERTConfig_Validate_InvalidPoolingStrategy
// ---------------------------------------------------------------------------

func TestClaimBERTConfig_Validate_InvalidPoolingStrategy(t *testing.T) {
	cfg := NewClaimBERTConfig(nil)
	cfg.PoolingStrategy = "Unknown"
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for unknown PoolingStrategy")
	}
	if !strings.Contains(err.Error(), "pooling_strategy") {
		t.Errorf("error should mention pooling_strategy, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// TestClaimBERTConfig_Validate_EmptyModelID
// ---------------------------------------------------------------------------

func TestClaimBERTConfig_Validate_EmptyModelID(t *testing.T) {
	cfg := NewClaimBERTConfig(nil)
	cfg.ModelID = ""
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for empty ModelID")
	}
}

// ---------------------------------------------------------------------------
// TestClaimBERTConfig_Validate_ZeroNumLayers
// ---------------------------------------------------------------------------

func TestClaimBERTConfig_Validate_ZeroNumLayers(t *testing.T) {
	cfg := NewClaimBERTConfig(nil)
	cfg.NumLayers = 0
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for zero NumLayers")
	}
}

// ---------------------------------------------------------------------------
// TestClaimBERTConfig_Validate_ZeroTimeoutMs
// ---------------------------------------------------------------------------

func TestClaimBERTConfig_Validate_ZeroTimeoutMs(t *testing.T) {
	cfg := NewClaimBERTConfig(nil)
	cfg.TimeoutMs = 0
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for zero TimeoutMs")
	}
}

// ---------------------------------------------------------------------------
// TestClaimBERTConfig_Validate_ZeroMaxBatchSize
// ---------------------------------------------------------------------------

func TestClaimBERTConfig_Validate_ZeroMaxBatchSize(t *testing.T) {
	cfg := NewClaimBERTConfig(nil)
	cfg.MaxBatchSize = 0
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for zero MaxBatchSize")
	}
}

// ---------------------------------------------------------------------------
// TestClaimBERTConfig_Validate_EmptyTaskHeadName
// ---------------------------------------------------------------------------

func TestClaimBERTConfig_Validate_EmptyTaskHeadName(t *testing.T) {
	cfg := NewClaimBERTConfig(nil)
	cfg.TaskHeads = []TaskHeadConfig{
		{TaskName: "", OutputDim: 5, ActivationType: ActivationSoftmax, Enabled: true},
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for empty task head name")
	}
	if !strings.Contains(err.Error(), "task_name") {
		t.Errorf("error should mention task_name, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// TestClaimBERTConfig_Validate_ZeroOutputDim
// ---------------------------------------------------------------------------

func TestClaimBERTConfig_Validate_ZeroOutputDim(t *testing.T) {
	cfg := NewClaimBERTConfig(nil)
	cfg.TaskHeads = []TaskHeadConfig{
		{TaskName: TaskClaimClassification, OutputDim: 0, ActivationType: ActivationSoftmax, Enabled: true},
	}
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for zero OutputDim")
	}
	if !strings.Contains(err.Error(), "output_dim") {
		t.Errorf("error should mention output_dim, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// TestClaimBERTConfig_Validate_PowerOfTwoBoundary
// ---------------------------------------------------------------------------

func TestClaimBERTConfig_Validate_PowerOfTwoBoundary(t *testing.T) {
	validPowers := []int{64, 128, 256, 512, 1024, 2048}
	for _, p := range validPowers {
		cfg := NewClaimBERTConfig(nil)
		cfg.MaxSequenceLength = p
		if err := cfg.Validate(); err != nil {
			t.Errorf("MaxSequenceLength=%d should be valid, got: %v", p, err)
		}
	}

	invalidValues := []int{3, 5, 6, 7, 9, 100, 300, 500, 600, 1000, 1500, 2000}
	for _, v := range invalidValues {
		cfg := NewClaimBERTConfig(nil)
		cfg.MaxSequenceLength = v
		if err := cfg.Validate(); err == nil {
			t.Errorf("MaxSequenceLength=%d should be invalid (not power of 2)", v)
		}
	}
}

// ---------------------------------------------------------------------------
// TestClaimBERTConfig_DefaultTaskHeads
// ---------------------------------------------------------------------------

func TestClaimBERTConfig_DefaultTaskHeads(t *testing.T) {
	heads := DefaultTaskHeads()
	if len(heads) != 4 {
		t.Fatalf("expected 4 default task heads, got %d", len(heads))
	}

	expectedTasks := map[string]struct {
		outputDim  int
		activation string
		enabled    bool
	}{
		TaskClaimClassification: {claimClassificationOutputDim, ActivationSoftmax, true},
		TaskFeatureExtraction:   {featureExtractionOutputDim, ActivationSoftmax, true},
		TaskScopeAnalysis:       {scopeAnalysisOutputDim, ActivationLinear, true},
		TaskDependencyParsing:   {dependencyParsingOutputDim, ActivationSoftmax, true},
	}

	for _, h := range heads {
		exp, ok := expectedTasks[h.TaskName]
		if !ok {
			t.Errorf("unexpected task head: %s", h.TaskName)
			continue
		}
		if h.OutputDim != exp.outputDim {
			t.Errorf("%s: OutputDim want %d, got %d", h.TaskName, exp.outputDim, h.OutputDim)
		}
		if h.ActivationType != exp.activation {
			t.Errorf("%s: ActivationType want %s, got %s", h.TaskName, exp.activation, h.ActivationType)
		}
		if h.Enabled != exp.enabled {
			t.Errorf("%s: Enabled want %v, got %v", h.TaskName, exp.enabled, h.Enabled)
		}
		if h.Description == "" {
			t.Errorf("%s: Description should not be empty", h.TaskName)
		}
	}
}

// ---------------------------------------------------------------------------
// TestTaskHeadConfig_ClaimClassification
// ---------------------------------------------------------------------------

func TestTaskHeadConfig_ClaimClassification(t *testing.T) {
	heads := DefaultTaskHeads()
	var found *TaskHeadConfig
	for i := range heads {
		if heads[i].TaskName == TaskClaimClassification {
			found = &heads[i]
			break
		}
	}
	if found == nil {
		t.Fatal("ClaimClassification task head not found")
	}
	if found.OutputDim != 5 {
		t.Errorf("ClaimClassification OutputDim: want 5, got %d", found.OutputDim)
	}
	if found.ActivationType != ActivationSoftmax {
		t.Errorf("ClaimClassification ActivationType: want Softmax, got %s", found.ActivationType)
	}
}

// ---------------------------------------------------------------------------
// TestTaskHeadConfig_FeatureExtraction
// ---------------------------------------------------------------------------

func TestTaskHeadConfig_FeatureExtraction(t *testing.T) {
	heads := DefaultTaskHeads()
	var found *TaskHeadConfig
	for i := range heads {
		if heads[i].TaskName == TaskFeatureExtraction {
			found = &heads[i]
			break
		}
	}
	if found == nil {
		t.Fatal("FeatureExtraction task head not found")
	}
	// BIO labels: B-COMPOUND, I-COMPOUND, B-CONDITION, I-CONDITION,
	//             B-PROPERTY, I-PROPERTY, B-MARKUSH, I-MARKUSH, O = 9
	if found.OutputDim != 9 {
		t.Errorf("FeatureExtraction OutputDim: want 9 (BIO labels), got %d", found.OutputDim)
	}
	if found.ActivationType != ActivationSoftmax {
		t.Errorf("FeatureExtraction ActivationType: want Softmax, got %s", found.ActivationType)
	}
}

// ---------------------------------------------------------------------------
// TestTaskHeadConfig_ScopeAnalysis
// ---------------------------------------------------------------------------

func TestTaskHeadConfig_ScopeAnalysis(t *testing.T) {
	heads := DefaultTaskHeads()
	var found *TaskHeadConfig
	for i := range heads {
		if heads[i].TaskName == TaskScopeAnalysis {
			found = &heads[i]
			break
		}
	}
	if found == nil {
		t.Fatal("ScopeAnalysis task head not found")
	}
	if found.OutputDim != 1 {
		t.Errorf("ScopeAnalysis OutputDim: want 1 (regression), got %d", found.OutputDim)
	}
	if found.ActivationType != ActivationLinear {
		t.Errorf("ScopeAnalysis ActivationType: want Linear, got %s", found.ActivationType)
	}
}

// ---------------------------------------------------------------------------
// TestTaskHeadConfig_DependencyParsing
// ---------------------------------------------------------------------------

func TestTaskHeadConfig_DependencyParsing(t *testing.T) {
	heads := DefaultTaskHeads()
	var found *TaskHeadConfig
	for i := range heads {
		if heads[i].TaskName == TaskDependencyParsing {
			found = &heads[i]
			break
		}
	}
	if found == nil {
		t.Fatal("DependencyParsing task head not found")
	}
	if found.OutputDim != 128 {
		t.Errorf("DependencyParsing OutputDim: want 128, got %d", found.OutputDim)
	}
	if found.ActivationType != ActivationSoftmax {
		t.Errorf("DependencyParsing ActivationType: want Softmax, got %s", found.ActivationType)
	}
}

// ---------------------------------------------------------------------------
// TestClaimBERTConfig_RegisterToRegistry_Success
// ---------------------------------------------------------------------------

func TestClaimBERTConfig_RegisterToRegistry_Success(t *testing.T) {
	cfg := NewClaimBERTConfig(nil)
	reg := &mockRegistry{}

	err := cfg.RegisterToRegistry(reg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(reg.registered) != 1 {
		t.Fatalf("expected 1 registered model, got %d", len(reg.registered))
	}
	desc := reg.registered[0]
	if desc.ModelID != cfg.ModelID {
		t.Errorf("registered ModelID: want %s, got %s", cfg.ModelID, desc.ModelID)
	}
	if desc.ModelType != "claim-bert" {
		t.Errorf("registered ModelType: want claim-bert, got %s", desc.ModelType)
	}
}

// ---------------------------------------------------------------------------
// TestClaimBERTConfig_RegisterToRegistry_Error
// ---------------------------------------------------------------------------

func TestClaimBERTConfig_RegisterToRegistry_Error(t *testing.T) {
	cfg := NewClaimBERTConfig(nil)
	reg := &mockRegistry{err: fmt.Errorf("registry full")}

	err := cfg.RegisterToRegistry(reg)
	if err == nil {
		t.Fatal("expected error from registry")
	}
	if !strings.Contains(err.Error(), "registry full") {
		t.Errorf("error should propagate registry error, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// TestClaimBERTConfig_RegisterToRegistry_NilRegistry
// ---------------------------------------------------------------------------

func TestClaimBERTConfig_RegisterToRegistry_NilRegistry(t *testing.T) {
	cfg := NewClaimBERTConfig(nil)
	err := cfg.RegisterToRegistry(nil)
	if err == nil {
		t.Fatal("expected error for nil registry")
	}
}

// ---------------------------------------------------------------------------
// TestClaimBERTConfig_RegisterToRegistry_InvalidConfig
// ---------------------------------------------------------------------------

func TestClaimBERTConfig_RegisterToRegistry_InvalidConfig(t *testing.T) {
	cfg := NewClaimBERTConfig(nil)
	cfg.VocabSize = 0 // invalid
	reg := &mockRegistry{}

	err := cfg.RegisterToRegistry(reg)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if len(reg.registered) != 0 {
		t.Error("should not register when config is invalid")
	}
}

// ---------------------------------------------------------------------------
// TestClaimBERTConfig_ModelDescriptor
// ---------------------------------------------------------------------------

func TestClaimBERTConfig_ModelDescriptor(t *testing.T) {
	cfg := NewClaimBERTConfig(nil)
	desc := cfg.ModelDescriptor()

	if desc.ModelID != cfg.ModelID {
		t.Errorf("ModelID: want %s, got %s", cfg.ModelID, desc.ModelID)
	}
	if desc.ModelVersion != "1.0.0" {
		t.Errorf("ModelVersion: want 1.0.0, got %s", desc.ModelVersion)
	}
	if desc.ModelType != "claim-bert" {
		t.Errorf("ModelType: want claim-bert, got %s", desc.ModelType)
	}
	if desc.Framework != "pytorch" {
		t.Errorf("Framework: want pytorch, got %s", desc.Framework)
	}

	// Input schema should have 3 fields.
	if len(desc.InputSchema.Fields) != 3 {
		t.Errorf("InputSchema fields: want 3, got %d", len(desc.InputSchema.Fields))
	}
	inputNames := map[string]bool{}
	for _, f := range desc.InputSchema.Fields {
		inputNames[f.Name] = true
	}
	for _, expected := range []string{"input_ids", "attention_mask", "token_type_ids"} {
		if !inputNames[expected] {
			t.Errorf("missing input field: %s", expected)
		}
	}

	// Output schema should have fields for each enabled task head.
	enabledCount := len(cfg.EnabledTaskHeads())
	if len(desc.OutputSchema.Fields) != enabledCount {
		t.Errorf("OutputSchema fields: want %d, got %d", enabledCount, len(desc.OutputSchema.Fields))
	}

	// Metadata checks.
	if desc.Metadata["max_sequence_length"] != "512" {
		t.Errorf("metadata max_sequence_length: want 512, got %s", desc.Metadata["max_sequence_length"])
	}
	if desc.Metadata["hidden_dim"] != "768" {
		t.Errorf("metadata hidden_dim: want 768, got %s", desc.Metadata["hidden_dim"])
	}
	if desc.Metadata["domain"] != "chemical-patent-claims" {
		t.Errorf("metadata domain: want chemical-patent-claims, got %s", desc.Metadata["domain"])
	}
	if !strings.Contains(desc.Metadata["languages"], "zh") {
		t.Errorf("metadata languages should contain zh, got %s", desc.Metadata["languages"])
	}
}

// ---------------------------------------------------------------------------
// TestClaimBERTConfig_ModelDescriptor_VersionExtraction
// ---------------------------------------------------------------------------

func TestClaimBERTConfig_ModelDescriptor_VersionExtraction(t *testing.T) {
	tests := []struct {
		modelID     string
		wantVersion string
	}{
		{"claim-bert-v1.0.0", "1.0.0"},
		{"claim-bert-v2.3.1", "2.3.1"},
		{"claim-bert-v10.0.0-beta", "10.0.0-beta"},
		{"custom-model", "custom-model"}, // no -v prefix → full ID as version
	}
	for _, tt := range tests {
		cfg := NewClaimBERTConfig(&ClaimBERTConfig{ModelID: tt.modelID})
		desc := cfg.ModelDescriptor()
		if desc.ModelVersion != tt.wantVersion {
			t.Errorf("ModelID=%s: version want %s, got %s", tt.modelID, tt.wantVersion, desc.ModelVersion)
		}
	}
}

// ---------------------------------------------------------------------------
// TestClaimBERTConfig_EnabledTaskHeads
// ---------------------------------------------------------------------------

func TestClaimBERTConfig_EnabledTaskHeads(t *testing.T) {
	cfg := NewClaimBERTConfig(nil)

	// All 4 enabled by default.
	enabled := cfg.EnabledTaskHeads()
	if len(enabled) != 4 {
		t.Errorf("expected 4 enabled heads, got %d", len(enabled))
	}

	// Disable two.
	cfg.TaskHeads[1].Enabled = false
	cfg.TaskHeads[3].Enabled = false
	enabled = cfg.EnabledTaskHeads()
	if len(enabled) != 2 {
		t.Errorf("expected 2 enabled heads after disabling, got %d", len(enabled))
	}
}

// ---------------------------------------------------------------------------
// TestClaimBERTConfig_HasTask
// ---------------------------------------------------------------------------

func TestClaimBERTConfig_HasTask(t *testing.T) {
	cfg := NewClaimBERTConfig(nil)

	if !cfg.HasTask(TaskClaimClassification) {
		t.Error("expected HasTask(ClaimClassification) = true")
	}
	if !cfg.HasTask(TaskScopeAnalysis) {
		t.Error("expected HasTask(ScopeAnalysis) = true")
	}
	if cfg.HasTask("NonExistentTask") {
		t.Error("expected HasTask(NonExistentTask) = false")
	}

	// Disable a task and check.
	for i := range cfg.TaskHeads {
		if cfg.TaskHeads[i].TaskName == TaskClaimClassification {
			cfg.TaskHeads[i].Enabled = false
		}
	}
	if cfg.HasTask(TaskClaimClassification) {
		t.Error("expected HasTask(ClaimClassification) = false after disabling")
	}
}

// ---------------------------------------------------------------------------
// TestClaimBERTConfig_HeadDimPerAttention
// ---------------------------------------------------------------------------

func TestClaimBERTConfig_HeadDimPerAttention(t *testing.T) {
	cfg := NewClaimBERTConfig(nil)
	// 768 / 12 = 64
	if got := cfg.HeadDimPerAttention(); got != 64 {
		t.Errorf("HeadDimPerAttention: want 64, got %d", got)
	}

	cfg.HiddenDim = 1024
	cfg.NumAttentionHeads = 16
	// 1024 / 16 = 64
	if got := cfg.HeadDimPerAttention(); got != 64 {
		t.Errorf("HeadDimPerAttention: want 64, got %d", got)
	}

	cfg.NumAttentionHeads = 0
	if got := cfg.HeadDimPerAttention(); got != 0 {
		t.Errorf("HeadDimPerAttention with 0 heads: want 0, got %d", got)
	}
}

// ---------------------------------------------------------------------------
// TestIsPowerOfTwo
// ---------------------------------------------------------------------------

func TestIsPowerOfTwo(t *testing.T) {
	positives := []int{1, 2, 4, 8, 16, 32, 64, 128, 256, 512, 1024, 2048}
	for _, v := range positives {
		if !isPowerOfTwo(v) {
			t.Errorf("isPowerOfTwo(%d) should be true", v)
		}
	}

	negatives := []int{0, -1, 3, 5, 6, 7, 9, 10, 12, 15, 100, 500, 1000}
	for _, v := range negatives {
		if isPowerOfTwo(v) {
			t.Errorf("isPowerOfTwo(%d) should be false", v)
		}
	}
}

// ---------------------------------------------------------------------------
// TestClaimBERTConfig_Validate_NegativeHiddenDim
// ---------------------------------------------------------------------------

func TestClaimBERTConfig_Validate_NegativeHiddenDim(t *testing.T) {
	cfg := NewClaimBERTConfig(nil)
	cfg.HiddenDim = -1
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for negative HiddenDim")
	}
}

// ---------------------------------------------------------------------------
// TestClaimBERTConfig_Validate_NegativeNumAttentionHeads
// ---------------------------------------------------------------------------

func TestClaimBERTConfig_Validate_NegativeNumAttentionHeads(t *testing.T) {
	cfg := NewClaimBERTConfig(nil)
	cfg.NumAttentionHeads = -1
	err := cfg.Validate()
	if err == nil {
		t.Fatal("expected error for negative NumAttentionHeads")
	}
}

// ---------------------------------------------------------------------------
// TestClaimBERTConfig_Validate_LargeValidConfig
// ---------------------------------------------------------------------------

func TestClaimBERTConfig_Validate_LargeValidConfig(t *testing.T) {
	cfg := NewClaimBERTConfig(&ClaimBERTConfig{
		ModelID:           "claim-bert-v3.0.0",
		MaxSequenceLength: 2048,
		HiddenDim:         1024,
		NumAttentionHeads: 16,
		NumLayers:         24,
		VocabSize:         50000,
		PoolingStrategy:   PoolingMax,
		TimeoutMs:         10000,
		MaxBatchSize:      128,
	})
	if err := cfg.Validate(); err != nil {
		t.Fatalf("large valid config should pass: %v", err)
	}
}

// ---------------------------------------------------------------------------
// TestClaimBERTConfig_Validate_MinimalValidConfig
// ---------------------------------------------------------------------------

func TestClaimBERTConfig_Validate_MinimalValidConfig(t *testing.T) {
	cfg := &ClaimBERTConfig{
		ModelID:           "claim-bert-v0.1.0",
		MaxSequenceLength: 64,
		HiddenDim:         64,
		NumAttentionHeads: 4,
		NumLayers:         1,
		VocabSize:         100,
		PoolingStrategy:   PoolingCLS,
		TaskHeads: []TaskHeadConfig{
			{TaskName: "test", OutputDim: 2, ActivationType: ActivationSoftmax, Enabled: true},
		},
		TimeoutMs:    100,
		MaxBatchSize: 1,
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("minimal valid config should pass: %v", err)
	}
}

//Personal.AI order the ending
