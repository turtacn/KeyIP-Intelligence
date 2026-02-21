package patent

import (
	"strings"
	"testing"

	pkgerrors "github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

func TestClaimType_String(t *testing.T) {
	tests := []struct {
		ct   ClaimType
		want string
	}{
		{ClaimTypeIndependent, "Independent"},
		{ClaimTypeDependent, "Dependent"},
		{ClaimTypeUnknown, "Unknown"},
		{ClaimType(255), "Unknown"},
	}
	for _, tt := range tests {
		if got := tt.ct.String(); got != tt.want {
			t.Errorf("ClaimType.String() = %v, want %v", got, tt.want)
		}
	}
}

func TestClaimType_IsValid(t *testing.T) {
	tests := []struct {
		ct   ClaimType
		want bool
	}{
		{ClaimTypeIndependent, true},
		{ClaimTypeDependent, true},
		{ClaimTypeUnknown, false},
		{ClaimType(255), false},
	}
	for _, tt := range tests {
		if got := tt.ct.IsValid(); got != tt.want {
			t.Errorf("ClaimType.IsValid() = %v, want %v", got, tt.want)
		}
	}
}

func TestClaimCategory_String(t *testing.T) {
	tests := []struct {
		cc   ClaimCategory
		want string
	}{
		{ClaimCategoryProduct, "Product"},
		{ClaimCategoryMethod, "Method"},
		{ClaimCategoryUse, "Use"},
		{ClaimCategoryUnknown, "Unknown"},
	}
	for _, tt := range tests {
		if got := tt.cc.String(); got != tt.want {
			t.Errorf("ClaimCategory.String() = %v, want %v", got, tt.want)
		}
	}
}

func TestClaimCategory_IsValid(t *testing.T) {
	tests := []struct {
		cc   ClaimCategory
		want bool
	}{
		{ClaimCategoryProduct, true},
		{ClaimCategoryMethod, true},
		{ClaimCategoryUse, true},
		{ClaimCategoryUnknown, false},
	}
	for _, tt := range tests {
		if got := tt.cc.IsValid(); got != tt.want {
			t.Errorf("ClaimCategory.IsValid() = %v, want %v", got, tt.want)
		}
	}
}

func TestClaimElementType_String(t *testing.T) {
	tests := []struct {
		cet  ClaimElementType
		want string
	}{
		{StructuralElement, "Structural"},
		{FunctionalElement, "Functional"},
		{ParameterElement, "Parameter"},
		{ProcessElement, "Process"},
		{ClaimElementTypeUnknown, "Unknown"},
	}
	for _, tt := range tests {
		if got := tt.cet.String(); got != tt.want {
			t.Errorf("ClaimElementType.String() = %v, want %v", got, tt.want)
		}
	}
}

func TestClaimElementType_IsValid(t *testing.T) {
	tests := []struct {
		cet  ClaimElementType
		want bool
	}{
		{StructuralElement, true},
		{FunctionalElement, true},
		{ParameterElement, true},
		{ProcessElement, true},
		{ClaimElementTypeUnknown, false},
	}
	for _, tt := range tests {
		if got := tt.cet.IsValid(); got != tt.want {
			t.Errorf("ClaimElementType.IsValid() = %v, want %v", got, tt.want)
		}
	}
}

func TestNewClaim(t *testing.T) {
	text := "An organic light-emitting compound having a structure represented by the following formula (I)..."
	shortText := "Too short"
	longText := strings.Repeat("a", 50001)

	t.Run("Success Independent", func(t *testing.T) {
		c, err := NewClaim(1, text, ClaimTypeIndependent, ClaimCategoryProduct)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if c.Number != 1 || c.Text != text || c.Type != ClaimTypeIndependent || c.Category != ClaimCategoryProduct {
			t.Errorf("Claim fields mismatch")
		}
	})

	t.Run("Success Dependent", func(t *testing.T) {
		c, err := NewClaim(2, text, ClaimTypeDependent, ClaimCategoryProduct)
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}
		if c.Number != 2 || c.Type != ClaimTypeDependent {
			t.Errorf("Claim fields mismatch")
		}
	})

	t.Run("Invalid Number Zero", func(t *testing.T) {
		_, err := NewClaim(0, text, ClaimTypeIndependent, ClaimCategoryProduct)
		if err == nil {
			t.Error("Expected error for number 0")
		}
	})

	t.Run("Invalid Number Negative", func(t *testing.T) {
		_, err := NewClaim(-1, text, ClaimTypeIndependent, ClaimCategoryProduct)
		if err == nil {
			t.Error("Expected error for negative number")
		}
	})

	t.Run("Empty Text", func(t *testing.T) {
		_, err := NewClaim(1, "          ", ClaimTypeIndependent, ClaimCategoryProduct)
		if err == nil {
			t.Error("Expected error for empty text")
		}
	})

	t.Run("Text Too Short", func(t *testing.T) {
		_, err := NewClaim(1, shortText, ClaimTypeIndependent, ClaimCategoryProduct)
		if err == nil {
			t.Error("Expected error for short text")
		}
	})

	t.Run("Text Too Long", func(t *testing.T) {
		_, err := NewClaim(1, longText, ClaimTypeIndependent, ClaimCategoryProduct)
		if err == nil {
			t.Error("Expected error for long text")
		}
	})

	t.Run("Invalid ClaimType", func(t *testing.T) {
		_, err := NewClaim(1, text, ClaimType(0), ClaimCategoryProduct)
		if err == nil {
			t.Error("Expected error for invalid claim type")
		}
	})

	t.Run("Invalid Category", func(t *testing.T) {
		_, err := NewClaim(1, text, ClaimTypeIndependent, ClaimCategory(0))
		if err == nil {
			t.Error("Expected error for invalid category")
		}
	})
}

func TestClaim_Validate(t *testing.T) {
	text := "An organic light-emitting compound having a structure represented by the following formula (I)..."

	t.Run("Independent Valid", func(t *testing.T) {
		c, _ := NewClaim(1, text, ClaimTypeIndependent, ClaimCategoryProduct)
		if err := c.Validate(); err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
	})

	t.Run("Dependent Valid", func(t *testing.T) {
		c, _ := NewClaim(2, text, ClaimTypeDependent, ClaimCategoryProduct)
		c.DependsOn = []int{1}
		if err := c.Validate(); err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
	})

	t.Run("Dependent Missing Deps", func(t *testing.T) {
		c, _ := NewClaim(2, text, ClaimTypeDependent, ClaimCategoryProduct)
		if err := c.Validate(); err == nil {
			t.Error("Expected error for missing dependencies in dependent claim")
		}
	})

	t.Run("Independent With Deps", func(t *testing.T) {
		c, _ := NewClaim(1, text, ClaimTypeIndependent, ClaimCategoryProduct)
		c.DependsOn = []int{0} // Force a dependency
		if err := c.Validate(); err == nil {
			t.Error("Expected error for dependencies in independent claim")
		}
	})

	t.Run("Depends On Self", func(t *testing.T) {
		c, _ := NewClaim(1, text, ClaimTypeDependent, ClaimCategoryProduct)
		c.DependsOn = []int{1}
		if err := c.Validate(); err == nil {
			t.Error("Expected error for self dependency")
		}
	})

	t.Run("Depends On Forward Ref", func(t *testing.T) {
		c, _ := NewClaim(1, text, ClaimTypeDependent, ClaimCategoryProduct)
		c.DependsOn = []int{2}
		if err := c.Validate(); err == nil {
			t.Error("Expected error for forward dependency")
		}
	})

	t.Run("Essential Elements Check", func(t *testing.T) {
		c, _ := NewClaim(1, text, ClaimTypeIndependent, ClaimCategoryProduct)
		c.AddElement(ClaimElement{ID: "E1", Text: "Element 1", Type: StructuralElement, IsEssential: false})
		if err := c.Validate(); err == nil {
			t.Error("Expected error for missing essential element")
		}

		c.Elements[0].IsEssential = true
		if err := c.Validate(); err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
	})
}

func TestClaim_SetDependencies(t *testing.T) {
	text := "Test claim text..."
	t.Run("Success", func(t *testing.T) {
		c, _ := NewClaim(3, text, ClaimTypeDependent, ClaimCategoryProduct)
		err := c.SetDependencies([]int{1, 2})
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if len(c.DependsOn) != 2 {
			t.Errorf("Expected 2 dependencies, got %d", len(c.DependsOn))
		}
	})

	t.Run("Independent Claim", func(t *testing.T) {
		c, _ := NewClaim(1, text, ClaimTypeIndependent, ClaimCategoryProduct)
		err := c.SetDependencies([]int{1})
		if err == nil {
			t.Error("Expected error for independent claim")
		}
	})

	t.Run("Invalid Number", func(t *testing.T) {
		c, _ := NewClaim(2, text, ClaimTypeDependent, ClaimCategoryProduct)
		err := c.SetDependencies([]int{0})
		if err == nil {
			t.Error("Expected error for dependency 0")
		}
	})

	t.Run("Forward Reference", func(t *testing.T) {
		c, _ := NewClaim(2, text, ClaimTypeDependent, ClaimCategoryProduct)
		err := c.SetDependencies([]int{3})
		if err == nil {
			t.Error("Expected error for forward reference")
		}
	})

	t.Run("Duplicates", func(t *testing.T) {
		c, _ := NewClaim(3, text, ClaimTypeDependent, ClaimCategoryProduct)
		err := c.SetDependencies([]int{1, 1})
		if err == nil {
			t.Error("Expected error for duplicates")
		}
	})
}

func TestClaim_AddElement(t *testing.T) {
	text := "Test claim text..."
	c, _ := NewClaim(1, text, ClaimTypeIndependent, ClaimCategoryProduct)

	t.Run("Success", func(t *testing.T) {
		err := c.AddElement(ClaimElement{ID: "E1", Text: "Feature 1", Type: StructuralElement})
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
	})

	t.Run("Duplicate ID", func(t *testing.T) {
		err := c.AddElement(ClaimElement{ID: "E1", Text: "Feature 1", Type: StructuralElement})
		if err == nil {
			t.Error("Expected error for duplicate ID")
		}
	})

	t.Run("Empty ID", func(t *testing.T) {
		err := c.AddElement(ClaimElement{ID: "", Text: "Feature 2", Type: StructuralElement})
		if err == nil {
			t.Error("Expected error for empty ID")
		}
	})

	t.Run("Empty Text", func(t *testing.T) {
		err := c.AddElement(ClaimElement{ID: "E2", Text: "", Type: StructuralElement})
		if err == nil {
			t.Error("Expected error for empty text")
		}
	})

	t.Run("Invalid Type", func(t *testing.T) {
		err := c.AddElement(ClaimElement{ID: "E3", Text: "Feature 3", Type: ClaimElementType(0)})
		if err == nil {
			t.Error("Expected error for invalid type")
		}
	})
}

func TestClaim_EssentialElements(t *testing.T) {
	c := &Claim{
		Elements: []ClaimElement{
			{ID: "E1", IsEssential: true},
			{ID: "E2", IsEssential: false},
			{ID: "E3", IsEssential: true},
		},
	}
	essential := c.EssentialElements()
	if len(essential) != 2 {
		t.Errorf("Expected 2 essential elements, got %d", len(essential))
	}
	// Verify it's a copy
	essential[0].ID = "Modified"
	if c.Elements[0].ID == "Modified" {
		t.Error("EssentialElements returned a pointer/reference to original data")
	}
}

func TestClaim_HasMarkushStructure(t *testing.T) {
	c := &Claim{}
	if c.HasMarkushStructure() {
		t.Error("Expected false for empty MarkushStructures")
	}
	c.MarkushStructures = []string{"M1"}
	if !c.HasMarkushStructure() {
		t.Error("Expected true for non-empty MarkushStructures")
	}
}

func TestClaim_ContainsMoleculeReference(t *testing.T) {
	c := &Claim{
		Elements: []ClaimElement{
			{ID: "E1", MoleculeRef: ""},
		},
	}
	if c.ContainsMoleculeReference() {
		t.Error("Expected false")
	}
	c.Elements[0].MoleculeRef = "InChIKey=..."
	if !c.ContainsMoleculeReference() {
		t.Error("Expected true")
	}
}

func TestClaimSet(t *testing.T) {
	cs := ClaimSet{
		{Number: 1, Type: ClaimTypeIndependent},
		{Number: 2, Type: ClaimTypeDependent, DependsOn: []int{1}},
		{Number: 3, Type: ClaimTypeDependent, DependsOn: []int{2}},
		{Number: 4, Type: ClaimTypeDependent, DependsOn: []int{1}},
	}

	t.Run("IndependentClaims", func(t *testing.T) {
		indeps := cs.IndependentClaims()
		if len(indeps) != 1 || indeps[0].Number != 1 {
			t.Errorf("IndependentClaims failed, got %d claims", len(indeps))
		}
	})

	t.Run("DependentClaimsOf", func(t *testing.T) {
		deps := cs.DependentClaimsOf(1)
		if len(deps) != 2 { // Claims 2 and 4
			t.Errorf("DependentClaimsOf(1) failed, got %d claims", len(deps))
		}
	})

	t.Run("ClaimTree", func(t *testing.T) {
		tree := cs.ClaimTree(1)
		if len(tree) != 4 {
			t.Errorf("ClaimTree(1) failed, got %d claims", len(tree))
		}

		tree2 := cs.ClaimTree(2)
		if len(tree2) != 2 { // 2 and 3
			t.Errorf("ClaimTree(2) failed, got %d claims", len(tree2))
		}

		tree4 := cs.ClaimTree(4)
		if len(tree4) != 1 { // only 4
			t.Errorf("ClaimTree(4) failed, got %d claims", len(tree4))
		}
	})

	t.Run("Validate Success", func(t *testing.T) {
		// Need proper claims for validation to pass
		validText := "This is a valid claim text of sufficient length."
		csValid := ClaimSet{
			{Number: 1, Text: validText, Type: ClaimTypeIndependent, Category: ClaimCategoryProduct},
			{Number: 2, Text: validText, Type: ClaimTypeDependent, Category: ClaimCategoryProduct, DependsOn: []int{1}},
		}
		if err := csValid.Validate(); err != nil {
			t.Errorf("Expected Validate to succeed, got %v", err)
		}
	})

	t.Run("Validate Empty", func(t *testing.T) {
		var csEmpty ClaimSet
		if err := csEmpty.Validate(); err == nil {
			t.Error("Expected error for empty ClaimSet")
		}
	})

	t.Run("Validate No Independent", func(t *testing.T) {
		validText := "This is a valid claim text of sufficient length."
		csNoIndep := ClaimSet{
			{Number: 1, Text: validText, Type: ClaimTypeDependent, Category: ClaimCategoryProduct, DependsOn: []int{0}},
		}
		if err := csNoIndep.Validate(); err == nil {
			t.Error("Expected error for no independent claim")
		}
	})

	t.Run("Validate Duplicate Numbers", func(t *testing.T) {
		validText := "This is a valid claim text of sufficient length."
		csDup := ClaimSet{
			{Number: 1, Text: validText, Type: ClaimTypeIndependent, Category: ClaimCategoryProduct},
			{Number: 1, Text: validText, Type: ClaimTypeIndependent, Category: ClaimCategoryProduct},
		}
		if err := csDup.Validate(); err == nil {
			t.Error("Expected error for duplicate numbers")
		}
	})

	t.Run("Validate Broken Reference", func(t *testing.T) {
		validText := "This is a valid claim text of sufficient length."
		csBroken := ClaimSet{
			{Number: 1, Text: validText, Type: ClaimTypeIndependent, Category: ClaimCategoryProduct},
			{Number: 2, Text: validText, Type: ClaimTypeDependent, Category: ClaimCategoryProduct, DependsOn: []int{3}},
		}
		if err := csBroken.Validate(); err == nil {
			t.Error("Expected error for broken reference")
		}
	})

	t.Run("FindByNumber", func(t *testing.T) {
		c, found := cs.FindByNumber(3)
		if !found || c.Number != 3 {
			t.Error("FindByNumber failed to find existing claim")
		}
		_, found = cs.FindByNumber(99)
		if found {
			t.Error("FindByNumber found non-existent claim")
		}
	})
}

func TestErrors(t *testing.T) {
	// Simple check to see if we are using the right errors package
	err := pkgerrors.InvalidParam("test")
	if !pkgerrors.IsCode(err, pkgerrors.ErrCodeBadRequest) {
		t.Errorf("Error code mismatch, got %v", err)
	}
}

//Personal.AI order the ending
