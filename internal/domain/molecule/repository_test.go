package molecule

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func float64Ptr(v float64) *float64 {
	return &v
}

func timePtr(t time.Time) *time.Time {
	return &t
}

func TestMoleculeQuery_Validate(t *testing.T) {
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
				IDs:       []string{"1", "2"},
				Limit:     50,
				SortBy:    "created_at",
				SortOrder: "asc",
			},
			wantErr: false,
		},
		{
			name: "valid_with_pagination",
			query: MoleculeQuery{
				Offset: 100,
				Limit:  50,
			},
			wantErr: false,
		},
		{
			name: "limit_exceeds_max",
			query: MoleculeQuery{
				Limit: 1001,
			},
			wantErr: true,
		},
		{
			name: "limit_zero_defaults",
			query: MoleculeQuery{
				Limit: 0,
			},
			wantErr: false, // Defaulted to 20
		},
		{
			name: "limit_negative",
			query: MoleculeQuery{
				Limit: -1,
			},
			wantErr: true,
		},
		{
			name: "offset_negative",
			query: MoleculeQuery{
				Offset: -1,
			},
			wantErr: true,
		},
		{
			name: "valid_sort_by_created_at",
			query: MoleculeQuery{
				SortBy: "created_at",
			},
			wantErr: false,
		},
		{
			name: "valid_sort_by_molecular_weight",
			query: MoleculeQuery{
				SortBy: "molecular_weight",
			},
			wantErr: false,
		},
		{
			name: "invalid_sort_by",
			query: MoleculeQuery{
				SortBy: "drop_table",
			},
			wantErr: true,
		},
		{
			name: "valid_sort_order_asc",
			query: MoleculeQuery{
				SortOrder: "asc",
			},
			wantErr: false,
		},
		{
			name: "valid_sort_order_desc",
			query: MoleculeQuery{
				SortOrder: "desc",
			},
			wantErr: false,
		},
		{
			name: "invalid_sort_order",
			query: MoleculeQuery{
				SortOrder: "random",
			},
			wantErr: true,
		},
		{
			name: "property_filter_min_gt_max",
			query: MoleculeQuery{
				PropertyFilters: []PropertyFilter{
					{Name: "p1", MinValue: float64Ptr(100), MaxValue: float64Ptr(50)},
				},
			},
			wantErr: true,
		},
		{
			name: "property_filter_valid_range",
			query: MoleculeQuery{
				PropertyFilters: []PropertyFilter{
					{Name: "p1", MinValue: float64Ptr(50), MaxValue: float64Ptr(100)},
				},
			},
			wantErr: false,
		},
		{
			name: "property_filter_only_min",
			query: MoleculeQuery{
				PropertyFilters: []PropertyFilter{
					{Name: "p1", MinValue: float64Ptr(50)},
				},
			},
			wantErr: false,
		},
		{
			name: "property_filter_only_max",
			query: MoleculeQuery{
				PropertyFilters: []PropertyFilter{
					{Name: "p1", MaxValue: float64Ptr(100)},
				},
			},
			wantErr: false,
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
		{
			name: "weight_range_min_gt_max",
			query: MoleculeQuery{
				MinMolecularWeight: float64Ptr(200),
				MaxMolecularWeight: float64Ptr(100),
			},
			wantErr: true,
		},
		{
			name: "weight_range_negative",
			query: MoleculeQuery{
				MinMolecularWeight: float64Ptr(-10),
			},
			wantErr: true,
		},
		{
			name: "valid_statuses",
			query: MoleculeQuery{
				Statuses: []MoleculeStatus{MoleculeStatusActive},
			},
			wantErr: false,
		},
		{
			name: "invalid_status",
			query: MoleculeQuery{
				Statuses: []MoleculeStatus{MoleculeStatus(99)},
			},
			wantErr: true,
		},
		{
			name: "valid_sources",
			query: MoleculeQuery{
				Sources: []MoleculeSource{SourcePatent},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.query.Validate()
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestMoleculeSearchResult_IsEmpty(t *testing.T) {
	// Empty
	res := &MoleculeSearchResult{Molecules: []*Molecule{}}
	assert.True(t, res.IsEmpty())

	// Nil
	res = &MoleculeSearchResult{Molecules: nil}
	assert.True(t, res.IsEmpty())

	// Non-empty
	res = &MoleculeSearchResult{Molecules: []*Molecule{{}}}
	assert.False(t, res.IsEmpty())
}

//Personal.AI order the ending
