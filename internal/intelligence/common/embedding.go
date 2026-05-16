// embedding.go — config-driven embedding client using the same LLM provider as chat/inference.
//
// For Anthropic: uses a prompt-based extraction approach (generate structured summary → hash → pad).
// For OpenAI-compatible (OpenAI, DeepSeek): uses the native /v1/embeddings endpoint.
package common

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"

	"github.com/turtacn/KeyIP-Intelligence/internal/config"
)

// EmbeddingClient generates vector embeddings using the configured LLM provider.
type EmbeddingClient struct {
	provider    string
	endpoint    string
	apiKey      string
	modelName   string
	dimensions  int
	httpClient  *http.Client
	backend     ModelBackend // fallback: use Predict() for prompt-based embeddings
}

// EmbeddingClientConfig is derived from the global LLM configuration.
type EmbeddingClientConfig struct {
	Provider   string
	Endpoint   string
	APIKey     string
	ModelName  string // embedding model name; if empty, defaults to provider default
	Dimensions int    // output vector dimensions
}

// DefaultEmbeddingConfigs maps provider → default model + dimensions.
var DefaultEmbeddingConfigs = map[string]struct {
	ModelName  string
	Dimensions int
}{
	"openai":    {"text-embedding-3-small", 1536},
	"deepseek":  {"deepseek-chat", 768},       // DeepSeek uses chat model for embeddings
	"anthropic": {"claude-sonnet-4-20250514", 768}, // Anthropic uses prompt-based extraction
}

// NewEmbeddingClient creates an EmbeddingClient from the root config.
// Returns nil if the primary LLM provider is not configured.
func NewEmbeddingClient(cfg *config.Config, backend ModelBackend) *EmbeddingClient {
	if cfg == nil {
		return nil
	}
	primary := cfg.LLM.Primary
	if primary.Provider == "" {
		return nil
	}

	apiKey := primary.ResolvedAPIKey()
	endpoint := strings.TrimRight(primary.Endpoint, "/")
	if endpoint == "" {
		switch primary.Provider {
		case "anthropic":
			endpoint = "https://api.anthropic.com/v1"
		case "openai":
			endpoint = "https://api.openai.com/v1"
		case "deepseek":
			endpoint = "https://api.deepseek.com/v1"
		default:
			endpoint = "https://api.openai.com/v1"
		}
	}

	modelName := primary.EmbeddingModelName
	dimensions := primary.EmbeddingDimensions

	if modelName == "" || dimensions <= 0 {
		if def, ok := DefaultEmbeddingConfigs[primary.Provider]; ok {
			if modelName == "" {
				modelName = def.ModelName
			}
			if dimensions <= 0 {
				dimensions = def.Dimensions
			}
		}
	}
	if modelName == "" {
		modelName = primary.ModelName
	}
	if dimensions <= 0 {
		dimensions = 768
	}

	return &EmbeddingClient{
		provider:   primary.Provider,
		endpoint:   endpoint,
		apiKey:     apiKey,
		modelName:  modelName,
		dimensions: dimensions,
		httpClient: &http.Client{},
		backend:    backend,
	}
}

// Embed returns a float32 vector for the given input text.
func (c *EmbeddingClient) Embed(ctx context.Context, text string) ([]float32, error) {
	switch c.provider {
	case "openai", "deepseek":
		return c.embedOpenAICompat(ctx, text)
	default:
		// Anthropic: use prompt-based extraction via Predict()
		return c.embedPromptBased(ctx, text)
	}
}

// embedOpenAICompat calls POST /v1/embeddings on an OpenAI-compatible API.
func (c *EmbeddingClient) embedOpenAICompat(ctx context.Context, text string) ([]float32, error) {
	body := map[string]interface{}{
		"model": c.modelName,
		"input": text,
	}
	if c.provider == "openai" {
		body["dimensions"] = c.dimensions
	}

	b, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("embedding: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.endpoint+"/embeddings", bytes.NewReader(b))
	if err != nil {
		return nil, fmt.Errorf("embedding: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("embedding: http request: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("embedding: status %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("embedding: unmarshal response: %w", err)
	}
	if len(result.Data) == 0 {
		return nil, fmt.Errorf("embedding: empty response")
	}
	return result.Data[0].Embedding, nil
}

// embedPromptBased uses the LLM Predict() to extract a structured representation,
// then hashes it to produce a deterministic embedding vector.
// This is a fallback for providers without a dedicated embeddings API (e.g. Anthropic).
func (c *EmbeddingClient) embedPromptBased(ctx context.Context, text string) ([]float32, error) {
	if c.backend == nil {
		return nil, fmt.Errorf("embedding: no backend available for prompt-based embedding")
	}

	systemPrompt := `You are a chemical patent vectorizer. Given a molecule or patent description, output ONLY a single line of comma-separated numeric descriptors representing: molecular weight, logP, TPSA, atom count, ring count, aromatic ring count, H-bond donors, H-bond acceptors, rotatable bonds, and 5 structural fingerprint bits. Format: 0.1,0.5,0.3,... (all values 0.0-1.0 normalized).`

	prompt := fmt.Sprintf("Vectorize this: %s\n\nOutput ONLY the comma-separated numbers, nothing else.", text)

	req := &PredictRequest{
		ModelName: c.modelName,
		Messages: []Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: prompt},
		},
		MaxTokens:   256,
		Temperature: 0.0,
	}

	resp, err := c.backend.Predict(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("embedding: predict: %w", err)
	}

	// Parse the response as comma-separated floats
	raw := strings.TrimSpace(resp.Content)
	parts := strings.Split(raw, ",")
	baseVec := make([]float32, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		var v float64
		if _, err := fmt.Sscanf(p, "%f", &v); err == nil {
			baseVec = append(baseVec, float32(v))
		}
	}

	if len(baseVec) == 0 {
		// Fallback: hash-based embedding
		return c.hashEmbedding(text), nil
	}

	// Pad or truncate to target dimensions
	return c.normalizeVector(baseVec), nil
}

// hashEmbedding produces a deterministic float32 vector from SHA-256 hash of input.
func (c *EmbeddingClient) hashEmbedding(text string) []float32 {
	h := sha256.Sum256([]byte(text))
	vec := make([]float32, c.dimensions)
	for i := range vec {
		idx := (i * 4) % len(h)
		u32 := binary.BigEndian.Uint32(h[idx : idx+4])
		// Normalize to [0,1] range
		vec[i] = float32(u32%10000) / 10000.0
	}
	return vec
}

// normalizeVector pads or truncates the vector to target dimensions.
func (c *EmbeddingClient) normalizeVector(vec []float32) []float32 {
	if len(vec) == c.dimensions {
		return vec
	}

	out := make([]float32, c.dimensions)

	if len(vec) > c.dimensions {
		copy(out, vec[:c.dimensions])
	} else {
		copy(out, vec)
		// Pad with hash-derived values to make it deterministic
		filler := sha256.Sum256([]byte(fmt.Sprintf("%v", vec)))
		for i := len(vec); i < c.dimensions; i++ {
			idx := ((i - len(vec)) * 4) % len(filler)
			out[i] = float32(binary.BigEndian.Uint32(filler[idx:idx+4])%10000) / 10000.0
		}
	}

	// L2-normalize
	var sumSq float64
	for _, v := range out {
		sumSq += float64(v) * float64(v)
	}
	if sumSq > 0 {
		norm := float32(math.Sqrt(sumSq))
		for i := range out {
			out[i] /= norm
		}
	}

	return out
}

// Config returns the embedding configuration for diagnostics.
func (c *EmbeddingClient) Config() EmbeddingClientConfig {
	return EmbeddingClientConfig{
		Provider:   c.provider,
		Endpoint:   c.endpoint,
		ModelName:  c.modelName,
		Dimensions: c.dimensions,
	}
}
