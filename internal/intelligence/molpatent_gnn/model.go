package molpatent_gnn

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

// MolecularGraphConfig holds configuration for molecular graph construction.
type MolecularGraphConfig struct {
	NodeFeatureDim      int    `json:"node_feature_dim"`
	EdgeFeatureDim      int    `json:"edge_feature_dim"`
	MessagePassingSteps int    `json:"message_passing_steps"`
	ReadoutType         string `json:"readout_type"`
}

// PatentGraphConfig holds configuration for patent graph construction.
type PatentGraphConfig struct {
	RelationTypes  []string `json:"relation_types"`
	MaxHops        int      `json:"max_hops"`
	AttentionHeads int      `json:"attention_heads"`
}

// GNNModelConfig holds configuration for the MolPatent-GNN model.
type GNNModelConfig struct {
	ModelID             string               `json:"model_id"`
	ModelPath           string               `json:"model_path"`
	BackendType         BackendType          `json:"backend_type"`
	EmbeddingDim        int                  `json:"embedding_dim"`
	MaxBatchSize        int                  `json:"max_batch_size"`
	TimeoutMs           int                  `json:"timeout_ms"`
	WarmupSamples       int                  `json:"warmup_samples"`
	MolecularGraphConfig MolecularGraphConfig `json:"molecular_graph_config"`
	PatentGraphConfig   PatentGraphConfig    `json:"patent_graph_config"`
}

// NewGNNModelConfig creates a new configuration with defaults.
func NewGNNModelConfig() *GNNModelConfig {
	return &GNNModelConfig{
		EmbeddingDim: 256,
		MaxBatchSize: 64,
		TimeoutMs:    5000,
		WarmupSamples: 10,
		MolecularGraphConfig: MolecularGraphConfig{
			NodeFeatureDim:      39,
			EdgeFeatureDim:      10,
			MessagePassingSteps: 3,
			ReadoutType:         "mean",
		},
		PatentGraphConfig: PatentGraphConfig{
			RelationTypes:  []string{"CITES", "SIMILAR_TO", "CONTAINS_MOLECULE"},
			MaxHops:        2,
			AttentionHeads: 4,
		},
	}
}

// Validate checks if the configuration is valid.
func (c *GNNModelConfig) Validate() error {
	if c.EmbeddingDim <= 0 {
		return errors.New("embedding dim must be positive")
	}
	if c.MaxBatchSize <= 0 || c.MaxBatchSize > 256 {
		return errors.New("max batch size must be positive and <= 256")
	}
	if c.TimeoutMs <= 0 {
		return errors.New("timeout must be positive")
	}
	if c.ModelPath == "" {
		// return errors.New("model path is required")
	}
	return nil
}

// RegisterToRegistry registers the model to the registry.
func (c *GNNModelConfig) RegisterToRegistry(registry common.ModelRegistry) error {
	meta := &common.ModelMetadata{
		ModelID:      c.ModelID,
		ModelName:    "MolPatent-GNN",
		Version:      "v1.0.0",
		ArtifactPath: c.ModelPath,
		Architecture: "GNN",
		Tasks:        []string{"Embedding", "Similarity"},
	}
	return registry.Register(context.Background(), meta)
}

// ModelDescriptor returns the model descriptor.
func (c *GNNModelConfig) ModelDescriptor() common.ModelDescriptor {
	return common.ModelDescriptor{
		ModelID:   c.ModelID,
		ModelType: common.ModelTypeGNN,
	}
}
//Personal.AI order the ending
