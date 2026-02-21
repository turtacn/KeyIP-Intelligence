package molecule

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func float64Ptr(v float64) *float64 {
	return &v
}

func TestMoleculeQuery_Validate(t *testing.T) {
	tests := []struct {
		name    string
		query   MoleculeQuery
		wantErr bool
	}{
		{"valid_default", MoleculeQuery{}, false},
		{"valid_full", MoleculeQuery{
			Limit:  100,
			Offset: 10,
			SortBy: "created_at",
			MinMolecularWeight: float64Ptr(100.0),
			MaxMolecularWeight: float64Ptr(500.0),
		}, false},
		{"limit_exceeds_max", MoleculeQuery{Limit: 1001}, true},
		{"limit_negative", MoleculeQuery{Limit: -1}, true},
		{"offset_negative", MoleculeQuery{Offset: -1}, true},
		{"invalid_sort_by", MoleculeQuery{SortBy: "invalid"}, true},
		{"invalid_sort_order", MoleculeQuery{SortOrder: "random"}, true},
		{"weight_range_invalid", MoleculeQuery{
			MinMolecularWeight: float64Ptr(500.0),
			MaxMolecularWeight: float64Ptr(100.0),
		}, true},
		{"weight_negative", MoleculeQuery{MinMolecularWeight: float64Ptr(-1.0)}, true},
		{"invalid_status", MoleculeQuery{Statuses: []MoleculeStatus{MoleculeStatus(99)}}, true},
		{"property_filter_invalid", MoleculeQuery{
			PropertyFilters: []PropertyFilter{
				{Name: "p", MinValue: float64Ptr(10.0), MaxValue: float64Ptr(5.0)},
			},
		}, true},
		{"property_filter_no_name", MoleculeQuery{
			PropertyFilters: []PropertyFilter{
				{MinValue: float64Ptr(10.0)},
			},
		}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.query.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("MoleculeQuery.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestMoleculeSearchResult_IsEmpty(t *testing.T) {
	res := &MoleculeSearchResult{}
	assert.True(t, res.IsEmpty())

	res.Molecules = []*Molecule{{}}
	assert.False(t, res.IsEmpty())
}

//Personal.AI order the ending
