package repositories

import (
	"context"
	"testing"
	"time"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"github.com/turtacn/KeyIP-Intelligence/internal/domain/family"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
)

type FamilyRepoTestSuite struct {
	suite.Suite
	mockDriver *MockInfraDriver
	mockTx     *MockInfraTransaction
	repo       family.FamilyRepository
	log        logging.Logger
}

func (s *FamilyRepoTestSuite) SetupTest() {
	s.log = logging.NewNopLogger()
	d, tx := SetupMockDriver(s.T())
	s.mockDriver = d
	s.mockTx = tx
	s.repo = NewNeo4jFamilyRepo(s.mockDriver, s.log)
}

func (s *FamilyRepoTestSuite) TestEnsureFamilyNode_Success() {
	s.mockTx.On("Run", mock.Anything, mock.Anything, mock.Anything).Return(new(MockResult), nil)

	err := s.repo.EnsureFamilyNode(context.Background(), "fam1", "simple", nil)
	assert.NoError(s.T(), err)
}

func (s *FamilyRepoTestSuite) TestGetFamily_Found() {
	mockRes := new(MockResult)

	fNode := neo4j.Node{
		Props: map[string]any{
			"family_id": "fam1",
			"family_type": "simple",
		},
	}

	pNode := neo4j.Node{
		Id: 10,
		Props: map[string]any{
			"id": "patent1",
			"patent_number": "US123",
			"jurisdiction": "US",
			"filing_date": time.Now(),
		},
	}

	rel := neo4j.Relationship{
		Props: map[string]any{"role": "priority"},
	}

	record := NewRecord([]string{"f", "members"}, []any{
		fNode,
		[]any{
			map[string]any{"patent": pNode, "relation": rel},
		},
	})

	mockRes.Records = []*neo4j.Record{record}

	s.mockTx.On("Run", mock.Anything, mock.Anything, mock.Anything).Return(mockRes, nil)

	fam, err := s.repo.GetFamily(context.Background(), "fam1")
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), "fam1", fam.FamilyID)
	assert.Len(s.T(), fam.Members, 1)
	assert.Equal(s.T(), "US123", fam.Members[0].PatentNumber)
}

func (s *FamilyRepoTestSuite) TestAddMember_Success() {
	mockRes := new(MockResult)
	mockSummary := new(MockResultSummary)
	mockCounters := new(MockCounters)

	// Set struct fields
	mockSummary.CountersObj = mockCounters
	mockRes.Summary = mockSummary

	s.mockTx.On("Run", mock.Anything, mock.Anything, mock.Anything).Return(mockRes, nil)

	err := s.repo.AddMember(context.Background(), "fam1", "patent1", "priority")
	assert.NoError(s.T(), err)
}

func TestFamilyRepo(t *testing.T) {
	suite.Run(t, new(FamilyRepoTestSuite))
}
