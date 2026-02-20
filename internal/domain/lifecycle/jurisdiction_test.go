// Package lifecycle_test provides unit tests for jurisdiction rules.
package lifecycle_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/turtacn/KeyIP-Intelligence/internal/domain/lifecycle"
	ptypes "github.com/turtacn/KeyIP-Intelligence/pkg/types/patent"
)

// ─────────────────────────────────────────────────────────────────────────────
// TestGetJurisdictionRules
// ─────────────────────────────────────────────────────────────────────────────

func TestGetJurisdictionRules_CN(t *testing.T) {
	t.Parallel()

	rules, err := lifecycle.GetJurisdictionRules(ptypes.JurisdictionCN)
	require.NoError(t, err)
	require.NotNil(t, rules)

	assert.Equal(t, ptypes.JurisdictionCN, rules.Code)
	assert.Equal(t, 20, rules.PatentTermYears)
	assert.Equal(t, 3, rules.AnnuityStartYear)
	assert.Equal(t, "annual", rules.AnnuityFrequency)
	assert.Equal(t, 6, rules.GracePeriodMonths)
	assert.Equal(t, 0.25, rules.SurchargeRate)
	assert.Equal(t, 36, rules.ExaminationDeadlineMonths)
	assert.Equal(t, 4, rules.OAResponseDeadlineMonths)
}

func TestGetJurisdictionRules_US(t *testing.T) {
	t.Parallel()

	rules, err := lifecycle.GetJurisdictionRules(ptypes.JurisdictionUS)
	require.NoError(t, err)
	require.NotNil(t, rules)

	assert.Equal(t, ptypes.JurisdictionUS, rules.Code)
	assert.Equal(t, "milestone", rules.AnnuityFrequency)
	assert.Equal(t, 0.50, rules.SurchargeRate)
}

func TestGetJurisdictionRules_EP(t *testing.T) {
	t.Parallel()

	rules, err := lifecycle.GetJurisdictionRules(ptypes.JurisdictionEP)
	require.NoError(t, err)
	require.NotNil(t, rules)

	assert.Equal(t, ptypes.JurisdictionEP, rules.Code)
	assert.Equal(t, 31, rules.PCTNationalPhaseMonths)
}

func TestGetJurisdictionRules_JP(t *testing.T) {
	t.Parallel()

	rules, err := lifecycle.GetJurisdictionRules(ptypes.JurisdictionJP)
	require.NoError(t, err)
	assert.Equal(t, 1, rules.AnnuityStartYear, "JP annuities start from year 1")
}

func TestGetJurisdictionRules_KR(t *testing.T) {
	t.Parallel()

	rules, err := lifecycle.GetJurisdictionRules(ptypes.JurisdictionKR)
	require.NoError(t, err)
	assert.Equal(t, 1, rules.AnnuityStartYear, "KR annuities start from year 1")
}

func TestGetJurisdictionRules_NotFound(t *testing.T) {
	t.Parallel()

	_, err := lifecycle.GetJurisdictionRules("XX")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

// ─────────────────────────────────────────────────────────────────────────────
// TestCalculateExpiryDate
// ─────────────────────────────────────────────────────────────────────────────

func TestCalculateExpiryDate_CN(t *testing.T) {
	t.Parallel()

	filingDate := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	expiryDate := lifecycle.CalculateExpiryDate(ptypes.JurisdictionCN, filingDate)

	expected := time.Date(2040, 1, 1, 0, 0, 0, 0, time.UTC)
	assert.Equal(t, expected, expiryDate)
}

func TestCalculateExpiryDate_US(t *testing.T) {
	t.Parallel()

	filingDate := time.Date(2015, 6, 15, 0, 0, 0, 0, time.UTC)
	expiryDate := lifecycle.CalculateExpiryDate(ptypes.JurisdictionUS, filingDate)

	expected := time.Date(2035, 6, 15, 0, 0, 0, 0, time.UTC)
	assert.Equal(t, expected, expiryDate)
}

func TestCalculateExpiryDate_UnknownJurisdiction(t *testing.T) {
	t.Parallel()

	filingDate := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	expiryDate := lifecycle.CalculateExpiryDate("XX", filingDate)

	// Should fallback to 20 years.
	expected := time.Date(2040, 1, 1, 0, 0, 0, 0, time.UTC)
	assert.Equal(t, expected, expiryDate)
}

// ─────────────────────────────────────────────────────────────────────────────
// TestGenerateInitialDeadlines
// ─────────────────────────────────────────────────────────────────────────────

func TestGenerateInitialDeadlines_CN(t *testing.T) {
	t.Parallel()

	filingDate := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	deadlines, err := lifecycle.GenerateInitialDeadlines(ptypes.JurisdictionCN, filingDate)

	require.NoError(t, err)
	assert.NotEmpty(t, deadlines, "CN patents should have initial deadlines")

	// Should include examination request deadline (36 months).
	var hasExamDeadline bool
	for _, d := range deadlines {
		if d.Type == lifecycle.DeadlineExamination {
			hasExamDeadline = true
			expected := filingDate.AddDate(0, 36, 0)
			assert.Equal(t, expected, d.DueDate)
			assert.Equal(t, lifecycle.PriorityHigh, d.Priority)
		}
	}
	assert.True(t, hasExamDeadline, "CN should have examination deadline")
}

func TestGenerateInitialDeadlines_US(t *testing.T) {
	t.Parallel()

	filingDate := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	deadlines, err := lifecycle.GenerateInitialDeadlines(ptypes.JurisdictionUS, filingDate)

	require.NoError(t, err)
	// US has automatic examination, so no examination request deadline.
	for _, d := range deadlines {
		assert.NotEqual(t, lifecycle.DeadlineExamination, d.Type)
	}
}

func TestGenerateInitialDeadlines_WO(t *testing.T) {
	t.Parallel()

	filingDate := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	deadlines, err := lifecycle.GenerateInitialDeadlines(ptypes.JurisdictionWO, filingDate)

	require.NoError(t, err)

	// Should include PCT national phase entry deadline.
	var hasPCTDeadline bool
	for _, d := range deadlines {
		if d.Type == lifecycle.DeadlinePCTNationalPhase {
			hasPCTDeadline = true
			assert.Equal(t, lifecycle.PriorityCritical, d.Priority)
		}
	}
	assert.True(t, hasPCTDeadline, "WO should have PCT national phase deadline")
}

// ─────────────────────────────────────────────────────────────────────────────
// TestGetAllJurisdictions
// ─────────────────────────────────────────────────────────────────────────────

func TestGetAllJurisdictions(t *testing.T) {
	t.Parallel()

	all := lifecycle.GetAllJurisdictions()
	assert.NotEmpty(t, all, "should return all registered jurisdictions")

	// Should include at least CN, US, EP, JP, KR, WO.
	codes := make(map[ptypes.JurisdictionCode]bool)
	for _, rules := range all {
		codes[rules.Code] = true
	}

	assert.True(t, codes[ptypes.JurisdictionCN])
	assert.True(t, codes[ptypes.JurisdictionUS])
	assert.True(t, codes[ptypes.JurisdictionEP])
	assert.True(t, codes[ptypes.JurisdictionJP])
	assert.True(t, codes[ptypes.JurisdictionKR])
	assert.True(t, codes[ptypes.JurisdictionWO])
}

//Personal.AI order the ending
