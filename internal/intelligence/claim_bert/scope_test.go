package claim_bert

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestComputeScopeBreadth_TransitionalPhrase(t *testing.T) {
	analyzer := NewScopeAnalyzer(nil, nil)

	claim := &ParsedClaim{
		TransitionalPhrase: "comprising",
		Features:           make([]*TechnicalFeature, 5),
		ScopeScore:         0.5,
	}

	score, _ := analyzer.ComputeScopeBreadth(context.Background(), claim)
	assert.Greater(t, score, 0.5) // Comprising adds 0.1
}

func TestComputeScopeBreadth_Narrow(t *testing.T) {
	analyzer := NewScopeAnalyzer(nil, nil)

	claim := &ParsedClaim{
		TransitionalPhrase: "consisting of",
		Features:           make([]*TechnicalFeature, 10), // >=8 subtracts 0.1
		ScopeScore:         0.5,
	}

	score, _ := analyzer.ComputeScopeBreadth(context.Background(), claim)
	// 0.5 - 0.15 (consisting) - 0.10 (features) = 0.25
	assert.InDelta(t, 0.25, score, 0.001)
}

func TestCompareScopes_Equivalent(t *testing.T) {
	analyzer := NewScopeAnalyzer(nil, nil)

	f1 := &TechnicalFeature{Text: "A"}
	f2 := &TechnicalFeature{Text: "B"}

	claimA := &ParsedClaim{ClaimNumber: 1, Features: []*TechnicalFeature{f1, f2}}
	claimB := &ParsedClaim{ClaimNumber: 2, Features: []*TechnicalFeature{f1, f2}}

	comp, err := analyzer.CompareScopes(context.Background(), claimA, claimB)
	assert.NoError(t, err)
	assert.Equal(t, RelEquivalent, comp.Relationship)
	assert.Equal(t, 1.0, comp.OverlapScore)
}

func TestCompareScopes_Subset(t *testing.T) {
	analyzer := NewScopeAnalyzer(nil, nil)

	f1 := &TechnicalFeature{Text: "A"}
	f2 := &TechnicalFeature{Text: "B"}

	claimA := &ParsedClaim{ClaimNumber: 1, Features: []*TechnicalFeature{f1}}
	claimB := &ParsedClaim{ClaimNumber: 2, Features: []*TechnicalFeature{f1, f2}}

	// A has fewer features -> A is Broader -> A Contains B
	comp, err := analyzer.CompareScopes(context.Background(), claimA, claimB)
	assert.NoError(t, err)
	assert.Equal(t, RelAContainsB, comp.Relationship)
}
//Personal.AI order the ending
