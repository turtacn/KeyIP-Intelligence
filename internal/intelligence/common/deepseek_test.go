package common

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

func TestDeepSeekIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}
	be := NewOpenAIBackend(&OpenAIConfig{
		BaseURL:   "https://api.deepseek.com/v1",
		APIKey:    "sk-8e699ba18d994650986ea4329db12987",
		ModelName: "deepseek-chat",
		Timeout:   60 * time.Second,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	err := be.Healthy(ctx)
	if err != nil {
		t.Logf("Health check: %v (non-fatal, API may not support /models endpoint)", err)
	} else {
		t.Log("✅ Health check passed")
	}

	input, _ := json.Marshal(map[string]interface{}{
		"messages": []map[string]string{
			{"role": "user", "content": "请用一句话介绍OLED有机发光二极管。"},
		},
		"max_tokens":    200,
		"temperature":   0.7,
	})
	resp, err := be.Predict(ctx, &PredictRequest{
		ModelName: "deepseek-chat",
		InputData: input,
	})
	if err != nil {
		t.Fatalf("Predict FAILED: %v", err)
	}
	if content, ok := resp.Outputs["content"]; ok {
		fmt.Printf("✅ DeepSeek Response: %s\n", string(content))
		if len(content) == 0 {
			t.Error("empty response content")
		}
	} else {
		t.Error("no 'content' in outputs")
	}
	t.Logf("Tokens: %v", resp.Metadata)
}
