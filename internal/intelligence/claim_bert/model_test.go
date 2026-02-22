package claim_bert

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewClaimBERTConfig(t *testing.T) {
	c := NewClaimBERTConfig()
	assert.Equal(t, 512, c.MaxSequenceLength)
	assert.Equal(t, 768, c.HiddenDim)
}

func TestClaimBERTConfig_Validate(t *testing.T) {
	c := NewClaimBERTConfig()
	assert.NoError(t, c.Validate())

	c.MaxSequenceLength = 500 // Not power of 2
	assert.Error(t, c.Validate())
}
//Personal.AI order the ending
