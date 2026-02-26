package repositories

import (
	"context"
	"testing"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
)

type KnowledgeGraphRepoTestSuite struct {
	suite.Suite
	mockDriver *MockInfraDriver
	mockTx     *MockInfraTransaction
	repo       KnowledgeGraphRepository
	log        logging.Logger
}

func (s *KnowledgeGraphRepoTestSuite) SetupTest() {
	s.log = logging.NewNopLogger()
	d, tx := SetupMockDriver(s.T())
	s.mockDriver = d
	s.mockTx = tx
	s.repo = NewNeo4jKnowledgeGraphRepo(s.mockDriver, s.log)
}

func (s *KnowledgeGraphRepoTestSuite) TestGetSubgraph_Success() {
	mockRes := new(MockResult)

	node := neo4j.Node{Id: 1, Labels: []string{"Patent"}, Props: map[string]any{"id": "p1"}}
	rel := neo4j.Relationship{Id: 100, Type: "CITES", StartId: 1, EndId: 2}

	record := NewRecord([]string{"nodes", "relations"}, []any{
		[]any{node},
		[]any{rel},
	})

	mockRes.Records = []*neo4j.Record{record}

	s.mockTx.On("Run", mock.Anything, mock.Anything, mock.Anything).Return(mockRes, nil)

	sg, err := s.repo.GetSubgraph(context.Background(), "p1", "Patent", 2, nil)
	assert.NoError(s.T(), err)
	assert.Len(s.T(), sg.Nodes, 1)
	assert.Len(s.T(), sg.Relations, 1)
	assert.Equal(s.T(), "p1", sg.Nodes[0].ID)
}

func (s *KnowledgeGraphRepoTestSuite) TestFindShortestPath_Found() {
	mockRes := new(MockResult)

	n1 := neo4j.Node{Id: 1, Props: map[string]any{"id": "p1"}}
	n2 := neo4j.Node{Id: 2, Props: map[string]any{"id": "p2"}}
	r1 := neo4j.Relationship{Id: 10, Type: "CITES", StartId: 1, EndId: 2}

	path := neo4j.Path{
		Nodes: []neo4j.Node{n1, n2},
		Relationships: []neo4j.Relationship{r1},
	}

	record := NewRecord([]string{"path"}, []any{path})
	mockRes.Records = []*neo4j.Record{record}

	s.mockTx.On("Run", mock.Anything, mock.Anything, mock.Anything).Return(mockRes, nil)

	gp, err := s.repo.FindShortestPath(context.Background(), "p1", "Patent", "p2", "Patent")
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), 1, gp.Length)
	assert.Len(s.T(), gp.Nodes, 2)
}

func (s *KnowledgeGraphRepoTestSuite) TestRunPageRank() {
	// ExecuteWrite called with work function
	// We mocked ExecuteWrite to call work(tx)

	// Expectations for tx.Run inside
	// 1. Drop existing
	// 2. Project
	// 3. PageRank
	// 4. Drop

	s.mockTx.On("Run", mock.Anything, mock.MatchedBy(func(cypher string) bool {
		return true // Simplified
	}), mock.Anything).Return(new(MockResult), nil)

	err := s.repo.RunPageRank(context.Background(), "Patent", "CITES", 20, 0.85)
	assert.NoError(s.T(), err)
}

func TestKnowledgeGraphRepo(t *testing.T) {
	suite.Run(t, new(KnowledgeGraphRepoTestSuite))
}
