package common

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// OpenAIBackend is a real ModelBackend that calls OpenAI-compatible chat completions APIs.
// Supports DeepSeek, OpenAI, and any compatible endpoint.
type OpenAIBackend struct {
	client    *http.Client
	baseURL   string
	apiKey    string
	modelName string
}

// OpenAIConfig configures the OpenAI-compatible backend.
type OpenAIConfig struct {
	BaseURL   string // e.g. "https://api.deepseek.com/v1"
	APIKey    string
	ModelName string // e.g. "deepseek-chat" or "deepseek-v4-pro"
	Timeout   time.Duration
}

// chatRequest is the OpenAI chat completions request body.
type chatRequest struct {
	Model       string          `json:"model"`
	Messages    []chatMessage   `json:"messages"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Temperature float64         `json:"temperature,omitempty"`
	Stream      bool            `json:"stream"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	ID      string         `json:"id"`
	Choices []chatChoice   `json:"choices"`
	Usage   *chatUsage     `json:"usage,omitempty"`
	Error   *chatAPIError  `json:"error,omitempty"`
}

type chatChoice struct {
	Index   int         `json:"index"`
	Message chatMessage `json:"message"`
}

type chatUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type chatAPIError struct {
	Message string `json:"message"`
	Type    string `json:"type"`
}

// NewOpenAIBackend creates a new OpenAI-compatible backend.
// If cfg is nil, reads from environment:
//   OPENAI_BASE_URL or DEEPSEEK_BASE_URL (default: https://api.deepseek.com/v1)
//   OPENAI_API_KEY or DEEPSEEK_API_KEY
//   OPENAI_MODEL or DEEPSEEK_MODEL (default: deepseek-chat)
func NewOpenAIBackend(cfg *OpenAIConfig) *OpenAIBackend {
	if cfg == nil {
		cfg = &OpenAIConfig{
			BaseURL:   os.Getenv("OPENAI_BASE_URL"),
			APIKey:    os.Getenv("OPENAI_API_KEY"),
			ModelName: os.Getenv("OPENAI_MODEL"),
		}
		if cfg.BaseURL == "" {
			cfg.BaseURL = os.Getenv("DEEPSEEK_BASE_URL")
		}
		if cfg.BaseURL == "" {
			cfg.BaseURL = "https://api.deepseek.com/v1"
		}
		if cfg.APIKey == "" {
			cfg.APIKey = os.Getenv("DEEPSEEK_API_KEY")
		}
		if cfg.ModelName == "" {
			cfg.ModelName = os.Getenv("DEEPSEEK_MODEL")
		}
		if cfg.ModelName == "" {
			cfg.ModelName = "deepseek-chat"
		}
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 120 * time.Second
	}
	return &OpenAIBackend{
		client:    &http.Client{Timeout: cfg.Timeout},
		baseURL:   strings.TrimRight(cfg.BaseURL, "/"),
		apiKey:    cfg.APIKey,
		modelName: cfg.ModelName,
	}
}

// Predict sends a chat completion request. inputData is expected to be JSON with
// "messages" (required), optional "max_tokens" and "temperature".
func (b *OpenAIBackend) Predict(ctx context.Context, req *PredictRequest) (*PredictResponse, error) {
	var input struct {
		Messages    []chatMessage `json:"messages"`
		MaxTokens   int           `json:"max_tokens,omitempty"`
		Temperature float64       `json:"temperature,omitempty"`
	}
	if err := json.Unmarshal(req.InputData, &input); err != nil {
		return nil, fmt.Errorf("failed to parse input: %w", err)
	}
	if len(input.Messages) == 0 {
		return nil, fmt.Errorf("messages are required")
	}
	if input.Temperature == 0 {
		input.Temperature = 0.7
	}
	if input.MaxTokens <= 0 {
		input.MaxTokens = 4096
	}

	chatReq := chatRequest{
		Model:       b.modelName,
		Messages:    input.Messages,
		MaxTokens:   input.MaxTokens,
		Temperature: input.Temperature,
		Stream:      false,
	}
	body, _ := json.Marshal(chatReq)

	httpReq, err := http.NewRequestWithContext(ctx, "POST", b.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+b.apiKey)

	resp, err := b.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	var chatResp chatResponse
	if err := json.Unmarshal(respBody, &chatResp); err != nil {
		return nil, fmt.Errorf("failed to parse response (HTTP %d): %w", resp.StatusCode, err)
	}

	if chatResp.Error != nil {
		return nil, fmt.Errorf("API error: %s (%s)", chatResp.Error.Message, chatResp.Error.Type)
	}

	if len(chatResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	content := chatResp.Choices[0].Message.Content
	return &PredictResponse{
		ModelName: chatReq.Model,
		Outputs:   map[string][]byte{"content": []byte(content)},
		Metadata: map[string]string{
			"prompt_tokens":     fmt.Sprintf("%d", chatResp.Usage.PromptTokens),
			"completion_tokens": fmt.Sprintf("%d", chatResp.Usage.CompletionTokens),
		},
	}, nil
}

// PredictStream is not supported for OpenAI backend yet.
func (b *OpenAIBackend) PredictStream(ctx context.Context, req *PredictRequest) (<-chan *PredictResponse, error) {
	return nil, fmt.Errorf("streaming not implemented for OpenAI backend")
}

// Healthy checks if the API is reachable.
func (b *OpenAIBackend) Healthy(ctx context.Context) error {
	httpReq, _ := http.NewRequestWithContext(ctx, "GET", b.baseURL+"/models", nil)
	httpReq.Header.Set("Authorization", "Bearer "+b.apiKey)
	resp, err := b.client.Do(httpReq)
	if err != nil {
		return err
	}
	resp.Body.Close()
	if resp.StatusCode >= 400 {
		return fmt.Errorf("API unhealthy: HTTP %d", resp.StatusCode)
	}
	return nil
}

// Close releases resources.
func (b *OpenAIBackend) Close() error {
	b.client.CloseIdleConnections()
	return nil
}
