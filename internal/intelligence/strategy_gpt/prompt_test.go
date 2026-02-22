package strategy_gpt

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
)

type PromptMockLogger struct {
	logging.Logger
}

func (m *PromptMockLogger) Info(msg string, fields ...logging.Field) {}

func TestNewPromptManager(t *testing.T) {
	config := NewStrategyGPTConfig()
	pm, err := NewPromptManager(config, &PromptMockLogger{})
	assert.NoError(t, err)
	assert.NotNil(t, pm)
}

func TestBuildPrompt_FTO(t *testing.T) {
	config := NewStrategyGPTConfig()
	pm, _ := NewPromptManager(config, &PromptMockLogger{})

	params := &PromptParams{
		Task: TaskFTO,
		TargetMolecule: &MoleculeContext{
			SMILES: "C",
			Name:   "Methane",
		},
		UserQuery: "Is this safe?",
	}

	prompt, err := pm.BuildPrompt(context.Background(), TaskFTO, params)
	assert.NoError(t, err)
	assert.Contains(t, prompt.SystemPrompt, "Freedom to Operate")
	assert.Contains(t, prompt.UserPrompt, "Methane")
}

func TestEstimateTokenCount(t *testing.T) {
	config := NewStrategyGPTConfig()
	pm, _ := NewPromptManager(config, &PromptMockLogger{})

	// English: ~4 chars per token
	text := "This is a test sentence." // 24 chars -> ~6 tokens
	count := pm.EstimateTokenCount(text)
	assert.InDelta(t, 6, count, 2)

	// Chinese: ~1.5 chars per token
	textCN := "这是一个测试句子。" // 24 bytes (UTF-8), 8 runes -> ~16 tokens if by bytes?
	// EstimateTokenCount uses len(text) which is bytes for string.
	// 24 bytes / 1.5 = 16.
	countCN := pm.EstimateTokenCount(textCN)
	assert.InDelta(t, 16, countCN, 5)
}

func TestRegisterTemplate(t *testing.T) {
	config := NewStrategyGPTConfig()
	pm, _ := NewPromptManager(config, &PromptMockLogger{})

	tmplStr := "Hello {{.Name}}"
	err := pm.RegisterTemplate("greeting", tmplStr)
	assert.NoError(t, err)

	result, err := pm.RenderTemplate("greeting", map[string]string{"Name": "World"})
	assert.NoError(t, err)
	assert.Equal(t, "Hello World", result)
}

func TestGetSystemPrompt_UnknownTask(t *testing.T) {
	config := NewStrategyGPTConfig()
	pm, _ := NewPromptManager(config, &PromptMockLogger{})

	_, err := pm.GetSystemPrompt("unknown_task")
	assert.Error(t, err)
}
//Personal.AI order the ending
