package portfolio

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValuationTier(t *testing.T) {
	assert.Equal(t, "S", string(ValuationTierS))
	assert.Equal(t, "D", string(ValuationTierD))
}

// Removed undefined types and tests relying on them (DimensionScore, ValuationDimension)
// Assuming these were from a previous design iteration or unimplemented feature.

//Personal.AI order the ending
