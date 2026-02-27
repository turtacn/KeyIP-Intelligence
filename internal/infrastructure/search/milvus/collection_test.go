package milvus

import (
	"context"
	"testing"

	"github.com/milvus-io/milvus-sdk-go/v2/client"
	"github.com/milvus-io/milvus-sdk-go/v2/entity"
	"github.com/stretchr/testify/assert"
	"github.com/turtacn/KeyIP-Intelligence/pkg/types/common"
)

// Define a new mock for collection operations
type mockCollectionClient struct {
	client.Client // Embed interface

	createCollectionFunc func(ctx context.Context, schema *entity.Schema, shardsNum int32) error
	dropCollectionFunc   func(ctx context.Context, name string) error
	hasCollectionFunc    func(ctx context.Context, name string) (bool, error)
	describeCollectionFunc func(ctx context.Context, name string) (*entity.Collection, error)
	getCollectionStatisticsFunc func(ctx context.Context, name string) (map[string]string, error)
	createIndexFunc      func(ctx context.Context, collName, fieldName string, idx entity.Index, async bool) error
	dropIndexFunc        func(ctx context.Context, collName, fieldName string) error
	loadCollectionFunc   func(ctx context.Context, name string, async bool) error
	releaseCollectionFunc func(ctx context.Context, name string) error
	getLoadingProgressFunc func(ctx context.Context, name string, partitions []string) (int64, error)
}

func (m *mockCollectionClient) CreateCollection(ctx context.Context, schema *entity.Schema, shardsNum int32, opts ...client.CreateCollectionOption) error {
	if m.createCollectionFunc != nil {
		return m.createCollectionFunc(ctx, schema, shardsNum)
	}
	return nil
}

func (m *mockCollectionClient) DropCollection(ctx context.Context, name string, opts ...client.DropCollectionOption) error {
	if m.dropCollectionFunc != nil {
		return m.dropCollectionFunc(ctx, name)
	}
	return nil
}

func (m *mockCollectionClient) HasCollection(ctx context.Context, name string) (bool, error) {
	if m.hasCollectionFunc != nil {
		return m.hasCollectionFunc(ctx, name)
	}
	return false, nil
}

func (m *mockCollectionClient) DescribeCollection(ctx context.Context, name string) (*entity.Collection, error) {
	if m.describeCollectionFunc != nil {
		return m.describeCollectionFunc(ctx, name)
	}
	return &entity.Collection{Name: name}, nil
}

func (m *mockCollectionClient) GetCollectionStatistics(ctx context.Context, name string) (map[string]string, error) {
	if m.getCollectionStatisticsFunc != nil {
		return m.getCollectionStatisticsFunc(ctx, name)
	}
	return map[string]string{"row_count": "100"}, nil
}

func (m *mockCollectionClient) CreateIndex(ctx context.Context, collName, fieldName string, idx entity.Index, async bool, opts ...client.IndexOption) error {
	if m.createIndexFunc != nil {
		return m.createIndexFunc(ctx, collName, fieldName, idx, async)
	}
	return nil
}

func (m *mockCollectionClient) DropIndex(ctx context.Context, collName, fieldName string, opts ...client.IndexOption) error {
	if m.dropIndexFunc != nil {
		return m.dropIndexFunc(ctx, collName, fieldName)
	}
	return nil
}

func (m *mockCollectionClient) LoadCollection(ctx context.Context, name string, async bool, opts ...client.LoadCollectionOption) error {
	if m.loadCollectionFunc != nil {
		return m.loadCollectionFunc(ctx, name, async)
	}
	return nil
}

func (m *mockCollectionClient) ReleaseCollection(ctx context.Context, name string, opts ...client.ReleaseCollectionOption) error {
	if m.releaseCollectionFunc != nil {
		return m.releaseCollectionFunc(ctx, name)
	}
	return nil
}

func (m *mockCollectionClient) GetLoadingProgress(ctx context.Context, name string, partitions []string) (int64, error) {
	if m.getLoadingProgressFunc != nil {
		return m.getLoadingProgressFunc(ctx, name, partitions)
	}
	return 100, nil
}

func newTestCollectionManager(mock client.Client) *CollectionManager {
	c := &Client{
		milvusClient: mock,
		logger:       newMockLogger(),
	}
	return NewCollectionManager(c, CollectionConfig{}, newMockLogger())
}

func TestCreateCollection_Success(t *testing.T) {
	mock := &mockCollectionClient{
		hasCollectionFunc: func(ctx context.Context, name string) (bool, error) {
			return false, nil
		},
		createCollectionFunc: func(ctx context.Context, schema *entity.Schema, shardsNum int32) error {
			assert.Equal(t, "test", schema.CollectionName)
			return nil
		},
	}
	mgr := newTestCollectionManager(mock)
	// We need to pass *entity.Field as interface{}
	fields := []interface{}{&entity.Field{Name: "id"}}
	schema := common.CollectionSchema{Name: "test", Fields: fields}
	err := mgr.CreateCollection(context.Background(), schema)
	assert.NoError(t, err)
}

func TestCreateCollection_AlreadyExists(t *testing.T) {
	mock := &mockCollectionClient{
		hasCollectionFunc: func(ctx context.Context, name string) (bool, error) {
			return true, nil
		},
	}
	mgr := newTestCollectionManager(mock)
	schema := common.CollectionSchema{Name: "test"}
	err := mgr.CreateCollection(context.Background(), schema)
	assert.Error(t, err)
	assert.Equal(t, ErrCollectionAlreadyExists, err)
}

func TestDropCollection_Success(t *testing.T) {
	mock := &mockCollectionClient{
		hasCollectionFunc: func(ctx context.Context, name string) (bool, error) {
			return true, nil
		},
		dropCollectionFunc: func(ctx context.Context, name string) error {
			return nil
		},
	}
	mgr := newTestCollectionManager(mock)
	err := mgr.DropCollection(context.Background(), "test")
	assert.NoError(t, err)
}

func TestDropCollection_NotFound(t *testing.T) {
	mock := &mockCollectionClient{
		hasCollectionFunc: func(ctx context.Context, name string) (bool, error) {
			return false, nil
		},
	}
	mgr := newTestCollectionManager(mock)
	err := mgr.DropCollection(context.Background(), "test")
	assert.Error(t, err)
	assert.Equal(t, ErrCollectionNotFound, err)
}

func TestDescribeCollection_Success(t *testing.T) {
	mock := &mockCollectionClient{
		describeCollectionFunc: func(ctx context.Context, name string) (*entity.Collection, error) {
			return &entity.Collection{Name: name}, nil
		},
	}
	mgr := newTestCollectionManager(mock)
	info, err := mgr.DescribeCollection(context.Background(), "test")
	assert.NoError(t, err)
	assert.Equal(t, "test", info.Name)
	// assert.Equal(t, uint64(12345), info.CreatedTimestamp)
}

func TestCreateIndex_Success(t *testing.T) {
	mock := &mockCollectionClient{
		createIndexFunc: func(ctx context.Context, collName, fieldName string, idx entity.Index, async bool) error {
			return nil
		},
	}
	mgr := newTestCollectionManager(mock)
	cfg := common.IndexConfig{FieldName: "vec", IndexType: "IVF_FLAT", MetricType: "COSINE"}
	err := mgr.CreateIndex(context.Background(), "test", cfg)
	assert.NoError(t, err)
}

func TestLoadCollection_Success(t *testing.T) {
	mock := &mockCollectionClient{
		loadCollectionFunc: func(ctx context.Context, name string, async bool) error {
			assert.False(t, async)
			return nil
		},
	}
	mgr := newTestCollectionManager(mock)
	err := mgr.LoadCollection(context.Background(), "test")
	assert.NoError(t, err)
}

func TestGetLoadState(t *testing.T) {
	mock := &mockCollectionClient{
		getLoadingProgressFunc: func(ctx context.Context, name string, partitions []string) (int64, error) {
			return 50, nil
		},
	}
	mgr := newTestCollectionManager(mock)
	state, err := mgr.GetLoadState(context.Background(), "test")
	assert.NoError(t, err)
	assert.Equal(t, "Loading", state)
}

func TestEnsureCollection_CreateAndLoad(t *testing.T) {
	created := false
	loaded := false
	mock := &mockCollectionClient{
		hasCollectionFunc: func(ctx context.Context, name string) (bool, error) {
			return false, nil
		},
		createCollectionFunc: func(ctx context.Context, schema *entity.Schema, shardsNum int32) error {
			created = true
			return nil
		},
		loadCollectionFunc: func(ctx context.Context, name string, async bool) error {
			loaded = true
			return nil
		},
		createIndexFunc: func(ctx context.Context, collName, fieldName string, idx entity.Index, async bool) error {
			return nil
		},
	}
	mgr := newTestCollectionManager(mock)
	fields := []interface{}{&entity.Field{Name: "vec"}}
	schema := common.CollectionSchema{Name: "test", Fields: fields}
	err := mgr.EnsureCollection(context.Background(), schema, []common.IndexConfig{{FieldName: "vec"}})
	assert.NoError(t, err)
	assert.True(t, created)
	assert.True(t, loaded)
}

func TestPatentVectorSchema(t *testing.T) {
	s := PatentVectorSchema()
	assert.Equal(t, "patents", s.Name)
	assert.Len(t, s.Fields, 8)
}
