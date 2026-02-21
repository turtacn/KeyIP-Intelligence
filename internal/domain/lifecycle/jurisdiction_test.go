package lifecycle

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewJurisdictionRegistry(t *testing.T) {
	reg := NewJurisdictionRegistry()
	assert.NotNil(t, reg)
	assert.True(t, len(reg.List()) >= 9)
}

func TestJurisdictionRegistry_Get_CN(t *testing.T) {
	reg := NewJurisdictionRegistry()
	j, err := reg.Get("CN")
	assert.NoError(t, err)
	assert.Equal(t, "CNIPA", j.PatentOffice)
	assert.Equal(t, 20, j.InventionTermYears)
	assert.True(t, j.SupportsUtilityModel)
}

func TestJurisdictionRegistry_Get_US(t *testing.T) {
	reg := NewJurisdictionRegistry()
	j, err := reg.Get("US")
	assert.NoError(t, err)
	assert.Equal(t, "USPTO", j.PatentOffice)
	assert.False(t, j.SupportsUtilityModel)
}

func TestJurisdictionRegistry_Get_NotFound(t *testing.T) {
	reg := NewJurisdictionRegistry()
	_, err := reg.Get("XYZ")
	assert.Error(t, err)
}

func TestJurisdictionRegistry_List_Sorted(t *testing.T) {
	reg := NewJurisdictionRegistry()
	list := reg.List()
	assert.Equal(t, "CN", list[0].Code)
	assert.Equal(t, "DE", list[1].Code)
	assert.Equal(t, "WO", list[len(list)-1].Code)
}

func TestJurisdictionRegistry_GetPatentTerm_CN(t *testing.T) {
	reg := NewJurisdictionRegistry()
	term, err := reg.GetPatentTerm("CN", "invention")
	assert.NoError(t, err)
	assert.Equal(t, 20, term)

	term, err = reg.GetPatentTerm("CN", "utility_model")
	assert.NoError(t, err)
	assert.Equal(t, 10, term)

	term, err = reg.GetPatentTerm("CN", "design")
	assert.NoError(t, err)
	assert.Equal(t, 15, term)
}

func TestJurisdictionRegistry_GetAnnuityRules_US(t *testing.T) {
	reg := NewJurisdictionRegistry()
	rules, err := reg.GetAnnuityRules("US")
	assert.NoError(t, err)
	assert.False(t, rules.IsAnnual)
	assert.Equal(t, 3, len(rules.PaymentSchedule))
}

func TestNormalizeJurisdictionCode(t *testing.T) {
	assert.Equal(t, "CN", NormalizeJurisdictionCode("cn"))
	assert.Equal(t, "US", NormalizeJurisdictionCode(" USA "))
	assert.Equal(t, "CN", NormalizeJurisdictionCode("CHINA"))
}

func TestCalculateExpirationDate(t *testing.T) {
	filingDate := time.Date(2020, 1, 15, 0, 0, 0, 0, time.UTC)
	expiry, err := CalculateExpirationDate(filingDate, "CN", "invention")
	assert.NoError(t, err)
	assert.Equal(t, time.Date(2040, 1, 15, 0, 0, 0, 0, time.UTC), expiry)
}

func TestCalculateAnnuityDueDate(t *testing.T) {
	filingDate := time.Date(2020, 3, 10, 0, 0, 0, 0, time.UTC)
	due := CalculateAnnuityDueDate(filingDate, 5)
	assert.Equal(t, time.Date(2025, 3, 10, 0, 0, 0, 0, time.UTC), due)
}

func TestCalculateGraceDeadline(t *testing.T) {
	dueDate := time.Date(2025, 3, 10, 0, 0, 0, 0, time.UTC)
	grace := CalculateGraceDeadline(dueDate, 6)
	assert.Equal(t, time.Date(2025, 9, 10, 0, 0, 0, 0, time.UTC), grace)
}

//Personal.AI order the ending
