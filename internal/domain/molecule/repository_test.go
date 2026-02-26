package molecule

import (
	"testing"
	"time"
)

func float64Ptr(v float64) *float64 {
	return &v
}

func timePtr(t time.Time) *time.Time {
	return &t
}

func TestMoleculeQuery_Validate(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		query   MoleculeQuery
		wantErr bool
	}{
		{
			name:    "valid_default",
			query:   MoleculeQuery{},
			wantErr: false,
		},
		{
			name: "valid_full",
			query: MoleculeQuery{
				SMILES:             "C",
				MinMolecularWeight: float64Ptr(10.0),
				MaxMolecularWeight: float64Ptr(100.0),
				Limit:              50,
				Offset:             0,
				SortBy:             "created_at",
				SortOrder:          "asc",
			},
			wantErr: false,
		},
		{
			name:    "limit_exceeds_max",
			query:   MoleculeQuery{Limit: 1001},
			wantErr: true,
		},
		{
			name:    "limit_negative",
			query:   MoleculeQuery{Limit: -1},
			wantErr: true,
		},
		{
			name:    "offset_negative",
			query:   MoleculeQuery{Offset: -1},
			wantErr: true,
		},
		{
			name:    "invalid_sort_by",
			query:   MoleculeQuery{SortBy: "invalid"},
			wantErr: true,
		},
		{
			name:    "invalid_sort_order",
			query:   MoleculeQuery{SortOrder: "invalid"},
			wantErr: true,
		},
		{
			name: "weight_range_invalid",
			query: MoleculeQuery{
				MinMolecularWeight: float64Ptr(100.0),
				MaxMolecularWeight: float64Ptr(50.0),
			},
			wantErr: true,
		},
		{
			name: "weight_negative",
			query: MoleculeQuery{
				MinMolecularWeight: float64Ptr(-1.0),
			},
			wantErr: true,
		},
		{
			name: "invalid_status",
			query: MoleculeQuery{
				Statuses: []MoleculeStatus{MoleculeStatus("invalid_status")},
			},
			wantErr: true,
		},
		{
			name: "invalid_source",
			query: MoleculeQuery{
				Sources: []MoleculeSource{MoleculeSource("invalid")},
			},
			wantErr: true,
		},
		{
			name: "property_filter_invalid_range",
			query: MoleculeQuery{
				PropertyFilters: []PropertyFilter{
					{Name: "p1", MinValue: float64Ptr(10.0), MaxValue: float64Ptr(5.0)},
				},
			},
			wantErr: true,
		},
		{
			name: "property_filter_empty_name",
			query: MoleculeQuery{
				PropertyFilters: []PropertyFilter{
					{Name: ""},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.query.Validate(); (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestMoleculeSearchResult_Methods(t *testing.T) {
	t.Parallel()

	// IsEmpty
	emptyRes := &MoleculeSearchResult{Molecules: nil}
	if !emptyRes.IsEmpty() {
		t.Error("IsEmpty() returned false for nil molecules")
	}

	emptyRes2 := &MoleculeSearchResult{Molecules: []*Molecule{}}
	if !emptyRes2.IsEmpty() {
		t.Error("IsEmpty() returned false for empty molecules")
	}

	nonEmptyRes := &MoleculeSearchResult{Molecules: []*Molecule{{}}}
	if nonEmptyRes.IsEmpty() {
		t.Error("IsEmpty() returned true for non-empty molecules")
	}
}

func TestPropertyFilter_Structure(t *testing.T) {
	t.Parallel()
	pf := PropertyFilter{
		Name:     "test",
		MinValue: float64Ptr(1.0),
		MaxValue: float64Ptr(2.0),
		Unit:     "unit",
	}
	if pf.Name != "test" || *pf.MinValue != 1.0 {
		t.Error("PropertyFilter struct fields not matching")
	}
}

//Personal.AI order the ending
