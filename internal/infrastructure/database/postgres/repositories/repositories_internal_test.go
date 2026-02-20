package repositories

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewRepositories(t *testing.T) {
	t.Parallel()

	t.Run("PatentRepository", func(t *testing.T) {
		repo := NewPatentRepository(nil, nil)
		assert.NotNil(t, repo)
	})

	t.Run("MoleculeRepository", func(t *testing.T) {
		repo := NewMoleculeRepository(nil, nil)
		assert.NotNil(t, repo)
	})

	t.Run("PortfolioRepository", func(t *testing.T) {
		repo := NewPortfolioRepository(nil, nil)
		assert.NotNil(t, repo)
	})

	t.Run("LifecycleRepository", func(t *testing.T) {
		repo := NewLifecycleRepository(nil, nil)
		assert.NotNil(t, repo)
	})

	t.Run("CollaborationRepository", func(t *testing.T) {
		repo := NewCollaborationRepository(nil, nil)
		assert.NotNil(t, repo)
	})
}
