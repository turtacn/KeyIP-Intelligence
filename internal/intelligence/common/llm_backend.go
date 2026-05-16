package common

import (
	"fmt"
	"os"
	"strings"

	"github.com/turtacn/KeyIP-Intelligence/internal/config"
)

// NewLLMBackend creates a ModelBackend from config.
// Reads llm.primary (required) and llm.fallback (optional) from config.
// Supports providers: "anthropic", "openai", "deepseek"
func NewLLMBackend(cfg *config.Config) (ModelBackend, error) {
	if cfg == nil {
		cfg = config.Get()
	}
	providerCfg := cfg.LLM.Primary
	if providerCfg.Provider == "" {
		providerCfg.Provider = "anthropic"
	}
	apiKey := providerCfg.ResolvedAPIKey()
	if apiKey == "" {
		// Fallback to env vars by common convention
		switch providerCfg.Provider {
		case "anthropic":
			apiKey = os.Getenv("ANTHROPIC_API_KEY")
		case "openai":
			apiKey = os.Getenv("OPENAI_API_KEY")
		case "deepseek":
			apiKey = os.Getenv("DEEPSEEK_API_KEY")
		}
	}
	endpoint := providerCfg.Endpoint
	if endpoint == "" {
		switch providerCfg.Provider {
		case "anthropic":
			endpoint = "https://api.anthropic.com/v1"
		case "openai":
			endpoint = "https://api.openai.com/v1"
		case "deepseek":
			endpoint = "https://api.deepseek.com/v1"
		}
	}
	modelName := providerCfg.ModelName
	if modelName == "" {
		switch providerCfg.Provider {
		case "anthropic":
			modelName = "claude-sonnet-4-20250514"
		case "deepseek":
			modelName = "deepseek-chat"
		}
	}
	maxTokens := providerCfg.MaxTokens
	if maxTokens <= 0 {
		maxTokens = 8192
	}
	temperature := providerCfg.Temperature
	if temperature == 0 {
		temperature = 0.7
	}
	timeoutSec := providerCfg.TimeoutSec
	if timeoutSec <= 0 {
		timeoutSec = 120
	}

	fmt.Printf("[LLM] Initializing %s backend (%s) -> %s\n", providerCfg.Provider, modelName, endpoint)

	switch providerCfg.Provider {
	case "anthropic":
		return NewAnthropicBackend(AnthropicConfig{
			BaseURL:     endpoint,
			APIKey:      apiKey,
			ModelName:   modelName,
			MaxTokens:   maxTokens,
			Temperature: temperature,
			TimeoutSec:  timeoutSec,
		}), nil
	default:
		// OpenAI-compatible (OpenAI, DeepSeek, etc.)
		return NewOpenAIBackend(&OpenAIConfig{
			BaseURL:   strings.TrimRight(endpoint, "/"),
			APIKey:    apiKey,
			ModelName: modelName,
		}), nil
	}
}
