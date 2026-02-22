package strategy_gpt

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/turtacn/KeyIP-Intelligence/internal/intelligence/common"
)

// ---------------------------------------------------------------------------
// Mock registry
// ---------------------------------------------------------------------------

type mockModelRegistry struct {
	registered []common.ModelDescriptor
	err        error
}

func (m *mockModelRegistry) Register(desc common.ModelDescriptor) error {
	if m.err != nil {
		return m.err
	}
	m.registered = append(m.registered, desc)
	return nil
}

func (m *mockModelRegistry) Unregister(modelID string) error { return nil }

func (m *mockModelRegistry) Get(modelID string) (*common.ModelDescriptor, error) {
	for _, d := range m.registered {
		if d.ModelID == modelID {
			return &d, nil
		}
	}
	return nil, fmt.Errorf("not found")
}

func (m *mockModelRegistry) List() ([]common.ModelDescriptor, error) {
	return m.registered, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func validConfig(t *testing.T) *StrategyGPTConfig {
	t.Helper()
	cfg := NewStrategyGPTConfig()
	cfg.RAGConfig.Enabled = false // avoid needing a real endpoint
	return cfg
}

func validConfigWithRAG(t *testing.T) *StrategyGPTConfig {
	t.Helper()
	cfg := NewStrategyGPTConfig()
	cfg.RAGConfig.Enabled = true
	cfg.RAGConfig.VectorStoreEndpoint = "localhost:19530"
	return cfg
}

func tempPromptFile(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "system_prompt.txt")
	if err := os.WriteFile(p, []byte("You are a patent strategy assistant."), 0644); err != nil {
		t.Fatalf("write temp prompt: %v", err)
	}
	return p
}

// ---------------------------------------------------------------------------
// StrategyGPTConfig defaults
// ---------------------------------------------------------------------------

func TestNewStrategyGPTConfig_Defaults(t *testing.T) {
	cfg := NewStrategyGPTConfig()

	if cfg.MaxContextLength != 32768 {
		t.Errorf("MaxContextLength = %d, want 32768", cfg.MaxContextLength)
	}
	if cfg.MaxOutputTokens != 4096 {
		t.Errorf("MaxOutputTokens = %d, want 4096", cfg.MaxOutputTokens)
	}
	if cfg.Temperature != 0.3 {
		t.Errorf("Temperature = %f, want 0.3", cfg.Temperature)
	}
	if cfg.TopP != 0.9 {
		t.Errorf("TopP = %f, want 0.9", cfg.TopP)
	}
	if cfg.FrequencyPenalty != 0.1 {
		t.Errorf("FrequencyPenalty = %f, want 0.1", cfg.FrequencyPenalty)
	}
	if cfg.PresencePenalty != 0.05 {
		t.Errorf("PresencePenalty = %f, want 0.05", cfg.PresencePenalty)
	}
	if !cfg.RAGConfig.Enabled {
		t.Error("RAGConfig.Enabled should be true by default")
	}
	if !cfg.StreamingEnabled {
		t.Error("StreamingEnabled should be true by default")
	}
	if cfg.TimeoutMs != 60000 {
		t.Errorf("TimeoutMs = %d, want 60000", cfg.TimeoutMs)
	}
	if cfg.BackendType != BackendVLLM {
		t.Errorf("BackendType = %s, want vllm", cfg.BackendType)
	}
}

func TestNewStrategyGPTConfig_FromConfig(t *testing.T) {
	cfg := NewStrategyGPTConfig()
	cfg.ModelID = "strategy-gpt-v2.1.0"
	cfg.Temperature = 0.7
	cfg.MaxContextLength = 65536
	cfg.BackendType = BackendOpenAI
	cfg.RAGConfig.Enabled = false

	if cfg.ModelID != "strategy-gpt-v2.1.0" {
		t.Errorf("ModelID = %s, want strategy-gpt-v2.1.0", cfg.ModelID)
	}
	if cfg.Temperature != 0.7 {
		t.Errorf("Temperature = %f, want 0.7", cfg.Temperature)
	}
	if cfg.MaxContextLength != 65536 {
		t.Errorf("MaxContextLength = %d, want 65536", cfg.MaxContextLength)
	}
	if cfg.BackendType != BackendOpenAI {
		t.Errorf("BackendType = %s, want openai", cfg.BackendType)
	}
}

// ---------------------------------------------------------------------------
// Validate — success
// ---------------------------------------------------------------------------

func TestStrategyGPTConfig_Validate_Success(t *testing.T) {
	cfg := validConfig(t)
	if err := cfg.Validate(); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestStrategyGPTConfig_Validate_SuccessWithRAG(t *testing.T) {
	cfg := validConfigWithRAG(t)
	if err := cfg.Validate(); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestStrategyGPTConfig_Validate_SuccessWithSystemPrompt(t *testing.T) {
	cfg := validConfig(t)
	cfg.SystemPromptPath = tempPromptFile(t)
	if err := cfg.Validate(); err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Validate — Temperature
// ---------------------------------------------------------------------------

func TestStrategyGPTConfig_Validate_InvalidTemperature_Negative(t *testing.T) {
	cfg := validConfig(t)
	cfg.Temperature = -0.1
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for negative temperature")
	}
}

func TestStrategyGPTConfig_Validate_InvalidTemperature_TooHigh(t *testing.T) {
	cfg := validConfig(t)
	cfg.Temperature = 2.5
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for temperature > 2.0")
	}
}

func TestStrategyGPTConfig_Validate_Temperature_BoundaryZero(t *testing.T) {
	cfg := validConfig(t)
	cfg.Temperature = 0
	if err := cfg.Validate(); err != nil {
		t.Fatalf("temperature=0 should be valid: %v", err)
	}
}

func TestStrategyGPTConfig_Validate_Temperature_BoundaryTwo(t *testing.T) {
	cfg := validConfig(t)
	cfg.Temperature = 2.0
	if err := cfg.Validate(); err != nil {
		t.Fatalf("temperature=2.0 should be valid: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Validate — TopP
// ---------------------------------------------------------------------------

func TestStrategyGPTConfig_Validate_InvalidTopP_Zero(t *testing.T) {
	cfg := validConfig(t)
	cfg.TopP = 0
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for top_p=0")
	}
}

func TestStrategyGPTConfig_Validate_InvalidTopP_GreaterThanOne(t *testing.T) {
	cfg := validConfig(t)
	cfg.TopP = 1.5
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for top_p > 1.0")
	}
}

func TestStrategyGPTConfig_Validate_TopP_BoundaryOne(t *testing.T) {
	cfg := validConfig(t)
	cfg.TopP = 1.0
	if err := cfg.Validate(); err != nil {
		t.Fatalf("top_p=1.0 should be valid: %v", err)
	}
}

func TestStrategyGPTConfig_Validate_InvalidTopP_Negative(t *testing.T) {
	cfg := validConfig(t)
	cfg.TopP = -0.5
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for negative top_p")
	}
}

// ---------------------------------------------------------------------------
// Validate — Context length
// ---------------------------------------------------------------------------

func TestStrategyGPTConfig_Validate_InvalidContextLength(t *testing.T) {
	cfg := validConfig(t)
	cfg.MaxContextLength = 0
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for max_context_length=0")
	}
}

func TestStrategyGPTConfig_Validate_ContextLengthTooLarge(t *testing.T) {
	cfg := validConfig(t)
	cfg.MaxContextLength = 200000
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for max_context_length > 131072")
	}
}

func TestStrategyGPTConfig_Validate_ContextLengthNegative(t *testing.T) {
	cfg := validConfig(t)
	cfg.MaxContextLength = -1
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for negative max_context_length")
	}
}

func TestStrategyGPTConfig_Validate_ContextLengthBoundary(t *testing.T) {
	cfg := validConfig(t)
	cfg.MaxContextLength = 131072
	cfg.MaxOutputTokens = 4096
	if err := cfg.Validate(); err != nil {
		t.Fatalf("max_context_length=131072 should be valid: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Validate — RAG
// ---------------------------------------------------------------------------

func TestStrategyGPTConfig_Validate_RAGEnabledNoEndpoint(t *testing.T) {
	cfg := NewStrategyGPTConfig()
	cfg.RAGConfig.Enabled = true
	cfg.RAGConfig.VectorStoreEndpoint = ""
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error when RAG enabled but no endpoint")
	}
}

func TestStrategyGPTConfig_Validate_RAGDisabledNoEndpoint(t *testing.T) {
	cfg := validConfig(t)
	cfg.RAGConfig.Enabled = false
	cfg.RAGConfig.VectorStoreEndpoint = ""
	if err := cfg.Validate(); err != nil {
		t.Fatalf("RAG disabled with no endpoint should be valid: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Validate — SystemPromptPath
// ---------------------------------------------------------------------------

func TestStrategyGPTConfig_Validate_SystemPromptNotFound(t *testing.T) {
	cfg := validConfig(t)
	cfg.SystemPromptPath = "/nonexistent/path/to/prompt.txt"
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for non-existent system_prompt_path")
	}
}

func TestStrategyGPTConfig_Validate_SystemPromptEmpty(t *testing.T) {
	cfg := validConfig(t)
	cfg.SystemPromptPath = ""
	if err := cfg.Validate(); err != nil {
		t.Fatalf("empty system_prompt_path should be valid: %v", err)
	}
}

// ---------------------------------------------------------------------------
// Validate — ModelID / ModelPath
// ---------------------------------------------------------------------------

func TestStrategyGPTConfig_Validate_EmptyModelID(t *testing.T) {
	cfg := validConfig(t)
	cfg.ModelID = ""
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for empty model_id")
	}
}

func TestStrategyGPTConfig_Validate_EmptyModelPath(t *testing.T) {
	cfg := validConfig(t)
	cfg.ModelPath = ""
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for empty model_path")
	}
}

// ---------------------------------------------------------------------------
// Validate — BackendType
// ---------------------------------------------------------------------------

func TestStrategyGPTConfig_Validate_InvalidBackendType(t *testing.T) {
	cfg := validConfig(t)
	cfg.BackendType = "unknown"
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for unsupported backend_type")
	}
}

func TestStrategyGPTConfig_Validate_AllBackendTypes(t *testing.T) {
	for _, bt := range []BackendType{BackendVLLM, BackendHTTP, BackendOpenAI} {
		cfg := validConfig(t)
		cfg.BackendType = bt
		if err := cfg.Validate(); err != nil {
			t.Errorf("backend_type %s should be valid: %v", bt, err)
		}
	}
}

// ---------------------------------------------------------------------------
// Validate — OutputTokens
// ---------------------------------------------------------------------------

func TestStrategyGPTConfig_Validate_OutputTokensExceedContext(t *testing.T) {
	cfg := validConfig(t)
	cfg.MaxOutputTokens = cfg.MaxContextLength + 1
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error when max_output_tokens > max_context_length")
	}
}

func TestStrategyGPTConfig_Validate_OutputTokensZero(t *testing.T) {
	cfg := validConfig(t)
	cfg.MaxOutputTokens = 0
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for max_output_tokens=0")
	}
}

// ---------------------------------------------------------------------------
// RetryConfig defaults & validation
// ---------------------------------------------------------------------------

func TestRetryConfig_Defaults(t *testing.T) {
	rc := DefaultRetryConfig()
	if rc.MaxRetries != 3 {
		t.Errorf("MaxRetries = %d, want 3", rc.MaxRetries)
	}
	if rc.InitialBackoffMs != 1000 {
		t.Errorf("InitialBackoffMs = %d, want 1000", rc.InitialBackoffMs)
	}
	if rc.BackoffMultiplier != 2.0 {
		t.Errorf("BackoffMultiplier = %f, want 2.0", rc.BackoffMultiplier)
	}
	if rc.MaxBackoffMs != 30000 {
		t.Errorf("MaxBackoffMs = %d, want 30000", rc.MaxBackoffMs)
	}
	if len(rc.RetryableErrors) == 0 {
		t.Error("expected non-empty RetryableErrors")
	}
}

func TestRetryConfig_Validate_NegativeRetries(t *testing.T) {
	rc := DefaultRetryConfig()
	rc.MaxRetries = -1
	if err := rc.Validate(); err == nil {
		t.Fatal("expected error for negative max_retries")
	}
}

func TestRetryConfig_Validate_ZeroBackoff(t *testing.T) {
	rc := DefaultRetryConfig()
	rc.InitialBackoffMs = 0
	if err := rc.Validate(); err == nil {
		t.Fatal("expected error for initial_backoff_ms=0")
	}
}

func TestRetryConfig_Validate_BackoffMultiplierLessThanOne(t *testing.T) {
	rc := DefaultRetryConfig()
	rc.BackoffMultiplier = 0.5
	if err := rc.Validate(); err == nil {
		t.Fatal("expected error for backoff_multiplier < 1.0")
	}
}

func TestRetryConfig_Validate_MaxLessThanInitial(t *testing.T) {
	rc := DefaultRetryConfig()
	rc.MaxBackoffMs = 500
	if err := rc.Validate(); err == nil {
		t.Fatal("expected error when max_backoff_ms < initial_backoff_ms")
	}
}

func TestRetryConfig_Validate_Success(t *testing.T) {
	rc := DefaultRetryConfig()
	if err := rc.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ---------------------------------------------------------------------------
// RAGConfig defaults & validation
// ---------------------------------------------------------------------------

func TestRAGConfig_Defaults(t *testing.T) {
	rc := DefaultRAGConfig()
	if rc.TopK != 10 {
		t.Errorf("TopK = %d, want 10", rc.TopK)
	}
	if rc.SimilarityThreshold != 0.70 {
		t.Errorf("SimilarityThreshold = %f, want 0.70", rc.SimilarityThreshold)
	}
	if rc.ChunkSize != 512 {
		t.Errorf("ChunkSize = %d, want 512", rc.ChunkSize)
	}
	if rc.ChunkOverlap != 64 {
		t.Errorf("ChunkOverlap = %d, want 64", rc.ChunkOverlap)
	}
	if !rc.RerankerEnabled {
		t.Error("RerankerEnabled should be true by default")
	}
	if rc.RerankerTopK != 5 {
		t.Errorf("RerankerTopK = %d, want 5", rc.RerankerTopK)
	}
}

func TestRAGConfig_Validate_InvalidTopK(t *testing.T) {
	rc := DefaultRAGConfig()
	rc.TopK = 0
	if err := rc.Validate(); err == nil {
		t.Fatal("expected error for top_k=0")
	}
}

func TestRAGConfig_Validate_InvalidThreshold(t *testing.T) {
	rc := DefaultRAGConfig()
	rc.SimilarityThreshold = 1.5
	if err := rc.Validate(); err == nil {
		t.Fatal("expected error for similarity_threshold > 1.0")
	}
}

func TestRAGConfig_Validate_NegativeThreshold(t *testing.T) {
	rc := DefaultRAGConfig()
	rc.SimilarityThreshold = -0.1
	if err := rc.Validate(); err == nil {
		t.Fatal("expected error for negative similarity_threshold")
	}
}

func TestRAGConfig_Validate_ChunkOverlapExceedsSize(t *testing.T) {
	rc := DefaultRAGConfig()
	rc.ChunkOverlap = 512
	rc.ChunkSize = 512
	if err := rc.Validate(); err == nil {
		t.Fatal("expected error when chunk_overlap >= chunk_size")
	}
}

func TestRAGConfig_Validate_ChunkOverlapGreaterThanSize(t *testing.T) {
	rc := DefaultRAGConfig()
	rc.ChunkOverlap = 600
	rc.ChunkSize = 512
	if err := rc.Validate(); err == nil {
		t.Fatal("expected error when chunk_overlap > chunk_size")
	}
}

func TestRAGConfig_Validate_RerankerTopKExceedsTopK(t *testing.T) {
	rc := DefaultRAGConfig()
	rc.RerankerEnabled = true
	rc.RerankerTopK = 15
	rc.TopK = 10
	if err := rc.Validate(); err == nil {
		t.Fatal("expected error when reranker_top_k > top_k")
	}
}

func TestRAGConfig_Validate_RerankerDisabledTopKIgnored(t *testing.T) {
	rc := DefaultRAGConfig()
	rc.RerankerEnabled = false
	rc.RerankerTopK = 100 // should be ignored
	if err := rc.Validate(); err != nil {
		t.Fatalf("reranker disabled should ignore reranker_top_k: %v", err)
	}
}

func TestRAGConfig_Validate_RerankerEnabledZeroTopK(t *testing.T) {
	rc := DefaultRAGConfig()
	rc.RerankerEnabled = true
	rc.RerankerTopK = 0
	if err := rc.Validate(); err == nil {
		t.Fatal("expected error for reranker_top_k=0 when reranker enabled")
	}
}

func TestRAGConfig_Validate_Success(t *testing.T) {
	rc := DefaultRAGConfig()
	if err := rc.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRAGConfig_Validate_NegativeChunkOverlap(t *testing.T) {
	rc := DefaultRAGConfig()
	rc.ChunkOverlap = -1
	if err := rc.Validate(); err == nil {
		t.Fatal("expected error for negative chunk_overlap")
	}
}

func TestRAGConfig_Validate_ZeroChunkSize(t *testing.T) {
	rc := DefaultRAGConfig()
	rc.ChunkSize = 0
	if err := rc.Validate(); err == nil {
		t.Fatal("expected error for chunk_size=0")
	}
}

// ---------------------------------------------------------------------------
// RegisterToRegistry
// ---------------------------------------------------------------------------

func TestStrategyGPTConfig_RegisterToRegistry_Success(t *testing.T) {
	cfg := validConfig(t)
	reg := &mockModelRegistry{}
	if err := cfg.RegisterToRegistry(reg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(reg.registered) != 1 {
		t.Fatalf("expected 1 registered model, got %d", len(reg.registered))
	}
	if reg.registered[0].ModelID != cfg.ModelID {
		t.Errorf("registered model_id = %s, want %s", reg.registered[0].ModelID, cfg.ModelID)
	}
}

func TestStrategyGPTConfig_RegisterToRegistry_Error(t *testing.T) {
	cfg := validConfig(t)
	reg := &mockModelRegistry{err: fmt.Errorf("registry full")}
	err := cfg.RegisterToRegistry(reg)
	if err == nil {
		t.Fatal("expected error from registry")
	}
	if err.Error() != "registry full" {
		t.Errorf("error = %v, want 'registry full'", err)
	}
}

func TestStrategyGPTConfig_RegisterToRegistry_NilRegistry(t *testing.T) {
	cfg := validConfig(t)
	err := cfg.RegisterToRegistry(nil)
	if err == nil {
		t.Fatal("expected error for nil registry")
	}
}

// ---------------------------------------------------------------------------
// ModelDescriptor
// ---------------------------------------------------------------------------

func TestStrategyGPTConfig_ModelDescriptor(t *testing.T) {
	cfg := validConfigWithRAG(t)
	desc := cfg.ModelDescriptor()

	if desc.ModelID != cfg.ModelID {
		t.Errorf("ModelID = %s, want %s", desc.ModelID, cfg.ModelID)
	}
	if desc.ModelType != common.ModelTypeLLM {
		t.Errorf("ModelType = %s, want %s", desc.ModelType, common.ModelTypeLLM)
	}
	if desc.BackendType != string(cfg.BackendType) {
		t.Errorf("BackendType = %s, want %s", desc.BackendType, cfg.BackendType)
	}
	if desc.Endpoint != cfg.ModelPath {
		t.Errorf("Endpoint = %s, want %s", desc.Endpoint, cfg.ModelPath)
	}

	// Check capabilities
	capSet := make(map[string]bool)
	for _, c := range desc.Capabilities {
		capSet[c] = true
	}
	requiredCaps := []string{"patent_strategy", "risk_assessment", "fto_analysis", "rag_augmented", "streaming"}
	for _, rc := range requiredCaps {
		if !capSet[rc] {
			t.Errorf("missing capability: %s", rc)
		}
	}

	// Check metadata
	if desc.Metadata["max_context_length"] != fmt.Sprintf("%d", cfg.MaxContextLength) {
		t.Errorf("metadata max_context_length mismatch")
	}
	if desc.Metadata["temperature"] != fmt.Sprintf("%.2f", cfg.Temperature) {
		t.Errorf("metadata temperature mismatch")
	}
	if desc.Metadata["rag_enabled"] != "true" {
		t.Errorf("metadata rag_enabled should be true")
	}
	if desc.Metadata["streaming_enabled"] != "true" {
		t.Errorf("metadata streaming_enabled should be true")
	}
}

func TestStrategyGPTConfig_ModelDescriptor_RAGDisabled(t *testing.T) {
	cfg := validConfig(t)
	cfg.RAGConfig.Enabled = false
	cfg.StreamingEnabled = false
	desc := cfg.ModelDescriptor()

	capSet := make(map[string]bool)
	for _, c := range desc.Capabilities {
		capSet[c] = true
	}
	if capSet["rag_augmented"] {
		t.Error("rag_augmented should not be present when RAG disabled")
	}
	if capSet["streaming"] {
		t.Error("streaming should not be present when streaming disabled")
	}
}

func TestStrategyGPTConfig_ModelDescriptor_AllBackends(t *testing.T) {
	for _, bt := range []BackendType{BackendVLLM, BackendHTTP, BackendOpenAI} {
		cfg := validConfig(t)
		cfg.BackendType = bt
		desc := cfg.ModelDescriptor()
		if desc.BackendType != string(bt) {
			t.Errorf("BackendType = %s, want %s", desc.BackendType, bt)
		}
	}
}

// ---------------------------------------------------------------------------
// Validate — timeout
// ---------------------------------------------------------------------------

func TestStrategyGPTConfig_Validate_TimeoutZero(t *testing.T) {
	cfg := validConfig(t)
	cfg.TimeoutMs = 0
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for timeout_ms=0")
	}
}

func TestStrategyGPTConfig_Validate_TimeoutNegative(t *testing.T) {
	cfg := validConfig(t)
	cfg.TimeoutMs = -100
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for negative timeout_ms")
	}
}

// ---------------------------------------------------------------------------
// Validate — frequency / presence penalty boundaries
// ---------------------------------------------------------------------------

func TestStrategyGPTConfig_Validate_FrequencyPenaltyBoundary(t *testing.T) {
	cfg := validConfig(t)
	cfg.FrequencyPenalty = -2.0
	if err := cfg.Validate(); err != nil {
		t.Fatalf("frequency_penalty=-2.0 should be valid: %v", err)
	}
	cfg.FrequencyPenalty = 2.0
	if err := cfg.Validate(); err != nil {
		t.Fatalf("frequency_penalty=2.0 should be valid: %v", err)
	}
	cfg.FrequencyPenalty = -2.1
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for frequency_penalty < -2.0")
	}
	cfg.FrequencyPenalty = 2.1
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for frequency_penalty > 2.0")
	}
}

func TestStrategyGPTConfig_Validate_PresencePenaltyBoundary(t *testing.T) {
	cfg := validConfig(t)
	cfg.PresencePenalty = -2.0
	if err := cfg.Validate(); err != nil {
		t.Fatalf("presence_penalty=-2.0 should be valid: %v", err)
	}
	cfg.PresencePenalty = 2.0
	if err := cfg.Validate(); err != nil {
		t.Fatalf("presence_penalty=2.0 should be valid: %v", err)
	}
	cfg.PresencePenalty = -2.1
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error for presence_penalty < -2.0")
	}
}

//Personal.AI order the ending

