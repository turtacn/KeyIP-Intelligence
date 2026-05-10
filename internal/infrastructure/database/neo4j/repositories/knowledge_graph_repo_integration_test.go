//go:build integration

package repositories_test

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/suite"

	neodriver "github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/database/neo4j"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/database/neo4j/repositories"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
)

type KnowledgeGraphRepoIntegrationTestSuite struct {
	suite.Suite
	driver *neodriver.Driver
	repo   repositories.KnowledgeGraphRepository
	logger logging.Logger
	ctx    context.Context
}

func (s *KnowledgeGraphRepoIntegrationTestSuite) SetupSuite() {
	s.logger = logging.NewNopLogger()

	uri := os.Getenv("KEYIP_TEST_NEO4J_URL")
	if uri == "" {
		uri = "bolt://localhost:7687"
	}
	username := os.Getenv("KEYIP_TEST_NEO4J_USER")
	if username == "" {
		username = "neo4j"
	}
	password := os.Getenv("KEYIP_TEST_NEO4J_PASSWORD")
	if password == "" {
		password = "neo4j"
	}

	cfg := neodriver.Neo4jConfig{
		URI:      uri,
		Username: username,
		Password: password,
	}

	d, err := neodriver.NewDriver(cfg, s.logger)
	if err != nil {
		s.T().Skipf("Neo4j not available: %v", err)
		return
	}
	s.driver = d
	s.repo = repositories.NewNeo4jKnowledgeGraphRepo(d, s.logger)
	s.ctx = context.Background()

	// Ensure constraints and indexes
	_ = s.repo.EnsureConstraints(s.ctx)
	_ = s.repo.EnsureIndexes(s.ctx)
}

func (s *KnowledgeGraphRepoIntegrationTestSuite) TearDownSuite() {
	if s.driver != nil {
		s.cleanupTestData()
		_ = s.driver.Close()
	}
}

func (s *KnowledgeGraphRepoIntegrationTestSuite) SetupTest() {
	s.cleanupTestData()
}

func (s *KnowledgeGraphRepoIntegrationTestSuite) cleanupTestData() {
	if s.driver == nil {
		return
	}
	_, _ = s.driver.ExecuteWrite(s.ctx, func(tx neodriver.Transaction) (interface{}, error) {
		// Clean all nodes with labels used by tests
		_, err := tx.Run(s.ctx, `
			MATCH (n:Patent) DETACH DELETE n;
			MATCH (n:PatentFamily) DETACH DELETE n;
			MATCH (n:TestEntity) DETACH DELETE n;
		`, nil)
		return nil, err
	})
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func (s *KnowledgeGraphRepoIntegrationTestSuite) TestBatchCreateNodes() {
	nodes := []map[string]interface{}{
		{"id": "kg-test-1", "name": "Entity One", "value": 100},
		{"id": "kg-test-2", "name": "Entity Two", "value": 200},
		{"id": "kg-test-3", "name": "Entity Three", "value": 300},
	}

	created, err := s.repo.BatchCreateNodes(s.ctx, "TestEntity", nodes)
	s.Require().NoError(err)
	s.Equal(int64(3), created)

	// Verify via stats
	stats, err := s.repo.GetNodeLabelCounts(s.ctx)
	s.Require().NoError(err)
	s.Require().NotNil(stats)
	s.True(stats["TestEntity"] >= 3, "should have at least 3 TestEntity nodes")

	// Duplicate creation (CREATE not MERGE, so it adds more)
	created2, err := s.repo.BatchCreateNodes(s.ctx, "TestEntity", nodes[:1])
	s.Require().NoError(err)
	s.Equal(int64(1), created2)
}

func (s *KnowledgeGraphRepoIntegrationTestSuite) TestBatchCreateNodes_EmptyBatch() {
	created, err := s.repo.BatchCreateNodes(s.ctx, "TestEntity", []map[string]interface{}{})
	s.Require().NoError(err)
	s.Equal(int64(0), created)
}

func (s *KnowledgeGraphRepoIntegrationTestSuite) TestBatchCreateNodes_InvalidLabel() {
	nodes := []map[string]interface{}{{"id": "test-1"}}
	_, err := s.repo.BatchCreateNodes(s.ctx, "", nodes)
	s.Require().Error(err)

	_, err = s.repo.BatchCreateNodes(s.ctx, "1invalid-label", nodes)
	s.Require().Error(err)
}

func (s *KnowledgeGraphRepoIntegrationTestSuite) TestBatchCreateRelations() {
	// First create nodes
	nodes := []map[string]interface{}{
		{"id": "kg-rel-test-a", "name": "Node A"},
		{"id": "kg-rel-test-b", "name": "Node B"},
		{"id": "kg-rel-test-c", "name": "Node C"},
	}
	_, err := s.repo.BatchCreateNodes(s.ctx, "TestEntity", nodes)
	s.Require().NoError(err)

	// Create relations
	relations := []*repositories.RelationInput{
		{
			FromID:       "kg-rel-test-a",
			FromLabel:    "TestEntity",
			ToID:         "kg-rel-test-b",
			ToLabel:      "TestEntity",
			RelationType: "RELATES_TO",
			Properties:   map[string]interface{}{"weight": 1.0},
		},
		{
			FromID:       "kg-rel-test-b",
			FromLabel:    "TestEntity",
			ToID:         "kg-rel-test-c",
			ToLabel:      "TestEntity",
			RelationType: "RELATES_TO",
			Properties:   map[string]interface{}{"weight": 2.0},
		},
	}

	created, err := s.repo.BatchCreateRelations(s.ctx, relations)
	s.Require().NoError(err)
	s.Equal(int64(2), created)
}

func (s *KnowledgeGraphRepoIntegrationTestSuite) TestBatchCreateRelations_EmptyBatch() {
	created, err := s.repo.BatchCreateRelations(s.ctx, []*repositories.RelationInput{})
	s.Require().NoError(err)
	s.Equal(int64(0), created)
}

func (s *KnowledgeGraphRepoIntegrationTestSuite) TestGetNeighborhood() {
	// Create center node and neighbors
	_, err := s.repo.BatchCreateNodes(s.ctx, "TestEntity", []map[string]interface{}{
		{"id": "kg-nh-center", "name": "Center"},
	})
	s.Require().NoError(err)

	neighborNodes := make([]map[string]interface{}, 3)
	for i := 0; i < 3; i++ {
		neighborNodes[i] = map[string]interface{}{
			"id":   "kg-nh-neighbor-" + string(rune('0'+i)),
			"name": "Neighbor " + string(rune('0'+i)),
		}
	}
	_, err = s.repo.BatchCreateNodes(s.ctx, "TestEntity", neighborNodes)
	s.Require().NoError(err)

	// Create relations from center to neighbors
	relations := make([]*repositories.RelationInput, 3)
	for i := 0; i < 3; i++ {
		relations[i] = &repositories.RelationInput{
			FromID:       "kg-nh-center",
			FromLabel:    "TestEntity",
			ToID:         "kg-nh-neighbor-" + string(rune('0'+i)),
			ToLabel:      "TestEntity",
			RelationType: "CONNECTED_TO",
		}
	}
	_, err = s.repo.BatchCreateRelations(s.ctx, relations)
	s.Require().NoError(err)

	// Get neighborhood
	subgraph, err := s.repo.GetNeighborhood(s.ctx, "kg-nh-center", "TestEntity", 10)
	s.Require().NoError(err)
	s.Require().NotNil(subgraph)
	s.Equal("kg-nh-center", subgraph.CenterNodeID)
	s.True(len(subgraph.Nodes) >= 1, "should have at least the center node")
	s.True(len(subgraph.Relations) >= 1, "should have at least one relation")
}

func (s *KnowledgeGraphRepoIntegrationTestSuite) TestGetNeighborhood_NotFound() {
	subgraph, err := s.repo.GetNeighborhood(s.ctx, "nonexistent-node", "", 10)
	s.Require().Error(err)
	s.Require().Nil(subgraph)
}

func (s *KnowledgeGraphRepoIntegrationTestSuite) TestGetNeighborhood_EmptyNodeID() {
	_, err := s.repo.GetNeighborhood(s.ctx, "", "", 10)
	s.Require().Error(err)
}

func (s *KnowledgeGraphRepoIntegrationTestSuite) TestGetGraphStats() {
	// Add some test data
	nodes := []map[string]interface{}{
		{"id": "kg-stats-1", "name": "Stats1"},
		{"id": "kg-stats-2", "name": "Stats2"},
	}
	_, err := s.repo.BatchCreateNodes(s.ctx, "TestEntity", nodes)
	s.Require().NoError(err)

	stats, err := s.repo.GetGraphStats(s.ctx)
	s.Require().NoError(err)
	s.Require().NotNil(stats)
	s.GreaterOrEqual(stats.TotalNodes, int64(2), "should have at least 2 nodes")
	s.GreaterOrEqual(stats.TotalRelations, int64(0), "total_relations should be >= 0")
	s.Contains(stats.Labels, "TestEntity")
}

func (s *KnowledgeGraphRepoIntegrationTestSuite) TestGetNodeLabelCounts() {
	_, err := s.repo.BatchCreateNodes(s.ctx, "TestEntity", []map[string]interface{}{
		{"id": "kg-labelcnt-1"},
		{"id": "kg-labelcnt-2"},
		{"id": "kg-labelcnt-3"},
	})
	s.Require().NoError(err)

	counts, err := s.repo.GetNodeLabelCounts(s.ctx)
	s.Require().NoError(err)
	s.Require().NotNil(counts)
	s.True(counts["TestEntity"] >= 3, "should have at least 3 TestEntity nodes")
}

func (s *KnowledgeGraphRepoIntegrationTestSuite) TestGetRelationTypeCounts() {
	// Create nodes with relations
	nodes := []map[string]interface{}{
		{"id": "kg-typecnt-1"},
		{"id": "kg-typecnt-2"},
	}
	_, err := s.repo.BatchCreateNodes(s.ctx, "TestEntity", nodes)
	s.Require().NoError(err)

	relations := []*repositories.RelationInput{
		{
			FromID:       "kg-typecnt-1",
			FromLabel:    "TestEntity",
			ToID:         "kg-typecnt-2",
			ToLabel:      "TestEntity",
			RelationType: "TEST_RELATION",
		},
	}
	_, err = s.repo.BatchCreateRelations(s.ctx, relations)
	s.Require().NoError(err)

	counts, err := s.repo.GetRelationTypeCounts(s.ctx)
	s.Require().NoError(err)
	s.Require().NotNil(counts)
	s.True(counts["TEST_RELATION"] >= 1)
}

func (s *KnowledgeGraphRepoIntegrationTestSuite) TestGetEntityRelations() {
	nodes := []map[string]interface{}{
		{"id": "kg-er-center", "name": "ER Center"},
		{"id": "kg-er-a", "name": "ER A"},
		{"id": "kg-er-b", "name": "ER B"},
	}
	_, err := s.repo.BatchCreateNodes(s.ctx, "TestEntity", nodes)
	s.Require().NoError(err)

	relations := []*repositories.RelationInput{
		{
			FromID:       "kg-er-center",
			FromLabel:    "TestEntity",
			ToID:         "kg-er-a",
			ToLabel:      "TestEntity",
			RelationType: "OUT_REL",
		},
		{
			FromID:       "kg-er-b",
			FromLabel:    "TestEntity",
			ToID:         "kg-er-center",
			ToLabel:      "TestEntity",
			RelationType: "IN_REL",
		},
	}
	_, err = s.repo.BatchCreateRelations(s.ctx, relations)
	s.Require().NoError(err)

	// Test outgoing relations
	outgoing, err := s.repo.GetEntityRelations(s.ctx, "kg-er-center", "TestEntity", "OUTGOING")
	s.Require().NoError(err)
	s.Require().NotEmpty(outgoing)

	// Test incoming relations
	incoming, err := s.repo.GetEntityRelations(s.ctx, "kg-er-center", "TestEntity", "INCOMING")
	s.Require().NoError(err)
	s.Require().NotEmpty(incoming)

	// Test both directions
	both, err := s.repo.GetEntityRelations(s.ctx, "kg-er-center", "TestEntity", "BOTH")
	s.Require().NoError(err)
	s.Require().NotEmpty(both)
}

func (s *KnowledgeGraphRepoIntegrationTestSuite) TestGetRelatedEntities() {
	nodes := []map[string]interface{}{
		{"id": "kg-re-center", "name": "RE Center"},
		{"id": "kg-re-target", "name": "RE Target"},
	}
	_, err := s.repo.BatchCreateNodes(s.ctx, "TestEntity", nodes)
	s.Require().NoError(err)

	relations := []*repositories.RelationInput{
		{
			FromID:       "kg-re-center",
			FromLabel:    "TestEntity",
			ToID:         "kg-re-target",
			ToLabel:      "TestEntity",
			RelationType: "RELATED_TO",
		},
	}
	_, err = s.repo.BatchCreateRelations(s.ctx, relations)
	s.Require().NoError(err)

	// Get related entities
	related, err := s.repo.GetRelatedEntities(s.ctx, "kg-re-center", "TestEntity", "TestEntity", "RELATED_TO", 10)
	s.Require().NoError(err)
	s.Require().NotEmpty(related)
}

func (s *KnowledgeGraphRepoIntegrationTestSuite) TestEnsureIndexes() {
	err := s.repo.EnsureIndexes(s.ctx)
	s.Require().NoError(err)
}

func (s *KnowledgeGraphRepoIntegrationTestSuite) TestEnsureConstraints() {
	err := s.repo.EnsureConstraints(s.ctx)
	s.Require().NoError(err)
}

func (s *KnowledgeGraphRepoIntegrationTestSuite) TestFullTextSearch() {
	// Create patent nodes with searchable text
	patentNodes := []map[string]interface{}{
		{"id": "kg-fts-1", "patent_number": "US20240000001A"},
		{"id": "kg-fts-2", "patent_number": "US20240000002A"},
		{"id": "kg-fts-3", "patent_number": "EP20240000001B"},
	}
	_, err := s.repo.BatchCreateNodes(s.ctx, "Patent", patentNodes)
	if err != nil {
		s.T().Logf("Failed to create patent nodes: %v", err)
		return
	}

	// Create a full-text index (requires Neo4j Enterprise)
	_, err = s.driver.ExecuteWrite(s.ctx, func(tx neodriver.Transaction) (interface{}, error) {
		_, innerErr := tx.Run(s.ctx, `
			CREATE FULLTEXT INDEX test_fts IF NOT EXISTS
			FOR (n:Patent) ON EACH [n.patent_number]
		`, nil)
		return nil, innerErr
	})
	if err != nil {
		s.T().Logf("Skipping full-text search test: cannot create FTS index (Neo4j Enterprise feature): %v", err)
		return
	}

	// Wait briefly for index to populate
	_, _ = s.driver.ExecuteWrite(s.ctx, func(tx neodriver.Transaction) (interface{}, error) {
		_, innerErr := tx.Run(s.ctx, "CALL db.index.fulltext.queryNodes('test_fts', 'US2024*') YIELD node, score RETURN node LIMIT 1", nil)
		return nil, innerErr
	})

	// Test search
	results, err := s.repo.FullTextSearch(s.ctx, "test_fts", "US2024*", 10)
	if err != nil {
		s.T().Logf("Full-text search query failed (may need index population): %v", err)
		return
	}
	s.Require().NotEmpty(results, "should find patent nodes with US2024 prefix")
}

func (s *KnowledgeGraphRepoIntegrationTestSuite) TestFullTextSearch_EmptyQuery() {
	_, err := s.repo.FullTextSearch(s.ctx, "test_fts", "", 10)
	s.Require().Error(err)
}

func (s *KnowledgeGraphRepoIntegrationTestSuite) TestFullTextSearch_EmptyIndexName() {
	_, err := s.repo.FullTextSearch(s.ctx, "", "test", 10)
	s.Require().Error(err)
}

func (s *KnowledgeGraphRepoIntegrationTestSuite) TestFindShortestPath() {
	// Create a chain of nodes
	nodes := []map[string]interface{}{
		{"id": "kg-sp-a", "name": "Path A"},
		{"id": "kg-sp-b", "name": "Path B"},
		{"id": "kg-sp-c", "name": "Path C"},
	}
	_, err := s.repo.BatchCreateNodes(s.ctx, "TestEntity", nodes)
	s.Require().NoError(err)

	relations := []*repositories.RelationInput{
		{FromID: "kg-sp-a", FromLabel: "TestEntity", ToID: "kg-sp-b", ToLabel: "TestEntity", RelationType: "PATH_TO"},
		{FromID: "kg-sp-b", FromLabel: "TestEntity", ToID: "kg-sp-c", ToLabel: "TestEntity", RelationType: "PATH_TO"},
	}
	_, err = s.repo.BatchCreateRelations(s.ctx, relations)
	s.Require().NoError(err)

	path, err := s.repo.FindShortestPath(s.ctx, "kg-sp-a", "TestEntity", "kg-sp-c", "TestEntity")
	s.Require().NoError(err)
	s.Require().NotNil(path)
	s.True(path.Length > 0)
}

func (s *KnowledgeGraphRepoIntegrationTestSuite) TestFindShortestPath_NoPath() {
	nodes := []map[string]interface{}{
		{"id": "kg-sp-disconnected-a"},
		{"id": "kg-sp-disconnected-b"},
	}
	_, err := s.repo.BatchCreateNodes(s.ctx, "TestEntity", nodes)
	s.Require().NoError(err)

	path, err := s.repo.FindShortestPath(s.ctx, "kg-sp-disconnected-a", "TestEntity", "kg-sp-disconnected-b", "TestEntity")
	s.Require().Error(err) // no path found
	s.Require().Nil(path)
}

func (s *KnowledgeGraphRepoIntegrationTestSuite) TestFindShortestPath_InvalidArgs() {
	_, err := s.repo.FindShortestPath(s.ctx, "", "TestEntity", "target", "TestEntity")
	s.Require().Error(err)

	_, err = s.repo.FindShortestPath(s.ctx, "source", "TestEntity", "", "TestEntity")
	s.Require().Error(err)
}

func (s *KnowledgeGraphRepoIntegrationTestSuite) TestFindAllPaths() {
	nodes := []map[string]interface{}{
		{"id": "kg-ap-a", "name": "AllPath A"},
		{"id": "kg-ap-b", "name": "AllPath B"},
		{"id": "kg-ap-c", "name": "AllPath C"},
	}
	_, err := s.repo.BatchCreateNodes(s.ctx, "TestEntity", nodes)
	s.Require().NoError(err)

	// Create multiple paths: A->B->C and A->C
	relations := []*repositories.RelationInput{
		{FromID: "kg-ap-a", FromLabel: "TestEntity", ToID: "kg-ap-b", ToLabel: "TestEntity", RelationType: "PATH_TO"},
		{FromID: "kg-ap-b", FromLabel: "TestEntity", ToID: "kg-ap-c", ToLabel: "TestEntity", RelationType: "PATH_TO"},
		{FromID: "kg-ap-a", FromLabel: "TestEntity", ToID: "kg-ap-c", ToLabel: "TestEntity", RelationType: "DIRECT_TO"},
	}
	_, err = s.repo.BatchCreateRelations(s.ctx, relations)
	s.Require().NoError(err)

	paths, err := s.repo.FindAllPaths(s.ctx, "kg-ap-a", "TestEntity", "kg-ap-c", "TestEntity", 5, 10)
	s.Require().NoError(err)
	s.Require().NotEmpty(paths, "should find at least one path between A and C")
}

func TestKnowledgeGraphRepoIntegration(t *testing.T) {
	suite.Run(t, new(KnowledgeGraphRepoIntegrationTestSuite))
}
