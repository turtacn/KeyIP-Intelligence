package strategy_gpt

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewStrategyGPTConfig(t *testing.T) {
	c := NewStrategyGPTConfig()
	assert.Equal(t, 32768, c.MaxContextLength)
}

func TestStrategyGPTConfig_Validate(t *testing.T) {
	c := NewStrategyGPTConfig()
	assert.NoError(t, c.Validate())

	c.MaxContextLength = -1
	assert.Error(t, c.Validate())
}
//Personal.AI order the ending
