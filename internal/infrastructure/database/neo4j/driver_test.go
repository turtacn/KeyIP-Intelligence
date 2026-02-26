package neo4j

import (
	"context"
	"net/url"
	"testing"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
)

// Mocks

type MockNeo4jDriver struct {
	mock.Mock
}

func (m *MockNeo4jDriver) Target() url.URL {
	return url.URL{}
}

func (m *MockNeo4jDriver) NewSession(ctx context.Context, config neo4j.SessionConfig) neo4j.SessionWithContext {
	args := m.Called(ctx, config)
	return args.Get(0).(neo4j.SessionWithContext)
}

func (m *MockNeo4jDriver) VerifyConnectivity(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockNeo4jDriver) VerifyAuthentication(ctx context.Context, auth *neo4j.AuthToken) error {
	return nil
}

func (m *MockNeo4jDriver) Close(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockNeo4jDriver) IsEncrypted() bool {
	return false
}

func (m *MockNeo4jDriver) ExecuteQuery(ctx context.Context, query string, params map[string]any, transformers ...neo4j.ExecuteQueryConfigurationOption) (*neo4j.EagerResult, error) {
	args := m.Called(ctx, query, params, transformers)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*neo4j.EagerResult), args.Error(1)
}

func (m *MockNeo4jDriver) ExecuteQueryBookmarkManager() neo4j.BookmarkManager {
    return nil
}

func (m *MockNeo4jDriver) IsMetricsEnabled() bool {
    return false
}

func (m *MockNeo4jDriver) GetServerInfo(ctx context.Context) (neo4j.ServerInfo, error) {
    return nil, nil
}

type MockSession struct {
	mock.Mock
}

func (m *MockSession) LastBookmarks() []string {
	return nil
}

func (m *MockSession) BeginTransaction(ctx context.Context, configgers ...func(*neo4j.TransactionConfig)) (neo4j.ExplicitTransaction, error) {
	return nil, nil
}

// Driver tests require ManagedTransaction which we can't implement fully (legacy method).
// So we mock ExecuteRead to just return nil or error, without calling work, OR we rely on integration testing.
// For unit tests of Driver wrapper, verifying Session creation and Close is enough if we trust neo4j driver.

func (m *MockSession) ExecuteRead(ctx context.Context, work neo4j.ManagedTransactionWork, configgers ...func(*neo4j.TransactionConfig)) (interface{}, error) {
	args := m.Called(ctx, work, configgers)
	// We cannot call work() here because we can't create a valid ManagedTransaction.
	return args.Get(0), args.Error(1)
}

func (m *MockSession) ExecuteWrite(ctx context.Context, work neo4j.ManagedTransactionWork, configgers ...func(*neo4j.TransactionConfig)) (interface{}, error) {
	args := m.Called(ctx, work, configgers)
	return args.Get(0), args.Error(1)
}

func (m *MockSession) Run(ctx context.Context, cypher string, params map[string]interface{}, configgers ...func(*neo4j.TransactionConfig)) (neo4j.ResultWithContext, error) {
	args := m.Called(ctx, cypher, params, configgers)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(neo4j.ResultWithContext), args.Error(1)
}

func (m *MockSession) Close(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

// Tests

func TestClose_Success(t *testing.T) {
	mockDriver := new(MockNeo4jDriver)
	d := &Driver{
		driver: mockDriver,
		logger: logging.NewNopLogger(),
	}

	mockDriver.On("Close", mock.Anything).Return(nil)

	err := d.Close(context.Background())
	assert.NoError(t, err)
	mockDriver.AssertExpectations(t)
}
//Personal.AI order the ending
