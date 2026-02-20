// Package portfolio_test provides unit tests for the Portfolio aggregate root
// and its invariants.
package portfolio_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/turtacn/KeyIP-Intelligence/internal/domain/portfolio"
	"github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
)

// ─────────────────────────────────────────────────────────────────────────────
// NewPortfolio factory tests
// ─────────────────────────────────────────────────────────────────────────────

func TestNewPortfolio_ValidParams(t *testing.T) {
	t.Parallel()

	p, err := portfolio.NewPortfolio("My Portfolio", "Test description", "user-123")

	require.NoError(t, err)
	require.NotNil(t, p)
	assert.NotEmpty(t, p.ID)
	assert.Equal(t, "My Portfolio", p.Name)
	assert.Equal(t, "Test description", p.Description)
	assert.Equal(t, common.UserID("user-123"), p.OwnerID)
	assert.Equal(t, common.StatusActive, p.Status)
	assert.Empty(t, p.PatentIDs)
	assert.Empty(t, p.Tags)
	assert.Nil(t, p.TotalValue)
	assert.Equal(t, 1, p.Version)
}

func TestNewPortfolio_EmptyName(t *testing.T) {
	t.Parallel()

	p, err := portfolio.NewPortfolio("", "description", "user-123")

	assert.Error(t, err)
	assert.Nil(t, p)
	assert.Contains(t, err.Error(), "name")
}

func TestNewPortfolio_EmptyOwnerID(t *testing.T) {
	t.Parallel()

	p, err := portfolio.NewPortfolio("Portfolio", "desc", "")

	assert.Error(t, err)
	assert.Nil(t, p)
	assert.Contains(t, err.Error(), "owner")
}

// ─────────────────────────────────────────────────────────────────────────────
// AddPatent tests
// ─────────────────────────────────────────────────────────────────────────────

func TestAddPatent_Success(t *testing.T) {
	t.Parallel()

	p, err := portfolio.NewPortfolio("Test", "desc", "user-1")
	require.NoError(t, err)

	patentID := common.NewID()
	err = p.AddPatent(patentID)

	assert.NoError(t, err)
	assert.Equal(t, 1, p.Size())
	assert.True(t, p.ContainsPatent(patentID))
	assert.Equal(t, 2, p.Version, "Version should increment after add")
}

func TestAddPatent_Duplicate(t *testing.T) {
	t.Parallel()

	p, err := portfolio.NewPortfolio("Test", "desc", "user-1")
	require.NoError(t, err)

	patentID := common.NewID()
	require.NoError(t, p.AddPatent(patentID))

	err = p.AddPatent(patentID)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "already in portfolio")
	assert.Equal(t, 1, p.Size(), "Size should not change after duplicate add")
}

func TestAddPatent_EmptyID(t *testing.T) {
	t.Parallel()

	p, err := portfolio.NewPortfolio("Test", "desc", "user-1")
	require.NoError(t, err)

	err = p.AddPatent("")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "patent ID")
}

func TestAddPatent_MultiplePatents(t *testing.T) {
	t.Parallel()

	p, err := portfolio.NewPortfolio("Test", "desc", "user-1")
	require.NoError(t, err)

	id1 := common.NewID()
	id2 := common.NewID()
	id3 := common.NewID()

	require.NoError(t, p.AddPatent(id1))
	require.NoError(t, p.AddPatent(id2))
	require.NoError(t, p.AddPatent(id3))

	assert.Equal(t, 3, p.Size())
	assert.True(t, p.ContainsPatent(id1))
	assert.True(t, p.ContainsPatent(id2))
	assert.True(t, p.ContainsPatent(id3))
}

// ─────────────────────────────────────────────────────────────────────────────
// RemovePatent tests
// ─────────────────────────────────────────────────────────────────────────────

func TestRemovePatent_Success(t *testing.T) {
	t.Parallel()

	p, err := portfolio.NewPortfolio("Test", "desc", "user-1")
	require.NoError(t, err)

	patentID := common.NewID()
	require.NoError(t, p.AddPatent(patentID))

	err = p.RemovePatent(patentID)

	assert.NoError(t, err)
	assert.Equal(t, 0, p.Size())
	assert.False(t, p.ContainsPatent(patentID))
}

func TestRemovePatent_NotFound(t *testing.T) {
	t.Parallel()

	p, err := portfolio.NewPortfolio("Test", "desc", "user-1")
	require.NoError(t, err)

	nonExistentID := common.NewID()
	err = p.RemovePatent(nonExistentID)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not found")
}

func TestRemovePatent_EmptyID(t *testing.T) {
	t.Parallel()

	p, err := portfolio.NewPortfolio("Test", "desc", "user-1")
	require.NoError(t, err)

	err = p.RemovePatent("")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "patent ID")
}

func TestRemovePatent_MultipleOperations(t *testing.T) {
	t.Parallel()

	p, err := portfolio.NewPortfolio("Test", "desc", "user-1")
	require.NoError(t, err)

	id1 := common.NewID()
	id2 := common.NewID()
	id3 := common.NewID()

	require.NoError(t, p.AddPatent(id1))
	require.NoError(t, p.AddPatent(id2))
	require.NoError(t, p.AddPatent(id3))

	require.NoError(t, p.RemovePatent(id2))

	assert.Equal(t, 2, p.Size())
	assert.True(t, p.ContainsPatent(id1))
	assert.False(t, p.ContainsPatent(id2))
	assert.True(t, p.ContainsPatent(id3))
}

// ─────────────────────────────────────────────────────────────────────────────
// ContainsPatent tests
// ─────────────────────────────────────────────────────────────────────────────

func TestContainsPatent_Exists(t *testing.T) {
	t.Parallel()

	p, err := portfolio.NewPortfolio("Test", "desc", "user-1")
	require.NoError(t, err)

	patentID := common.NewID()
	require.NoError(t, p.AddPatent(patentID))

	assert.True(t, p.ContainsPatent(patentID))
}

func TestContainsPatent_NotExists(t *testing.T) {
	t.Parallel()

	p, err := portfolio.NewPortfolio("Test", "desc", "user-1")
	require.NoError(t, err)

	nonExistentID := common.NewID()
	assert.False(t, p.ContainsPatent(nonExistentID))
}

func TestContainsPatent_EmptyPortfolio(t *testing.T) {
	t.Parallel()

	p, err := portfolio.NewPortfolio("Test", "desc", "user-1")
	require.NoError(t, err)

	assert.False(t, p.ContainsPatent(common.NewID()))
}

// ─────────────────────────────────────────────────────────────────────────────
// Size tests
// ─────────────────────────────────────────────────────────────────────────────

func TestSize_EmptyPortfolio(t *testing.T) {
	t.Parallel()

	p, err := portfolio.NewPortfolio("Test", "desc", "user-1")
	require.NoError(t, err)

	assert.Equal(t, 0, p.Size())
}

func TestSize_AfterAddingPatents(t *testing.T) {
	t.Parallel()

	p, err := portfolio.NewPortfolio("Test", "desc", "user-1")
	require.NoError(t, err)

	assert.Equal(t, 0, p.Size())

	require.NoError(t, p.AddPatent(common.NewID()))
	assert.Equal(t, 1, p.Size())

	require.NoError(t, p.AddPatent(common.NewID()))
	assert.Equal(t, 2, p.Size())

	require.NoError(t, p.AddPatent(common.NewID()))
	assert.Equal(t, 3, p.Size())
}

func TestSize_AfterRemovingPatents(t *testing.T) {
	t.Parallel()

	p, err := portfolio.NewPortfolio("Test", "desc", "user-1")
	require.NoError(t, err)

	id1 := common.NewID()
	id2 := common.NewID()
	require.NoError(t, p.AddPatent(id1))
	require.NoError(t, p.AddPatent(id2))

	assert.Equal(t, 2, p.Size())

	require.NoError(t, p.RemovePatent(id1))
	assert.Equal(t, 1, p.Size())

	require.NoError(t, p.RemovePatent(id2))
	assert.Equal(t, 0, p.Size())
}

// ─────────────────────────────────────────────────────────────────────────────
// SetValuation tests
// ─────────────────────────────────────────────────────────────────────────────

func TestSetValuation_UpdatesValueAndVersion(t *testing.T) {
	t.Parallel()

	p, err := portfolio.NewPortfolio("Test", "desc", "user-1")
	require.NoError(t, err)
	initialVersion := p.Version

	result := portfolio.ValuationResult{
		TotalValue:   1000000.0,
		AverageValue: 250000.0,
		Method:       "MultiFactorV1",
	}

	p.SetValuation(result)

	require.NotNil(t, p.TotalValue)
	assert.Equal(t, 1000000.0, p.TotalValue.TotalValue)
	assert.Equal(t, 250000.0, p.TotalValue.AverageValue)
	assert.Equal(t, "MultiFactorV1", p.TotalValue.Method)
	assert.Equal(t, initialVersion+1, p.Version)
}

