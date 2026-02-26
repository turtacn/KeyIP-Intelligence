package patent

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockMarkushMatcher is a mock implementation of MarkushMatcher interface.
type MockMarkushMatcher struct {
	mock.Mock
}

func (m *MockMarkushMatcher) IsSubstructure(core, molecule string) (bool, error) {
	args := m.Called(core, molecule)
	return args.Bool(0), args.Error(1)
}

func (m *MockMarkushMatcher) ExtractSubstituents(core, molecule string) (map[string]string, error) {
	args := m.Called(core, molecule)
	return args.Get(0).(map[string]string), args.Error(1)
}

func (m *MockMarkushMatcher) MatchSubstituent(actual string, allowed []Substituent) (bool, string, error) {
	args := m.Called(actual, allowed)
	return args.Bool(0), args.String(1), args.Error(2)
}

func TestSubstituentType_String(t *testing.T) {
	assert.Equal(t, "Alkyl", SubstituentTypeAlkyl.String())
	assert.Equal(t, "Aryl", SubstituentTypeAryl.String())
	assert.Equal(t, "Unknown", SubstituentType(255).String())
}

func TestSubstituentType_IsValid(t *testing.T) {
	assert.True(t, SubstituentTypeAlkyl.IsValid())
	assert.False(t, SubstituentType(255).IsValid())
}

func TestSubstituent_Validate_Success(t *testing.T) {
	s := Substituent{ID: "S1", Name: "Methyl", Type: SubstituentTypeAlkyl}
	assert.NoError(t, s.Validate())
}

func TestSubstituent_Validate_EmptyID(t *testing.T) {
	s := Substituent{ID: "", Name: "Methyl", Type: SubstituentTypeAlkyl}
	assert.Error(t, s.Validate())
}

func TestSubstituent_Validate_EmptyName(t *testing.T) {
	s := Substituent{ID: "S1", Name: "", Type: SubstituentTypeAlkyl}
	assert.Error(t, s.Validate())
}

func TestSubstituent_Validate_InvalidType(t *testing.T) {
	s := Substituent{ID: "S1", Name: "Methyl", Type: SubstituentType(255)}
	assert.Error(t, s.Validate())
}

func TestSubstituent_Validate_InvalidCarbonRange(t *testing.T) {
	s := Substituent{ID: "S1", Name: "Methyl", Type: SubstituentTypeAlkyl, CarbonRange: [2]int{5, 1}}
	assert.Error(t, s.Validate())
}

func TestVariablePosition_Validate_Success(t *testing.T) {
	vp := VariablePosition{
		Symbol:       "R1",
		Substituents: []Substituent{{ID: "S1", Name: "Methyl", Type: SubstituentTypeAlkyl}},
	}
	assert.NoError(t, vp.Validate())
}

func TestVariablePosition_Validate_EmptySymbol(t *testing.T) {
	vp := VariablePosition{
		Symbol:       "",
		Substituents: []Substituent{{ID: "S1", Name: "Methyl", Type: SubstituentTypeAlkyl}},
	}
	assert.Error(t, vp.Validate())
}

func TestVariablePosition_Validate_NoSubstituents(t *testing.T) {
	vp := VariablePosition{
		Symbol:       "R1",
		Substituents: []Substituent{},
		IsOptional:   false,
	}
	assert.Error(t, vp.Validate())
}

func TestVariablePosition_Validate_OptionalNoSubstituents(t *testing.T) {
	vp := VariablePosition{
		Symbol:       "R1",
		Substituents: []Substituent{},
		IsOptional:   true,
	}
	assert.NoError(t, vp.Validate())
}

func TestVariablePosition_SubstituentCount(t *testing.T) {
	vp := VariablePosition{
		Substituents: []Substituent{
			{ID: "S1", Name: "Methyl", Type: SubstituentTypeAlkyl},
			{ID: "S2", Name: "Ethyl", Type: SubstituentTypeAlkyl},
		},
	}
	assert.Equal(t, 2, vp.SubstituentCount())
}

func TestNewMarkushStructure_Success(t *testing.T) {
	ms, err := NewMarkushStructure("Test Markush", "Core", 1)
	require.NoError(t, err)
	assert.NotNil(t, ms)
	assert.Equal(t, "Test Markush", ms.Name)
	assert.Equal(t, "Core", ms.CoreStructure)
	assert.Equal(t, 1, ms.ClaimNumber)
	assert.NotEmpty(t, ms.ID)
}

func TestNewMarkushStructure_EmptyName(t *testing.T) {
	_, err := NewMarkushStructure("", "Core", 1)
	assert.Error(t, err)
}

func TestNewMarkushStructure_EmptyCoreStructure(t *testing.T) {
	_, err := NewMarkushStructure("Test", "", 1)
	assert.Error(t, err)
}

func TestNewMarkushStructure_InvalidClaimNumber(t *testing.T) {
	_, err := NewMarkushStructure("Test", "Core", 0)
	assert.Error(t, err)
}

func TestMarkushStructure_AddPosition_Success(t *testing.T) {
	ms, _ := NewMarkushStructure("Test", "Core", 1)
	vp := VariablePosition{Symbol: "R1", Substituents: []Substituent{{ID: "S1", Name: "H", Type: SubstituentTypeHydrogen}}}
	err := ms.AddPosition(vp)
	assert.NoError(t, err)
	assert.Len(t, ms.Positions, 1)
}

func TestMarkushStructure_AddPosition_DuplicateSymbol(t *testing.T) {
	ms, _ := NewMarkushStructure("Test", "Core", 1)
	vp := VariablePosition{Symbol: "R1", Substituents: []Substituent{{ID: "S1", Name: "H", Type: SubstituentTypeHydrogen}}}
	ms.AddPosition(vp)
	err := ms.AddPosition(vp)
	assert.Error(t, err)
}

func TestMarkushStructure_AddConstraint_Success(t *testing.T) {
	ms, _ := NewMarkushStructure("Test", "Core", 1)
	err := ms.AddConstraint("Constraint 1")
	assert.NoError(t, err)
	assert.Len(t, ms.Constraints, 1)
}

func TestMarkushStructure_AddConstraint_Empty(t *testing.T) {
	ms, _ := NewMarkushStructure("Test", "Core", 1)
	err := ms.AddConstraint("")
	assert.Error(t, err)
}

func TestMarkushStructure_Validate_Success(t *testing.T) {
	ms, _ := NewMarkushStructure("Test", "Core", 1)
	vp := VariablePosition{Symbol: "R1", Substituents: []Substituent{{ID: "S1", Name: "H", Type: SubstituentTypeHydrogen}}}
	ms.AddPosition(vp)
	assert.NoError(t, ms.Validate())
}

func TestMarkushStructure_Validate_NoPositions(t *testing.T) {
	ms, _ := NewMarkushStructure("Test", "Core", 1)
	assert.Error(t, ms.Validate())
}

func TestMarkushStructure_Validate_InvalidPosition(t *testing.T) {
	ms, _ := NewMarkushStructure("Test", "Core", 1)
	vp := VariablePosition{Symbol: "", Substituents: []Substituent{{ID: "S1", Name: "H", Type: SubstituentTypeHydrogen}}}
	// AddPosition doesn't validate, only checks duplicates.
	// But let's check if AddPosition allows empty symbol?
	// AddPosition checks duplicates.
	// Let's manually set Positions if AddPosition validates symbol?
	// AddPosition logic: checks duplicate.
	// Validate calls pos.Validate().
	ms.Positions = append(ms.Positions, vp)
	assert.Error(t, ms.Validate())
}

func TestMarkushStructure_Validate_DuplicateSymbols(t *testing.T) {
	ms, _ := NewMarkushStructure("Test", "Core", 1)
	vp := VariablePosition{Symbol: "R1", Substituents: []Substituent{{ID: "S1", Name: "H", Type: SubstituentTypeHydrogen}}}
	ms.Positions = append(ms.Positions, vp, vp) // Manually duplicate
	assert.Error(t, ms.Validate())
}

func TestMarkushStructure_Validate_BrokenLinkedPosition(t *testing.T) {
	ms, _ := NewMarkushStructure("Test", "Core", 1)
	vp := VariablePosition{Symbol: "R1", Substituents: []Substituent{{ID: "S1", Name: "H", Type: SubstituentTypeHydrogen}}, LinkedPositions: []string{"R2"}}
	ms.Positions = []VariablePosition{vp}
	assert.Error(t, ms.Validate())
}

func TestMarkushStructure_CalculateCombinations_SinglePosition(t *testing.T) {
	ms, _ := NewMarkushStructure("Test", "Core", 1)
	vp := VariablePosition{Symbol: "R1", Substituents: []Substituent{
		{ID: "S1", Name: "A", Type: SubstituentTypeAlkyl},
		{ID: "S2", Name: "B", Type: SubstituentTypeAlkyl},
		{ID: "S3", Name: "C", Type: SubstituentTypeAlkyl},
	}}
	ms.Positions = []VariablePosition{vp}
	assert.Equal(t, int64(3), ms.CalculateCombinations())
}

func TestMarkushStructure_CalculateCombinations_MultiplePositions(t *testing.T) {
	ms, _ := NewMarkushStructure("Test", "Core", 1)
	vp1 := VariablePosition{Symbol: "R1", Substituents: []Substituent{
		{ID: "S1", Name: "A", Type: SubstituentTypeAlkyl},
		{ID: "S2", Name: "B", Type: SubstituentTypeAlkyl},
		{ID: "S3", Name: "C", Type: SubstituentTypeAlkyl},
	}}
	vp2 := VariablePosition{Symbol: "R2", Substituents: []Substituent{
		{ID: "S4", Name: "D", Type: SubstituentTypeAlkyl},
		{ID: "S5", Name: "E", Type: SubstituentTypeAlkyl},
		{ID: "S6", Name: "F", Type: SubstituentTypeAlkyl},
		{ID: "S7", Name: "G", Type: SubstituentTypeAlkyl},
	}}
	ms.Positions = []VariablePosition{vp1, vp2}
	assert.Equal(t, int64(12), ms.CalculateCombinations()) // 3 * 4
}

func TestMarkushStructure_CalculateCombinations_OptionalPosition(t *testing.T) {
	ms, _ := NewMarkushStructure("Test", "Core", 1)
	vp1 := VariablePosition{Symbol: "R1", Substituents: []Substituent{
		{ID: "S1", Name: "A", Type: SubstituentTypeAlkyl},
		{ID: "S2", Name: "B", Type: SubstituentTypeAlkyl},
		{ID: "S3", Name: "C", Type: SubstituentTypeAlkyl},
	}}
	vp2 := VariablePosition{Symbol: "R2", IsOptional: true, Substituents: []Substituent{
		{ID: "S4", Name: "D", Type: SubstituentTypeAlkyl},
		{ID: "S5", Name: "E", Type: SubstituentTypeAlkyl},
		{ID: "S6", Name: "F", Type: SubstituentTypeAlkyl},
		{ID: "S7", Name: "G", Type: SubstituentTypeAlkyl},
	}}
	ms.Positions = []VariablePosition{vp1, vp2}
	// R1: 3
	// R2: 4 + 1 (optional) = 5
	// Total: 3 * 5 = 15
	assert.Equal(t, int64(15), ms.CalculateCombinations())
}

func TestMarkushStructure_CalculateCombinations_RepeatRange(t *testing.T) {
	ms, _ := NewMarkushStructure("Test", "Core", 1)
	vp := VariablePosition{
		Symbol: "R1",
		RepeatRange: [2]int{0, 3}, // Span = 3 - 0 + 1 = 4
		Substituents: []Substituent{{ID: "S1", Name: "A", Type: SubstituentTypeAlkyl}},
	}
	ms.Positions = []VariablePosition{vp}
	// Count = 1 substituent * 4 repeats = 4
	assert.Equal(t, int64(4), ms.CalculateCombinations())
}

func TestMarkushStructure_CalculateCombinations_Overflow(t *testing.T) {
	ms, _ := NewMarkushStructure("Test", "Core", 1)
	// Create a scenario that overflows int64
	// Not easy to create substituents array that big, but we can simulate large repeat range or many positions
	// Using mock logic or setting Positions manually to have high counts (not possible via slice, but logic iterates slices)
	// We can use RepeatRange to boost numbers
	// If RepeatRange is huge?
	vp := VariablePosition{
		Symbol: "R1",
		RepeatRange: [2]int{0, math.MaxInt32}, // Large span
		Substituents: make([]Substituent, 1000), // 1000 subs
	}
	// Multiplier = 1000 * MaxInt32 approx 2e12
	// If we have 2 such positions -> 4e24 which overflows int64
	ms.Positions = []VariablePosition{vp, vp}
	assert.Equal(t, int64(math.MaxInt64), ms.CalculateCombinations())
}

func TestMarkushStructure_CalculateCombinations_Empty(t *testing.T) {
	ms, _ := NewMarkushStructure("Test", "Core", 1)
	assert.Equal(t, int64(0), ms.CalculateCombinations())
}

func TestMarkushStructure_MatchesMolecule_ExactMatch(t *testing.T) {
	ms, _ := NewMarkushStructure("Test", "Core", 1)
	ms.PreferredExamples = []string{"C1CCCCC1"}

	matched, conf, err := ms.MatchesMolecule("C1CCCCC1", nil)
	assert.NoError(t, err)
	assert.True(t, matched)
	assert.Equal(t, 1.0, conf)
}

func TestMarkushStructure_MatchesMolecule_NoMatch(t *testing.T) {
	ms, _ := NewMarkushStructure("Test", "Core", 1)
	matched, conf, err := ms.MatchesMolecule("C1CCCCC1", nil)
	assert.NoError(t, err)
	assert.False(t, matched)
	assert.Equal(t, 0.0, conf)
}

func TestMarkushStructure_MatchesMolecule_EmptySMILES(t *testing.T) {
	ms, _ := NewMarkushStructure("Test", "Core", 1)
	_, _, err := ms.MatchesMolecule("", nil)
	assert.Error(t, err)
}

func TestMarkushStructure_MatchesMolecule_WithMatcher(t *testing.T) {
	ms, _ := NewMarkushStructure("Test", "Core", 1)
	vp := VariablePosition{Symbol: "R1", Substituents: []Substituent{{ID: "S1", Name: "Me", Type: SubstituentTypeAlkyl}}}
	ms.Positions = []VariablePosition{vp}

	mockMatcher := new(MockMarkushMatcher)
	mockMatcher.On("IsSubstructure", "Core", "Molecule").Return(true, nil)
	mockMatcher.On("ExtractSubstituents", "Core", "Molecule").Return(map[string]string{"R1": "Methyl"}, nil)
	mockMatcher.On("MatchSubstituent", "Methyl", vp.Substituents).Return(true, "S1", nil)

	matched, conf, err := ms.MatchesMolecule("Molecule", mockMatcher)
	assert.NoError(t, err)
	assert.True(t, matched)
	assert.Equal(t, 1.0, conf)
}

func TestMarkushStructure_GetPosition_Found(t *testing.T) {
	ms, _ := NewMarkushStructure("Test", "Core", 1)
	vp := VariablePosition{Symbol: "R1", Substituents: []Substituent{{ID: "S1", Name: "H", Type: SubstituentTypeHydrogen}}}
	ms.Positions = []VariablePosition{vp}

	p, found := ms.GetPosition("R1")
	assert.True(t, found)
	assert.Equal(t, "R1", p.Symbol)
}

func TestMarkushStructure_GetPosition_NotFound(t *testing.T) {
	ms, _ := NewMarkushStructure("Test", "Core", 1)
	_, found := ms.GetPosition("R1")
	assert.False(t, found)
}

func TestMarkushStructure_PositionCount(t *testing.T) {
	ms, _ := NewMarkushStructure("Test", "Core", 1)
	vp := VariablePosition{Symbol: "R1", Substituents: []Substituent{{ID: "S1", Name: "H", Type: SubstituentTypeHydrogen}}}
	ms.Positions = []VariablePosition{vp}
	assert.Equal(t, 1, ms.PositionCount())
}

//Personal.AI order the ending
