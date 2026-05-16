package common

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// AnthropicBackend calls the Anthropic Messages API.
type AnthropicBackend struct {
	client      *http.Client
	baseURL     string
	apiKey      string
	modelName   string
	maxTokens   int
	temperature float64
	apiVersion  string
}

// AnthropicConfig configures the Anthropic backend.
type AnthropicConfig struct {
	BaseURL     string
	APIKey      string
	ModelName   string
	MaxTokens   int
	Temperature float64
	TimeoutSec  int
}

type anthropicRequest struct {
	Model       string             `json:"model"`
	Messages    []anthropicMessage `json:"messages"`
	System      string             `json:"system,omitempty"`
	MaxTokens   int                `json:"max_tokens"`
	Temperature float64            `json:"temperature,omitempty"`
	TopP        float64            `json:"top_p,omitempty"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicResponse struct {
	ID      string             `json:"id"`
	Type    string             `json:"type"`
	Role    string             `json:"role"`
	Content []anthropicContent `json:"content"`
	Usage   anthropicUsage     `json:"usage"`
	Error   *anthropicAPIError `json:"error,omitempty"`
}

type anthropicContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type anthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

type anthropicAPIError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

// NewAnthropicBackend creates a new Anthropic backend.
func NewAnthropicBackend(cfg AnthropicConfig) *AnthropicBackend {
	if cfg.MaxTokens <= 0 {
		cfg.MaxTokens = 8192
	}
	if cfg.Temperature == 0 {
		cfg.Temperature = 0.7
	}
	if cfg.TimeoutSec <= 0 {
		cfg.TimeoutSec = 120
	}
	return &AnthropicBackend{
		client:      &http.Client{Timeout: time.Duration(cfg.TimeoutSec) * time.Second},
		baseURL:     cfg.BaseURL,
		apiKey:      cfg.APIKey,
		modelName:   cfg.ModelName,
		maxTokens:   cfg.MaxTokens,
		temperature: cfg.Temperature,
		apiVersion:  "2023-06-01",
	}
}

// Predict sends a message to Anthropic.
func (b *AnthropicBackend) Predict(ctx context.Context, req *PredictRequest) (*PredictResponse, error) {
	var input struct {
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
		System      string  `json:"system,omitempty"`
		MaxTokens   int     `json:"max_tokens,omitempty"`
		Temperature float64 `json:"temperature,omitempty"`
	}
	if err := json.Unmarshal(req.InputData, &input); err != nil {
		return nil, fmt.Errorf("anthropic: parse input: %w", err)
	}
	if len(input.Messages) == 0 {
		return nil, fmt.Errorf("anthropic: messages required")
	}

	anthropicMessages := make([]anthropicMessage, len(input.Messages))
	for i, m := range input.Messages {
		anthropicMessages[i] = anthropicMessage{Role: m.Role, Content: m.Content}
	}

	maxTokens := b.maxTokens
	if input.MaxTokens > 0 {
		maxTokens = input.MaxTokens
	}
	temp := b.temperature
	if input.Temperature > 0 {
		temp = input.Temperature
	}

	ar := anthropicRequest{
		Model:       b.modelName,
		Messages:    anthropicMessages,
		System:      input.System,
		MaxTokens:   maxTokens,
		Temperature: temp,
	}
	body, _ := json.Marshal(ar)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", b.baseURL+"/messages", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", b.apiKey)
	httpReq.Header.Set("anthropic-version", b.apiVersion)

	resp, err := b.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("anthropic: request: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	var arResp anthropicResponse
	if err := json.Unmarshal(respBody, &arResp); err != nil {
		return nil, fmt.Errorf("anthropic: parse response (HTTP %d): %w", resp.StatusCode, err)
	}

	if arResp.Error != nil {
		return nil, fmt.Errorf("anthropic: %s: %s", arResp.Error.Type, arResp.Error.Message)
	}

	if len(arResp.Content) == 0 || arResp.Content[0].Text == "" {
		return nil, fmt.Errorf("anthropic: empty response content")
	}

	return &PredictResponse{
		ModelName: b.modelName,
		Outputs:   map[string][]byte{"content": []byte(arResp.Content[0].Text)},
		Metadata: map[string]string{
			"input_tokens":  fmt.Sprintf("%d", arResp.Usage.InputTokens),
			"output_tokens": fmt.Sprintf("%d", arResp.Usage.OutputTokens),
		},
	}, nil
}

// PredictStream is not implemented.
func (b *AnthropicBackend) PredictStream(ctx context.Context, req *PredictRequest) (<-chan *PredictResponse, error) {
	return nil, fmt.Errorf("anthropic: streaming not implemented")
}

// Healthy checks API reachability.
func (b *AnthropicBackend) Healthy(ctx context.Context) error {
	httpReq, _ := http.NewRequestWithContext(ctx, "GET", b.baseURL+"/models", nil)
	httpReq.Header.Set("x-api-key", b.apiKey)
	httpReq.Header.Set("anthropic-version", b.apiVersion)
	resp, err := b.client.Do(httpReq)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("anthropic unhealthy: HTTP %d", resp.StatusCode)
	}
	return nil
}

// Close releases resources.
func (b *AnthropicBackend) Close() error {
	b.client.CloseIdleConnections()
	return nil
}
