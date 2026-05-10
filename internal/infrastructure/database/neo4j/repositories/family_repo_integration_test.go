//go:build integration

package repositories_test

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/suite"

	"github.com/turtacn/KeyIP-Intelligence/internal/domain/citation"
	"github.com/turtacn/KeyIP-Intelligence/internal/domain/family"
	neodriver "github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/database/neo4j"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/database/neo4j/repositories"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
)

type FamilyRepoIntegrationTestSuite struct {
	suite.Suite
	driver    *neodriver.Driver
	familyRepo family.FamilyRepository
	citationRepo citation.CitationRepository
	logger    logging.Logger
	ctx       context.Context

	// Helper: we need a citation repo to create Patent nodes for member tests
}

func (s *FamilyRepoIntegrationTestSuite) SetupSuite() {
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
	s.familyRepo = repositories.NewNeo4jFamilyRepo(d, s.logger)
	s.citationRepo = repositories.NewNeo4jCitationRepo(d, s.logger)
	s.ctx = context.Background()

	// Ensure constraints
	kgRepo := repositories.NewNeo4jKnowledgeGraphRepo(d, s.logger)
	_ = kgRepo.EnsureConstraints(s.ctx)
	_ = kgRepo.EnsureIndexes(s.ctx)
}

func (s *FamilyRepoIntegrationTestSuite) TearDownSuite() {
	if s.driver != nil {
		s.cleanupTestData()
		_ = s.driver.Close()
	}
}

func (s *FamilyRepoIntegrationTestSuite) SetupTest() {
	s.cleanupTestData()
}

func (s *FamilyRepoIntegrationTestSuite) cleanupTestData() {
	if s.driver == nil {
		return
	}
	_, _ = s.driver.ExecuteWrite(s.ctx, func(tx neodriver.Transaction) (interface{}, error) {
		_, err := tx.Run(s.ctx, `
			MATCH (p:Patent) DETACH DELETE p;
			MATCH (f:PatentFamily) DETACH DELETE f;
		`, nil)
		return nil, err
	})
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// ensurePatentNode creates a patent node via the citation repo for use in family tests.
func (s *FamilyRepoIntegrationTestSuite) ensurePatentNode(patentNumber, jurisdiction string) string {
	id := uuid.New()
	err := s.citationRepo.EnsurePatentNode(s.ctx, id, patentNumber, jurisdiction, nil)
	s.Require().NoError(err)
	return id.String()
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

func (s *FamilyRepoIntegrationTestSuite) TestEnsureFamilyNode_CreateAndGet() {
	familyID := "test-family-" + uuid.New().String()

	err := s.familyRepo.EnsureFamilyNode(s.ctx, familyID, "US", nil)
	s.Require().NoError(err)

	agg, err := s.familyRepo.GetFamily(s.ctx, familyID)
	s.Require().NoError(err)
	s.Require().NotNil(agg)
	s.Equal(familyID, agg.FamilyID)
	s.Equal("US", agg.FamilyType)
	s.Empty(agg.Members)
}

func (s *FamilyRepoIntegrationTestSuite) TestEnsureFamilyNode_WithMetadata() {
	familyID := "test-family-meta-" + uuid.New().String()
	metadata := map[string]interface{}{
		"description": "Test family group",
		"priority":    "high",
		"count":       42,
	}

	err := s.familyRepo.EnsureFamilyNode(s.ctx, familyID, "EP", metadata)
	s.Require().NoError(err)

	agg, err := s.familyRepo.GetFamily(s.ctx, familyID)
	s.Require().NoError(err)
	s.Require().NotNil(agg)
	s.Equal(familyID, agg.FamilyID)
	s.Equal("EP", agg.FamilyType)
}

func (s *FamilyRepoIntegrationTestSuite) TestGetFamily_NotFound() {
	_, err := s.familyRepo.GetFamily(s.ctx, "nonexistent-family")
	s.Require().Error(err)
}

func (s *FamilyRepoIntegrationTestSuite) TestDeleteFamily() {
	familyID := "test-family-del-" + uuid.New().String()

	err := s.familyRepo.EnsureFamilyNode(s.ctx, familyID, "US", nil)
	s.Require().NoError(err)

	err = s.familyRepo.DeleteFamily(s.ctx, familyID)
	s.Require().NoError(err)

	// Verify deletion
	_, err = s.familyRepo.GetFamily(s.ctx, familyID)
	s.Require().Error(err)
}

func (s *FamilyRepoIntegrationTestSuite) TestAddMemberAndGetMembers() {
	familyID := "test-family-members-" + uuid.New().String()
	patentID := s.ensurePatentNode("US-FAM-MEM-001", "US")

	err := s.familyRepo.EnsureFamilyNode(s.ctx, familyID, "US", nil)
	s.Require().NoError(err)

	err = s.familyRepo.AddMember(s.ctx, familyID, patentID, "priority")
	s.Require().NoError(err)

	// Get all members
	members, err := s.familyRepo.GetMembers(s.ctx, familyID, nil)
	s.Require().NoError(err)
	s.Require().Len(members, 1)
	s.Equal(patentID, members[0].PatentID)
	s.Equal("priority", members[0].Role)

	// Filter by role
	role := "priority"
	filteredMembers, err := s.familyRepo.GetMembers(s.ctx, familyID, &role)
	s.Require().NoError(err)
	s.Require().Len(filteredMembers, 1)

	wrongRole := "member"
	filteredWrong, err := s.familyRepo.GetMembers(s.ctx, familyID, &wrongRole)
	s.Require().NoError(err)
	s.Require().Len(filteredWrong, 0)
}

func (s *FamilyRepoIntegrationTestSuite) TestRemoveMember() {
	familyID := "test-family-rm-" + uuid.New().String()
	patentID := s.ensurePatentNode("US-FAM-RM-001", "US")

	s.Require().NoError(s.familyRepo.EnsureFamilyNode(s.ctx, familyID, "US", nil))
	s.Require().NoError(s.familyRepo.AddMember(s.ctx, familyID, patentID, "member"))
	s.Require().NoError(s.familyRepo.RemoveMember(s.ctx, familyID, patentID))

	members, err := s.familyRepo.GetMembers(s.ctx, familyID, nil)
	s.Require().NoError(err)
	s.Require().Len(members, 0)
}

func (s *FamilyRepoIntegrationTestSuite) TestGetFamilyWithMembers() {
	familyID := "test-family-agg-" + uuid.New().String()
	patentID := s.ensurePatentNode("US-FAM-AGG-001", "US")

	s.Require().NoError(s.familyRepo.EnsureFamilyNode(s.ctx, familyID, "US", nil))
	s.Require().NoError(s.familyRepo.AddMember(s.ctx, familyID, patentID, "priority"))

	agg, err := s.familyRepo.GetFamily(s.ctx, familyID)
	s.Require().NoError(err)
	s.Require().NotNil(agg)
	s.Require().Len(agg.Members, 1)
	s.Equal(patentID, agg.Members[0].PatentID)
	s.Equal("priority", agg.Members[0].Role)
}

func (s *FamilyRepoIntegrationTestSuite) TestBatchAddMembers() {
	familyID := "test-family-batch-" + uuid.New().String()
	s.Require().NoError(s.familyRepo.EnsureFamilyNode(s.ctx, familyID, "US", nil))

	patentIDs := make([]string, 3)
	for i := 0; i < 3; i++ {
		patentIDs[i] = s.ensurePatentNode("US-FAM-BATCH-00"+string(rune('0'+i)), "US")
	}

	members := []*family.FamilyMemberInput{
		{PatentID: patentIDs[0], Role: "priority"},
		{PatentID: patentIDs[1], Role: "member"},
		{PatentID: patentIDs[2], Role: "member"},
	}

	err := s.familyRepo.BatchAddMembers(s.ctx, familyID, members)
	s.Require().NoError(err)

	agg, err := s.familyRepo.GetFamily(s.ctx, familyID)
	s.Require().NoError(err)
	s.Require().Len(agg.Members, 3)
}

func (s *FamilyRepoIntegrationTestSuite) TestGetFamilyStats() {
	familyID := "test-family-stats-" + uuid.New().String()
	s.Require().NoError(s.familyRepo.EnsureFamilyNode(s.ctx, familyID, "US", nil))

	patentIDs := make([]string, 3)
	for i := 0; i < 3; i++ {
		patentIDs[i] = s.ensurePatentNode("US-FAM-STATS-00"+string(rune('0'+i)), "US")
	}
	for _, pid := range patentIDs {
		s.Require().NoError(s.familyRepo.AddMember(s.ctx, familyID, pid, "member"))
	}

	stats, err := s.familyRepo.GetFamilyStats(s.ctx, familyID)
	s.Require().NoError(err)
	s.Require().NotNil(stats)
	s.Equal(int64(3), stats.TotalMembers)
	s.Contains(stats.Jurisdictions, "US")
	s.Equal(int64(3), stats.JurisdictionCounts["US"])
}

func (s *FamilyRepoIntegrationTestSuite) TestGetFamilyCoverage() {
	familyID := "test-family-cov-" + uuid.New().String()
	s.Require().NoError(s.familyRepo.EnsureFamilyNode(s.ctx, familyID, "WO", nil))

	usPat := s.ensurePatentNode("WO-FAM-COV-001", "US")
	epPat := s.ensurePatentNode("WO-FAM-COV-002", "EP")
	s.Require().NoError(s.familyRepo.AddMember(s.ctx, familyID, usPat, "member"))
	s.Require().NoError(s.familyRepo.AddMember(s.ctx, familyID, epPat, "member"))

	coverage, err := s.familyRepo.GetFamilyCoverage(s.ctx, familyID)
	s.Require().NoError(err)
	s.Require().NotNil(coverage)
	s.Equal(int64(1), coverage["US"])
	s.Equal(int64(1), coverage["EP"])
}

func (s *FamilyRepoIntegrationTestSuite) TestListFamilies() {
	// Create two families
	f1 := "test-family-list-1-" + uuid.New().String()
	f2 := "test-family-list-2-" + uuid.New().String()
	s.Require().NoError(s.familyRepo.EnsureFamilyNode(s.ctx, f1, "US", nil))
	s.Require().NoError(s.familyRepo.EnsureFamilyNode(s.ctx, f2, "US", nil))

	families, total, err := s.familyRepo.ListFamilies(s.ctx, nil, 10, 0)
	s.Require().NoError(err)
	s.Require().GreaterOrEqual(total, int64(2))
	s.Require().NotEmpty(families)

	// Filter by type
	familyType := "US"
	filteredFamilies, filteredTotal, err := s.familyRepo.ListFamilies(s.ctx, &familyType, 10, 0)
	s.Require().NoError(err)
	s.Require().GreaterOrEqual(filteredTotal, int64(2))
	s.Require().NotEmpty(filteredFamilies)
}

func (s *FamilyRepoIntegrationTestSuite) TestGetFamilyByPatent() {
	familyID := "test-family-by-pat-" + uuid.New().String()
	patentID := s.ensurePatentNode("US-FAM-BP-001", "US")

	s.Require().NoError(s.familyRepo.EnsureFamilyNode(s.ctx, familyID, "US", nil))
	s.Require().NoError(s.familyRepo.AddMember(s.ctx, familyID, patentID, "member"))

	families, err := s.familyRepo.GetFamilyByPatent(s.ctx, patentID)
	s.Require().NoError(err)
	s.Require().Len(families, 1)
	s.Equal(familyID, families[0].FamilyID)
}

func (s *FamilyRepoIntegrationTestSuite) TestFindRelatedFamilies() {
	f1 := "test-family-rel-1-" + uuid.New().String()
	f2 := "test-family-rel-2-" + uuid.New().String()
	sharedPatent := s.ensurePatentNode("US-FAM-REL-001", "US")

	s.Require().NoError(s.familyRepo.EnsureFamilyNode(s.ctx, f1, "US", nil))
	s.Require().NoError(s.familyRepo.EnsureFamilyNode(s.ctx, f2, "US", nil))
	s.Require().NoError(s.familyRepo.AddMember(s.ctx, f1, sharedPatent, "member"))
	s.Require().NoError(s.familyRepo.AddMember(s.ctx, f2, sharedPatent, "member"))

	related, err := s.familyRepo.FindRelatedFamilies(s.ctx, f1, 1)
	s.Require().NoError(err)
	s.Require().NotEmpty(related, "f2 should be related to f1 via shared patent")

	found := false
	for _, r := range related {
		if r.FamilyID == f2 {
			found = true
			s.GreaterOrEqual(r.OverlapCount, int64(1))
			break
		}
	}
	s.True(found, "f2 should appear in related families for f1")
}

func (s *FamilyRepoIntegrationTestSuite) TestCreatePriorityLinkAndGetChain() {
	familyID := "test-family-prio-" + uuid.New().String()
	parentID := s.ensurePatentNode("US-FAM-PRIO-001", "US")
	childID := s.ensurePatentNode("US-FAM-PRIO-002", "US")

	s.Require().NoError(s.familyRepo.EnsureFamilyNode(s.ctx, familyID, "US", nil))
	s.Require().NoError(s.familyRepo.AddMember(s.ctx, familyID, parentID, "priority"))
	s.Require().NoError(s.familyRepo.AddMember(s.ctx, familyID, childID, "member"))

	priorityDate := "2023-01-15"
	err := s.familyRepo.CreatePriorityLink(s.ctx, parentID, childID, priorityDate, "US")
	s.Require().NoError(err)

	// Get priority chain from parent
	chain, err := s.familyRepo.GetPriorityChain(s.ctx, parentID)
	s.Require().NoError(err)
	s.Require().NotEmpty(chain, "parent should have outgoing priority links")
}

func (s *FamilyRepoIntegrationTestSuite) TestGetDerivedPatents() {
	parentID := s.ensurePatentNode("US-FAM-DERIV-001", "US")
	derivedID := s.ensurePatentNode("US-FAM-DERIV-002", "US")
	s.Require().NoError(s.familyRepo.CreatePriorityLink(s.ctx, derivedID, parentID, "2023-06-01", "US"))

	derived, err := s.familyRepo.GetDerivedPatents(s.ctx, parentID)
	s.Require().NoError(err)
	s.Require().NotEmpty(derived, "parent should have incoming priority links from derived patents")
}

func TestFamilyRepoIntegration(t *testing.T) {
	suite.Run(t, new(FamilyRepoIntegrationTestSuite))
}
