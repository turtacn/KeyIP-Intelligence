package lifecycle

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewJurisdictionRegistry(t *testing.T) {
	r := NewJurisdictionRegistry()
	assert.NotNil(t, r)
	assert.NotEmpty(t, r.List())
}

func TestJurisdictionRegistry_Get_CN(t *testing.T) {
	r := NewJurisdictionRegistry()
	j, err := r.Get("CN")
	assert.NoError(t, err)
	assert.Equal(t, "CN", j.Code)
	assert.Equal(t, "China", j.Name)
	assert.Equal(t, 20, j.InventionTermYears)
	assert.True(t, j.SupportsUtilityModel)
}

func TestJurisdictionRegistry_Get_US(t *testing.T) {
	r := NewJurisdictionRegistry()
	j, err := r.Get("US")
	assert.NoError(t, err)
	assert.Equal(t, "US", j.Code)
	assert.False(t, j.SupportsUtilityModel)
	assert.Equal(t, 0, j.UtilityModelTermYears)
}

func TestJurisdictionRegistry_Get_NotFound(t *testing.T) {
	r := NewJurisdictionRegistry()
	_, err := r.Get("XX")
	assert.Error(t, err)
}

func TestJurisdictionRegistry_IsSupported(t *testing.T) {
	r := NewJurisdictionRegistry()
	assert.True(t, r.IsSupported("CN"))
	assert.False(t, r.IsSupported("XX"))
}

func TestNormalizeJurisdictionCode(t *testing.T) {
	assert.Equal(t, "CN", NormalizeJurisdictionCode("cn"))
	assert.Equal(t, "CN", NormalizeJurisdictionCode("CHINA"))
	assert.Equal(t, "US", NormalizeJurisdictionCode(" usa "))
	assert.Equal(t, "EP", NormalizeJurisdictionCode("EUROPE"))
	assert.Equal(t, "JP", NormalizeJurisdictionCode("JAPAN"))
	assert.Equal(t, "DE", NormalizeJurisdictionCode("DE"))
}

func TestCalculateExpirationDate(t *testing.T) {
	filingDate := time.Date(2020, 1, 15, 0, 0, 0, 0, time.UTC)

	// CN Invention: 20 years
	exp, err := CalculateExpirationDate(filingDate, "CN", "invention")
	assert.NoError(t, err)
	assert.Equal(t, time.Date(2040, 1, 15, 0, 0, 0, 0, time.UTC), exp)

	// CN Utility Model: 10 years
	exp, err = CalculateExpirationDate(filingDate, "CN", "utility_model")
	assert.NoError(t, err)
	assert.Equal(t, time.Date(2030, 1, 15, 0, 0, 0, 0, time.UTC), exp)

	// Invalid type
	_, err = CalculateExpirationDate(filingDate, "CN", "invalid")
	assert.Error(t, err)
}

func TestCalculateAnnuityDueDate(t *testing.T) {
	filingDate := time.Date(2020, 3, 10, 0, 0, 0, 0, time.UTC)
	dueDate := CalculateAnnuityDueDate(filingDate, 5)
	assert.Equal(t, time.Date(2025, 3, 10, 0, 0, 0, 0, time.UTC), dueDate)
}

func TestCalculateGraceDeadline(t *testing.T) {
	dueDate := time.Date(2025, 3, 10, 0, 0, 0, 0, time.UTC)
	graceDeadline := CalculateGraceDeadline(dueDate, 6)
	assert.Equal(t, time.Date(2025, 9, 10, 0, 0, 0, 0, time.UTC), graceDeadline)
}

//Personal.AI order the ending
