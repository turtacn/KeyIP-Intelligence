package neo4j

import (
	"context"
	"testing"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"github.com/stretchr/testify/mock"
	"github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/monitoring/logging"
)

// MockDriver
type MockDriver struct {
	mock.Mock
}

func (m *MockDriver) VerifyConnectivity(ctx context.Context) error {
	return m.Called(ctx).Error(0)
}
func (m *MockDriver) NewSession(ctx context.Context, config neo4j.SessionConfig) internalSession {
	return m.Called(ctx, config).Get(0).(internalSession)
}
func (m *MockDriver) Close(ctx context.Context) error {
	return m.Called(ctx).Error(0)
}

// MockSession
type MockSession struct {
	mock.Mock
}

func (m *MockSession) ExecuteRead(ctx context.Context, work func(Transaction) (any, error)) (interface{}, error) {
	tx := new(MockTransaction)
	return work(tx)
}
func (m *MockSession) ExecuteWrite(ctx context.Context, work func(Transaction) (any, error)) (interface{}, error) {
	tx := new(MockTransaction)
	return work(tx)
}
func (m *MockSession) Close(ctx context.Context) error {
	return m.Called(ctx).Error(0)
}

// MockTransaction
type MockTransaction struct {
	mock.Mock
}
func (m *MockTransaction) Run(ctx context.Context, cypher string, params map[string]any) (Result, error) {
	return new(MockResult), nil
}

// MockResult
type MockResult struct {
	mock.Mock
}
func (m *MockResult) Next(ctx context.Context) bool { return false }
func (m *MockResult) Record() *neo4j.Record { return nil }
func (m *MockResult) Err() error { return nil }
func (m *MockResult) Consume(ctx context.Context) (neo4j.ResultSummary, error) { return nil, nil }

func TestDriver_HealthCheck(t *testing.T) {
	mockDriver := new(MockDriver)
	logger := logging.NewNopLogger()
	d := &Driver{
		driver: mockDriver,
		logger: logger,
	}

	mockDriver.On("VerifyConnectivity", mock.Anything).Return(nil)

	mockSession := new(MockSession)
	mockDriver.On("NewSession", mock.Anything, mock.Anything).Return(mockSession)
	mockSession.On("Close", mock.Anything).Return(nil)

	_ = d.HealthCheck(context.Background())
}

//Personal.AI order the ending
