package repositories

import (
	"context"
	"testing"
	"time"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
	"github.com/stretchr/testify/mock"
	infraNeo4j "github.com/turtacn/KeyIP-Intelligence/internal/infrastructure/database/neo4j"
)

// MockInfraDriver implements infraNeo4j.DriverInterface
type MockInfraDriver struct {
	mock.Mock
}

func (m *MockInfraDriver) ExecuteRead(ctx context.Context, work infraNeo4j.TransactionWork) (interface{}, error) {
	args := m.Called(ctx, work)
	if fn, ok := args.Get(0).(func(context.Context, infraNeo4j.TransactionWork) (interface{}, error)); ok {
		return fn(ctx, work)
	}
	tx := new(MockInfraTransaction)
	return work(tx)
}

func (m *MockInfraDriver) ExecuteWrite(ctx context.Context, work infraNeo4j.TransactionWork) (interface{}, error) {
	args := m.Called(ctx, work)
	if fn, ok := args.Get(0).(func(context.Context, infraNeo4j.TransactionWork) (interface{}, error)); ok {
		return fn(ctx, work)
	}
	tx := new(MockInfraTransaction)
	return work(tx)
}

func (m *MockInfraDriver) HealthCheck(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockInfraDriver) Close(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

// MockInfraTransaction implements infraNeo4j.Transaction
type MockInfraTransaction struct {
	mock.Mock
}

func (m *MockInfraTransaction) Run(ctx context.Context, cypher string, params map[string]any) (infraNeo4j.Result, error) {
	args := m.Called(ctx, cypher, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(infraNeo4j.Result), args.Error(1)
}

// MockResult implements neo4j.ResultWithContext
type MockResult struct {
	mock.Mock
	Records []*neo4j.Record
	Current int
	Summary neo4j.ResultSummary
}

func (m *MockResult) Keys() ([]string, error) {
	return nil, nil
}

func (m *MockResult) Next(ctx context.Context) bool {
	if m.Current < len(m.Records) {
		return true
	}
	return false
}

func (m *MockResult) NextRecord(ctx context.Context, record **neo4j.Record) bool {
	if m.Current < len(m.Records) {
		*record = m.Records[m.Current]
		m.Current++
		return true
	}
	return false
}

func (m *MockResult) PeekRecord(ctx context.Context, record **neo4j.Record) bool {
	if m.Current < len(m.Records) {
		*record = m.Records[m.Current]
		return true
	}
	return false
}

func (m *MockResult) Peek(ctx context.Context) bool {
	if m.Current < len(m.Records) {
		return true
	}
	return false
}

func (m *MockResult) Err() error {
	return nil
}

func (m *MockResult) Record() *neo4j.Record {
	if m.Current < len(m.Records) {
		rec := m.Records[m.Current]
		m.Current++
		return rec
	}
	return nil
}

func (m *MockResult) Consume(ctx context.Context) (neo4j.ResultSummary, error) {
	if m.Summary != nil {
		return m.Summary, nil
	}
	return nil, nil
}

func (m *MockResult) Collect(ctx context.Context) ([]*neo4j.Record, error) {
	return m.Records, nil
}

func (m *MockResult) Single(ctx context.Context) (*neo4j.Record, error) {
	if len(m.Records) > 0 {
		return m.Records[0], nil
	}
	return nil, nil
}

func (m *MockResult) IsOpen() bool {
	return true
}

func (m *MockResult) Buffer(ctx context.Context) bool {
    return true
}

// MockResultSummary implements neo4j.ResultSummary
type MockResultSummary struct {
	mock.Mock
	CountersObj neo4j.Counters
}

func (m *MockResultSummary) Counters() neo4j.Counters {
	return m.CountersObj
}

func (m *MockResultSummary) Query() neo4j.Query {
	var q neo4j.Query
	return q
}

func (m *MockResultSummary) Database() neo4j.DatabaseInfo {
	return nil
}

func (m *MockResultSummary) Notifications() []neo4j.Notification {
	return nil
}

func (m *MockResultSummary) Plan() neo4j.Plan {
	return nil
}

func (m *MockResultSummary) Profile() neo4j.ProfiledPlan {
	return nil
}

func (m *MockResultSummary) ResultAvailableAfter() time.Duration {
	return 0
}

func (m *MockResultSummary) ResultConsumedAfter() time.Duration {
	return 0
}

func (m *MockResultSummary) Server() neo4j.ServerInfo {
	return nil
}

func (m *MockResultSummary) StatementType() neo4j.StatementType {
	return neo4j.StatementTypeUnknown
}

type MockCounters struct {
	NodesCreatedVal         int
	NodesDeletedVal         int
	RelationshipsCreatedVal int
	RelationshipsDeletedVal int
}

func (m *MockCounters) NodesCreated() int { return m.NodesCreatedVal }
func (m *MockCounters) NodesDeleted() int { return m.NodesDeletedVal }
func (m *MockCounters) RelationshipsCreated() int { return m.RelationshipsCreatedVal }
func (m *MockCounters) RelationshipsDeleted() int { return m.RelationshipsDeletedVal }
func (m *MockCounters) PropertiesSet() int { return 0 }
func (m *MockCounters) LabelsAdded() int { return 0 }
func (m *MockCounters) LabelsRemoved() int { return 0 }
func (m *MockCounters) IndexesAdded() int { return 0 }
func (m *MockCounters) IndexesRemoved() int { return 0 }
func (m *MockCounters) ConstraintsAdded() int { return 0 }
func (m *MockCounters) ConstraintsRemoved() int { return 0 }
func (m *MockCounters) SystemUpdates() int { return 0 }
func (m *MockCounters) ContainsUpdates() bool {
	return m.NodesCreatedVal > 0 || m.NodesDeletedVal > 0 || m.RelationshipsCreatedVal > 0 || m.RelationshipsDeletedVal > 0
}
func (m *MockCounters) ContainsSystemUpdates() bool { return false }

// Helper to create a record with values
func NewRecord(keys []string, values []any) *neo4j.Record {
	return &neo4j.Record{
		Keys:   keys,
		Values: values,
	}
}

// Helper for tests to setup mock driver to return a transaction mock
func SetupMockDriver(t *testing.T) (*MockInfraDriver, *MockInfraTransaction) {
	d := new(MockInfraDriver)
	tx := new(MockInfraTransaction)

	// Setup ExecuteRead/Write to call the work function with our mock tx
	d.On("ExecuteRead", mock.Anything, mock.Anything).Return(func(ctx context.Context, work infraNeo4j.TransactionWork) (interface{}, error) {
		return work(tx)
	})
	d.On("ExecuteWrite", mock.Anything, mock.Anything).Return(func(ctx context.Context, work infraNeo4j.TransactionWork) (interface{}, error) {
		return work(tx)
	})

	return d, tx
}
