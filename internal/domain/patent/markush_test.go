package patent

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSubstituentType_String(t *testing.T) {
	tests := []struct {
		val      SubstituentType
		expected string
	}{
		{SubstituentTypeAlkyl, "Alkyl"},
		{SubstituentTypeAryl, "Aryl"},
		{SubstituentTypeHeteroaryl, "Heteroaryl"},
		{SubstituentTypeHalogen, "Halogen"},
		{SubstituentTypeAlkoxy, "Alkoxy"},
		{SubstituentTypeAmino, "Amino"},
		{SubstituentTypeCyano, "Cyano"},
		{SubstituentTypeHydrogen, "Hydrogen"},
		{SubstituentTypeCustom, "Custom"},
		{SubstituentTypeUnknown, "Unknown"},
		{SubstituentType(99), "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.val.String())
		})
	}
}

func TestSubstituentType_IsValid(t *testing.T) {
	assert.True(t, SubstituentTypeAlkyl.IsValid())
	assert.True(t, SubstituentTypeCustom.IsValid())
	assert.False(t, SubstituentTypeUnknown.IsValid())
	assert.False(t, SubstituentType(99).IsValid())
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
	s := Substituent{ID: "S1", Name: "Methyl", Type: SubstituentTypeUnknown}
	assert.Error(t, s.Validate())
}

func TestSubstituent_Validate_InvalidCarbonRange(t *testing.T) {
	s := Substituent{ID: "S1", Name: "Methyl", Type: SubstituentTypeAlkyl, CarbonRange: [2]int{6, 1}}
	assert.Error(t, s.Validate())
}

func TestVariablePosition_Validate_Success(t *testing.T) {
	vp := VariablePosition{
		Symbol: "R1",
		Substituents: []Substituent{
			{ID: "S1", Name: "Methyl", Type: SubstituentTypeAlkyl},
		},
	}
	assert.NoError(t, vp.Validate())
}

func TestVariablePosition_Validate_EmptySymbol(t *testing.T) {
	vp := VariablePosition{Symbol: ""}
	assert.Error(t, vp.Validate())
}

func TestVariablePosition_Validate_NoSubstituents(t *testing.T) {
	vp := VariablePosition{Symbol: "R1", IsOptional: false}
	assert.Error(t, vp.Validate())
}

func TestVariablePosition_Validate_OptionalNoSubstituents(t *testing.T) {
	vp := VariablePosition{Symbol: "R1", IsOptional: true}
	// Currently Validate allows 0 substituents if IsOptional?
	// The code says: `if !vp.IsOptional && len(vp.Substituents) == 0`
	// So optional can have 0? Let's assume yes (e.g. only "null" option implied).
	assert.NoError(t, vp.Validate())
}

func TestVariablePosition_SubstituentCount(t *testing.T) {
	vp := VariablePosition{
		Symbol:       "R1",
		Substituents: []Substituent{{ID: "1"}, {ID: "2"}},
	}
	assert.Equal(t, 2, vp.SubstituentCount())
}

func TestNewMarkushStructure_Success(t *testing.T) {
	ms, err := NewMarkushStructure("M1", "C1=CC=C[R1]", 1)
	assert.NoError(t, err)
	assert.NotNil(t, ms)
	assert.Equal(t, "M1", ms.Name)
	assert.Equal(t, "C1=CC=C[R1]", ms.CoreStructure)
	assert.Equal(t, 1, ms.ClaimNumber)
	assert.False(t, ms.CreatedAt.IsZero())
}

func TestNewMarkushStructure_EmptyName(t *testing.T) {
	_, err := NewMarkushStructure("", "Core", 1)
	assert.Error(t, err)
}

func TestNewMarkushStructure_EmptyCoreStructure(t *testing.T) {
	_, err := NewMarkushStructure("M1", "", 1)
	assert.Error(t, err)
}

func TestNewMarkushStructure_InvalidClaimNumber(t *testing.T) {
	_, err := NewMarkushStructure("M1", "Core", 0)
	assert.Error(t, err)
}

func TestMarkushStructure_AddPosition_Success(t *testing.T) {
	ms, _ := NewMarkushStructure("M1", "Core", 1)
	vp := VariablePosition{Symbol: "R1", Substituents: []Substituent{{ID: "S1", Name: "A", Type: SubstituentTypeAlkyl}}}
	err := ms.AddPosition(vp)
	assert.NoError(t, err)
	assert.Len(t, ms.Positions, 1)
}

func TestMarkushStructure_AddPosition_DuplicateSymbol(t *testing.T) {
	ms, _ := NewMarkushStructure("M1", "Core", 1)
	vp := VariablePosition{Symbol: "R1", Substituents: []Substituent{{ID: "S1", Name: "A", Type: SubstituentTypeAlkyl}}}
	ms.AddPosition(vp)
	err := ms.AddPosition(vp)
	assert.Error(t, err)
}

func TestMarkushStructure_AddConstraint_Success(t *testing.T) {
	ms, _ := NewMarkushStructure("M1", "Core", 1)
	err := ms.AddConstraint("R1 != H")
	assert.NoError(t, err)
	assert.Len(t, ms.Constraints, 1)
}

func TestMarkushStructure_AddConstraint_Empty(t *testing.T) {
	ms, _ := NewMarkushStructure("M1", "Core", 1)
	err := ms.AddConstraint("")
	assert.Error(t, err)
}

func TestMarkushStructure_Validate_Success(t *testing.T) {
	ms, _ := NewMarkushStructure("M1", "Core", 1)
	vp := VariablePosition{Symbol: "R1", Substituents: []Substituent{{ID: "S1", Name: "A", Type: SubstituentTypeAlkyl}}}
	ms.AddPosition(vp)
	assert.NoError(t, ms.Validate())
}

func TestMarkushStructure_Validate_NoPositions(t *testing.T) {
	ms, _ := NewMarkushStructure("M1", "Core", 1)
	assert.Error(t, ms.Validate())
}

func TestMarkushStructure_Validate_InvalidPosition(t *testing.T) {
	ms, _ := NewMarkushStructure("M1", "Core", 1)
	vp := VariablePosition{Symbol: "", Substituents: []Substituent{{ID: "S1", Name: "A", Type: SubstituentTypeAlkyl}}}
	// We bypass AddPosition validation by appending directly if we want to test Validate catching it
	ms.Positions = append(ms.Positions, vp)
	assert.Error(t, ms.Validate())
}

func TestMarkushStructure_Validate_DuplicateSymbols(t *testing.T) {
	ms, _ := NewMarkushStructure("M1", "Core", 1)
	vp := VariablePosition{Symbol: "R1", Substituents: []Substituent{{ID: "S1", Name: "A", Type: SubstituentTypeAlkyl}}}
	ms.Positions = append(ms.Positions, vp, vp)
	assert.Error(t, ms.Validate())
}

func TestMarkushStructure_Validate_BrokenLinkedPosition(t *testing.T) {
	ms, _ := NewMarkushStructure("M1", "Core", 1)
	vp := VariablePosition{
		Symbol:          "R1",
		Substituents:    []Substituent{{ID: "S1", Name: "A", Type: SubstituentTypeAlkyl}},
		LinkedPositions: []string{"R99"},
	}
	ms.Positions = append(ms.Positions, vp)
	assert.Error(t, ms.Validate())
}

func TestMarkushStructure_CalculateCombinations_SinglePosition(t *testing.T) {
	ms, _ := NewMarkushStructure("M1", "Core", 1)
	vp := VariablePosition{
		Symbol: "R1",
		Substituents: []Substituent{
			{ID: "S1", Name: "A", Type: SubstituentTypeAlkyl},
			{ID: "S2", Name: "B", Type: SubstituentTypeAlkyl},
			{ID: "S3", Name: "C", Type: SubstituentTypeAlkyl},
		},
	}
	ms.AddPosition(vp)
	assert.Equal(t, int64(3), ms.CalculateCombinations())
}

func TestMarkushStructure_CalculateCombinations_MultiplePositions(t *testing.T) {
	ms, _ := NewMarkushStructure("M1", "Core", 1)
	vp1 := VariablePosition{
		Symbol: "R1",
		Substituents: []Substituent{
			{ID: "S1", Name: "A", Type: SubstituentTypeAlkyl},
			{ID: "S2", Name: "B", Type: SubstituentTypeAlkyl},
			{ID: "S3", Name: "C", Type: SubstituentTypeAlkyl},
		},
	}
	vp2 := VariablePosition{
		Symbol: "R2",
		Substituents: []Substituent{
			{ID: "S4", Name: "D", Type: SubstituentTypeAlkyl},
			{ID: "S5", Name: "E", Type: SubstituentTypeAlkyl},
			{ID: "S6", Name: "F", Type: SubstituentTypeAlkyl},
			{ID: "S7", Name: "G", Type: SubstituentTypeAlkyl},
		},
	}
	ms.AddPosition(vp1)
	ms.AddPosition(vp2)
	assert.Equal(t, int64(12), ms.CalculateCombinations())
}

func TestMarkushStructure_CalculateCombinations_OptionalPosition(t *testing.T) {
	ms, _ := NewMarkushStructure("M1", "Core", 1)
	vp1 := VariablePosition{
		Symbol: "R1",
		Substituents: []Substituent{
			{ID: "S1", Name: "A", Type: SubstituentTypeAlkyl},
			{ID: "S2", Name: "B", Type: SubstituentTypeAlkyl},
			{ID: "S3", Name: "C", Type: SubstituentTypeAlkyl},
		},
	}
	vp2 := VariablePosition{
		Symbol:     "R2",
		IsOptional: true,
		Substituents: []Substituent{
			{ID: "S4", Name: "D", Type: SubstituentTypeAlkyl},
			{ID: "S5", Name: "E", Type: SubstituentTypeAlkyl},
			{ID: "S6", Name: "F", Type: SubstituentTypeAlkyl},
			{ID: "S7", Name: "G", Type: SubstituentTypeAlkyl},
		},
	}
	ms.AddPosition(vp1)
	ms.AddPosition(vp2)
	// R1: 3
	// R2: 4 + 1 (optional) = 5
	// Total: 3 * 5 = 15
	assert.Equal(t, int64(15), ms.CalculateCombinations())
}

func TestMarkushStructure_CalculateCombinations_RepeatRange(t *testing.T) {
	ms, _ := NewMarkushStructure("M1", "Core", 1)
	vp := VariablePosition{
		Symbol:      "n",
		RepeatRange: [2]int{0, 3}, // 0, 1, 2, 3 => 4 variations
		Substituents: []Substituent{
			{ID: "S1", Name: "A", Type: SubstituentTypeAlkyl}, // Substituents here mean what is being repeated?
			// The logic in markush.go multiplies count by range span.
			// count = 1 (substituent)
			// range = 3 - 0 + 1 = 4
			// total = 1 * 4 = 4
		},
	}
	ms.AddPosition(vp)
	assert.Equal(t, int64(4), ms.CalculateCombinations())
}

func TestMarkushStructure_CalculateCombinations_Overflow(t *testing.T) {
	ms, _ := NewMarkushStructure("M1", "Core", 1)
	// Create positions with large counts
	vp := VariablePosition{Symbol: "R1", Substituents: make([]Substituent, 1000)}
	// Add many times to cause overflow
	for i := 0; i < 25; i++ {
		// Just simulate by reusing vp but bypassing AddPosition symbol check or creating new symbol
		// Actually calculating overflow with 1000^25 is hard, max int64 is ~9e18
		// 1000^7 is 1e21, already overflows.
	}

	// Let's modify ms.Positions directly to avoid symbol uniqueness check for test
	for i := 0; i < 10; i++ {
		ms.Positions = append(ms.Positions, vp)
	}
	// 1000^10 = 1e30 > MaxInt64
	assert.Equal(t, int64(math.MaxInt64), ms.CalculateCombinations())
}

func TestMarkushStructure_CalculateCombinations_Empty(t *testing.T) {
	ms, _ := NewMarkushStructure("M1", "Core", 1)
	assert.Equal(t, int64(0), ms.CalculateCombinations())
}

func TestMarkushStructure_MatchesMolecule_ExactMatch(t *testing.T) {
	ms, _ := NewMarkushStructure("M1", "Core", 1)
	ms.PreferredExamples = []string{"C1=CC=CC=C1"}
	match, confidence, err := ms.MatchesMolecule("C1=CC=CC=C1")
	assert.NoError(t, err)
	assert.True(t, match)
	assert.Equal(t, 1.0, confidence)
}

func TestMarkushStructure_MatchesMolecule_NoMatch(t *testing.T) {
	ms, _ := NewMarkushStructure("M1", "Core", 1)
	match, confidence, err := ms.MatchesMolecule("C1=CC=CC=C1")
	assert.NoError(t, err)
	assert.False(t, match)
	assert.Equal(t, 0.0, confidence)
}

func TestMarkushStructure_MatchesMolecule_EmptySMILES(t *testing.T) {
	ms, _ := NewMarkushStructure("M1", "Core", 1)
	_, _, err := ms.MatchesMolecule("")
	assert.Error(t, err)
}

func TestMarkushStructure_GetPosition_Found(t *testing.T) {
	ms, _ := NewMarkushStructure("M1", "Core", 1)
	ms.AddPosition(VariablePosition{Symbol: "R1", Substituents: []Substituent{{ID: "S1", Name: "A", Type: SubstituentTypeAlkyl}}})
	pos, found := ms.GetPosition("R1")
	assert.True(t, found)
	assert.Equal(t, "R1", pos.Symbol)
}

func TestMarkushStructure_GetPosition_NotFound(t *testing.T) {
	ms, _ := NewMarkushStructure("M1", "Core", 1)
	_, found := ms.GetPosition("R1")
	assert.False(t, found)
}

func TestMarkushStructure_PositionCount(t *testing.T) {
	ms, _ := NewMarkushStructure("M1", "Core", 1)
	ms.AddPosition(VariablePosition{Symbol: "R1", Substituents: []Substituent{{ID: "S1", Name: "A", Type: SubstituentTypeAlkyl}}})
	assert.Equal(t, 1, ms.PositionCount())
}

func TestParseMarkushFromText_ChinesePatent(t *testing.T) {
	text := "通式(I)其中R1选自..."
	ms, err := ParseMarkushFromText(text)
	assert.NoError(t, err)
	assert.NotNil(t, ms)
}

func TestParseMarkushFromText_EnglishPatent(t *testing.T) {
	text := "Formula (I) wherein R1 is..."
	ms, err := ParseMarkushFromText(text)
	assert.NoError(t, err)
	assert.NotNil(t, ms)
}

func TestParseMarkushFromText_EmptyText(t *testing.T) {
	_, err := ParseMarkushFromText("")
	assert.Error(t, err)
}

func TestParseMarkushFromText_NoMarkush(t *testing.T) {
	_, err := ParseMarkushFromText("Some random text without keywords")
	assert.Error(t, err)
}

func TestMoleculeMatchResult_Fields(t *testing.T) {
	res := MoleculeMatchResult{
		MarkushID: "M1",
		IsMatch:   true,
	}
	assert.Equal(t, "M1", res.MarkushID)
	assert.True(t, res.IsMatch)
}

func TestMarkushCoverageAnalysis_Fields(t *testing.T) {
	res := MarkushCoverageAnalysis{
		MarkushID:    "M1",
		CoverageRate: 0.5,
	}
	assert.Equal(t, "M1", res.MarkushID)
	assert.Equal(t, 0.5, res.CoverageRate)
}
