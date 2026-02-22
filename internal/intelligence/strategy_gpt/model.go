package strategy_gpt

import (
	"fmt"
	"os"
	"strings"

	"github.com/turtacn/KeyIP-Intelligence/internal/intelligence/common"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

// ---------------------------------------------------------------------------
// Backend type enumeration
// ---------------------------------------------------------------------------

// BackendType enumerates the supported LLM inference backends.
type BackendType string

const (
	BackendVLLM   BackendType = "vllm"
	BackendHTTP   BackendType = "http"
	BackendOpenAI BackendType = "openai"
)

// ---------------------------------------------------------------------------
// RetryConfig
// ---------------------------------------------------------------------------

// RetryConfig controls exponential-backoff retry behaviour for LLM calls.
type RetryConfig struct {
	MaxRetries        int      `json:"max_retries" yaml:"max_retries"`
	InitialBackoffMs  int      `json:"initial_backoff_ms" yaml:"initial_backoff_ms"`
	MaxBackoffMs      int      `json:"max_backoff_ms" yaml:"max_backoff_ms"`
	BackoffMultiplier float64  `json:"backoff_multiplier" yaml:"backoff_multiplier"`
	RetryableErrors   []string `json:"retryable_errors,omitempty" yaml:"retryable_errors,omitempty"`
}

// DefaultRetryConfig returns production-grade retry defaults.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:        3,
		InitialBackoffMs:  1000,
		MaxBackoffMs:      30000,
		BackoffMultiplier: 2.0,
		RetryableErrors: []string{
			"503",
			"rate_limit",
			"timeout",
			"connection_reset",
		},
	}
}

// Validate checks RetryConfig for logical consistency.
func (r *RetryConfig) Validate() error {
	if r.MaxRetries < 0 {
		return errors.NewInvalidInputError("retry max_retries must be >= 0")
	}
	if r.InitialBackoffMs <= 0 {
		return errors.NewInvalidInputError("retry initial_backoff_ms must be > 0")
	}
	if r.MaxBackoffMs <= 0 {
		return errors.NewInvalidInputError("retry max_backoff_ms must be > 0")
	}
	if r.BackoffMultiplier < 1.0 {
		return errors.NewInvalidInputError("retry backoff_multiplier must be >= 1.0")
	}
	if r.MaxBackoffMs < r.InitialBackoffMs {
		return errors.NewInvalidInputError("retry max_backoff_ms must be >= initial_backoff_ms")
	}
	return nil
}

// ---------------------------------------------------------------------------
// RAGConfig
// ---------------------------------------------------------------------------

// RAGConfig controls Retrieval-Augmented Generation behaviour.
type RAGConfig struct {
	Enabled              bool    `json:"enabled" yaml:"enabled"`
	VectorStoreEndpoint  string  `json:"vector_store_endpoint" yaml:"vector_store_endpoint"`
	TopK                 int     `json:"top_k" yaml:"top_k"`
	SimilarityThreshold  float64 `json:"similarity_threshold" yaml:"similarity_threshold"`
	ChunkSize            int     `json:"chunk_size" yaml:"chunk_size"`
	ChunkOverlap         int     `json:"chunk_overlap" yaml:"chunk_overlap"`
	RerankerEnabled      bool    `json:"reranker_enabled" yaml:"reranker_enabled"`
	RerankerTopK         int     `json:"reranker_top_k" yaml:"reranker_top_k"`
}

// DefaultRAGConfig returns sensible RAG defaults for patent domain retrieval.
func DefaultRAGConfig() RAGConfig {
	return RAGConfig{
		Enabled:             true,
		VectorStoreEndpoint: "",
		TopK:                10,
		SimilarityThreshold: 0.70,
		ChunkSize:           512,
		ChunkOverlap:        64,
		RerankerEnabled:     true,
		RerankerTopK:        5,
	}
}

// Validate checks RAGConfig for logical consistency.
func (r *RAGConfig) Validate() error {
	if r.TopK <= 0 {
		return errors.NewInvalidInputError("rag top_k must be > 0")
	}
	if r.SimilarityThreshold < 0 || r.SimilarityThreshold > 1.0 {
		return errors.NewInvalidInputError("rag similarity_threshold must be in [0, 1.0]")
	}
	if r.ChunkSize <= 0 {
		return errors.NewInvalidInputError("rag chunk_size must be > 0")
	}
	if r.ChunkOverlap < 0 {
		return errors.NewInvalidInputError("rag chunk_overlap must be >= 0")
	}
	if r.ChunkOverlap >= r.ChunkSize {
		return errors.NewInvalidInputError("rag chunk_overlap must be < chunk_size")
	}
	if r.RerankerEnabled && r.RerankerTopK <= 0 {
		return errors.NewInvalidInputError("rag reranker_top_k must be > 0 when reranker is enabled")
	}
	if r.RerankerEnabled && r.RerankerTopK > r.TopK {
		return errors.NewInvalidInputError("rag reranker_top_k must be <= top_k")
	}
	return nil
}

// ---------------------------------------------------------------------------
// StrategyGPTConfig
// ---------------------------------------------------------------------------

// StrategyGPTConfig holds all configuration for the StrategyGPT LLM engine.
type StrategyGPTConfig struct {
	ModelID          string      `json:"model_id" yaml:"model_id"`
	ModelPath        string      `json:"model_path" yaml:"model_path"`
	BackendType      BackendType `json:"backend_type" yaml:"backend_type"`
	MaxContextLength int         `json:"max_context_length" yaml:"max_context_length"`
	MaxOutputTokens  int         `json:"max_output_tokens" yaml:"max_output_tokens"`
	Temperature      float64     `json:"temperature" yaml:"temperature"`
	TopP             float64     `json:"top_p" yaml:"top_p"`
	FrequencyPenalty float64     `json:"frequency_penalty" yaml:"frequency_penalty"`
	PresencePenalty  float64     `json:"presence_penalty" yaml:"presence_penalty"`
	SystemPromptPath string      `json:"system_prompt_path" yaml:"system_prompt_path"`
	RAGConfig        RAGConfig   `json:"rag_config" yaml:"rag_config"`
	TimeoutMs        int         `json:"timeout_ms" yaml:"timeout_ms"`
	StreamingEnabled bool        `json:"streaming_enabled" yaml:"streaming_enabled"`
	RetryConfig      RetryConfig `json:"retry_config" yaml:"retry_config"`
}

// NewStrategyGPTConfig returns a StrategyGPTConfig populated with production defaults.
//
// Temperature is set to 0.3 (not 0) to allow a degree of creative reasoning
// while keeping outputs professionally grounded. RAG is enabled by default
// because retrieval of patent literature, legal precedents and examination
// guidelines is the primary knowledge-augmentation mechanism for the model.
// Streaming is enabled because strategy reports may take 30-60 s to generate
// and incremental output dramatically improves perceived latency.
func NewStrategyGPTConfig() *StrategyGPTConfig {
	return &StrategyGPTConfig{
		ModelID:          "strategy-gpt-v1.0.0",
		ModelPath:        "http://localhost:8000/v1",
		BackendType:      BackendVLLM,
		MaxContextLength: 32768,
		MaxOutputTokens:  4096,
		Temperature:      0.3,
		TopP:             0.9,
		FrequencyPenalty: 0.1,
		PresencePenalty:  0.05,
		SystemPromptPath: "",
		RAGConfig:        DefaultRAGConfig(),
		TimeoutMs:        60000,
		StreamingEnabled: true,
		RetryConfig:      DefaultRetryConfig(),
	}
}

// Validate performs comprehensive validation of the configuration.
func (c *StrategyGPTConfig) Validate() error {
	if c.ModelID == "" {
		return errors.NewInvalidInputError("model_id is required")
	}
	if c.ModelPath == "" {
		return errors.NewInvalidInputError("model_path is required")
	}

	// Context length
	if c.MaxContextLength <= 0 {
		return errors.NewInvalidInputError("max_context_length must be > 0")
	}
	if c.MaxContextLength > 131072 {
		return errors.NewInvalidInputError("max_context_length must be <= 131072")
	}

	// Output tokens
	if c.MaxOutputTokens <= 0 {
		return errors.NewInvalidInputError("max_output_tokens must be > 0")
	}
	if c.MaxOutputTokens > c.MaxContextLength {
		return errors.NewInvalidInputError("max_output_tokens must be <= max_context_length")
	}

	// Sampling parameters
	if c.Temperature < 0 || c.Temperature > 2.0 {
		return errors.NewInvalidInputError("temperature must be in [0, 2.0]")
	}
	if c.TopP <= 0 || c.TopP > 1.0 {
		return errors.NewInvalidInputError("top_p must be in (0, 1.0]")
	}
	if c.FrequencyPenalty < -2.0 || c.FrequencyPenalty > 2.0 {
		return errors.NewInvalidInputError("frequency_penalty must be in [-2.0, 2.0]")
	}
	if c.PresencePenalty < -2.0 || c.PresencePenalty > 2.0 {
		return errors.NewInvalidInputError("presence_penalty must be in [-2.0, 2.0]")
	}

	// System prompt file existence (only when path is specified)
	if c.SystemPromptPath != "" {
		if _, err := os.Stat(c.SystemPromptPath); os.IsNotExist(err) {
			return errors.NewInvalidInputError(fmt.Sprintf("system_prompt_path does not exist: %s", c.SystemPromptPath))
		}
	}

	// RAG validation (endpoint required only when enabled)
	if c.RAGConfig.Enabled {
		if strings.TrimSpace(c.RAGConfig.VectorStoreEndpoint) == "" {
			return errors.NewInvalidInputError("rag vector_store_endpoint is required when RAG is enabled")
		}
	}
	if err := c.RAGConfig.Validate(); err != nil {
		return fmt.Errorf("rag_config: %w", err)
	}

	// Timeout
	if c.TimeoutMs <= 0 {
		return errors.NewInvalidInputError("timeout_ms must be > 0")
	}

	// Retry
	if err := c.RetryConfig.Validate(); err != nil {
		return fmt.Errorf("retry_config: %w", err)
	}

	// Backend type
	switch c.BackendType {
	case BackendVLLM, BackendHTTP, BackendOpenAI:
		// ok
	default:
		return errors.NewInvalidInputError(fmt.Sprintf("unsupported backend_type: %s", c.BackendType))
	}

	return nil
}

// ---------------------------------------------------------------------------
// Model descriptor & registry integration
// ---------------------------------------------------------------------------

// ModelDescriptor returns a common.ModelDescriptor for registry integration.
func (c *StrategyGPTConfig) ModelDescriptor() common.ModelDescriptor {
	caps := []string{
		"patent_strategy",
		"risk_assessment",
		"fto_analysis",
		"patent_landscape",
		"claim_drafting",
		"prior_art_analysis",
	}
	if c.RAGConfig.Enabled {
		caps = append(caps, "rag_augmented")
	}
	if c.StreamingEnabled {
		caps = append(caps, "streaming")
	}

	return common.ModelDescriptor{
		ModelID:      c.ModelID,
		ModelType:    common.ModelTypeLLM,
		BackendType:  string(c.BackendType),
		Endpoint:     c.ModelPath,
		Capabilities: caps,
		Metadata: map[string]string{
			"max_context_length": fmt.Sprintf("%d", c.MaxContextLength),
			"max_output_tokens":  fmt.Sprintf("%d", c.MaxOutputTokens),
			"temperature":        fmt.Sprintf("%.2f", c.Temperature),
			"top_p":              fmt.Sprintf("%.2f", c.TopP),
			"rag_enabled":        fmt.Sprintf("%t", c.RAGConfig.Enabled),
			"streaming_enabled":  fmt.Sprintf("%t", c.StreamingEnabled),
		},
	}
}

// RegisterToRegistry registers this model configuration with the given registry.
func (c *StrategyGPTConfig) RegisterToRegistry(registry common.ModelRegistry) error {
	if registry == nil {
		return errors.NewInvalidInputError("registry is required")
	}
	desc := c.ModelDescriptor()
	return registry.Register(desc)
}

//Personal.AI order the ending
