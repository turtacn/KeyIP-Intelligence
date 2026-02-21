package patent

import (
	"fmt"
	"math"
	"testing"
)

func TestSubstituentType_String(t *testing.T) {
	tests := []struct {
		st   SubstituentType
		want string
	}{
		{SubstituentTypeAlkyl, "Alkyl"},
		{SubstituentTypeAryl, "Aryl"},
		{SubstituentTypeHalogen, "Halogen"},
		{SubstituentTypeUnknown, "Unknown"},
	}
	for _, tt := range tests {
		if got := tt.st.String(); got != tt.want {
			t.Errorf("SubstituentType.String() = %v, want %v", got, tt.want)
		}
	}
}

func TestSubstituent_Validate(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		s := Substituent{ID: "S1", Name: "Methyl", Type: SubstituentTypeAlkyl}
		if err := s.Validate(); err != nil {
			t.Errorf("Expected valid, got %v", err)
		}
	})

	t.Run("Invalid ID", func(t *testing.T) {
		s := Substituent{ID: "", Name: "Methyl", Type: SubstituentTypeAlkyl}
		if err := s.Validate(); err == nil {
			t.Error("Expected error for empty ID")
		}
	})

	t.Run("Invalid Carbon Range", func(t *testing.T) {
		s := Substituent{ID: "S1", Name: "Methyl", Type: SubstituentTypeAlkyl, CarbonRange: [2]int{5, 1}}
		if err := s.Validate(); err == nil {
			t.Error("Expected error for invalid carbon range")
		}
	})
}

func TestVariablePosition_Validate(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		vp := VariablePosition{
			Symbol: "R1",
			Substituents: []Substituent{
				{ID: "S1", Name: "H", Type: SubstituentTypeHydrogen},
			},
		}
		if err := vp.Validate(); err != nil {
			t.Errorf("Expected valid, got %v", err)
		}
	})

	t.Run("Empty Symbol", func(t *testing.T) {
		vp := VariablePosition{Symbol: ""}
		if err := vp.Validate(); err == nil {
			t.Error("Expected error for empty symbol")
		}
	})

	t.Run("No Substituents Non-Optional", func(t *testing.T) {
		vp := VariablePosition{Symbol: "R1", IsOptional: false}
		if err := vp.Validate(); err == nil {
			t.Error("Expected error for no substituents in non-optional position")
		}
	})

	t.Run("No Substituents Optional", func(t *testing.T) {
		vp := VariablePosition{Symbol: "R1", IsOptional: true}
		if err := vp.Validate(); err != nil {
			t.Errorf("Expected valid, got %v", err)
		}
	})
}

func TestMarkushStructure_CalculateCombinations(t *testing.T) {
	t.Run("Single Position", func(t *testing.T) {
		ms, _ := NewMarkushStructure("T1", "C", 1)
		ms.AddPosition(VariablePosition{
			Symbol: "R1",
			Substituents: []Substituent{
				{ID: "S1", Name: "A", Type: SubstituentTypeAlkyl},
				{ID: "S2", Name: "B", Type: SubstituentTypeAlkyl},
				{ID: "S3", Name: "C", Type: SubstituentTypeAlkyl},
			},
		})
		if count := ms.CalculateCombinations(); count != 3 {
			t.Errorf("Expected 3, got %d", count)
		}
	})

	t.Run("Multiple Positions", func(t *testing.T) {
		ms, _ := NewMarkushStructure("T1", "C", 1)
		ms.AddPosition(VariablePosition{
			Symbol: "R1",
			Substituents: []Substituent{
				{ID: "S1", Name: "A", Type: SubstituentTypeAlkyl},
				{ID: "S2", Name: "B", Type: SubstituentTypeAlkyl},
				{ID: "S3", Name: "C", Type: SubstituentTypeAlkyl},
			},
		})
		ms.AddPosition(VariablePosition{
			Symbol: "R2",
			Substituents: []Substituent{
				{ID: "S4", Name: "D", Type: SubstituentTypeAlkyl},
				{ID: "S5", Name: "E", Type: SubstituentTypeAlkyl},
				{ID: "S6", Name: "F", Type: SubstituentTypeAlkyl},
				{ID: "S7", Name: "G", Type: SubstituentTypeAlkyl},
			},
		})
		if count := ms.CalculateCombinations(); count != 12 {
			t.Errorf("Expected 12, got %d", count)
		}
	})

	t.Run("Optional Position", func(t *testing.T) {
		ms, _ := NewMarkushStructure("T1", "C", 1)
		ms.AddPosition(VariablePosition{
			Symbol: "R1",
			Substituents: []Substituent{
				{ID: "S1", Name: "A", Type: SubstituentTypeAlkyl},
				{ID: "S2", Name: "B", Type: SubstituentTypeAlkyl},
				{ID: "S3", Name: "C", Type: SubstituentTypeAlkyl},
			},
		})
		ms.AddPosition(VariablePosition{
			Symbol: "R2",
			Substituents: []Substituent{
				{ID: "S4", Name: "D", Type: SubstituentTypeAlkyl},
				{ID: "S5", Name: "E", Type: SubstituentTypeAlkyl},
				{ID: "S6", Name: "F", Type: SubstituentTypeAlkyl},
				{ID: "S7", Name: "G", Type: SubstituentTypeAlkyl},
			},
			IsOptional: true,
		})
		// R1(3) * R2(4+1) = 15
		if count := ms.CalculateCombinations(); count != 15 {
			t.Errorf("Expected 15, got %d", count)
		}
	})

	t.Run("Repeat Range", func(t *testing.T) {
		ms, _ := NewMarkushStructure("T1", "C", 1)
		ms.AddPosition(VariablePosition{
			Symbol: "R1",
			Substituents: []Substituent{
				{ID: "S1", Name: "A", Type: SubstituentTypeAlkyl},
				{ID: "S2", Name: "B", Type: SubstituentTypeAlkyl},
			},
			RepeatRange: [2]int{1, 3}, // 1, 2, 3 -> 3 options
		})
		// 2 substituents * 3 repeat options = 6
		if count := ms.CalculateCombinations(); count != 6 {
			t.Errorf("Expected 6, got %d", count)
		}
	})

	t.Run("Overflow", func(t *testing.T) {
		ms, _ := NewMarkushStructure("T1", "C", 1)
		for i := 0; i < 20; i++ {
			subs := make([]Substituent, 100)
			for j := 0; j < 100; j++ {
				subs[j] = Substituent{ID: fmt.Sprintf("S%d", j), Name: "A", Type: SubstituentTypeAlkyl}
			}
			ms.AddPosition(VariablePosition{
				Symbol:       string(rune('A' + i)),
				Substituents: subs,
			})
		}
		// 100^20 is definitely > MaxInt64
		count := ms.CalculateCombinations()
		if count != math.MaxInt64 {
			t.Errorf("Expected MaxInt64, got %d", count)
		}
	})
}

func TestMarkushStructure_MatchesMolecule(t *testing.T) {
	ms, _ := NewMarkushStructure("T1", "C", 1)
	ms.PreferredExamples = []string{"C1=CC=C(C=C1)N"}

	t.Run("Exact Match", func(t *testing.T) {
		match, conf, err := ms.MatchesMolecule("C1=CC=C(C=C1)N")
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if !match || conf != 1.0 {
			t.Errorf("Expected match with confidence 1.0")
		}
	})

	t.Run("No Match", func(t *testing.T) {
		match, _, _ := ms.MatchesMolecule("CCCC")
		if match {
			t.Error("Expected no match")
		}
	})
}

func TestParseMarkushFromText(t *testing.T) {
	t.Run("Chinese", func(t *testing.T) {
		text := "其中通式(I)中，R1选自C1-C6烷基..."
		ms, err := ParseMarkushFromText(text)
		if err != nil {
			t.Errorf("Parse failed: %v", err)
		}
		if ms.Name == "" {
			t.Error("Parsed Markush should have a name")
		}
	})

	t.Run("English", func(t *testing.T) {
		text := "wherein R1 is selected from alkyl..."
		ms, err := ParseMarkushFromText(text)
		if err != nil {
			t.Errorf("Parse failed: %v", err)
		}
		if ms.PositionCount() == 0 {
			t.Error("Expected at least one position")
		}
	})

	t.Run("Empty Text", func(t *testing.T) {
		_, err := ParseMarkushFromText("")
		if err == nil {
			t.Error("Expected error for empty text")
		}
	})
}

//Personal.AI order the ending
