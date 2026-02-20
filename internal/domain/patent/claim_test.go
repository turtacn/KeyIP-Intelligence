// Package patent_test provides unit tests for the patent domain value objects,
// entities, services, and repository contracts.
package patent_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/turtacn/KeyIP-Intelligence/internal/domain/patent"
	ptypes "github.com/turtacn/KeyIP-Intelligence/pkg/types/patent"
)

// ─────────────────────────────────────────────────────────────────────────────
// helpers
// ─────────────────────────────────────────────────────────────────────────────

func intPtr(n int) *int { return &n }

// ─────────────────────────────────────────────────────────────────────────────
// TestNewClaim
// ─────────────────────────────────────────────────────────────────────────────

func TestNewClaim_ValidIndependentClaim(t *testing.T) {
	t.Parallel()

	c, err := patent.NewClaim(1, "A compound comprising a benzene ring.", ptypes.ClaimTypeIndependent, nil)
	require.NoError(t, err)
	require.NotNil(t, c)
	assert.Equal(t, 1, c.Number)
	assert.Equal(t, "A compound comprising a benzene ring.", c.Text)
	assert.Equal(t, ptypes.ClaimTypeIndependent, c.Type)
	assert.Nil(t, c.ParentClaimNumber)
	assert.NotEmpty(t, string(c.ID))
}

func TestNewClaim_ValidDependentClaim(t *testing.T) {
	t.Parallel()

	parent := intPtr(1)
	c, err := patent.NewClaim(2, "The compound of claim 1, wherein R1 is methyl.", ptypes.ClaimTypeDependent, parent)
	require.NoError(t, err)
	require.NotNil(t, c)
	assert.Equal(t, 2, c.Number)
	assert.Equal(t, ptypes.ClaimTypeDependent, c.Type)
	require.NotNil(t, c.ParentClaimNumber)
	assert.Equal(t, 1, *c.ParentClaimNumber)
}

func TestNewClaim_ZeroNumber(t *testing.T) {
	t.Parallel()

	_, err := patent.NewClaim(0, "Some claim text.", ptypes.ClaimTypeIndependent, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "number")
}

func TestNewClaim_NegativeNumber(t *testing.T) {
	t.Parallel()

	_, err := patent.NewClaim(-3, "Some claim text.", ptypes.ClaimTypeIndependent, nil)
	require.Error(t, err)
}

func TestNewClaim_EmptyText(t *testing.T) {
	t.Parallel()

	_, err := patent.NewClaim(1, "", ptypes.ClaimTypeIndependent, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "text")
}

func TestNewClaim_WhitespaceOnlyText(t *testing.T) {
	t.Parallel()

	_, err := patent.NewClaim(1, "   \t\n  ", ptypes.ClaimTypeIndependent, nil)
	require.Error(t, err)
}

func TestNewClaim_DependentWithoutParent(t *testing.T) {
	t.Parallel()

	_, err := patent.NewClaim(2, "The compound of claim 1.", ptypes.ClaimTypeDependent, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parent")
}

func TestNewClaim_DependentParentNotLessThanNumber(t *testing.T) {
	t.Parallel()

	// Parent == number is invalid.
	_, err := patent.NewClaim(2, "Dependent on itself.", ptypes.ClaimTypeDependent, intPtr(2))
	require.Error(t, err)
}

func TestNewClaim_IndependentWithParent(t *testing.T) {
	t.Parallel()

	// Independent claims must not carry a parent reference.
	_, err := patent.NewClaim(3, "Independent claim.", ptypes.ClaimTypeIndependent, intPtr(1))
	require.Error(t, err)
}

func TestNewClaim_TextIsTrimmed(t *testing.T) {
	t.Parallel()

	c, err := patent.NewClaim(1, "  A method.  ", ptypes.ClaimTypeIndependent, nil)
	require.NoError(t, err)
	assert.Equal(t, "A method.", c.Text)
}

// ─────────────────────────────────────────────────────────────────────────────
// TestIsIndependent
// ─────────────────────────────────────────────────────────────────────────────

func TestIsIndependent_ReturnsTrue(t *testing.T) {
	t.Parallel()

	c, err := patent.NewClaim(1, "An independent claim.", ptypes.ClaimTypeIndependent, nil)
	require.NoError(t, err)
	assert.True(t, c.IsIndependent())
}

func TestIsIndependent_ReturnsFalse(t *testing.T) {
	t.Parallel()

	c, err := patent.NewClaim(2, "A dependent claim of claim 1.", ptypes.ClaimTypeDependent, intPtr(1))
	require.NoError(t, err)
	assert.False(t, c.IsIndependent())
}

// ─────────────────────────────────────────────────────────────────────────────
// TestContainsChemicalEntity
// ─────────────────────────────────────────────────────────────────────────────

func TestContainsChemicalEntity_True(t *testing.T) {
	t.Parallel()

	c, err := patent.NewClaim(1, "A compound comprising a benzene ring.", ptypes.ClaimTypeIndependent, nil)
	require.NoError(t, err)

	el := patent.ClaimElement{
		Text:             "a benzene ring",
		IsStructural:     true,
		ChemicalEntities: []string{"c1ccccc1"},
	}
	require.NoError(t, c.AddElement(el))
	assert.True(t, c.ContainsChemicalEntity())
}

func TestContainsChemicalEntity_False_NoElements(t *testing.T) {
	t.Parallel()

	c, err := patent.NewClaim(1, "A method of treatment.", ptypes.ClaimTypeIndependent, nil)
	require.NoError(t, err)
	assert.False(t, c.ContainsChemicalEntity())
}

func TestContainsChemicalEntity_False_ElementsWithoutChemicals(t *testing.T) {
	t.Parallel()

	c, err := patent.NewClaim(1, "A device comprising a housing.", ptypes.ClaimTypeIndependent, nil)
	require.NoError(t, err)

	el := patent.ClaimElement{
		Text:             "a housing",
		IsStructural:     true,
		ChemicalEntities: nil,
	}
	require.NoError(t, c.AddElement(el))
	assert.False(t, c.ContainsChemicalEntity())
}

// ─────────────────────────────────────────────────────────────────────────────
// TestExtractKeyTerms
// ─────────────────────────────────────────────────────────────────────────────

func TestExtractKeyTerms_NonEmpty(t *testing.T) {
	t.Parallel()

	c, err := patent.NewClaim(1,
		"A pharmaceutical composition comprising an active compound and a pharmaceutically acceptable carrier.",
		ptypes.ClaimTypeIndependent, nil)
	require.NoError(t, err)

	terms := c.ExtractKeyTerms()
	assert.NotEmpty(t, terms, "ExtractKeyTerms must return at least one term for a substantive claim")
}

func TestExtractKeyTerms_ExcludesStopWords(t *testing.T) {
	t.Parallel()

	c, err := patent.NewClaim(1,
		"A composition comprising an organic molecule wherein said molecule is active.",
		ptypes.ClaimTypeIndependent, nil)
	require.NoError(t, err)

	terms := c.ExtractKeyTerms()

	forbidden := []string{"a", "an", "the", "wherein", "comprising", "said", "is"}
	for _, fw := range forbidden {
		for _, term := range terms {
			assert.NotEqual(t, fw, term, "stop word %q must not appear in key terms", fw)
		}
	}
}

func TestExtractKeyTerms_Deduplicated(t *testing.T) {
	t.Parallel()

	c, err := patent.NewClaim(1,
		"A compound, wherein the compound comprises a ring, the ring being aromatic.",
		ptypes.ClaimTypeIndependent, nil)
	require.NoError(t, err)

	terms := c.ExtractKeyTerms()
	seen := make(map[string]int)
	for _, term := range terms {
		seen[term]++
		assert.Equal(t, 1, seen[term], "term %q should appear only once in key terms", term)
	}
}

func TestExtractKeyTerms_ShortTokensExcluded(t *testing.T) {
	t.Parallel()

	c, err := patent.NewClaim(1,
		"A method of treating a disease by administering a compound.",
		ptypes.ClaimTypeIndependent, nil)
	require.NoError(t, err)

	terms := c.ExtractKeyTerms()
	for _, term := range terms {
		assert.GreaterOrEqual(t, len(term), 3, "term %q is too short and should have been filtered", term)
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TestAddElement
// ─────────────────────────────────────────────────────────────────────────────

func TestAddElement_Success(t *testing.T) {
	t.Parallel()

	c, err := patent.NewClaim(1, "A compound.", ptypes.ClaimTypeIndependent, nil)
	require.NoError(t, err)

	el := patent.ClaimElement{Text: "a benzene core", IsStructural: true}
	require.NoError(t, c.AddElement(el))
	assert.Len(t, c.Elements, 1)
}

func TestAddElement_EmptyTextReturnsError(t *testing.T) {
	t.Parallel()

	c, err := patent.NewClaim(1, "A compound.", ptypes.ClaimTypeIndependent, nil)
	require.NoError(t, err)

	el := patent.ClaimElement{Text: ""}
	assert.Error(t, c.AddElement(el))
	assert.Empty(t, c.Elements)
}

func TestAddElement_IDIsAutoAssigned(t *testing.T) {
	t.Parallel()

	c, err := patent.NewClaim(1, "A compound.", ptypes.ClaimTypeIndependent, nil)
	require.NoError(t, err)

	el := patent.ClaimElement{Text: "a ring system"}
	require.NoError(t, c.AddElement(el))
	assert.NotEmpty(t, string(c.Elements[0].ID))
}

// ─────────────────────────────────────────────────────────────────────────────
// TestToDTO
// ─────────────────────────────────────────────────────────────────────────────

func TestToDTO_FieldsMatch(t *testing.T) {
	t.Parallel()

	c, err := patent.NewClaim(1, "A compound comprising indole.", ptypes.ClaimTypeIndependent, nil)
	require.NoError(t, err)

	_ = c.AddElement(patent.ClaimElement{
		Text:             "indole",
		IsStructural:     true,
		ChemicalEntities: []string{"c1ccc2[nH]ccc2c1"},
	})

	dto := c.ToDTO()
	assert.Equal(t, c.ID, dto.ID)
	assert.Equal(t, c.Number, dto.Number)
	assert.Equal(t, c.Text, dto.Text)
	assert.Equal(t, c.Type, dto.Type)
	assert.Nil(t, dto.ParentClaimNumber)
	require.Len(t, dto.Elements, 1)
	assert.Equal(t, "indole", dto.Elements[0].Text)
	assert.Equal(t, []string{"c1ccc2[nH]ccc2c1"}, dto.Elements[0].ChemicalEntities)
}

//Personal.AI order the ending
