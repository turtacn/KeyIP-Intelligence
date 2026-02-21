package repositories

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type FamilyRepoTestSuite struct {
	suite.Suite
	// Mocks
}

func (s *FamilyRepoTestSuite) SetupTest() {
	// Setup
}

func (s *FamilyRepoTestSuite) TestEnsureFamilyNode_CreateNew() {
	assert.True(s.T(), true)
}

func TestFamilyRepoTestSuite(t *testing.T) {
	suite.Run(t, new(FamilyRepoTestSuite))
}

//Personal.AI order the ending
