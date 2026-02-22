package claim_bert

import (
	"context"
	"errors"

	"github.com/turtacn/KeyIP-Intelligence/internal/intelligence/common"
)

// BackendType defines the type of inference backend.
type BackendType string

const (
	BackendONNX   BackendType = "onnx"
	BackendTriton BackendType = "triton"
	BackendVLLM   BackendType = "vllm"
	BackendHTTP   BackendType = "http"
)

// TaskHeadConfig configuration for multi-task heads.
type TaskHeadConfig struct {
	TaskName       string `json:"task_name"`
	OutputDim      int    `json:"output_dim"`
	ActivationType string `json:"activation_type"`
	Enabled        bool   `json:"enabled"`
}

// Predefined task names.
const (
	TaskClaimClassification = "claim_classification"
	TaskFeatureExtraction   = "feature_extraction"
	TaskScopeAnalysis       = "scope_analysis"
	TaskDependencyParsing   = "dependency_parsing"
)

// ClaimBERTConfig holds configuration for the ClaimBERT model.
type ClaimBERTConfig struct {
	ModelID           string           `json:"model_id"`
	ModelPath         string           `json:"model_path"`
	BackendType       BackendType      `json:"backend_type"`
	MaxSequenceLength int              `json:"max_sequence_length"`
	HiddenDim         int              `json:"hidden_dim"`
	NumAttentionHeads int              `json:"num_attention_heads"`
	NumLayers         int              `json:"num_layers"`
	VocabSize         int              `json:"vocab_size"`
	PoolingStrategy   string           `json:"pooling_strategy"` // CLS / Mean / Max
	TaskHeads         []TaskHeadConfig `json:"task_heads"`
	TimeoutMs         int              `json:"timeout_ms"`
	MaxBatchSize      int              `json:"max_batch_size"`
}

// NewClaimBERTConfig creates a new configuration with defaults.
func NewClaimBERTConfig() *ClaimBERTConfig {
	return &ClaimBERTConfig{
		MaxSequenceLength: 512,
		HiddenDim:         768,
		NumAttentionHeads: 12,
		NumLayers:         12,
		VocabSize:         31000,
		PoolingStrategy:   "CLS",
		TimeoutMs:         3000,
		MaxBatchSize:      32,
		TaskHeads:         DefaultTaskHeads(),
	}
}

// DefaultTaskHeads returns the default task heads configuration.
func DefaultTaskHeads() []TaskHeadConfig {
	return []TaskHeadConfig{
		{TaskName: TaskClaimClassification, OutputDim: 5, ActivationType: "Softmax", Enabled: true},
		{TaskName: TaskFeatureExtraction, OutputDim: 15, ActivationType: "Softmax", Enabled: true}, // BIO tags
		{TaskName: TaskScopeAnalysis, OutputDim: 1, ActivationType: "Linear", Enabled: true},
		{TaskName: TaskDependencyParsing, OutputDim: 1, ActivationType: "Linear", Enabled: true},
	}
}

// Validate checks if the configuration is valid.
func (c *ClaimBERTConfig) Validate() error {
	if c.MaxSequenceLength <= 0 || c.MaxSequenceLength > 2048 {
		return errors.New("invalid max sequence length")
	}
	// Check if power of 2
	if (c.MaxSequenceLength & (c.MaxSequenceLength - 1)) != 0 {
		return errors.New("max sequence length must be a power of 2")
	}

	if c.HiddenDim <= 0 || c.NumAttentionHeads <= 0 {
		return errors.New("invalid dimensions")
	}

	if c.HiddenDim%c.NumAttentionHeads != 0 {
		return errors.New("hidden dim must be divisible by num attention heads")
	}

	if c.VocabSize <= 0 {
		return errors.New("vocab size must be positive")
	}

	enabledHead := false
	for _, head := range c.TaskHeads {
		if head.Enabled {
			enabledHead = true
			break
		}
	}
	if !enabledHead {
		return errors.New("at least one task head must be enabled")
	}

	return nil
}

// RegisterToRegistry registers the model to the registry.
func (c *ClaimBERTConfig) RegisterToRegistry(registry common.ModelRegistry) error {
	meta := &common.ModelMetadata{
		ModelID:      c.ModelID,
		ModelName:    "ClaimBERT",
		Version:      "v1.0.0", // Simplified
		ArtifactPath: c.ModelPath,
		Architecture: "BERT",
		Tasks:        []string{"Classification", "NER", "Regression"},
		Parameters: map[string]interface{}{
			"max_seq_len": c.MaxSequenceLength,
			"hidden_dim":  c.HiddenDim,
		},
	}
	return registry.Register(context.Background(), meta)
}

//Personal.AI order the ending
