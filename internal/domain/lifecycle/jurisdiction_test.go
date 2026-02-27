package lifecycle

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/turtacn/KeyIP-Intelligence/pkg/errors"
)

func TestJurisdictionRegistry_Normalize(t *testing.T) {
	registry := NewJurisdictionRegistry()

	tests := []struct {
		name    string
		input   string
		want    Jurisdiction
		wantErr bool
	}{
		{"Exact Match CN", "CN", JurisdictionCN, false},
		{"Alias CHN", "CHN", JurisdictionCN, false},
		{"Alias China (case insensitive)", "China", JurisdictionCN, false},
		{"Exact Match US", "US", JurisdictionUS, false},
		{"Alias USA", "USA", JurisdictionUS, false},
		{"Invalid Code", "XYZ", "", true},
		{"Empty Code", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := registry.Normalize(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
				assert.True(t, errors.IsValidation(err))
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}

func TestJurisdictionRegistry_Get(t *testing.T) {
	registry := NewJurisdictionRegistry()

	// Found
	info, err := registry.Get("CN")
	assert.NoError(t, err)
	assert.NotNil(t, info)
	assert.Equal(t, JurisdictionCN, info.Code)
	assert.Equal(t, "CNY", info.Currency)

	// Not Found (via Normalize error)
	info, err = registry.Get("XYZ")
	assert.Error(t, err)
	assert.Nil(t, info)
}

func TestJurisdictionRegistry_List(t *testing.T) {
	registry := NewJurisdictionRegistry()
	list := registry.List()
	assert.NotEmpty(t, list)
	assert.GreaterOrEqual(t, len(list), 5) // We added 5 initial ones
}

//Personal.AI order the ending
