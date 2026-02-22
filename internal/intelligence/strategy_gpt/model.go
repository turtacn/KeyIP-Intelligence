package strategy_gpt

import (
	"context"
	"errors"

	"github.com/turtacn/KeyIP-Intelligence/internal/intelligence/common"
)

// BackendType defines the type of inference backend.
type BackendType string

const (
	BackendVLLM   BackendType = "vllm"
	BackendHTTP   BackendType = "http"
	BackendOpenAI BackendType = "openai"
)

// RAGConfig holds configuration for RAG.
type RAGConfig struct {
	Enabled             bool    `json:"enabled"`
	VectorStoreEndpoint string  `json:"vector_store_endpoint"`
	TopK                int     `json:"top_k"`
	SimilarityThreshold float64 `json:"similarity_threshold"`
	ChunkSize           int     `json:"chunk_size"`
	ChunkOverlap        int     `json:"chunk_overlap"`
	RerankerEnabled     bool    `json:"reranker_enabled"`
	RerankerTopK        int     `json:"reranker_top_k"`
}

// RetryConfig holds retry settings.
type RetryConfig struct {
	MaxRetries       int           `json:"max_retries"`
	InitialBackoffMs int           `json:"initial_backoff_ms"`
	MaxBackoffMs     int           `json:"max_backoff_ms"`
	BackoffMultiplier float64      `json:"backoff_multiplier"`
	RetryableErrors  []string      `json:"retryable_errors"`
}

// StrategyGPTConfig holds configuration for StrategyGPT.
type StrategyGPTConfig struct {
	ModelID          string       `json:"model_id"`
	ModelPath        string       `json:"model_path"`
	BackendType      BackendType  `json:"backend_type"`
	MaxContextLength int          `json:"max_context_length"`
	MaxOutputTokens  int          `json:"max_output_tokens"`
	Temperature      float64      `json:"temperature"`
	TopP             float64      `json:"top_p"`
	FrequencyPenalty float64      `json:"frequency_penalty"`
	PresencePenalty  float64      `json:"presence_penalty"`
	SystemPromptPath string       `json:"system_prompt_path"`
	RAGConfig        RAGConfig    `json:"rag_config"`
	TimeoutMs        int          `json:"timeout_ms"`
	StreamingEnabled bool         `json:"streaming_enabled"`
	RetryConfig      RetryConfig  `json:"retry_config"`
}

// NewStrategyGPTConfig creates a new configuration with defaults.
func NewStrategyGPTConfig() *StrategyGPTConfig {
	return &StrategyGPTConfig{
		MaxContextLength: 32768,
		MaxOutputTokens:  4096,
		Temperature:      0.3,
		TopP:             0.9,
		FrequencyPenalty: 0.1,
		PresencePenalty:  0.05,
		TimeoutMs:        60000,
		StreamingEnabled: true,
		RAGConfig: RAGConfig{
			Enabled:             true,
			TopK:                10,
			SimilarityThreshold: 0.70,
			ChunkSize:           512,
			ChunkOverlap:        64,
			RerankerEnabled:     true,
			RerankerTopK:        5,
		},
		RetryConfig: RetryConfig{
			MaxRetries:        3,
			InitialBackoffMs:  1000,
			MaxBackoffMs:      30000,
			BackoffMultiplier: 2.0,
		},
	}
}

// Validate checks if the configuration is valid.
func (c *StrategyGPTConfig) Validate() error {
	if c.MaxContextLength <= 0 || c.MaxContextLength > 131072 {
		return errors.New("invalid max context length")
	}
	if c.Temperature < 0 || c.Temperature > 2.0 {
		return errors.New("temperature must be between 0 and 2.0")
	}
	if c.TopP <= 0 || c.TopP > 1.0 {
		return errors.New("top_p must be between 0 and 1.0")
	}
	if c.RAGConfig.Enabled && c.RAGConfig.VectorStoreEndpoint == "" {
		// Just a placeholder validation, could be empty if mocked
		// return errors.New("vector store endpoint required if RAG enabled")
	}
	return nil
}

// RegisterToRegistry registers the model to the registry.
func (c *StrategyGPTConfig) RegisterToRegistry(registry common.ModelRegistry) error {
	meta := &common.ModelMetadata{
		ModelID:      c.ModelID,
		ModelName:    "StrategyGPT",
		Version:      "v1.0.0",
		ArtifactPath: c.ModelPath,
		Architecture: "GPT-4o-mini", // example
		Tasks:        []string{"StrategyAnalysis", "Drafting"},
	}
	return registry.Register(context.Background(), meta)
}
//Personal.AI order the ending
