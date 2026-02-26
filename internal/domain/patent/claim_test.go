package patent

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClaimType_String(t *testing.T) {
	assert.Equal(t, "Independent", ClaimTypeIndependent.String())
	assert.Equal(t, "Dependent", ClaimTypeDependent.String())
	assert.Equal(t, "Unknown", ClaimType(255).String())
}

func TestClaimType_IsValid(t *testing.T) {
	assert.True(t, ClaimTypeIndependent.IsValid())
	assert.True(t, ClaimTypeDependent.IsValid())
	assert.False(t, ClaimType(255).IsValid())
}

func TestClaimCategory_String(t *testing.T) {
	assert.Equal(t, "Product", ClaimCategoryProduct.String())
	assert.Equal(t, "Method", ClaimCategoryMethod.String())
	assert.Equal(t, "Use", ClaimCategoryUse.String())
	assert.Equal(t, "Unknown", ClaimCategory(255).String())
}

func TestClaimCategory_IsValid(t *testing.T) {
	assert.True(t, ClaimCategoryProduct.IsValid())
	assert.True(t, ClaimCategoryMethod.IsValid())
	assert.True(t, ClaimCategoryUse.IsValid())
	assert.False(t, ClaimCategory(255).IsValid())
}

func TestClaimElementType_String(t *testing.T) {
	assert.Equal(t, "Structural", StructuralElement.String())
	assert.Equal(t, "Functional", FunctionalElement.String())
	assert.Equal(t, "Parameter", ParameterElement.String())
	assert.Equal(t, "Process", ProcessElement.String())
	assert.Equal(t, "Unknown", ClaimElementType(255).String())
}

func TestClaimElementType_IsValid(t *testing.T) {
	assert.True(t, StructuralElement.IsValid())
	assert.True(t, FunctionalElement.IsValid())
	assert.True(t, ParameterElement.IsValid())
	assert.True(t, ProcessElement.IsValid())
	assert.False(t, ClaimElementType(255).IsValid())
}

func TestNewClaim_Success_Independent(t *testing.T) {
	c, err := NewClaim(1, "A generic text for claim", ClaimTypeIndependent, ClaimCategoryProduct)
	require.NoError(t, err)
	assert.NotNil(t, c)
	assert.Equal(t, 1, c.Number)
	assert.Equal(t, ClaimTypeIndependent, c.Type)
}

func TestNewClaim_Success_Dependent(t *testing.T) {
	c, err := NewClaim(2, "A generic text for claim", ClaimTypeDependent, ClaimCategoryProduct)
	require.NoError(t, err)
	assert.NotNil(t, c)
	assert.Equal(t, ClaimTypeDependent, c.Type)
}

func TestNewClaim_InvalidNumber_Zero(t *testing.T) {
	_, err := NewClaim(0, "valid text valid text", ClaimTypeIndependent, ClaimCategoryProduct)
	assert.Error(t, err)
}

func TestNewClaim_InvalidNumber_Negative(t *testing.T) {
	_, err := NewClaim(-1, "valid text valid text", ClaimTypeIndependent, ClaimCategoryProduct)
	assert.Error(t, err)
}

func TestNewClaim_EmptyText(t *testing.T) {
	_, err := NewClaim(1, "", ClaimTypeIndependent, ClaimCategoryProduct)
	assert.Error(t, err)
}

func TestNewClaim_TextTooShort(t *testing.T) {
	_, err := NewClaim(1, "short", ClaimTypeIndependent, ClaimCategoryProduct)
	assert.Error(t, err)
}

func TestNewClaim_TextTooLong(t *testing.T) {
	longText := strings.Repeat("a", 50001)
	_, err := NewClaim(1, longText, ClaimTypeIndependent, ClaimCategoryProduct)
	assert.Error(t, err)
}

func TestNewClaim_InvalidClaimType(t *testing.T) {
	_, err := NewClaim(1, "valid text valid text", ClaimType(255), ClaimCategoryProduct)
	assert.Error(t, err)
}

func TestNewClaim_InvalidCategory(t *testing.T) {
	_, err := NewClaim(1, "valid text valid text", ClaimTypeIndependent, ClaimCategory(255))
	assert.Error(t, err)
}

func TestClaim_Validate_IndependentValid(t *testing.T) {
	c, _ := NewClaim(1, "valid text valid text", ClaimTypeIndependent, ClaimCategoryProduct)
	err := c.Validate()
	assert.NoError(t, err)
}

func TestClaim_Validate_DependentValid(t *testing.T) {
	c, _ := NewClaim(2, "valid text valid text", ClaimTypeDependent, ClaimCategoryProduct)
	c.SetDependencies([]int{1})
	err := c.Validate()
	assert.NoError(t, err)
}

func TestClaim_Validate_DependentMissingDeps(t *testing.T) {
	c, _ := NewClaim(2, "valid text valid text", ClaimTypeDependent, ClaimCategoryProduct)
	// No dependencies set
	err := c.Validate()
	assert.Error(t, err)
}

func TestClaim_Validate_IndependentWithDeps(t *testing.T) {
	c, _ := NewClaim(1, "valid text valid text", ClaimTypeIndependent, ClaimCategoryProduct)
	c.DependsOn = []int{2} // Manually set to bypass SetDependencies check
	err := c.Validate()
	assert.Error(t, err)
}

func TestClaim_Validate_DependsOnSelf(t *testing.T) {
	c, _ := NewClaim(2, "valid text valid text", ClaimTypeDependent, ClaimCategoryProduct)
	c.DependsOn = []int{2}
	err := c.Validate()
	assert.Error(t, err)
}

func TestClaim_Validate_DependsOnForwardRef(t *testing.T) {
	c, _ := NewClaim(2, "valid text valid text", ClaimTypeDependent, ClaimCategoryProduct)
	c.DependsOn = []int{3}
	err := c.Validate()
	assert.Error(t, err)
}

func TestClaim_SetDependencies_Success(t *testing.T) {
	c, _ := NewClaim(2, "valid text valid text", ClaimTypeDependent, ClaimCategoryProduct)
	err := c.SetDependencies([]int{1})
	assert.NoError(t, err)
	assert.Equal(t, []int{1}, c.DependsOn)
}

func TestClaim_SetDependencies_IndependentClaim(t *testing.T) {
	c, _ := NewClaim(1, "valid text valid text", ClaimTypeIndependent, ClaimCategoryProduct)
	err := c.SetDependencies([]int{2})
	assert.Error(t, err)
}

func TestClaim_SetDependencies_InvalidNumber(t *testing.T) {
	c, _ := NewClaim(2, "valid text valid text", ClaimTypeDependent, ClaimCategoryProduct)
	err := c.SetDependencies([]int{0})
	assert.Error(t, err)
}

func TestClaim_SetDependencies_ForwardReference(t *testing.T) {
	c, _ := NewClaim(2, "valid text valid text", ClaimTypeDependent, ClaimCategoryProduct)
	err := c.SetDependencies([]int{2}) // Equal
	assert.Error(t, err)
	err = c.SetDependencies([]int{3}) // Greater
	assert.Error(t, err)
}

func TestClaim_SetDependencies_Duplicates(t *testing.T) {
	c, _ := NewClaim(3, "valid text valid text", ClaimTypeDependent, ClaimCategoryProduct)
	err := c.SetDependencies([]int{1, 1})
	assert.Error(t, err)
}

func TestClaim_AddElement_Success(t *testing.T) {
	c, _ := NewClaim(1, "valid text valid text", ClaimTypeIndependent, ClaimCategoryProduct)
	elem := ClaimElement{ID: "E1", Text: "text", Type: StructuralElement}
	err := c.AddElement(elem)
	assert.NoError(t, err)
	assert.Len(t, c.Elements, 1)
}

func TestClaim_AddElement_DuplicateID(t *testing.T) {
	c, _ := NewClaim(1, "valid text valid text", ClaimTypeIndependent, ClaimCategoryProduct)
	elem := ClaimElement{ID: "E1", Text: "text", Type: StructuralElement}
	c.AddElement(elem)
	err := c.AddElement(elem)
	assert.Error(t, err)
}

func TestClaim_AddElement_EmptyID(t *testing.T) {
	c, _ := NewClaim(1, "valid text valid text", ClaimTypeIndependent, ClaimCategoryProduct)
	elem := ClaimElement{ID: "", Text: "text", Type: StructuralElement}
	err := c.AddElement(elem)
	assert.Error(t, err)
}

func TestClaim_AddElement_EmptyText(t *testing.T) {
	c, _ := NewClaim(1, "valid text valid text", ClaimTypeIndependent, ClaimCategoryProduct)
	elem := ClaimElement{ID: "E1", Text: "", Type: StructuralElement}
	err := c.AddElement(elem)
	assert.Error(t, err)
}

func TestClaim_AddElement_InvalidType(t *testing.T) {
	c, _ := NewClaim(1, "valid text valid text", ClaimTypeIndependent, ClaimCategoryProduct)
	elem := ClaimElement{ID: "E1", Text: "text", Type: ClaimElementType(255)}
	err := c.AddElement(elem)
	assert.Error(t, err)
}

func TestClaim_EssentialElements(t *testing.T) {
	c, _ := NewClaim(1, "valid text valid text", ClaimTypeIndependent, ClaimCategoryProduct)
	c.AddElement(ClaimElement{ID: "E1", Text: "t1", Type: StructuralElement, IsEssential: true})
	c.AddElement(ClaimElement{ID: "E2", Text: "t2", Type: StructuralElement, IsEssential: false})

	ess := c.EssentialElements()
	assert.Len(t, ess, 1)
	assert.Equal(t, "E1", ess[0].ID)
}

func TestClaim_EssentialElements_Empty(t *testing.T) {
	c, _ := NewClaim(1, "valid text valid text", ClaimTypeIndependent, ClaimCategoryProduct)
	ess := c.EssentialElements()
	assert.Empty(t, ess)
}

func TestClaim_HasMarkushStructure_True(t *testing.T) {
	c, _ := NewClaim(1, "valid text valid text", ClaimTypeIndependent, ClaimCategoryProduct)
	c.MarkushStructures = []string{"M1"}
	assert.True(t, c.HasMarkushStructure())
}

func TestClaim_HasMarkushStructure_False(t *testing.T) {
	c, _ := NewClaim(1, "valid text valid text", ClaimTypeIndependent, ClaimCategoryProduct)
	assert.False(t, c.HasMarkushStructure())
}

func TestClaim_ContainsMoleculeReference_True(t *testing.T) {
	c, _ := NewClaim(1, "valid text valid text", ClaimTypeIndependent, ClaimCategoryProduct)
	c.AddElement(ClaimElement{ID: "E1", Text: "t1", Type: StructuralElement, MoleculeRef: "SMILES"})
	assert.True(t, c.ContainsMoleculeReference())
}

func TestClaim_ContainsMoleculeReference_False(t *testing.T) {
	c, _ := NewClaim(1, "valid text valid text", ClaimTypeIndependent, ClaimCategoryProduct)
	c.AddElement(ClaimElement{ID: "E1", Text: "t1", Type: StructuralElement})
	assert.False(t, c.ContainsMoleculeReference())
}

func TestClaimSet_IndependentClaims(t *testing.T) {
	c1, err := NewClaim(1, "valid text valid text", ClaimTypeIndependent, ClaimCategoryProduct)
	require.NoError(t, err)
	c2, err := NewClaim(2, "valid text valid text", ClaimTypeDependent, ClaimCategoryProduct)
	require.NoError(t, err)
	cs := ClaimSet{*c1, *c2}

	indeps := cs.IndependentClaims()
	assert.Len(t, indeps, 1)
	assert.Equal(t, 1, indeps[0].Number)
}

func TestClaimSet_DependentClaimsOf(t *testing.T) {
	c1, err := NewClaim(1, "valid text valid text", ClaimTypeIndependent, ClaimCategoryProduct)
	require.NoError(t, err)
	c2, err := NewClaim(2, "valid text valid text", ClaimTypeDependent, ClaimCategoryProduct)
	require.NoError(t, err)
	c2.SetDependencies([]int{1})
	cs := ClaimSet{*c1, *c2}

	deps := cs.DependentClaimsOf(1)
	assert.Len(t, deps, 1)
	assert.Equal(t, 2, deps[0].Number)
}

func TestClaimSet_DependentClaimsOf_NotFound(t *testing.T) {
	c1, err := NewClaim(1, "valid text valid text", ClaimTypeIndependent, ClaimCategoryProduct)
	require.NoError(t, err)
	cs := ClaimSet{*c1}
	deps := cs.DependentClaimsOf(99)
	assert.Empty(t, deps)
}

func TestClaimSet_ClaimTree(t *testing.T) {
	c1, err := NewClaim(1, "valid text valid text", ClaimTypeIndependent, ClaimCategoryProduct)
	require.NoError(t, err)
	c2, err := NewClaim(2, "valid text valid text", ClaimTypeDependent, ClaimCategoryProduct)
	require.NoError(t, err)
	c2.SetDependencies([]int{1})
	c3, err := NewClaim(3, "valid text valid text", ClaimTypeDependent, ClaimCategoryProduct)
	require.NoError(t, err)
	c3.SetDependencies([]int{2})
	c4, err := NewClaim(4, "valid text valid text", ClaimTypeDependent, ClaimCategoryProduct)
	require.NoError(t, err)
	c4.SetDependencies([]int{1})

	cs := ClaimSet{*c1, *c2, *c3, *c4}

	tree := cs.ClaimTree(1)
	// Expect 1, 2, 3, 4
	assert.Len(t, tree, 4)

	tree2 := cs.ClaimTree(2)
	// Expect 2, 3
	assert.Len(t, tree2, 2)
}

func TestClaimSet_ClaimTree_SingleClaim(t *testing.T) {
	c1, err := NewClaim(1, "valid text valid text", ClaimTypeIndependent, ClaimCategoryProduct)
	require.NoError(t, err)
	cs := ClaimSet{*c1}
	tree := cs.ClaimTree(1)
	assert.Len(t, tree, 1)
}

func TestClaimSet_Validate_Success(t *testing.T) {
	c1, err := NewClaim(1, "valid text valid text", ClaimTypeIndependent, ClaimCategoryProduct)
	require.NoError(t, err)
	cs := ClaimSet{*c1}
	assert.NoError(t, cs.Validate())
}

func TestClaimSet_Validate_Empty(t *testing.T) {
	cs := ClaimSet{}
	assert.Error(t, cs.Validate())
}

func TestClaimSet_Validate_NoIndependent(t *testing.T) {
	c2, err := NewClaim(2, "valid text valid text", ClaimTypeDependent, ClaimCategoryProduct)
	require.NoError(t, err)
	// Cannot set dependency 1 because 1 doesn't exist in set (logic in Validate checks referencing existent claims)
	// But let's try just setting dependency logic
	c2.DependsOn = []int{1}
	cs := ClaimSet{*c2}
	assert.Error(t, cs.Validate())
}

func TestClaimSet_Validate_DuplicateNumbers(t *testing.T) {
	c1, err := NewClaim(1, "valid text valid text", ClaimTypeIndependent, ClaimCategoryProduct)
	require.NoError(t, err)
	c2, err := NewClaim(1, "valid text valid text", ClaimTypeIndependent, ClaimCategoryProduct)
	require.NoError(t, err)
	cs := ClaimSet{*c1, *c2}
	assert.Error(t, cs.Validate())
}

func TestClaimSet_Validate_BrokenReference(t *testing.T) {
	c1, err := NewClaim(1, "valid text valid text", ClaimTypeIndependent, ClaimCategoryProduct)
	require.NoError(t, err)
	c2, err := NewClaim(2, "valid text valid text", ClaimTypeDependent, ClaimCategoryProduct)
	require.NoError(t, err)
	c2.DependsOn = []int{99} // 99 not in set
	cs := ClaimSet{*c1, *c2}
	assert.Error(t, cs.Validate())
}

func TestClaimSet_FindByNumber_Found(t *testing.T) {
	c1, err := NewClaim(1, "valid text valid text", ClaimTypeIndependent, ClaimCategoryProduct)
	require.NoError(t, err)
	cs := ClaimSet{*c1}
	found, ok := cs.FindByNumber(1)
	assert.True(t, ok)
	assert.Equal(t, 1, found.Number)
}

func TestClaimSet_FindByNumber_NotFound(t *testing.T) {
	c1, err := NewClaim(1, "valid text valid text", ClaimTypeIndependent, ClaimCategoryProduct)
	require.NoError(t, err)
	cs := ClaimSet{*c1}
	_, ok := cs.FindByNumber(2)
	assert.False(t, ok)
}

//Personal.AI order the ending
