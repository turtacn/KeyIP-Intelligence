package repositories

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"github.com/turtacn/KeyIP-Intelligence/internal/domain/citation"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
)

type CitationRepoTestSuite struct {
	suite.Suite
	mockDriver *MockInfraDriver
	mockTx     *MockInfraTransaction
	repo       citation.CitationRepository
	log        logging.Logger
}

func (s *CitationRepoTestSuite) SetupTest() {
	s.mockDriver = new(MockInfraDriver)
	s.mockTx = new(MockInfraTransaction)
	s.log = logging.NewNopLogger()

	// Setup driver to use our mockTx
	s.mockDriver.On("ExecuteRead", mock.Anything, mock.Anything).Return(func(ctx context.Context, work func(tx interface{}) (interface{}, error)) (interface{}, error) {
		// Wait, work signature in infraNeo4j is TransactionWork func(Transaction) (any, error)
		// But reflection/mock might confuse types if not careful.
		// We need to cast work to proper type.
		// Actually, mock.Anything matches.
		// We assume the caller passes the right function.
		// But we need to call it.
		// The `work` argument is `infraNeo4j.TransactionWork`.
		// We cannot cast `work` to `func(tx interface{})` easily if types don't match.
		// But in Go, we can assertion.
		return work(s.mockTx)
	})

	// To make it simpler, we just implement ExecuteRead behavior directly in the mock call
	// But `work` is passed by the repo. We need to call it.
	// Since infraNeo4j types are imported in repo, we need to match them.
	// We rely on `MockInfraDriver` definition in neo4j_mocks_test.go which imports infraNeo4j.

	// Let's use the pattern:
	// s.mockDriver.On("ExecuteRead", ...).Return(...)
	// But ExecuteRead is generic.
	// In SetupMockDriver (in neo4j_mocks_test.go), we did it right.
	// I'll just replicate it here or call it if exported.
	// It is exported `SetupMockDriver`. I can use it if I pass `s.T()`.
	// But `SetupMockDriver` returns new instances.

	d, tx := SetupMockDriver(s.T())
	s.mockDriver = d
	s.mockTx = tx

	s.repo = NewNeo4jCitationRepo(s.mockDriver, s.log)
}

func (s *CitationRepoTestSuite) TestEnsurePatentNode_Success() {
	s.mockTx.On("Run", mock.Anything, mock.MatchedBy(func(query string) bool {
		return true
	}), mock.Anything).Return(new(MockResult), nil)

	err := s.repo.EnsurePatentNode(context.Background(), uuid.New(), "US123", "US", nil)
	assert.NoError(s.T(), err)
}

func (s *CitationRepoTestSuite) TestCreateCitation_Success() {
	mockRes := new(MockResult)
	mockSummary := new(MockResultSummary)
	mockCounters := new(MockCounters)
	mockCounters.RelationshipsCreatedVal = 1

	// Set the struct field directly as MockResult implementation uses it
	mockSummary.CountersObj = mockCounters
	mockRes.Summary = mockSummary

	s.mockTx.On("Run", mock.Anything, mock.Anything, mock.Anything).Return(mockRes, nil)

	err := s.repo.CreateCitation(context.Background(), uuid.New(), uuid.New(), "forward", nil)
	assert.NoError(s.T(), err)
}

func (s *CitationRepoTestSuite) TestGetCitationCount() {
	mockRes := new(MockResult)
	mockRes.Records = []*neo4j.Record{
		{
			Keys: []string{"forward_count", "backward_count"},
			Values: []any{int64(5), int64(3)},
		},
	}

	s.mockTx.On("Run", mock.Anything, mock.Anything, mock.Anything).Return(mockRes, nil)

	stats, err := s.repo.GetCitationCount(context.Background(), uuid.New())
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), int64(5), stats.ForwardCount)
	assert.Equal(s.T(), int64(3), stats.BackwardCount)
	assert.Equal(s.T(), int64(8), stats.TotalCount)
}

func (s *CitationRepoTestSuite) TestCalculatePageRank() {
	s.mockTx.On("Run", mock.Anything, mock.Anything, mock.Anything).Return(new(MockResult), nil)

	err := s.repo.CalculatePageRank(context.Background(), 20, 0.85)
	assert.NoError(s.T(), err)
}

func TestCitationRepo(t *testing.T) {
	suite.Run(t, new(CitationRepoTestSuite))
}
