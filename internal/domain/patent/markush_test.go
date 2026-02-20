package patent_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/turtacn/KeyIP-Intelligence/internal/domain/patent"
	common "github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
)

// ─────────────────────────────────────────────────────────────────────────────
// Fixtures
// ─────────────────────────────────────────────────────────────────────────────

var (
	testPatentID = common.NewID()
	testClaimID  = common.NewID()
)

func validRGroups() []patent.RGroup {
	return []patent.RGroup{
		{
			Position:     "R1",
			Alternatives: []string{"C", "N", "O"},
			Description:  "first position",
		},
		{
			Position:     "R2",
			Alternatives: []string{"F", "Cl", "Br"},
			Description:  "halogen position",
		},
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// TestNewMarkush
// ─────────────────────────────────────────────────────────────────────────────

func TestNewMarkush_ValidParams(t *testing.T) {
	t.Parallel()

	m, err := patent.NewMarkush(
		testPatentID,
		testClaimID,
		"c1cc([R1])cc([R2])c1",
		validRGroups(),
	)
	require.NoError(t, err)
	require.NotNil(t, m)
	assert.NotEmpty(t, string(m.ID))
	assert.Equal(t, testPatentID, m.PatentID)
	assert.Equal(t, testClaimID, m.ClaimID)
	assert.Equal(t, "c1cc([R1])cc([R2])c1", m.CoreStructure)
	assert.Len(t, m.RGroups, 2)
	assert.Greater(t, m.EnumeratedCount, int64(0))
}

func TestNewMarkush_EmptyCoreStructure(t *testing.T) {
	t.Parallel()

	_, err := patent.NewMarkush(testPatentID, testClaimID, "", validRGroups())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "core structure")
}

func TestNewMarkush_WhitespaceCoreStructure(t *testing.T) {
	t.Parallel()

	_, err := patent.NewMarkush(testPatentID, testClaimID, "   ", validRGroups())
	require.Error(t, err)
}

func TestNewMarkush_EmptyRGroups(t *testing.T) {
	t.Parallel()

	_, err := patent.NewMarkush(testPatentID, testClaimID, "c1ccccc1", []patent.RGroup{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "R-group")
}

func TestNewMarkush_RGroupWithNoAlternatives(t *testing.T) {
	t.Parallel()

	rGroups := []patent.RGroup{
		{Position: "R1", Alternatives: []string{}},
	}
	_, err := patent.NewMarkush(testPatentID, testClaimID, "c1ccccc1", rGroups)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "alternative")
}

func TestNewMarkush_RGroupWithEmptyPosition(t *testing.T) {
	t.Parallel()

	rGroups := []patent.RGroup{
		{Position: "", Alternatives: []string{"C", "N"}},
	}
	_, err := patent.NewMarkush(testPatentID, testClaimID, "c1ccccc1", rGroups)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Position")
}

func TestNewMarkush_InvalidSMILES_Unbalanced(t *testing.T) {
	t.Parallel()

	_, err := patent.NewMarkush(testPatentID, testClaimID, "c1ccccc1((", validRGroups())
	require.Error(t, err)
}

func TestNewMarkush_EmptyPatentID(t *testing.T) {
	t.Parallel()

	_, err := patent.NewMarkush("", testClaimID, "c1ccccc1", validRGroups())
	require.Error(t, err)
}

// ─────────────────────────────────────────────────────────────────────────────
// TestCalculateEnumeratedCount
// ─────────────────────────────────────────────────────────────────────────────

func TestCalculateEnumeratedCount_TwoGroupsThreeAlternatives(t *testing.T) {
	t.Parallel()

	// 3 alternatives × 3 alternatives = 9
	rGroups := []patent.RGroup{
		{Position: "R1", Alternatives: []string{"C", "N", "O"}},
		{Position: "R2", Alternatives: []string{"F", "Cl", "Br"}},
	}
	m, err := patent.NewMarkush(testPatentID, testClaimID, "c1cc([R1])cc([R2])c1", rGroups)
	require.NoError(t, err)

	count := m.CalculateEnumeratedCount()
	assert.Equal(t, int64(9), count)
}

func TestCalculateEnumeratedCount_SingleGroup(t *testing.T) {
	t.Parallel()

	rGroups := []patent.RGroup{
		{Position: "R1", Alternatives: []string{"C", "N", "O", "S"}},
	}
	m, err := patent.NewMarkush(testPatentID, testClaimID, "c1cc([R1])ccc1", rGroups)
	require.NoError(t, err)
	assert.Equal(t, int64(4), m.CalculateEnumeratedCount())
}

func TestCalculateEnumeratedCount_ThreeGroups(t *testing.T) {
	t.Parallel()

	rGroups := []patent.RGroup{
		{Position: "R1", Alternatives: []string{"C", "N"}},
		{Position: "R2", Alternatives: []string{"F", "Cl", "Br"}},
		{Position: "R3", Alternatives: []string{"c1ccccc1", "c1ccncc1"}},
	}
	m, err := patent.NewMarkush(testPatentID, testClaimID, "c1cc([R1])c([R2])c([R3])c1", rGroups)
	require.NoError(t, err)
	// 2 × 3 × 2 = 12
	assert.Equal(t, int64(12), m.CalculateEnumeratedCount())
}

// ─────────────────────────────────────────────────────────────────────────────
// TestEnumerateExemplary
// ─────────────────────────────────────────────────────────────────────────────

func TestEnumerateExemplary_CountNotExceedsMax(t *testing.T) {
	t.Parallel()

	rGroups := []patent.RGroup{
		{Position: "R1", Alternatives: []string{"C", "N", "O", "S", "P"}},
		{Position: "R2", Alternatives: []string{"F", "Cl", "Br", "I"}},
	}
	m, err := patent.NewMarkush(testPatentID, testClaimID, "c1cc([R1])cc([R2])c1", rGroups)
	require.NoError(t, err)

	maxCount := 5
	results := m.EnumerateExemplary(maxCount)
	assert.LessOrEqual(t, len(results), maxCount)
}

func TestEnumerateExemplary_ReturnsResults(t *testing.T) {
	t.Parallel()

	rGroups := []patent.RGroup{
		{Position: "R1", Alternatives: []string{"C", "N"}},
	}
	m, err := patent.NewMarkush(testPatentID, testClaimID, "c1cc([R1])ccc1", rGroups)
	require.NoError(t, err)

	results := m.EnumerateExemplary(10)
	assert.NotEmpty(t, results)
}

func TestEnumerateExemplary_SMILESAreStrings(t *testing.T) {
	t.Parallel()

	rGroups := []patent.RGroup{
		{Position: "R1", Alternatives: []string{"C", "N"}},
		{Position: "R2", Alternatives: []string{"F", "Cl"}},
	}
	m, err := patent.NewMarkush(testPatentID, testClaimID, "c1cc([R1])cc([R2])c1", rGroups)
	require.NoError(t, err)

	results := m.EnumerateExemplary(10)
	for _, s := range results {
		assert.NotEmpty(t, s, "enumerated SMILES must not be empty strings")
		assert.Greater(t, len(s), 0)
	}
}

func TestEnumerateExemplary_ZeroMaxDefaultsTen(t *testing.T) {
	t.Parallel()

	rGroups := make([]patent.RGroup, 1)
	alts := make([]string, 20)
	for i := range alts {
		alts[i] = "C"
	}
	rGroups[0] = patent.RGroup{Position: "R1", Alternatives: alts}

	m, err := patent.NewMarkush(testPatentID, testClaimID, "c1cc([R1])ccc1", rGroups)
	require.NoError(t, err)

	results := m.EnumerateExemplary(0)
	assert.LessOrEqual(t, len(results), 10, "default maxCount should be 10")
}

// ─────────────────────────────────────────────────────────────────────────────
// TestContainsMolecule
// ─────────────────────────────────────────────────────────────────────────────

func TestContainsMolecule_CoreScaffoldMatchReturnsTrue(t *testing.T) {
	t.Parallel()

	// Core scaffold: benzene ring portion remains after stripping R-groups.
	rGroups := []patent.RGroup{
		{Position: "R1", Alternatives: []string{"C"}},
	}
	// Core: c1ccccc1 with R1 placeholder — scaffold is "c1cc" + "cc" + "c1" region.
	m, err := patent.NewMarkush(testPatentID, testClaimID, "c1ccccc1", rGroups)
	require.NoError(t, err)

	// A molecule that contains the benzene ring scaffold.
	assert.True(t, m.ContainsMolecule("c1ccccc1C"), "molecule containing the core scaffold should match")
}

func TestContainsMolecule_CompletelyDifferentReturnsFalse(t *testing.T) {
	t.Parallel()

	rGroups := []patent.RGroup{
		{Position: "R1", Alternatives: []string{"C"}},
	}
	m, err := patent.NewMarkush(testPatentID, testClaimID, "c1ccccc1", rGroups)
	require.NoError(t, err)

	// A completely unrelated molecule (pure aliphatic chain with no aromatic atoms).
	assert.False(t, m.ContainsMolecule("CCCCCCCCCC"), "aliphatic chain should not match aromatic core")
}

func TestContainsMolecule_EmptySMILESReturnsFalse(t *testing.T) {
	t.Parallel()

	rGroups := []patent.RGroup{
		{Position: "R1", Alternatives: []string{"C"}},
	}
	m, err := patent.NewMarkush(testPatentID, testClaimID, "c1ccccc1", rGroups)
	require.NoError(t, err)

	assert.False(t, m.ContainsMolecule(""))
}

// ─────────────────────────────────────────────────────────────────────────────
// TestToDTO
// ─────────────────────────────────────────────────────────────────────────────

func TestMarkushToDTO_FieldsMatch(t *testing.T) {
	t.Parallel()

	rGroups := validRGroups()
	m, err := patent.NewMarkush(testPatentID, testClaimID, "c1cc([R1])cc([R2])c1", rGroups)
	require.NoError(t, err)
	m.Description = "benzene disubstituted Markush"

	dto := m.ToDTO()
	assert.Equal(t, m.ID, dto.ID)
	assert.Equal(t, m.PatentID, dto.PatentID)
	assert.Equal(t, m.ClaimID, dto.ClaimID)
	assert.Equal(t, m.CoreStructure, dto.CoreStructure)
	assert.Equal(t, m.Description, dto.Description)
	assert.Equal(t, m.EnumeratedCount, dto.EnumeratedCount)
	require.Len(t, dto.RGroups, 2)
	assert.Equal(t, "R1", dto.RGroups[0].Position)
	assert.Equal(t, []string{"C", "N", "O"}, dto.RGroups[0].Alternatives)
}

//Personal.AI order the ending
