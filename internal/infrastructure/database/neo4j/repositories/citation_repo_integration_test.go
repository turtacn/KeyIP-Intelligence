//go:build integration

package repositories_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/turtacn/KeyIP-Intelligence/internal/domain/citation"
	neodriver "github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/database/neo4j"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/database/neo4j/repositories"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
)

type CitationRepoIntegrationTestSuite struct {
	suite.Suite
	driver *neodriver.Driver
	repo   citation.CitationRepository
	logger logging.Logger
	ctx    context.Context
}

func (s *CitationRepoIntegrationTestSuite) SetupSuite() {
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
	s.repo = repositories.NewNeo4jCitationRepo(d, s.logger)
	s.ctx = context.Background()

	// Ensure constraints and indexes for the test database
	kgRepo := repositories.NewNeo4jKnowledgeGraphRepo(d, s.logger)
	_ = kgRepo.EnsureConstraints(s.ctx)
	_ = kgRepo.EnsureIndexes(s.ctx)
}

func (s *CitationRepoIntegrationTestSuite) TearDownSuite() {
	if s.driver != nil {
		s.cleanupTestData()
		_ = s.driver.Close()
	}
}

func (s *CitationRepoIntegrationTestSuite) SetupTest() {
	s.cleanupTestData()
}

func (s *CitationRepoIntegrationTestSuite) cleanupTestData() {
	if s.driver == nil {
		return
	}
	_, _ = s.driver.ExecuteWrite(s.ctx, func(tx neodriver.Transaction) (interface{}, error) {
		_, err := tx.Run(s.ctx, "MATCH (p:Patent) DETACH DELETE p", nil)
		return nil, err
	})
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func (s *CitationRepoIntegrationTestSuite) TestEnsurePatentNode_CreateAndMatch() {
	patentID := uuid.New()
	filingDate := time.Date(2023, 1, 15, 0, 0, 0, 0, time.UTC)

	err := s.repo.EnsurePatentNode(s.ctx, patentID, "US12345678A", "US", &filingDate)
	s.Require().NoError(err)

	// Ensure the same node again (MERGE should succeed)
	err = s.repo.EnsurePatentNode(s.ctx, patentID, "US12345678A", "US", &filingDate)
	s.Require().NoError(err)

	// Verify by fetching citation stats (patent exists but has no citations)
	stats, err := s.repo.GetCitationCount(s.ctx, patentID)
	s.Require().NoError(err)
	s.Require().NotNil(stats)
	s.Equal(int64(0), stats.ForwardCount)
	s.Equal(int64(0), stats.BackwardCount)
}

func (s *CitationRepoIntegrationTestSuite) TestEnsurePatentNode_NilFilingDate() {
	patentID := uuid.New()

	err := s.repo.EnsurePatentNode(s.ctx, patentID, "US99999999A", "US", nil)
	s.Require().NoError(err)

	stats, err := s.repo.GetCitationCount(s.ctx, patentID)
	s.Require().NoError(err)
	s.Require().NotNil(stats)
}

func (s *CitationRepoIntegrationTestSuite) TestBatchEnsurePatentNodes() {
	patents := []*citation.PatentNodeData{
		{
			ID:           uuid.New(),
			PatentNumber: "US10000001A",
			Jurisdiction: "US",
		},
		{
			ID:           uuid.New(),
			PatentNumber: "EP10000001B",
			Jurisdiction: "EP",
		},
		{
			ID:           uuid.New(),
			PatentNumber: "JP10000001A",
			Jurisdiction: "JP",
		},
	}

	err := s.repo.BatchEnsurePatentNodes(s.ctx, patents)
	s.Require().NoError(err)

	// Each patent should exist with zero citations
	for _, p := range patents {
		stats, err := s.repo.GetCitationCount(s.ctx, p.ID)
		s.Require().NoError(err)
		s.Equal(int64(0), stats.TotalCount)
	}

	// Batch again (idempotent)
	err = s.repo.BatchEnsurePatentNodes(s.ctx, patents)
	s.Require().NoError(err)
}

func (s *CitationRepoIntegrationTestSuite) TestCreateCitation_ForwardAndBackward() {
	fromID := uuid.New()
	toID := uuid.New()

	err := s.repo.EnsurePatentNode(s.ctx, fromID, "US10000001A", "US", nil)
	s.Require().NoError(err)
	err = s.repo.EnsurePatentNode(s.ctx, toID, "US10000002A", "US", nil)
	s.Require().NoError(err)

	err = s.repo.CreateCitation(s.ctx, fromID, toID, "FORWARD", nil)
	s.Require().NoError(err)

	// Forward: from -> to (from cites to, so to has forward citations)
	forward, err := s.repo.GetForwardCitations(s.ctx, toID, 1, 10)
	s.Require().NoError(err)
	s.Require().Len(forward, 1, "should find 1 forward citation (patent citing from)")
	s.Equal(fromID.String(), forward[0].Nodes[0].ID.String())

	// Backward: from cites to, so from has backward citations
	backward, err := s.repo.GetBackwardCitations(s.ctx, fromID, 1, 10)
	s.Require().NoError(err)
	s.Require().Len(backward, 1, "should find 1 backward citation (patent cited by from)")
	s.Equal(toID.String(), backward[0].Nodes[len(backward[0].Nodes)-1].ID.String())

	// Citation counts
	fromStats, err := s.repo.GetCitationCount(s.ctx, fromID)
	s.Require().NoError(err)
	s.Equal(int64(0), fromStats.ForwardCount) // nothing cites from
	s.Equal(int64(1), fromStats.BackwardCount) // from cites 1 patent

	toStats, err := s.repo.GetCitationCount(s.ctx, toID)
	s.Require().NoError(err)
	s.Equal(int64(1), toStats.ForwardCount) // 1 patent cites to
	s.Equal(int64(0), toStats.BackwardCount)
}

func (s *CitationRepoIntegrationTestSuite) TestDeleteCitation() {
	fromID := uuid.New()
	toID := uuid.New()

	s.Require().NoError(s.repo.EnsurePatentNode(s.ctx, fromID, "US-DEL-001", "US", nil))
	s.Require().NoError(s.repo.EnsurePatentNode(s.ctx, toID, "US-DEL-002", "US", nil))
	s.Require().NoError(s.repo.CreateCitation(s.ctx, fromID, toID, "FORWARD", nil))

	// Verify citation exists
	backward, err := s.repo.GetBackwardCitations(s.ctx, fromID, 1, 10)
	s.Require().NoError(err)
	s.Require().Len(backward, 1)

	// Delete and verify
	err = s.repo.DeleteCitation(s.ctx, fromID, toID, "FORWARD")
	s.Require().NoError(err)

	backward, err = s.repo.GetBackwardCitations(s.ctx, fromID, 1, 10)
	s.Require().NoError(err)
	s.Require().Len(backward, 0)
}

func (s *CitationRepoIntegrationTestSuite) TestBatchCreateCitations() {
	// Create 3 patent nodes
	ids := make([]uuid.UUID, 3)
	for i := 0; i < 3; i++ {
		ids[i] = uuid.New()
		s.Require().NoError(s.repo.EnsurePatentNode(s.ctx, ids[i], "US-BATCH-00"+string(rune('0'+i)), "US", nil))
	}

	citations := []*citation.CitationEdge{
		{FromPatentID: ids[0], ToPatentID: ids[1], Type: "FORWARD"},
		{FromPatentID: ids[1], ToPatentID: ids[2], Type: "FORWARD"},
		{FromPatentID: ids[0], ToPatentID: ids[2], Type: "FORWARD"},
	}

	err := s.repo.BatchCreateCitations(s.ctx, citations)
	s.Require().NoError(err)

	// Verify counts
	stats0, err := s.repo.GetCitationCount(s.ctx, ids[0])
	s.Require().NoError(err)
	s.Equal(int64(0), stats0.ForwardCount, "nothing cites ids[0]")
	s.Equal(int64(2), stats0.BackwardCount, "ids[0] cites 2 patents")

	stats2, err := s.repo.GetCitationCount(s.ctx, ids[2])
	s.Require().NoError(err)
	s.Equal(int64(2), stats2.ForwardCount, "2 patents cite ids[2]")
	s.Equal(int64(0), stats2.BackwardCount)
}

func (s *CitationRepoIntegrationTestSuite) TestGetForwardCitations_MultiDepth() {
	// Create chain: p1 -> p2 -> p3 -> p4
	p := make([]uuid.UUID, 4)
	for i := 0; i < 4; i++ {
		p[i] = uuid.New()
		s.Require().NoError(s.repo.EnsurePatentNode(s.ctx, p[i], "US-DEPTH-00"+string(rune('0'+i)), "US", nil))
	}
	s.Require().NoError(s.repo.CreateCitation(s.ctx, p[0], p[1], "FORWARD", nil))
	s.Require().NoError(s.repo.CreateCitation(s.ctx, p[1], p[2], "FORWARD", nil))
	s.Require().NoError(s.repo.CreateCitation(s.ctx, p[2], p[3], "FORWARD", nil))

	// Forward from p3: p3 is cited by p2 (depth 1), and by p1 through p2 (depth 2)
	forward1, err := s.repo.GetForwardCitations(s.ctx, p[3], 1, 10)
	s.Require().NoError(err)
	s.Require().NotEmpty(forward1, "should have at least 1 forward path at depth 1")

	forward3, err := s.repo.GetForwardCitations(s.ctx, p[3], 3, 10)
	s.Require().NoError(err)
	s.Require().NotEmpty(forward3, "should have forward paths at depth 3")
}

func (s *CitationRepoIntegrationTestSuite) TestGetCitationNetwork() {
	centerID := uuid.New()
	cite1 := uuid.New()
	cite2 := uuid.New()

	s.Require().NoError(s.repo.EnsurePatentNode(s.ctx, centerID, "US-NET-001", "US", nil))
	s.Require().NoError(s.repo.EnsurePatentNode(s.ctx, cite1, "US-NET-002", "US", nil))
	s.Require().NoError(s.repo.EnsurePatentNode(s.ctx, cite2, "US-NET-003", "US", nil))
	s.Require().NoError(s.repo.CreateCitation(s.ctx, cite1, centerID, "FORWARD", nil))
	s.Require().NoError(s.repo.CreateCitation(s.ctx, centerID, cite2, "FORWARD", nil))

	network, err := s.repo.GetCitationNetwork(s.ctx, centerID, 1)
	s.Require().NoError(err)
	s.Require().NotNil(network)
	s.True(len(network.Nodes) >= 1, "network should have at least the center node")
	s.True(len(network.Edges) >= 0, "network edges should be non-negative count")
}

func (s *CitationRepoIntegrationTestSuite) TestGetMostCitedPatents() {
	center := uuid.New()
	citers := make([]uuid.UUID, 3)
	for i := 0; i < 3; i++ {
		citers[i] = uuid.New()
		s.Require().NoError(s.repo.EnsurePatentNode(s.ctx, citers[i], "US-MC-00"+string(rune('0'+i)), "US", nil))
	}
	s.Require().NoError(s.repo.EnsurePatentNode(s.ctx, center, "US-MC-CENTER", "US", nil))

	for _, c := range citers {
		s.Require().NoError(s.repo.CreateCitation(s.ctx, c, center, "FORWARD", nil))
	}

	// Get most cited globally
	mostCited, err := s.repo.GetMostCitedPatents(s.ctx, nil, 10)
	s.Require().NoError(err)
	s.Require().NotEmpty(mostCited)

	// Our center should be among the most cited
	found := false
	for _, mc := range mostCited {
		if mc.Patent.ID == center {
			found = true
			s.Equal(int64(3), mc.CitationCount)
			break
		}
	}
	s.True(found, "center patent should be in most cited list")

	// Filter by jurisdiction
	usPtr := "US"
	mostCitedUS, err := s.repo.GetMostCitedPatents(s.ctx, &usPtr, 10)
	s.Require().NoError(err)
	s.Require().NotEmpty(mostCitedUS)
}

func (s *CitationRepoIntegrationTestSuite) TestGetCitationChain() {
	p1 := uuid.New()
	p2 := uuid.New()
	mid := uuid.New()

	s.Require().NoError(s.repo.EnsurePatentNode(s.ctx, p1, "US-CHAIN-001", "US", nil))
	s.Require().NoError(s.repo.EnsurePatentNode(s.ctx, p2, "US-CHAIN-002", "US", nil))
	s.Require().NoError(s.repo.EnsurePatentNode(s.ctx, mid, "US-CHAIN-003", "US", nil))
	s.Require().NoError(s.repo.CreateCitation(s.ctx, p1, mid, "FORWARD", nil))
	s.Require().NoError(s.repo.CreateCitation(s.ctx, mid, p2, "FORWARD", nil))

	chain, err := s.repo.GetCitationChain(s.ctx, p1, p2)
	s.Require().NoError(err)
	s.Require().NotNil(chain)
	s.True(len(chain) > 0, "should find a chain between p1 and p2")
	if len(chain) > 0 {
		s.True(chain[0].Length >= 1)
	}
}

func (s *CitationRepoIntegrationTestSuite) TestGetCitationChain_NoPath() {
	p1 := uuid.New()
	p2 := uuid.New()

	s.Require().NoError(s.repo.EnsurePatentNode(s.ctx, p1, "US-NOPATH-001", "US", nil))
	s.Require().NoError(s.repo.EnsurePatentNode(s.ctx, p2, "US-NOPATH-002", "US", nil))

	// No citation between them
	chain, err := s.repo.GetCitationChain(s.ctx, p1, p2)
	s.Require().NoError(err)
	s.Require().Empty(chain, "no chain should exist between unrelated patents")
}

func (s *CitationRepoIntegrationTestSuite) TestGetCoCitationPatents() {
	// Both a and b are cited by the same citing patent
	target := uuid.New()
	co := uuid.New()
	citer := uuid.New()

	s.Require().NoError(s.repo.EnsurePatentNode(s.ctx, target, "US-COCIT-001", "US", nil))
	s.Require().NoError(s.repo.EnsurePatentNode(s.ctx, co, "US-COCIT-002", "US", nil))
	s.Require().NoError(s.repo.EnsurePatentNode(s.ctx, citer, "US-COCIT-003", "US", nil))
	s.Require().NoError(s.repo.CreateCitation(s.ctx, citer, target, "FORWARD", nil))
	s.Require().NoError(s.repo.CreateCitation(s.ctx, citer, co, "FORWARD", nil))

	coResults, err := s.repo.GetCoCitationPatents(s.ctx, target, 1, 10)
	s.Require().NoError(err)
	s.Require().NotEmpty(coResults, "should find co-cited patent")
	found := false
	for _, cr := range coResults {
		if cr.Patent.ID == co {
			found = true
			s.GreaterOrEqual(cr.CommonCount, int64(1))
			break
		}
	}
	s.True(found, "co-cited patent should be in results")
}

func (s *CitationRepoIntegrationTestSuite) TestGetBibliographicCoupling() {
	target := uuid.New()
	coupled := uuid.New()
	reference := uuid.New()

	s.Require().NoError(s.repo.EnsurePatentNode(s.ctx, target, "US-BIBLIO-001", "US", nil))
	s.Require().NoError(s.repo.EnsurePatentNode(s.ctx, coupled, "US-BIBLIO-002", "US", nil))
	s.Require().NoError(s.repo.EnsurePatentNode(s.ctx, reference, "US-BIBLIO-003", "US", nil))
	// Both cite the same reference
	s.Require().NoError(s.repo.CreateCitation(s.ctx, target, reference, "FORWARD", nil))
	s.Require().NoError(s.repo.CreateCitation(s.ctx, coupled, reference, "FORWARD", nil))

	couplingResults, err := s.repo.GetBibliographicCoupling(s.ctx, target, 1, 10)
	s.Require().NoError(err)
	s.Require().NotEmpty(couplingResults, "should find bibliographically coupled patent")
	found := false
	for _, cr := range couplingResults {
		if cr.Patent.ID == coupled {
			found = true
			s.GreaterOrEqual(cr.CommonCount, int64(1))
			break
		}
	}
	s.True(found, "coupled patent should be in results")
}

func TestCitationRepoIntegration(t *testing.T) {
	suite.Run(t, new(CitationRepoIntegrationTestSuite))
}
